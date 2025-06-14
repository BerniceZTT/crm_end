package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

// RegisterPublicPoolRoutes 注册公海池相关路由
func RegisterPublicPoolRoutes(router *gin.Engine) {
	// 公海池路由组
	publicPoolGroup := router.Group("/api/public-pool")

	// 所有路由都需要认证
	publicPoolGroup.Use(middleware.AuthMiddleware())

	// 获取公海客户列表
	publicPoolGroup.GET("", controllers.GetPublicPoolCustomers)

	// 获取可分配的销售人员列表
	publicPoolGroup.GET("/assignable-users", controllers.GetAssignableUsers)
}
