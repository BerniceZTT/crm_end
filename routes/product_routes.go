package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BerniceZTT/crm_end/controllers"
	"github.com/BerniceZTT/crm_end/middleware"
)

func RegisterProductRoutes(router *gin.Engine) {
	productGroup := router.Group("/api/products")
	productGroup.Use(middleware.AuthMiddleware())

	// 获取产品列表
	productGroup.GET("", controllers.GetProductList)

	// 获取单个产品
	productGroup.GET("/:id", controllers.GetProduct)

	// 创建产品
	productGroup.POST("", controllers.CreateProduct)

	// 批量导入产品
	productGroup.POST("/bulk-import", controllers.BulkImportProducts)

	// 更新产品
	productGroup.PUT("/:id", controllers.UpdateProduct)

	// 删除产品
	productGroup.DELETE("/:id", controllers.DeleteProduct)

	// 入库操作
	productGroup.POST("/:id/stock-in", controllers.StockInProduct)

	// 出库操作
	productGroup.POST("/:id/stock-out", controllers.StockOutProduct)

	// 批量库存操作
	productGroup.POST("/bulk-stock", controllers.BulkStockProduct)

	// 产品数据导出
	productGroup.GET("/export", controllers.ExportProduct)
}
