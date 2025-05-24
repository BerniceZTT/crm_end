// server2/routes/change_customer_routes.go
package routes

import (
	"github.com/gin-gonic/gin"

	"server2/controllers"
	"server2/middleware"
)

func RegisterChangeCustomerRoutes(router *gin.Engine) {
	// 路由组
	changeRoutes := router.Group("/api/change_customers")
	changeRoutes.Use(middleware.AuthMiddleware())

	// 客户分配路由
	changeRoutes.POST("/:id/assign", controllers.AssignCustomer)

	// 客户进展变更路由
	changeRoutes.POST("/:id/progress", controllers.ChangeCustomerProgress)
}
