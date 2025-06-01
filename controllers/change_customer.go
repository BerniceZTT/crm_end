package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
)

// AssignCustomer 处理客户分配请求
// 分配客户到指定销售和(可选)代理商
func AssignCustomer(c *gin.Context) {
	// 获取客户ID
	customerId := c.Param("id")
	if customerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户ID不能为空"})
		return
	}

	// 解析请求数据
	var assignRequest struct {
		SalesId string `json:"salesId" binding:"required"`
		AgentId string `json:"agentId"`
	}

	if err := c.ShouldBindJSON(&assignRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取相关集合
	customersCollection := repository.Collection(repository.CustomersCollection)
	usersCollection := repository.Collection(repository.UsersCollection)
	agentsCollection := repository.Collection(repository.AgentsCollection)

	// 将客户ID转换为ObjectID
	customerObjID, err := primitive.ObjectIDFromHex(customerId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID格式"})
		return
	}

	// 查询客户
	var customer models.Customer
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
		} else {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "分配客户失败"})
		}
		return
	}

	// 查询销售信息
	salesObjID, err := primitive.ObjectIDFromHex(assignRequest.SalesId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的销售ID格式"})
		return
	}

	var salesUser models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(&salesUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "指定的销售人员不存在"})
		} else {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "分配客户失败"})
		}
		return
	}

	// 准备更新数据
	updateData := bson.M{
		"relatedsalesid":   assignRequest.SalesId,
		"relatedsalesname": salesUser.Username,
		"lastupdatetime":   time.Now(),
		"updatedat":        time.Now(),
		"isinpublicpool":   false,
		"progress":         models.CustomerProgressNormal,
	}

	// 处理代理商信息
	var agentInfo map[string]string
	if assignRequest.AgentId != "" {
		agentObjID, err := primitive.ObjectIDFromHex(assignRequest.AgentId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的代理商ID格式"})
			return
		}

		var agent models.Agent
		err = agentsCollection.FindOne(ctx, bson.M{"_id": agentObjID}).Decode(&agent)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, gin.H{"error": "指定的代理商不存在"})
			} else {
				utils.HandleError(c, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "分配客户失败"})
			}
			return
		}

		updateData["relatedagentid"] = assignRequest.AgentId
		updateData["relatedagentname"] = agent.CompanyName

		agentInfo = map[string]string{
			"id":   assignRequest.AgentId,
			"name": agent.CompanyName,
		}
	} else if assignRequest.AgentId == "" {
		// 清空代理商关联
		updateData["relatedagentid"] = nil
		updateData["relatedagentname"] = nil
	}

	// 更新客户数据
	result, err := customersCollection.UpdateOne(
		ctx,
		bson.M{"_id": customerObjID},
		bson.M{"$set": updateData},
	)

	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分配客户失败"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在或已被修改"})
		return
	}

	// 检查是否需要记录分配历史
	salesChanged := assignRequest.SalesId != customer.RelatedSalesID
	agentChanged := assignRequest.AgentId != customer.RelatedAgentID

	// 只在有变化时记录历史
	if salesChanged || agentChanged {
		operationType := "分配"

		// 确定操作类型
		if customer.IsInPublicPool {
			operationType = "认领"
		} else {
			// 判断是否为认领：当前用户是被分配的销售或代理商
			if (user.Role == "FACTORY_SALES" && user.ID == assignRequest.SalesId) ||
				(user.Role == "AGENT" && user.ID == assignRequest.AgentId) {
				operationType = "认领"
			}
		}
		// 记录分配历史
		err = AddAssignmentHistory(ctx, models.CustomerAssignmentHistory{
			CustomerID:   customerId,
			CustomerName: customer.Name,
			// 销售信息
			FromRelatedSalesID:   customer.RelatedSalesID,
			FromRelatedSalesName: customer.RelatedSalesName,
			ToRelatedSalesID:     assignRequest.SalesId,
			ToRelatedSalesName:   salesUser.Username,
			// 代理商信息
			FromRelatedAgentID:   customer.RelatedAgentID,
			FromRelatedAgentName: customer.RelatedAgentName,
			ToRelatedAgentID:     assignRequest.AgentId,
			ToRelatedAgentName:   agentInfo["name"],
			// 操作者信息
			OperatorID:    user.ID,
			OperatorName:  user.Username,
			OperationType: operationType,
		})

		if err != nil {
			utils.HandleError(c, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "客户分配成功",
		"data": gin.H{
			"salesId":   assignRequest.SalesId,
			"salesName": salesUser.Username,
			"agentId":   assignRequest.AgentId,
			"agentName": agentInfo["name"],
		},
	})
}

