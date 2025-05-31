package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
)

// 1. 获取项目跟进记录
func GetProjectFollowUpRecords(c *gin.Context) {
	projectID := c.Param("projectId")
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"projectId": projectID,
		"user":      currentUser.Username,
	}, "[项目跟进记录] 获取项目跟进记录")

	// 验证项目ID
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目ID不能为空"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 验证项目是否存在及权限
	projectsCollection := repository.Collection(repository.ProjectsCollection)

	projectObjID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		utils.LogError(err, make(map[string]interface{}), "无效的项目ID格式")
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID格式"})
		return
	}

	var project models.Project
	err = projectsCollection.FindOne(ctx, bson.M{"_id": projectObjID}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 检查关联客户权限
	customersCollection := repository.Collection(repository.CustomersCollection)
	customerObjID, err := primitive.ObjectIDFromHex(project.CustomerID.Hex())
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	var customer models.Customer
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "关联客户不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 权限验证
	if currentUser.Role != "SUPER_ADMIN" {
		if (currentUser.Role == "FACTORY_SALES" && customer.RelatedSalesID != currentUser.ID && !customer.IsInPublicPool) ||
			(currentUser.Role == "AGENT" && customer.RelatedAgentID != currentUser.ID && !customer.IsInPublicPool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目的跟进记录"})
			return
		}
	}

	// 获取跟进记录
	recordsCollection := repository.Collection(repository.ProjectFollowUpRecordsCollection)
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := recordsCollection.Find(ctx, bson.M{"projectId": projectID}, opts)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var records []models.ProjectFollowUpRecord
	if err = cursor.All(ctx, &records); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"projectId":   projectID,
		"recordCount": len(records),
	}, "[项目跟进记录] 获取跟进记录成功")

	c.JSON(http.StatusOK, gin.H{"records": records})
}

// 2. 创建项目跟进记录
type CreateProjectFollowUpRecordInput struct {
	ProjectID string `json:"projectId" binding:"required"`
	Title     string `json:"title" binding:"required,min=1"`
	Content   string `json:"content" binding:"required,min=1"`
}

func CreateProjectFollowUpRecord(c *gin.Context) {
	var input CreateProjectFollowUpRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"projectId": input.ProjectID,
		"user":      currentUser.Username,
	}, "[项目跟进记录] 创建跟进记录")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 验证项目是否存在及权限
	projectsCollection := repository.Collection(repository.ProjectsCollection)

	projectObjID, err := primitive.ObjectIDFromHex(input.ProjectID)
	if err != nil {
		utils.LogError(err, make(map[string]interface{}), "无效的项目ID格式")
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID格式"})
		return
	}

	var project models.Project
	err = projectsCollection.FindOne(ctx, bson.M{"_id": projectObjID}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 检查关联客户权限
	customersCollection := repository.Collection(repository.CustomersCollection)
	customerObjID, err := primitive.ObjectIDFromHex(project.CustomerID.Hex())
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	var customer models.Customer
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "关联客户不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 权限验证
	if currentUser.Role != "SUPER_ADMIN" {
		if (currentUser.Role == "FACTORY_SALES" && customer.RelatedSalesID != currentUser.ID && !customer.IsInPublicPool) ||
			(currentUser.Role == "AGENT" && customer.RelatedAgentID != currentUser.ID && !customer.IsInPublicPool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权为该项目添加跟进记录"})
			return
		}
	}

	// 创建记录
	recordsCollection := repository.Collection(repository.ProjectFollowUpRecordsCollection)
	now := time.Now()

	newRecord := models.ProjectFollowUpRecord{
		ProjectID:   input.ProjectID,
		Title:       input.Title,
		Content:     input.Content,
		CreatorID:   currentUser.ID,
		CreatorName: currentUser.Username,
		CreatorType: string(currentUser.Role),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result, err := recordsCollection.InsertOne(ctx, newRecord)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 更新项目最后更新时间
	_, err = projectsCollection.UpdateOne(
		ctx,
		bson.M{"_id": projectObjID},
		bson.M{"$set": bson.M{"updatedAt": now}},
	)
	if err != nil {
		utils.LogError(err, make(map[string]interface{}), "更新项目最后更新时间失败")
	}

	// 获取插入的ID
	newRecord.ID = result.InsertedID.(primitive.ObjectID)

	utils.LogInfo(map[string]interface{}{
		"recordId":  newRecord.ID.Hex(),
		"projectId": input.ProjectID,
	}, "[项目跟进记录] 跟进记录创建成功")

	c.JSON(http.StatusCreated, gin.H{
		"message": "创建项目跟进记录成功",
		"record":  newRecord,
	})
}

// 3. 删除项目跟进记录
func DeleteProjectFollowUpRecord(c *gin.Context) {
	id := c.Param("id")
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"recordId": id,
		"user":     currentUser.Username,
	}, "[项目跟进记录] 删除跟进记录")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	recordsCollection := repository.Collection(repository.ProjectFollowUpRecordsCollection)

	recordObjID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.LogError(err, make(map[string]interface{}), "无效的记录ID格式")
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记录ID格式"})
		return
	}

	// 查找记录
	var record models.ProjectFollowUpRecord
	err = recordsCollection.FindOne(ctx, bson.M{"_id": recordObjID}).Decode(&record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "跟进记录不存在"})
			return
		}
		utils.HandleError(c, err)
		return
	}

	// 权限验证：只有创建者和超级管理员可以删除
	if record.CreatorID != currentUser.ID && currentUser.Role != "SUPER_ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该跟进记录"})
		return
	}

	// 删除记录
	result, err := recordsCollection.DeleteOne(ctx, bson.M{"_id": recordObjID})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "跟进记录不存在或已被删除"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"recordId": id,
	}, "[项目跟进记录] 跟进记录删除成功")

	c.JSON(http.StatusOK, gin.H{"message": "删除项目跟进记录成功"})
}
