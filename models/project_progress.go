package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectProgressHistory struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID    string             `bson:"projectId" json:"projectId" binding:"required"`
	ProjectName  string             `bson:"projectName" json:"projectName" binding:"required"`
	FromProgress string             `bson:"fromProgress" json:"fromProgress" binding:"required"`
	ToProgress   string             `bson:"toProgress" json:"toProgress" binding:"required"`
	OperatorID   string             `bson:"operatorId" json:"operatorId" binding:"required"`
	OperatorName string             `bson:"operatorName" json:"operatorName" binding:"required"`
	Remark       string             `bson:"remark,omitempty" json:"remark,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}
