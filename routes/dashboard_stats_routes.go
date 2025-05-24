package routes

import (
	"server2/controllers"
	"server2/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterDashboardStatsRoutes 注册数据看板统计相关路由
func RegisterDashboardStatsRoutes(router *gin.Engine) {
	dashboardStatsRoutes := router.Group("/api/dashboard-stats")
	dashboardStatsRoutes.Use(middleware.AuthMiddleware())

	dashboardStatsRoutes.GET("", controllers.GetDashboardStats)
}
