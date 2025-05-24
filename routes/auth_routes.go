package routes

import (
	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes 注册认证路由
func RegisterAuthRoutes(router *gin.Engine) {
	auth := router.Group("/api/auth")

	// 公开路由 - 不需要认证
	auth.POST("/login", controllers.Login)
	auth.POST("/agent/login", controllers.AgentLogin)
	auth.POST("/register", controllers.Register)
	auth.POST("/agent/register", controllers.AgentRegister)

	// 需要认证的路由
	auth.GET("/validate", middleware.AuthMiddleware(), controllers.ValidateToken)
}
