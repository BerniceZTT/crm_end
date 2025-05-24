package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PricingTier 产品价格阶梯
type PricingTier struct {
	Quantity int     `json:"quantity" bson:"quantity"`
	Price    float64 `json:"price" bson:"price"`
}

// Product 产品模型
type Product struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ModelName   string             `json:"modelName" bson:"modelName"`
	PackageType string             `json:"packageType" bson:"packageType"`
	Stock       int                `json:"stock" bson:"stock"`
	Pricing     []PricingTier      `json:"pricing" bson:"pricing"`
	CreatedAt   time.Time          `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt   time.Time          `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

// StockOperation 库存操作请求
type StockOperation struct {
	Quantity int    `json:"quantity" binding:"required,min=1"`
	Remark   string `json:"remark"`
}

// BulkImportProduct 批量导入产品请求
type BulkImportProduct struct {
	Products []Product `json:"products" binding:"required"`
}

// BulkStockOperation 批量库存操作请求
type BulkStockOperation struct {
	Operations []struct {
		ProductID string `json:"productId" binding:"required"`
		Quantity  int    `json:"quantity" binding:"required,min=1"`
		Type      string `json:"type" binding:"required,oneof=in out"`
	} `json:"operations" binding:"required"`
}
