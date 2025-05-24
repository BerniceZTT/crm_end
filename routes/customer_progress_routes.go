// github.com/BerniceZTT/crm_end/routes/customer_progress_routes.go
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

// RegisterCustomerProgressRoutes 注册客户进展相关路由
func RegisterCustomerProgressRoutes(router *gin.Engine) {
	progressRoutes := router.Group("/api/customer-progress")
	progressRoutes.Use(middleware.AuthMiddleware())

	// 获取指定客户的进展历史记录
	progressRoutes.GET("/:customerId", controllers.GetCustomerProgressHistory)

	// 添加客户进展历史记录
	progressRoutes.POST("/", controllers.AddCustomerProgressHistory)

	// 获取所有客户进展历史记录（可按条件筛选）
	progressRoutes.GET("/", controllers.GetAllCustomerProgressHistory)
}
