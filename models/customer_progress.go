package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 客户进展状态常量
const (
	CustomerProgressNormal     = "正常"
	CustomerProgressPublicPool = "进入公海"
)

// IsValidCustomerProgress 验证进展状态是否有效
func IsValidCustomerProgress(progress string) bool {
	validProgress := []string{
		CustomerProgressNormal,
		CustomerProgressPublicPool,
	}

	for _, p := range validProgress {
		if p == progress {
			return true
		}
	}
	return false
}

// CustomerProgressHistory 客户进展历史记录
type CustomerProgressHistory struct {
	ID           primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	CustomerID   string             `json:"customerId" bson:"customerid"`
	CustomerName string             `json:"customerName" bson:"customername"`
	FromProgress string             `json:"fromProgress" bson:"fromprogress"`
	ToProgress   string             `json:"toProgress" bson:"toprogress"`
	OperatorID   string             `json:"operatorId" bson:"operatorid"`
	OperatorName string             `json:"operatorName" bson:"operatorname"`
	Remark       string             `json:"remark" bson:"remark"`
	CreatedAt    time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt" bson:"updatedAt"`
}
