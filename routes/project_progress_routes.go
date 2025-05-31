package routes

import (
	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterProjectProgressRoutes(router *gin.Engine) {

	projectProgressGroup := router.Group("/api/project-progress")

	projectProgressGroup.Use(middleware.AuthMiddleware())

	projectProgressGroup.GET("/:projectId", controllers.GetProjectProgressHistory)
	projectProgressGroup.GET("/", controllers.GetAllProjectProgressHistory)
	projectProgressGroup.POST("/", controllers.AddProjectProgressHistory)
}
