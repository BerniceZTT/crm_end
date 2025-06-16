package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ConfigType 配置类型枚举
type ConfigType string

const (
	// ConfigTypeCustomerAutoTransfer 客户自动转移配置
	ConfigTypeCustomerAutoTransfer ConfigType = "customer_auto_transfer"
)

type ConfigItem struct {
	Key   string      `bson:"Key" json:"Key"`
	Value interface{} `bson:"Value" json:"Value"`
}

type AutoTransferConfig struct {
	TargetSalesID       string `json:"targetSalesId"`
	TargetSalesName     string `json:"targetSalesName"`
	DaysWithoutProgress int    `json:"daysWithoutProgress"`
}

// SystemConfig 系统配置模型 (MongoDB文档结构)
type SystemConfig struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ConfigType  ConfigType         `bson:"configType" json:"configType" binding:"required"`
	ConfigKey   string             `bson:"configKey" json:"configKey" binding:"required"`
	ConfigValue interface{}        `bson:"configValue" json:"configValue" binding:"required"` // 使用interface{}存储任意类型值
	Description string             `bson:"description" json:"description"`
	IsEnabled   bool               `bson:"isEnabled" json:"isEnabled"`

	// 创建信息
	CreatorID   string    `bson:"creatorId" json:"creatorId"`
	CreatorName string    `bson:"creatorName" json:"creatorName"`
	CreatedAt   time.Time `bson:"createdAt,omitempty" json:"createdAt,omitempty"`

	// 更新信息
	UpdaterID   string    `bson:"updaterId,omitempty" json:"updaterId,omitempty"`
	UpdaterName string    `bson:"updaterName,omitempty" json:"updaterName,omitempty"`
	UpdatedAt   time.Time `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	ConfigType  ConfigType  `json:"configType" binding:"required"`
	ConfigKey   string      `json:"configKey" binding:"required"`
	ConfigValue interface{} `json:"configValue" binding:"required"`
	Description string      `json:"description"`
	IsEnabled   *bool       `json:"isEnabled,omitempty"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	ConfigValue interface{} `json:"configValue,omitempty"`
	Description string      `json:"description,omitempty"`
	IsEnabled   *bool       `json:"isEnabled,omitempty"`
}
