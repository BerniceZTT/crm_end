package controllers

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
)

// AddCustomerProgressHistoryFn 添加客户进展历史记录的工具函数（可在其他服务中调用）
func AddCustomerProgressHistoryFn(ctx context.Context, historyData models.CustomerProgressHistory) error {
	// 验证必要字段
	if historyData.CustomerID == "" || historyData.CustomerName == "" ||
		historyData.FromProgress == "" || historyData.ToProgress == "" ||
		historyData.OperatorID == "" || historyData.OperatorName == "" {
		return &utils.AppError{Message: "缺少必要字段", StatusCode: http.StatusBadRequest}
	}

	// 确保有创建时间字段
	now := time.Now()
	if historyData.CreatedAt.IsZero() {
		historyData.CreatedAt = now
	}
	if historyData.UpdatedAt.IsZero() {
		historyData.UpdatedAt = now
	}

	// 获取进展历史集合
	collection := repository.Collection(repository.CustProgressCollection)

	// 添加历史记录
	result, err := collection.InsertOne(ctx, historyData)
	if err != nil {
		utils.LogError2("添加客户进展历史记录", err, map[string]interface{}{
			"function": "AddCustomerProgressHistoryFn",
		})
		return err
	}

	// 记录日志
	utils.LogInfo(map[string]interface{}{
		"id":           result.InsertedID.(primitive.ObjectID).Hex(),
		"customerId":   historyData.CustomerID,
		"customerName": historyData.CustomerName,
		"fromProgress": historyData.FromProgress,
		"toProgress":   historyData.ToProgress,
	}, "添加客户进展历史记录成功")

	return nil
}
