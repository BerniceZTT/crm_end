package routes

import (
	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterProjectRoutes(router *gin.Engine) {

	projectGroup := router.Group("/api/projects")

	projectGroup.Use(middleware.AuthMiddleware())

	projectGroup.GET("/", controllers.GetAllProjects)
	projectGroup.GET("/customer/:customerId", controllers.GetCustomerProjects)
	projectGroup.GET("/download/:projectId/:fileId", controllers.DownloadProjectFile)
	projectGroup.GET("/:id", controllers.GetProjectDetail)
	projectGroup.POST("/", controllers.CreateProject)
	projectGroup.PUT("/:id", controllers.UpdateProject)
	projectGroup.DELETE("/:id", controllers.DeleteProject)
}
