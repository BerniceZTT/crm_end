// github.com/BerniceZTT/crm_end/routes/agent_routes.go
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

func RegisterSystemConfigtRoutes(router *gin.Engine) {
	systemConfigtRoutes := router.Group("/api/system-configs")

	systemConfigtRoutes.Use(middleware.AuthMiddleware())

	systemConfigtRoutes.GET("", controllers.GetAllConfigs)
	systemConfigtRoutes.GET("/type/:configType", controllers.GetConfigsByType)
	systemConfigtRoutes.GET("/:id", controllers.GetConfigDetail)
	systemConfigtRoutes.POST("", controllers.CreateConfig)
	systemConfigtRoutes.PUT("/:id", controllers.UpdateConfig)
	systemConfigtRoutes.DELETE("/:id", controllers.DeleteConfig)
	systemConfigtRoutes.PATCH("/:id/toggle", controllers.ToggleConfigStatus)
}
