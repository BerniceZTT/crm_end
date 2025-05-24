package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PublicPoolCustomer 公海客户响应类型
type PublicPoolCustomer struct {
	ID                       primitive.ObjectID `json:"_id"`
	Name                     string             `json:"name"`
	Nature                   string             `json:"nature"`
	Importance               string             `json:"importance"`
	ApplicationField         string             `json:"applicationField"`
	Progress                 string             `json:"progress"`
	Address                  string             `json:"address"`
	ProductNeeds             []string           `json:"productNeeds"`
	EnterPoolTime            time.Time          `json:"enterPoolTime"`
	PreviousOwnerName        string             `json:"previousOwnerName,omitempty"`
	PreviousOwnerType        string             `json:"previousOwnerType,omitempty"`
	PreviousRelatedSalesName string             `json:"previousRelatedSalesName,omitempty"`
	PreviousRelatedAgentName string             `json:"previousRelatedAgentName,omitempty"`
	CreatorID                string             `json:"creatorId"`
	CreatorName              string             `json:"creatorName"`
	CreatorType              string             `json:"creatorType"`
	CreatedAt                time.Time          `json:"createdAt"`
}

// AssignPublicPoolRequest 公海客户分配请求
type AssignPublicPoolRequest struct {
	TargetID   string `json:"targetId" binding:"required"`
	TargetType string `json:"targetType" binding:"required,oneof=FACTORY_SALES AGENT"`
}

// UserBrief 用户简要信息
type UserBrief struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Username string             `json:"username"`
	Role     UserRole           `json:"role"`
}

// AgentBrief 代理商简要信息
type AgentBrief struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CompanyName      string             `json:"companyName"`
	ContactPerson    string             `json:"contactPerson"`
	RelatedSalesID   string             `json:"relatedSalesId,omitempty"`
	RelatedSalesName string             `json:"relatedSalesName,omitempty"`
}
