package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/BerniceZTT/crm_end/utils"

	"github.com/gin-gonic/gin"
)

// bodyLogWriter 用于记录响应内容
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 实现 ResponseWriter 接口
func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 记录请求头
		headers := make(map[string]string)
		for k, v := range c.Request.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		// 记录请求体
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			// 恢复请求体以便后续处理
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 创建响应体捕获器
		blw := &bodyLogWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = blw

		// 记录请求信息
		utils.LogApiRequest(
			method,
			path,
			c.Request.URL.Query(),
			string(requestBody),
			headers,
		)

		// 处理请求
		c.Next()

		// 计算处理时间
		duration := time.Since(start)

		// 获取响应状态码
		statusCode := c.Writer.Status()

		// 记录响应信息
		utils.LogApiResponse(
			method,
			path,
			statusCode,
			duration,
			blw.body.String(),
		)
	}
}

// Recovery 恢复中间件
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// 记录崩溃信息
		utils.Logger.Error().
			Interface("panic", recovered).
			Str("path", c.Request.URL.Path).
			Msg("服务崩溃")

		// 返回500错误
		c.AbortWithStatusJSON(500, gin.H{
			"success": false,
			"error":   "服务器内部错误",
		})
	})
}
