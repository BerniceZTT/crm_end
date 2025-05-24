package controllers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"server2/models"
	"server2/repository"
	"server2/utils"
)

// getPublicPoolCustomers 获取公海客户列表
func GetPublicPoolCustomers(c *gin.Context) {
	// 获取查询参数
	keyword := c.Query("keyword")
	nature := c.Query("nature")
	importance := c.Query("importance")
	progress := c.Query("progress")
	applicationField := c.Query("applicationField")

	utils.LogInfo(map[string]interface{}{
		"keyword":          keyword,
		"nature":           nature,
		"importance":       importance,
		"progress":         progress,
		"applicationField": applicationField,
	}, "公海客户查询条件")

	// 构建查询
	filter := bson.M{
		"isinpublicpool": true,
		"progress":       models.CustomerProgressPublicPool,
	}

	// 关键词搜索 - 仅搜索客户名称
	if keyword != "" {
		filter["name"] = bson.M{"$regex": keyword, "$options": "i"}
		utils.LogInfo(map[string]interface{}{"keyword": keyword}, "设置客户名称搜索条件")
	}

	// 客户性质筛选
	if nature != "" {
		filter["nature"] = nature
		utils.LogInfo(map[string]interface{}{"nature": nature}, "设置客户性质筛选条件")
	}

	// 客户重要性筛选
	if importance != "" {
		filter["importance"] = importance
		utils.LogInfo(map[string]interface{}{"importance": importance}, "设置客户重要性筛选条件")
	}

	// 应用领域筛选
	if applicationField != "" {
		filter["applicationField"] = bson.M{"$regex": applicationField, "$options": "i"}
		utils.LogInfo(map[string]interface{}{"applicationField": applicationField}, "设置应用领域筛选条件")
	}

	utils.LogInfo(map[string]interface{}{"filter": filter}, "最终查询条件")

	// 查询所有公海客户
	customersCollection := repository.Collection(repository.CustomersCollection)
	cursor, err := customersCollection.Find(context.Background(), filter)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(context.Background())

	// 解析查询结果
	var customers []models.Customer
	if err := cursor.All(context.Background(), &customers); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{"count": len(customers)}, "查询到符合条件的公海客户数量")

	// 转换为公海客户响应格式
	var publicCustomers []models.PublicPoolCustomer
	for _, customer := range customers {
		// 确定进入公海时间
		enterPoolTime := customer.UpdatedAt
		if !customer.LastUpdateTime.IsZero() {
			enterPoolTime = customer.LastUpdateTime
		}

		publicCustomer := models.PublicPoolCustomer{
			ID:                       customer.ID,
			Name:                     customer.Name,
			Nature:                   customer.Nature,
			Importance:               customer.Importance,
			ApplicationField:         customer.ApplicationField,
			Progress:                 customer.Progress,
			Address:                  customer.Address,
			ProductNeeds:             customer.ProductNeeds,
			EnterPoolTime:            enterPoolTime,
			PreviousOwnerName:        customer.PreviousOwnerName,
			PreviousOwnerType:        customer.PreviousOwnerType,
			PreviousRelatedSalesName: customer.RelatedSalesName,
			PreviousRelatedAgentName: customer.RelatedAgentName,
			CreatorID:                customer.OwnerID,
			CreatorName:              customer.OwnerName,
			CreatorType:              customer.OwnerType,
			CreatedAt:                customer.CreatedAt,
		}

		publicCustomers = append(publicCustomers, publicCustomer)
	}

	c.JSON(200, gin.H{
		"publicCustomers": publicCustomers,
	})
}

// getAssignableUsers 获取可分配的销售人员列表
func GetAssignableUsers(c *gin.Context) {
	// 获取当前用户
	user, err := utils.GetUser(c)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 检查权限
	if !utils.CanAssignPublicPoolCustomer(user.Role) {
		c.JSON(403, gin.H{"error": "无权获取可分配用户列表"})
		return
	}

	// 获取销售人员列表
	salesCollection := repository.Collection(repository.UsersCollection)
	salesCursor, err := salesCollection.Find(
		context.Background(),
		bson.M{
			"role":   models.UserRoleFACTORY_SALES,
			"status": models.UserStatusAPPROVED,
		},
		nil, // 这里简化了，实际应当使用options.Find().SetProjection()
	)

	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer salesCursor.Close(context.Background())

	// 解析销售人员
	var salesUsers []models.UserBrief
	if err := salesCursor.All(context.Background(), &salesUsers); err != nil {
		utils.HandleError(c, err)
		return
	}

	// 获取代理商列表
	agentsCollection := repository.Collection(repository.AgentsCollection)
	agentsCursor, err := agentsCollection.Find(
		context.Background(),
		bson.M{"status": models.UserStatusAPPROVED},
		nil,
	)

	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer agentsCursor.Close(context.Background())

	// 解析代理商
	var agents []models.AgentBrief
	if err := agentsCursor.All(context.Background(), &agents); err != nil {
		utils.HandleError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"salesUsers": salesUsers,
		"agents":     agents,
	})
}

