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

// GetCustomerFollowUpRecords 获取某个客户的跟进记录列表
func GetCustomerFollowUpRecords(c *gin.Context) {
	customerId := c.Param("customerId")
	// 验证客户ID
	if customerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户ID不能为空"})
		return
	}

	ctx := context.Background()

	// 先验证用户是否有权限查看该客户
	customersCollection := repository.Collection(repository.CustomersCollection)

	customerObjId, err := primitive.ObjectIDFromHex(customerId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.LogInfo(map[string]interface{}{
		"customerObjId": customerObjId,
		"customerId":    customerId,
	}, "GetCustomerFollowUpRecords")
	var customer bson.M
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjId}).Decode(&customer)
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 查询跟进记录
	collection := repository.Collection(repository.FollowUpCollection)

	// 设置排序选项，按创建时间倒序
	opts := options.Find().SetSort(bson.M{"createdAt": -1})

	cursor, err := collection.Find(ctx, bson.M{"customerId": customerId}, opts)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var records []models.FollowUpRecord
	if err = cursor.All(ctx, &records); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"customerId":  customerId,
		"recordCount": len(records),
	}, "获取客户跟进记录成功")

	c.JSON(http.StatusOK, gin.H{"records": records})
}

// CreateFollowUpRecord 创建跟进记录
func CreateFollowUpRecord(c *gin.Context) {
	var input models.CreateFollowUpRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// 验证客户是否存在
	customersCollection := repository.Collection(repository.CustomersCollection)
	customerObjId, err := primitive.ObjectIDFromHex(input.CustomerId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	var customer bson.M
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjId}).Decode(&customer)
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 创建跟进记录
	collection := repository.Collection(repository.FollowUpCollection)

	now := time.Now()
	newRecord := models.FollowUpRecord{
		CustomerId:  input.CustomerId,
		Title:       input.Title,
		Content:     input.Content,
		CreatorId:   user.ID,
		CreatorName: user.Username,
		CreatorType: string(user.Role),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 如果是代理商，使用公司名称作为创建者名称
	if user.Role == "AGENT" {
		newRecord.CreatorName = user.Username
	}

	result, err := collection.InsertOne(ctx, newRecord)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 更新客户最后更新时间
	_, err = customersCollection.UpdateOne(
		ctx,
		bson.M{"_id": customerObjId},
		bson.M{"$set": bson.M{"lastUpdateTime": now}},
	)
	if err != nil {
		// 只记录错误但不影响主流程
		utils.LogInfo(map[string]interface{}{
			"customerId": input.CustomerId,
			"error":      err.Error(),
		}, "更新客户最后更新时间失败")
	}

	// 设置返回记录的ID
	newRecord.ID = result.InsertedID.(primitive.ObjectID)

	utils.LogInfo(map[string]interface{}{
		"recordId":   newRecord.ID.Hex(),
		"customerId": input.CustomerId,
	}, "创建跟进记录成功")

	c.JSON(http.StatusCreated, gin.H{
		"message": "创建跟进记录成功",
		"record":  newRecord,
	})
}

// DeleteFollowUpRecord 删除跟进记录
func DeleteFollowUpRecord(c *gin.Context) {
	id := c.Param("id")

	// 获取当前用户
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// 查找记录
	collection := repository.Collection(repository.FollowUpCollection)
	recordId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	var record models.FollowUpRecord
	err = collection.FindOne(ctx, bson.M{"_id": recordId}).Decode(&record)
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			c.JSON(http.StatusNotFound, gin.H{"error": "跟进记录不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 检查权限：只有创建者和超级管理员可以删除
	if record.CreatorId != user.ID && user.Role != "SUPER_ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该跟进记录"})
		return
	}

	// 删除记录
	result, err := collection.DeleteOne(ctx, bson.M{"_id": recordId})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "跟进记录不存在或已被删除"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"recordId":   id,
		"customerId": record.CustomerId,
	}, "删除跟进记录成功")

	c.JSON(http.StatusOK, gin.H{"message": "删除跟进记录成功"})
}
