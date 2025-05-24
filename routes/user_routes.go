package routes

import (
	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户管理路由
func RegisterUserRoutes(router *gin.Engine) {
	users := router.Group("/api/users")
	users.Use(middleware.AuthMiddleware())

	// 获取所有用户 (仅超级管理员)
	users.GET("/", middleware.PermissionMiddleware("users", "read"), controllers.GetAllUsers)

	// 获取所有销售人员
	users.GET("/sales", controllers.GetSalesUsers)

	// 获取待审批用户 (仅超级管理员)
	users.GET("/pending/approval", middleware.PermissionMiddleware("users", "read"), controllers.GetPendingApprovalUsers)

	// 审批用户 (仅超级管理员)
	users.POST("/approve", middleware.PermissionMiddleware("users", "update"), controllers.ApproveUser)

	// 创建用户 (仅超级管理员)
	users.POST("/", middleware.PermissionMiddleware("users", "create"), controllers.CreateUser)

	// 更新用户 (仅超级管理员)
	users.PUT("/:id", middleware.PermissionMiddleware("users", "update"), controllers.UpdateUser)

	// 删除用户 (仅超级管理员)
	users.DELETE("/:id", middleware.PermissionMiddleware("users", "delete"), controllers.DeleteUser)
}