// ChangeCustomerProgress 处理客户进展状态变更
func ChangeCustomerProgress(c *gin.Context) {
	// 获取客户ID
	customerId := c.Param("id")
	if customerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户ID不能为空"})
		return
	}

	// 解析请求数据
	var progressRequest struct {
		Progress string `json:"progress" binding:"required"`
		Remark   string `json:"remark"`
	}

	if err := c.ShouldBindJSON(&progressRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证进展状态的有效性
	if !models.IsValidCustomerProgress(progressRequest.Progress) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户进展状态"})
		return
	}

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取客户集合
	customersCollection := repository.Collection(repository.CustomersCollection)

	// 将客户ID转换为ObjectID
	customerObjID, err := primitive.ObjectIDFromHex(customerId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID格式"})
		return
	}

	// 查询客户
	var customer models.Customer
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
		} else {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新客户进展失败"})
		}
		return
	}

	// 如果新进展与当前进展相同，则不需要更新
	if customer.Progress == progressRequest.Progress {
		c.JSON(http.StatusOK, gin.H{
			"message": "客户进展未变化",
			"data": gin.H{
				"progress": customer.Progress,
			},
		})
		return
	}

	// 特殊处理：如果新状态是公海池
	isMovingToPublicPool := progressRequest.Progress == models.CustomerProgressPublicPool

	// 准备更新数据
	updateData := bson.M{
		"progress":       progressRequest.Progress,
		"lastupdatetime": time.Now(),
		"updatedat":      time.Now(),
	}

	// 如果移入公海池，更新相关标志
	if isMovingToPublicPool {
		updateData["isinpublicpool"] = true
		updateData["previousownerid"] = customer.RelatedSalesID
		updateData["previousownername"] = customer.RelatedSalesName
		updateData["previousownertype"] = "FACTORY_SALES"

		// 清空关联信息
		updateData["relatedsalesid"] = nil
		updateData["relatedsalesname"] = nil
		updateData["relatedagentid"] = nil
		updateData["relatedagentname"] = nil
	}

	// 更新客户数据
	result, err := customersCollection.UpdateOne(
		ctx,
		bson.M{"_id": customerObjID},
		bson.M{"$set": updateData},
	)

	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新客户进展失败"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在或已被修改"})
		return
	}

	// 记录进展历史
	err = addProgressHistory(ctx, models.CustomerProgressHistory{
		CustomerID:   customerId,
		CustomerName: customer.Name,
		FromProgress: customer.Progress,
		ToProgress:   progressRequest.Progress,
		OperatorID:   user.ID,
		OperatorName: user.Username,
		Remark:       progressRequest.Remark,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	if err != nil {
		utils.HandleError(c, err)
		// 不要因为记录历史失败而阻止整个进展更新流程
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message": "客户进展更新成功",
		"data": gin.H{
			"previousProgress": customer.Progress,
			"currentProgress":  progressRequest.Progress,
		},
	})
}

// AddAssignmentHistory 添加客户分配历史记录
func AddAssignmentHistory(ctx context.Context, historyData models.CustomerAssignmentHistory) error {
	collection := repository.Collection(repository.CustAssignCollection)

	// 确保有创建时间字段
	if historyData.CreatedAt.IsZero() {
		historyData.CreatedAt = time.Now()
	}
	if historyData.UpdatedAt.IsZero() {
		historyData.UpdatedAt = time.Now()
	}
	bytes, _ := json.Marshal(historyData)
	utils.LogInfo(map[string]interface{}{
		"historyData": string(bytes),
	}, "添加客户分配历史记录成功")

	// 插入历史记录
	result, err := collection.InsertOne(ctx, historyData)
	if err != nil {
		utils.LogError2("添加客户分配历史记录", err, map[string]interface{}{
			"function": "AddAssignmentHistory",
		})

		return err
	}
	if result == nil {
		utils.LogError2("添加客户分配历史记录", fmt.Errorf("result == nil"), map[string]interface{}{
			"function": "AddAssignmentHistory",
		})

		return fmt.Errorf("result == nil")
	}

	var insertedDoc models.CustomerAssignmentHistory
	err = collection.FindOne(ctx, bson.M{"_id": result.InsertedID}).Decode(&insertedDoc)
	if err != nil {
		utils.LogError2("查询刚插入的记录失败", err, nil)
	} else {
		utils.LogInfo(map[string]interface{}{
			"insertedDoc": insertedDoc,
		}, "成功查询到刚插入的记录")
	}

	utils.LogInfo(map[string]interface{}{
		"customerId":    historyData.CustomerID,
		"customerName":  historyData.CustomerName,
		"operationType": historyData.OperationType,
		"_id":           result.InsertedID,
	}, "添加客户分配历史记录成功")

	return nil
}

// addProgressHistory 添加客户进展历史记录
func addProgressHistory(ctx context.Context, historyData models.CustomerProgressHistory) error {
	collection := repository.Collection(repository.CustomerProgressCollection)

	// 确保有创建时间字段
	if historyData.CreatedAt.IsZero() {
		historyData.CreatedAt = time.Now()
	}
	if historyData.UpdatedAt.IsZero() {
		historyData.UpdatedAt = time.Now()
	}

	// 插入历史记录
	_, err := collection.InsertOne(ctx, historyData)
	if err != nil {
		utils.LogError2("插入客户进展历史记录失败", err, map[string]interface{}{
			"function": "addProgressHistory",
		})
		return err
	}

	utils.LogInfo(map[string]interface{}{
		"customerId":   historyData.CustomerID,
		"customerName": historyData.CustomerName,
		"fromProgress": historyData.FromProgress,
		"toProgress":   historyData.ToProgress,
	}, "添加客户进展历史记录成功")
	return nil
}