// assignPublicPoolCustomer 从公海分配客户
func AssignPublicPoolCustomer(c *gin.Context) {
	customerId := c.Param("id")
	if customerId == "" {
		c.JSON(400, gin.H{"error": "客户ID不能为空"})
		return
	}

	// 获取当前用户
	currentUser, err := utils.GetUser(c)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 解析请求体
	var req models.AssignPublicPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 查找目标客户
	customerObjID, err := primitive.ObjectIDFromHex(customerId)
	if err != nil {
		c.JSON(400, gin.H{"error": "客户ID格式错误"})
		return
	}

	customersCollection := repository.Collection(repository.CustomersCollection)
	var customer models.Customer
	err = customersCollection.FindOne(
		context.Background(),
		bson.M{"_id": customerObjID, "isinpublicpool": true},
	).Decode(&customer)

	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 查找目标用户或代理商
	var targetName string
	var isAgent bool

	if req.TargetType == string(models.UserRoleFACTORY_SALES) {
		// 查找销售
		salesCollection := repository.Collection(repository.UsersCollection)
		var sales models.User

		salesID, err := primitive.ObjectIDFromHex(req.TargetID)
		if err != nil {
			c.JSON(400, gin.H{"error": "销售ID格式错误"})
			return
		}

		err = salesCollection.FindOne(
			context.Background(),
			bson.M{
				"_id":    salesID,
				"role":   models.UserRoleFACTORY_SALES,
				"status": models.UserStatusAPPROVED,
			},
		).Decode(&sales)

		if err != nil {
			utils.HandleError(c, err)
			return
		}

		targetName = sales.Username
		isAgent = false
	} else if req.TargetType == string(models.UserRoleAGENT) {
		// 查找代理商
		agentsCollection := repository.Collection(repository.AgentsCollection)
		var agent models.Agent

		agentID, err := primitive.ObjectIDFromHex(req.TargetID)
		if err != nil {
			c.JSON(400, gin.H{"error": "代理商ID格式错误"})
			return
		}

		err = agentsCollection.FindOne(
			context.Background(),
			bson.M{
				"_id":    agentID,
				"status": models.UserStatusAPPROVED,
			},
		).Decode(&agent)

		if err != nil {
			utils.HandleError(c, err)
			return
		}

		targetName = agent.CompanyName
		isAgent = true
	} else {
		c.JSON(400, gin.H{"error": "目标类型必须是销售或代理商"})
		return
	}

	// 准备更新数据
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"isinpublicpool":    false,
			"progress":          models.CustomerProgressSampleEvaluation, // 从公海池取出默认为"样板评估"
			"previousOwnerId":   customer.PreviousOwnerID,
			"previousOwnerName": customer.PreviousOwnerName,
			"previousOwnerType": customer.PreviousOwnerType,
			"updatedAt":         now,
			"lastUpdateTime":    now,
		},
	}

	// 根据目标类型添加适当的关联信息
	if isAgent {
		update["$set"].(bson.M)["relatedAgentId"] = req.TargetID
		update["$set"].(bson.M)["relatedAgentName"] = targetName

		// 保持销售关联信息为空
		update["$unset"] = bson.M{"relatedSalesId": "", "relatedSalesName": ""}
	} else {
		update["$set"].(bson.M)["relatedSalesId"] = req.TargetID
		update["$set"].(bson.M)["relatedSalesName"] = targetName

		// 保持代理商关联信息为空
		update["$unset"] = bson.M{"relatedAgentId": "", "relatedAgentName": ""}
	}

	// 更新客户信息
	_, err = customersCollection.UpdateOne(
		context.Background(),
		bson.M{"_id": customerObjID},
		update,
	)

	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 添加分配历史记录
	history := models.CustomerAssignmentHistory{
		CustomerID:    customerId,
		CustomerName:  customer.Name,
		OperatorID:    currentUser.ID,
		OperatorName:  currentUser.Username,
		OperationType: "分配",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 根据目标类型设置适当的销售或代理商信息
	if isAgent {
		history.ToRelatedAgentID = req.TargetID
		history.ToRelatedAgentName = targetName
	} else {
		history.ToRelatedSalesID = req.TargetID
		history.ToRelatedSalesName = targetName
	}
	ctx := repository.GetContext()
	err = AddAssignmentHistory(ctx, history)
	if err != nil {
		utils.LogError(err, map[string]interface{}{
			"path":   c.Request.URL.Path,
			"method": c.Request.Method,
		}, "添加客户分配历史记录失败")
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "客户分配成功",
	})
}
