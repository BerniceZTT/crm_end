package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"server2/models"
	"server2/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		requestPath := c.Request.URL.Path
		requestMethod := c.Request.Method

		utils.Logger.Info().
			Str("path", requestPath).
			Str("method", requestMethod).
			Str("authorization", getShortAuthHeader(authHeader)).
			Msg("验证请求")

		// 检查Authorization头
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			utils.Logger.Info().Msg("缺少Authorization头或格式错误")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "未授权访问",
				"code":    "MISSING_TOKEN",
			})
			return
		}

		// 提取token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			utils.Logger.Info().Msg("从Authorization头中提取token失败")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "未授权访问",
				"code":    "MISSING_TOKEN",
			})
			return
		}

		utils.Logger.Info().Str("token", token[:10]+"...").Msg("开始验证token")

		// 解析token
		claims, err := utils.ParseToken(token)
		if err != nil {
			utils.Logger.Error().Err(err).Msg("Token验证失败")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "无效的token: " + err.Error(),
				"code":    "INVALID_TOKEN",
			})
			return
		}

		// 检查必要字段
		if claims["id"] == nil || claims["role"] == nil || claims["username"] == nil {
			utils.Logger.Warn().Interface("claims", claims).Msg("Token负载缺少必要字段")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Token缺少必要字段",
				"code":    "INVALID_TOKEN",
			})
			return
		}

		// 将用户信息存储到上下文
		c.Set("user", claims)

		utils.Logger.Info().
			Str("username", claims["username"].(string)).
			Str("role", claims["role"].(string)).
			Msg("验证成功")

		c.Next()
	}
}

func PermissionMiddleware(resource string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户信息
		userClaims, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "用户未认证",
				"code":    "UNAUTHENTICATED",
			})
			return
		}

		// 处理不同类型的 userClaims
		var claims map[string]interface{}
		switch v := userClaims.(type) {
		case string:
			// 如果是字符串，尝试解析为JSON
			if err := json.Unmarshal([]byte(v), &claims); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "无法解析用户信息",
					"code":    "INVALID_CLAIMS_FORMAT",
				})
				return
			}
		case jwt.MapClaims:
			claims = make(map[string]interface{})
			for key, val := range v {
				claims[key] = val
			}
		case map[string]interface{}:
			claims = v
		default:
			// 尝试通过JSON序列化/反序列化转换
			data, err := json.Marshal(userClaims)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "无法处理用户信息格式",
					"code":    "INVALID_CLAIMS",
				})
				return
			}
			if err := json.Unmarshal(data, &claims); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "无法转换用户信息格式",
					"code":    "INVALID_CLAIMS",
				})
				return
			}
		}

		// 获取角色和用户名
		roleStr, ok := claims["role"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "用户角色信息无效",
				"code":    "INVALID_ROLE",
			})
			return
		}
		role := models.UserRole(roleStr)

		username, ok := claims["username"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "用户名信息无效",
				"code":    "INVALID_USERNAME",
			})
			return
		}

		// 检查权限
		if !utils.HasPermission(role, resource, action) {
			utils.Logger.Info().
				Str("username", username).
				Str("role", string(role)).
				Str("resource", resource).
				Str("action", action).
				Msg("权限不足")

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "权限不足",
				"code":    "INSUFFICIENT_PERMISSION",
			})
			return
		}

		utils.Logger.Info().
			Str("username", username).
			Str("role", string(role)).
			Str("resource", resource).
			Str("action", action).
			Msg("权限验证通过")

		c.Next()
	}
}

// getShortAuthHeader 获取截断的授权头，保护敏感信息
func getShortAuthHeader(header string) string {
	if header == "" {
		return ""
	}

	if len(header) > 15 {
		return header[:15] + "..."
	}

	return header
}
