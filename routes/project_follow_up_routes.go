package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

// RegisterProjectFollowUpRoutes 注册项目跟进记录相关路由
func RegisterProjectFollowUpRoutes(router *gin.Engine) {
	followUpGroup := router.Group("/api/projectFollowUpRecords")
	followUpGroup.Use(middleware.AuthMiddleware())

	followUpGroup.GET("/:projectId", controllers.GetProjectFollowUpRecords)
	followUpGroup.POST("/", controllers.CreateProjectFollowUpRecord)
	followUpGroup.DELETE("/:id", controllers.DeleteProjectFollowUpRecord)
}
