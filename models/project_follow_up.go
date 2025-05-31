package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectFollowUpRecord struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID   string             `bson:"projectId" json:"projectId" binding:"required"`
	Title       string             `bson:"title" json:"title" binding:"required"`
	Content     string             `bson:"content" json:"content" binding:"required"`
	CreatorID   string             `bson:"creatorId" json:"creatorId"`
	CreatorName string             `bson:"creatorName" json:"creatorName"`
	CreatorType string             `bson:"creatorType" json:"creatorType"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}
