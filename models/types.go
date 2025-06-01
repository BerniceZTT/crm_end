package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRole 用户角色枚举
type UserRole string

const (
	UserRoleSUPER_ADMIN       UserRole = "SUPER_ADMIN"       // 超级管理员
	UserRoleFACTORY_SALES     UserRole = "FACTORY_SALES"     // 原厂销售
	UserRoleAGENT             UserRole = "AGENT"             // 代理商
	UserRoleINVENTORY_MANAGER UserRole = "INVENTORY_MANAGER" // 库存管理员
)

// UserStatus 用户状态枚举
type UserStatus string

const (
	UserStatusPENDING  UserStatus = "pending"
	UserStatusAPPROVED UserStatus = "approved"
	UserStatusREJECTED UserStatus = "rejected"
)

// User 用户类型
type User struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Username         string             `bson:"username" json:"username"`
	Password         string             `bson:"password" json:"-"` // 不返回密码
	Phone            string             `bson:"phone" json:"phone"`
	Role             UserRole           `bson:"role" json:"role"`
	Status           UserStatus         `bson:"status" json:"status"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
	RejectionReason  string             `bson:"rejectionReason,omitempty" json:"rejectionReason,omitempty"`
	RelatedSalesID   string             `bson:"relatedSalesId,omitempty" json:"relatedSalesId,omitempty"`
	RelatedSalesName string             `bson:"relatedSalesName,omitempty" json:"relatedSalesName,omitempty"`
}

// Agent 代理商类型
type Agent struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CompanyName      string             `bson:"companyName" json:"companyName"`
	Password         string             `bson:"password" json:"password"` // 不返回密码
	ContactPerson    string             `bson:"contactPerson" json:"contactPerson"`
	Phone            string             `bson:"phone" json:"phone"`
	RelatedSalesID   string             `bson:"relatedSalesId,omitempty" json:"relatedSalesId,omitempty"`
	RelatedSalesName string             `bson:"relatedSalesName,omitempty" json:"relatedSalesName,omitempty"`
	Status           UserStatus         `bson:"status" json:"status"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// Customer 客户模型
type Customer struct {
	ID               primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Name             string             `json:"name" bson:"name"`
	Nature           string             `json:"nature" bson:"nature"`
	Importance       string             `json:"importance" bson:"importance"`
	ApplicationField string             `json:"applicationField" bson:"applicationfield"`
	ProductNeeds     []string           `json:"productNeeds" bson:"productneeds"`
	ContactPerson    string             `json:"contactPerson" bson:"contactperson"`
	ContactPhone     string             `json:"contactPhone" bson:"contactphone"`
	Address          string             `json:"address" bson:"address"`
	Progress         string             `json:"progress" bson:"progress"`
	AnnualDemand     float64            `json:"annualDemand" bson:"annualdemand"`

	// 创建人信息
	OwnerID          string `json:"ownerId" bson:"ownerid"`
	OwnerName        string `json:"ownerName" bson:"ownername"`
	OwnerType        string `json:"ownerType" bson:"ownertype"`
	OwnerTypeDisplay string `json:"ownerTypeDisplay" bson:"ownertypedisplay,omitempty"`

	// 关联销售信息
	RelatedSalesID   string `json:"relatedSalesId" bson:"relatedsalesid"`
	RelatedSalesName string `json:"relatedSalesName" bson:"relatedsalesname"`

	// 关联代理商信息
	RelatedAgentID   string `json:"relatedAgentId" bson:"relatedagentid"`
	RelatedAgentName string `json:"relatedAgentName" bson:"relatedagentname"`

	IsInPublicPool bool      `json:"isInPublicPool" bson:"isinpublicpool"`
	LastUpdateTime time.Time `json:"lastUpdateTime" bson:"lastupdatetime"`
	CreatedAt      time.Time `json:"createdAt" bson:"createdat"`
	UpdatedAt      time.Time `json:"updatedAt" bson:"updatedat"`

	// 公海池恢复相关字段
	PreviousOwnerID   string `json:"previousOwnerId,omitempty" bson:"previousownerid,omitempty"`
	PreviousOwnerName string `json:"previousOwnerName,omitempty" bson:"previousownername,omitempty"`
	PreviousOwnerType string `json:"previousOwnerType,omitempty" bson:"previousownertype,omitempty"`
}

// CustomerCreateRequest 创建客户请求
type CustomerCreateRequest struct {
	Name             string   `json:"name"`
	Nature           string   `json:"nature"`
	Importance       string   `json:"importance"`
	ApplicationField string   `json:"applicationField"`
	ProductNeeds     []string `json:"productNeeds"`
	ContactPerson    string   `json:"contactPerson"`
	ContactPhone     string   `json:"contactPhone"`
	Address          string   `json:"address"`
	Progress         string   `json:"progress"`
	AnnualDemand     float64  `json:"annualDemand"`
	RelatedSalesID   string   `json:"relatedSalesId"`
	RelatedAgentID   string   `json:"relatedAgentId,omitempty"`
}

// 各种请求和响应结构
type (
	// LoginRequest 登录请求
	LoginRequest struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		IsAgent  bool   `json:"isAgent"`
	}

	// LoginResponse 登录响应
	LoginResponse struct {
		Token string      `json:"token"`
		User  interface{} `json:"user"`
	}

	// RegisterRequest 注册请求
	RegisterRequest struct {
		Username string   `json:"username" binding:"required,min=2"`
		Password string   `json:"password" binding:"required,min=6"`
		Phone    string   `json:"phone" binding:"required,len=11"`
		Role     UserRole `json:"role" binding:"required"`
	}

	// AgentRegisterRequest 代理商注册请求
	AgentRegisterRequest struct {
		CompanyName   string `json:"companyName" binding:"required,min=2"`
		ContactPerson string `json:"contactPerson" binding:"required,min=2"`
		Password      string `json:"password" binding:"required,min=6"`
		Phone         string `json:"phone" binding:"required,len=11"`
	}

	// ApprovalRequest 审批请求
	ApprovalRequest struct {
		ID       string `json:"id" binding:"required"`
		Type     string `json:"type" binding:"required,oneof=user agent"`
		Approved bool   `json:"approved" binding:"required"`
		Reason   string `json:"reason"`
	}

	// CreateUserRequest 创建用户请求
	CreateUserRequest struct {
		Username string   `json:"username" binding:"required,min=2"`
		Password string   `json:"password" binding:"required,min=6"`
		Phone    string   `json:"phone" binding:"required,len=11"`
		Role     UserRole `json:"role" binding:"required"`
	}

	// UpdateUserRequest 更新用户请求
	UpdateUserRequest struct {
		Username string   `json:"username" binding:"omitempty,min=2"`
		Password string   `json:"password" binding:"omitempty,min=6"`
		Phone    string   `json:"phone" binding:"omitempty,len=11"`
		Role     UserRole `json:"role" binding:"omitempty"`
	}
)
