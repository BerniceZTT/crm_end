package routes

import (
	"github.com/gin-gonic/gin"

	"server2/controllers"
	"server2/middleware"
)

// RegisterInventoryRoutes 注册库存管理相关路由
func RegisterInventoryRoutes(router *gin.Engine) {
	// 使用认证中间件
	inventoryRoutes := router.Group("/api/inventory")
	inventoryRoutes.Use(middleware.AuthMiddleware())

	// 获取库存操作记录
	inventoryRoutes.GET("/records", controllers.GetInventoryRecords)

	// 获取库存统计信息
	inventoryRoutes.GET("/stats", controllers.GetInventoryStats)
}
