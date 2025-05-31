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

// 1. 获取指定项目的进展历史记录
func GetProjectProgressHistory(c *gin.Context) {
	projectID := c.Param("projectId")
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"projectId": projectID,
		"user":      currentUser.Username,
	}, "[项目进展历史] 获取项目进展历史")

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
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目的进展历史"})
			return
		}
	}

	// 获取项目进展历史记录
	progressCollection := repository.Collection(repository.ProjectProgressHistoryCollection)
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := progressCollection.Find(ctx, bson.M{"projectId": projectID}, opts)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var progressHistory []models.ProjectProgressHistory
	if err = cursor.All(ctx, &progressHistory); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"projectId":   projectID,
		"recordCount": len(progressHistory),
	}, "[项目进展历史] 获取项目进展历史成功")

	if len(progressHistory) == 0 {
		c.JSON(http.StatusOK, []models.ProjectProgressHistory{})
	}
	c.JSON(http.StatusOK, progressHistory)
}

// 2. 添加项目进展历史记录 (通过HTTP)
func AddProjectProgressHistory(c *gin.Context) {
	var input models.ProjectProgressHistory
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	// 验证必要字段
	if input.ProjectID == "" || input.ProjectName == "" || input.FromProgress == "" ||
		input.ToProgress == "" || input.OperatorID == "" || input.OperatorName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要字段"})
		return
	}

	// 使用工具函数添加历史记录
	result, err := AddProjectProgressHistoryFn(context.Background(), input)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// 3. 获取所有项目进展历史记录（可按条件筛选）
func GetAllProjectProgressHistory(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	progress := c.Query("progress")

	utils.LogInfo(map[string]interface{}{
		"startDate": startDate,
		"endDate":   endDate,
		"progress":  progress,
	}, "[项目进展历史] 获取所有项目进展历史记录")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 构建查询过滤器
	filter := bson.M{}

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
			filter["createdAt"] = dateFilter
		}
	}

	// 应用进展状态过滤
	if progress != "" {
		filter["$or"] = []bson.M{
			{"fromProgress": progress},
			{"toProgress": progress},
		}
	}

	// 获取项目进展历史记录
	progressCollection := repository.Collection(repository.ProjectProgressHistoryCollection)
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := progressCollection.Find(ctx, filter, opts)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var progressHistory []models.ProjectProgressHistory
	if err = cursor.All(ctx, &progressHistory); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"count": len(progressHistory),
	}, "[项目进展历史] 获取所有项目进展历史记录成功")

	c.JSON(http.StatusOK, progressHistory)
}

// 4. 添加项目进展历史记录的工具函数（可在其他服务中调用）
func AddProjectProgressHistoryFn(ctx context.Context, history models.ProjectProgressHistory) (models.ProjectProgressHistory, error) {
	// 验证必要字段
	if history.ProjectID == "" || history.ProjectName == "" || history.FromProgress == "" ||
		history.ToProgress == "" || history.OperatorID == "" || history.OperatorName == "" {
		return models.ProjectProgressHistory{}, &utils.AppError{
			Message:    "缺少必要字段",
			StatusCode: http.StatusBadRequest,
		}
	}

	utils.LogInfo(map[string]interface{}{
		"projectId":   history.ProjectID,
		"projectName": history.ProjectName,
		"from":        history.FromProgress,
		"to":          history.ToProgress,
	}, "[项目进展历史] 添加项目进展历史记录")

	// 设置创建和更新时间
	now := time.Now()
	if history.CreatedAt.IsZero() {
		history.CreatedAt = now
	}
	if history.UpdatedAt.IsZero() {
		history.UpdatedAt = now
	}

	// 插入记录
	progressCollection := repository.Collection(repository.ProjectProgressHistoryCollection)
	result, err := progressCollection.InsertOne(ctx, history)
	if err != nil {
		utils.LogError(err, make(map[string]interface{}), "添加项目进展历史记录失败")
		return models.ProjectProgressHistory{}, err
	}

	// 设置返回的ID
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		history.ID = oid
	}

	utils.LogInfo(map[string]interface{}{
		"recordId":    history.ID.Hex(),
		"projectId":   history.ProjectID,
		"projectName": history.ProjectName,
	}, "[项目进展历史] 添加项目进展历史记录成功")

	return history, nil
}
