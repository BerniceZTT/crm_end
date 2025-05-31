package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

// 需要记录的HTTP方法
var loggedMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodDelete: true,
	http.MethodPatch:  true,
}

// 不需要记录的路径
var excludedPaths = map[string]bool{
	"/api/auth/validate": true,
	"/api/health":        true,
	"/api/db-status":     true,
	"/api/auth/login":    true,
}

// OperationLoggerMiddleware 操作日志记录中间件
func OperationLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要记录此操作
		if !shouldLogOperation(c) {
			c.Next()
			return
		}

		startTime := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		utils.Logger.Info().
			Str("method", method).
			Str("path", path).
			Msg("开始操作日志记录")

		// 创建自定义响应写入器以捕获响应体
		blw := &bodyLogWriter{
			body:           bytes.NewBufferString(""),
			ResponseWriter: c.Writer,
		}
		c.Writer = blw

		// 读取并重置请求体
		var requestBody interface{}
		var requestBodyBytes []byte
		if c.Request.Body != nil {
			var err error
			requestBodyBytes, err = io.ReadAll(c.Request.Body)
			if err != nil {
				utils.Logger.Error().Err(err).Msg("读取请求体失败")
			} else {
				// 重置请求体，以便后续处理
				c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))

				// 尝试解析JSON请求体
				if strings.Contains(c.Request.Header.Get("Content-Type"), "application/json") {
					if err := json.Unmarshal(requestBodyBytes, &requestBody); err != nil {
						utils.Logger.Warn().Err(err).Msg("解析JSON请求体失败")
						requestBody = string(requestBodyBytes)
					}
				} else {
					requestBody = string(requestBodyBytes)
				}
			}
		}

		// 清理敏感数据
		sanitizedRequestBody := sanitizeData(requestBody)

		// 提取用户信息
		operatorID, operatorName, operatorType := extractUserInfo(c)

		// 记录请求头（清理敏感信息）
		sanitizedHeaders := sanitizeHeaders(c.Request.Header)

		// 处理请求
		c.Next()

		// 计算响应时间
		responseTime := time.Since(startTime).Milliseconds()

		// 获取响应数据
		var responseData interface{}
		if strings.Contains(c.Writer.Header().Get("Content-Type"), "application/json") {
			if err := json.Unmarshal(blw.body.Bytes(), &responseData); err != nil {
				utils.Logger.Warn().Err(err).Msg("解析JSON响应体失败")
				responseData = blw.body.String()
			}
		} else {
			responseData = blw.body.String()
		}

		// 清理响应数据
		sanitizedResponseData := sanitizeData(responseData)

		// 获取错误信息（如果有）
		var errorMessage string
		if len(c.Errors) > 0 {
			errorMessage = c.Errors.String()
		}

		// 构建操作日志
		operationLog := models.OperationLog{
			Method:        method,
			Path:          path,
			OperatorID:    operatorID,
			OperatorName:  operatorName,
			OperatorType:  operatorType,
			RequestBody:   sanitizedRequestBody,
			RequestHeader: sanitizedHeaders,
			ResponseData:  sanitizedResponseData,
			StatusCode:    c.Writer.Status(),
			Success:       c.Writer.Status() < http.StatusBadRequest,
			ErrorMessage:  errorMessage,
			OperationTime: startTime,
			ResponseTime:  responseTime,
			IPAddress:     getClientIP(c),
			UserAgent:     c.Request.UserAgent(),
		}

		// 保存操作日志
		if err := saveOperationLog(&operationLog); err != nil {
			utils.Logger.Error().Err(err).Msg("保存操作日志失败")
			// 尝试保存最小日志
			minimalLog := operationLog
			minimalLog.RequestBody = nil
			minimalLog.RequestHeader = nil
			minimalLog.ResponseData = nil
			minimalLog.ErrorMessage = fmt.Sprintf("保存详细日志失败: %v", err)

			if saveErr := saveOperationLog(&minimalLog); saveErr != nil {
				utils.Logger.Error().Err(saveErr).Msg("保存最小日志失败")
			}
		}

		utils.Logger.Info().
			Str("method", method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Str("operator", operatorName).
			Int64("responseTime", responseTime).
			Msg("操作日志记录完成")
	}
}

// shouldLogOperation 检查是否需要记录此操作
func shouldLogOperation(c *gin.Context) bool {
	path := c.Request.URL.Path
	method := c.Request.Method

	// 检查是否在排除路径中
	if _, excluded := excludedPaths[path]; excluded {
		return false
	}

	// 检查是否为需要记录的方法
	return loggedMethods[method]
}

// extractUserInfo 从上下文中提取用户信息
func extractUserInfo(c *gin.Context) (string, string, string) {
	// 默认匿名用户
	operatorID := "anonymous"
	operatorName := "匿名用户"
	operatorType := "UNKNOWN"

	// 尝试从上下文获取用户信息
	if userClaims, exists := c.Get("user"); exists {
		switch v := userClaims.(type) {
		case jwt.MapClaims:
			if id, ok := v["id"].(string); ok {
				operatorID = id
			}
			if username, ok := v["username"].(string); ok {
				operatorName = username
			}
			if role, ok := v["role"].(string); ok {
				operatorType = role
			}
			return operatorID, operatorName, operatorType
		case map[string]interface{}:
			if id, ok := v["id"].(string); ok {
				operatorID = id
			}
			if username, ok := v["username"].(string); ok {
				operatorName = username
			}
			if role, ok := v["role"].(string); ok {
				operatorType = role
			}
			return operatorID, operatorName, operatorType
		}
	}

	// 尝试从Authorization头解析JWT
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := utils.ParseToken(token); err == nil {
			if id, ok := claims["id"].(string); ok {
				operatorID = id
			}
			if username, ok := claims["username"].(string); ok {
				operatorName = username
			}
			if role, ok := claims["role"].(string); ok {
				operatorType = role
			}
		}
	}

	return operatorID, operatorName, operatorType
}

// sanitizeData 清理数据中的敏感信息
func sanitizeData(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	// 处理map类型
	if m, ok := data.(map[string]interface{}); ok {
		sanitized := make(map[string]interface{})
		for k, v := range m {
			switch strings.ToLower(k) {
			case "password", "token", "authorization", "secret", "key":
				sanitized[k] = "******"
			default:
				sanitized[k] = sanitizeData(v)
			}
		}
		return sanitized
	}

	// 处理切片类型
	if s, ok := data.([]interface{}); ok {
		sanitized := make([]interface{}, len(s))
		for i, v := range s {
			sanitized[i] = sanitizeData(v)
		}
		return sanitized
	}

	return data
}

// sanitizeHeaders 清理请求头中的敏感信息
func sanitizeHeaders(headers http.Header) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range headers {
		switch strings.ToLower(k) {
		case "authorization":
			if len(v) > 0 {
				auth := v[0]
				if len(auth) > 15 {
					sanitized[k] = auth[:15] + "..."
				} else {
					sanitized[k] = auth
				}
			}
		case "cookie", "x-api-key":
			sanitized[k] = "******"
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}

// getClientIP 获取客户端IP地址
func getClientIP(c *gin.Context) string {
	// 尝试从各种可能的头获取真实IP
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.Request.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

// saveOperationLog 保存操作日志到数据库
func saveOperationLog(log *models.OperationLog) error {
	apiOperationLogsCollection := repository.Collection(repository.ApiOperationLogsCollection)
	_, err := apiOperationLogsCollection.InsertOne(context.Background(), *log)
	return err
}
