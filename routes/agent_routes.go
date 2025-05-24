// github.com/BerniceZTT/crm_end/routes/agent_routes.go
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

func RegisterAgentRoutes(router *gin.Engine) {
	// 所有路由都需要认证
	agentRoutes := router.Group("/api/agents")
	agentRoutes.Use(middleware.AuthMiddleware())

	// 获取所有代理商
	agentRoutes.GET("", controllers.GetAllAgents)

	// 创建代理商(仅超级管理员)
	agentRoutes.POST("", middleware.PermissionMiddleware("agents", "create"), controllers.CreateAgent)

	// 更新代理商
	agentRoutes.PUT("/:id", controllers.UpdateAgent)

	// 删除代理商(仅超级管理员)
	agentRoutes.DELETE("/:id", middleware.PermissionMiddleware("agents", "delete"), controllers.DeleteAgent)

	// 获取特定销售的代理商列表
	agentRoutes.GET("/by-sales/:salesId", controllers.GetAgentsBySalesId)

	// 导出代理商列表为CSV(仅超级管理员)
	agentRoutes.GET("/export/csv", middleware.PermissionMiddleware("exports", "agents"), controllers.ExportAgentsToCSV)

	// 获取可分配的代理商列表
	agentRoutes.GET("/assignable", controllers.GetAssignableAgents)
}
