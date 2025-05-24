package middleware

import (
	"github.com/BerniceZTT/crm_end/utils"

	"github.com/gin-gonic/gin"
)

// ErrorHandler 全局错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 如果已经存在错误响应，不重复处理
		if c.Writer.Status() >= 400 {
			return
		}

		// 检查是否有错误
		if len(c.Errors) > 0 {
			// 获取最后一个错误
			err := c.Errors.Last()
			utils.HandleError(c, err.Err)
			return
		}
	}
}
