package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// InventoryRecord 库存操作记录结构
type InventoryRecord struct {
	ID            primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ProductID     string             `json:"productId" bson:"productId"`
	ModelName     string             `json:"modelName" bson:"modelName"`
	PackageType   string             `json:"packageType" bson:"packageType"`
	OperationType string             `json:"operationType" bson:"operationType"`
	Quantity      int                `json:"quantity" bson:"quantity"`
	Remark        string             `json:"remark,omitempty" bson:"remark,omitempty"`
	Operator      string             `json:"operator" bson:"operator"`
	OperatorID    string             `json:"operatorId" bson:"operatorId"`
	OperationTime time.Time          `json:"operationTime" bson:"operationTime"`
	OperationID   string             `json:"operationId,omitempty" bson:"operationId,omitempty"`
}

// InventoryStats 库存统计信息
type InventoryStats struct {
	TotalProducts    int64         `json:"totalProducts"`
	LowStockProducts int64         `json:"lowStockProducts"`
	TotalStock       int64         `json:"totalStock"`
	RecentChanges    RecentChanges `json:"recentChanges"`
}

// RecentChanges 最近库存变动
type RecentChanges struct {
	In  int64 `json:"in"`
	Out int64 `json:"out"`
	Net int64 `json:"net"`
}

// PaginatedInventoryResponse 分页库存记录响应
type PaginatedInventoryResponse struct {
	Records    []InventoryRecord `json:"records"`
	Pagination Pagination        `json:"pagination"`
}

// Pagination 分页信息
type Pagination struct {
	Total int64 `json:"total"`
	Page  int64 `json:"page"`
	Limit int64 `json:"limit"`
	Pages int64 `json:"pages"`
}
