package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
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
	customerId := c.Query("customerId")

	utils.LogInfo(map[string]interface{}{
		"customerId": customerId,
	}, "获取客户分配历史记录")

	// 构建查询过滤器
	filter := bson.M{}

	// 应用客户ID过滤
	if customerId != "" {
		filter["customerid"] = customerId
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
			"customerId": customerId,
		},
	}, "成功获取客户分配历史记录")

	c.JSON(http.StatusOK, gin.H{
		"history": assignmentHistory,
	})
}
