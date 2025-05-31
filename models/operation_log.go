package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OperationLog 操作日志结构体
type OperationLog struct {
	ID            primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Method        string             `json:"method" bson:"method"`
	Path          string             `json:"path" bson:"path"`
	OperatorID    string             `json:"operatorId" bson:"operatorId"`
	OperatorName  string             `json:"operatorName" bson:"operatorName"`
	OperatorType  string             `json:"operatorType" bson:"operatorType"`
	RequestBody   interface{}        `json:"requestBody" bson:"requestBody"`
	RequestHeader interface{}        `json:"requestHeaders" bson:"requestHeaders"`
	ResponseData  interface{}        `json:"responseData" bson:"responseData"`
	StatusCode    int                `json:"statusCode" bson:"statusCode"`
	Success       bool               `json:"success" bson:"success"`
	ErrorMessage  string             `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`
	OperationTime time.Time          `json:"operationTime" bson:"operationTime"`
	ResponseTime  int64              `json:"responseTime" bson:"responseTime"` // 毫秒
	IPAddress     string             `json:"ipAddress" bson:"ipAddress"`
	UserAgent     string             `json:"userAgent" bson:"userAgent"`
}
