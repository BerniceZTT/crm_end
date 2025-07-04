package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

// RegisterCustomerRoutes 注册客户相关路由
func RegisterCustomerRoutes(router *gin.Engine) {
	customerRoutes := router.Group("/api/customers")
	customerRoutes.Use(middleware.AuthMiddleware())

	customerRoutes.GET("/", controllers.GetCustomerList)
	customerRoutes.POST("/check-duplicates", controllers.CheckDuplicateCustomer)
	customerRoutes.GET("/complete-company-names", controllers.CompleteCompanyNamesHandler)
	customerRoutes.POST("/", controllers.CreateCustomer)
	customerRoutes.POST("/bulk-import", controllers.BulkImportCustomers)
	customerRoutes.GET("/:id", controllers.GetCustomerDetail)
	customerRoutes.PUT("/:id", controllers.UpdateCustomer)
	customerRoutes.DELETE("/:id", controllers.DeleteCustomer)
	customerRoutes.POST("/:id/move-to-public", controllers.MoveCustomerToPublic)
}
