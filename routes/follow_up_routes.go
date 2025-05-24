package routes

import (
	"github.com/gin-gonic/gin"

	"server2/controllers"
	"server2/middleware"
)

// RegisterFollowUpRoutes 注册客户跟进记录相关路由
func RegisterFollowUpRoutes(router *gin.Engine) {
	followUpGroup := router.Group("/api/followUpRecords")
	followUpGroup.Use(middleware.AuthMiddleware())

	// 获取某个客户的跟进记录列表
	followUpGroup.GET("/:customerId", controllers.GetCustomerFollowUpRecords)

	// 创建跟进记录
	followUpGroup.POST("/", controllers.CreateFollowUpRecord)

	// 删除跟进记录
	followUpGroup.DELETE("/:id", controllers.DeleteFollowUpRecord)
}
