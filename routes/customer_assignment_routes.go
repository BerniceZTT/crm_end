package routes

import (
	"github.com/gin-gonic/gin"

	"server2/controllers"
	"server2/middleware"
)

func RegisterCustomerAssignmentRoutes(router *gin.Engine) {
	assignmentRoutes := router.Group("/api/customer-assignments")
	assignmentRoutes.Use(middleware.AuthMiddleware())

	// 获取指定客户的分配历史记录
	assignmentRoutes.GET("/:customerId", controllers.GetCustomerAssignmentHistory)

	// 添加客户分配历史记录
	assignmentRoutes.POST("/", controllers.AddCustomerAssignmentHistory)

	// 获取所有客户分配历史记录（可按条件筛选）
	assignmentRoutes.GET("/", controllers.GetAllCustomerAssignmentHistory)
}
