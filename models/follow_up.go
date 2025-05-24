package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FollowUpRecord 客户跟进记录
type FollowUpRecord struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CustomerId  string             `bson:"customerId" json:"customerId"`
	Title       string             `bson:"title" json:"title"`
	Content     string             `bson:"content" json:"content"`
	CreatorId   string             `bson:"creatorId" json:"creatorId"`
	CreatorName string             `bson:"creatorName" json:"creatorName"`
	CreatorType string             `bson:"creatorType" json:"creatorType"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// CreateFollowUpRecordInput 创建跟进记录的输入数据
type CreateFollowUpRecordInput struct {
	CustomerId string `json:"customerId" binding:"required"`
	Title      string `json:"title" binding:"required"`
	Content    string `json:"content" binding:"required"`
}
