package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"server2/models"
	"server2/repository"
	"server2/utils"
)

// GetCustomerAssignmentHistory 获取指定客户的分配历史记录
func GetCustomerAssignmentHistory(c *gin.Context) {
	// 获取客户ID
	customerId := c.Param("customerId")
	utils.LogInfo(map[string]interface{}{
		"customerId": customerId,
	}, "获取客户分配历史记录")

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取分配历史集合
	collection := repository.Collection(repository.CustAssignCollection)

	// 按创建时间降序排序选项
	findOptions := options.Find()
	findOptions.SetSort(bson.M{"createdat": -1})

	// 查询该客户的所有分配历史
	cursor, err := collection.Find(ctx, bson.M{"customerid": customerId}, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	// 解析结果
	var assignmentHistory []models.CustomerAssignmentHistory
	if err := cursor.All(ctx, &assignmentHistory); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"customerId": customerId,
		"count":      len(assignmentHistory),
	}, "成功获取客户分配历史记录")

	c.JSON(http.StatusOK, gin.H{
		"history": assignmentHistory,
	})
}

// AddCustomerAssignmentHistory 添加客户分配历史记录
func AddCustomerAssignmentHistory(c *gin.Context) {
	// 解析请求数据
	var requestData models.CustomerAssignmentHistory
	if err := c.ShouldBindJSON(&requestData); err != nil {
		utils.HandleError(c, err)
		return
	}

	// 验证必要字段
	if requestData.CustomerID == "" || requestData.CustomerName == "" ||
		requestData.OperatorID == "" || requestData.OperatorName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要字段"})
		return
	}

	// 获取当前时间作为创建时间和更新时间
	now := time.Now()
	requestData.CreatedAt = now
	requestData.UpdatedAt = now

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取分配历史集合
	collection := repository.Collection(repository.CustAssignCollection)

	// 添加历史记录
	result, err := collection.InsertOne(ctx, requestData)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 将插入的ID设置回数据结构
	insertedID := result.InsertedID.(primitive.ObjectID)
	requestData.ID = insertedID

	utils.LogInfo(map[string]interface{}{
		"id":            insertedID.Hex(),
		"customerId":    requestData.CustomerID,
		"customerName":  requestData.CustomerName,
		"operationType": requestData.OperationType,
	}, "成功添加客户分配历史记录")

	c.JSON(http.StatusCreated, requestData)
}

// GetAllCustomerAssignmentHistory 获取所有客户分配历史记录（可按条件筛选）
func GetAllCustomerAssignmentHistory(c *gin.Context) {
	// 获取查询参数
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	operationType := c.Query("operationType")
	customerId := c.Query("customerId")

	utils.LogInfo(map[string]interface{}{
		"startDate":     startDate,
		"endDate":       endDate,
		"operationType": operationType,
		"customerId":    customerId,
	}, "获取客户分配历史记录")

	// 构建查询过滤器
	filter := bson.M{}

	// 应用客户ID过滤
	if customerId != "" {
		filter["customerid"] = customerId
	}

	// 应用日期过滤
	if startDate != "" || endDate != "" {
		dateFilter := bson.M{}
		if startDate != "" {
			startTime, err := time.Parse(time.RFC3339, startDate)
			if err == nil {
				dateFilter["$gte"] = startTime
			}
		}
		if endDate != "" {
			endTime, err := time.Parse(time.RFC3339, endDate)
			if err == nil {
				dateFilter["$lte"] = endTime
			}
		}
		if len(dateFilter) > 0 {
			filter["createdat"] = dateFilter
		}
	}

	// 应用操作类型过滤
	if operationType != "" {
		filter["operationtype"] = operationType
	}

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取分配历史集合
	collection := repository.Collection(repository.CustAssignCollection)

	// 按创建时间降序排序选项
	findOptions := options.Find()
	findOptions.SetSort(bson.M{"createdat": -1})

	// 查询分配历史记录
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	// 解析结果
	var assignmentHistory []models.CustomerAssignmentHistory
	if err := cursor.All(ctx, &assignmentHistory); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"count": len(assignmentHistory),
		"filter": map[string]interface{}{
			"startDate":     startDate,
			"endDate":       endDate,
			"operationType": operationType,
			"customerId":    customerId,
		},
	}, "成功获取客户分配历史记录")

	c.JSON(http.StatusOK, gin.H{
		"history": assignmentHistory,
	})
}

// AddCustomerAssignmentHistoryFn 添加客户分配历史记录的工具函数（可在其他服务中调用）
func AddCustomerAssignmentHistoryFn(ctx context.Context, historyData models.CustomerAssignmentHistory) error {
	// 验证必要字段
	if historyData.CustomerID == "" || historyData.CustomerName == "" ||
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

	// 获取分配历史集合
	collection := repository.Collection(repository.CustAssignCollection)

	// 添加历史记录
	result, err := collection.InsertOne(ctx, historyData)
	if err != nil {
		return err
	}

	// 记录日志
	utils.LogInfo(map[string]interface{}{
		"id":            result.InsertedID.(primitive.ObjectID).Hex(),
		"customerId":    historyData.CustomerID,
		"customerName":  historyData.CustomerName,
		"operationType": historyData.OperationType,
	}, "添加客户分配历史记录成功")

	return nil
}
