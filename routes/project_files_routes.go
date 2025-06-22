package routes

import (
	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterProjectFilesRoutes(router *gin.Engine) {
	projectFilesGroup := router.Group("/api/files")
	projectFilesGroup.Use(middleware.AuthMiddleware())

	// 专用文件上传接口
	projectFilesGroup.POST("/upload", controllers.UploadFile)

	// 文件下载接口
	projectFilesGroup.GET("/download/:fileId", controllers.DownloadFile)

	// 删除文件接口
	projectFilesGroup.DELETE("/:fileId", controllers.DeleteFile)

}
