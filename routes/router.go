package routes

import (
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(router *gin.Engine) {
	// 注册认证路由
	RegisterAuthRoutes(router)

	// 注册用户管理路由
	RegisterUserRoutes(router)

	// 注册其他路由
	RegisterProductRoutes(router)
	RegisterCustomerRoutes(router)
	RegisterAgentRoutes(router)
	RegisterPublicPoolRoutes(router)
	RegisterInventoryRoutes(router)
	RegisterFollowUpRoutes(router)
	RegisterCustomerAssignmentRoutes(router)
	RegisterChangeCustomerRoutes(router)
	RegisterDashboardStatsRoutes(router)
	RegisterProjectRoutes(router)
	RegisterProjectFollowUpRoutes(router)
	RegisterProjectProgressRoutes(router)
	RegisterSystemConfigtRoutes(router)

	// 健康检查路由
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 数据库状态检查路由
	router.GET("/api/db-status", func(c *gin.Context) {
		status, err := repository.GetDatabaseStatus()
		if err != nil {
			utils.ErrorResponse(c, "获取数据库状态失败: "+err.Error(), 500)
			return
		}
		c.JSON(200, status)
	})
}
