package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func HasProjects(ctx context.Context, customerID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"customerId": customerID,
		"webHidden":  false,
	}

	collection := repository.Collection(repository.ProjectsCollection)

	findOptions := options.Find().SetLimit(1)

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		log.Printf("查询测试中的项目失败: %v", err)
		return false, fmt.Errorf("查询测试中的项目失败: %v", err)
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err = cursor.All(ctx, &projects); err != nil {
		log.Printf("解析项目失败: %v", err)
		return false, fmt.Errorf("解析项目失败: %v", err)
	}

	return len(projects) > 0, nil
}

func UpdateCustomerProgress(ctx context.Context, customerObjID primitive.ObjectID, progress string) error {
	customersCollection := repository.Collection(repository.CustomersCollection)
	updateData := bson.M{
		"progress":       progress,
		"lastupdatetime": time.Now(),
		"updatedAt":      time.Now(),
	}
	_, err := customersCollection.UpdateOne(
		ctx,
		bson.M{"_id": customerObjID},
		bson.M{"$set": updateData},
	)
	return err
}

func UpdateCustomerProgressByName(ctx context.Context, name string, progress string) error {
	customersCollection := repository.Collection(repository.CustomersCollection)
	updateData := bson.M{
		"progress":       progress,
		"lastupdatetime": time.Now(),
		"updatedAt":      time.Now(),
	}
	_, err := customersCollection.UpdateMany(
		ctx,
		bson.M{"name": name, "progress": models.CustomerProgressInitialContact},
		bson.M{"$set": updateData},
	)
	return err
}

// 解析请求数据
type AssignRequest struct {
	SalesId string `json:"salesId" binding:"required"`
	AgentId string `json:"agentId"`
}

func AssignCustomer(ctx context.Context, c *gin.Context, customerId string, assignRequest AssignRequest, user *utils.LoginUser) (error, int, gin.H) {
	// 获取相关集合
	customersCollection := repository.Collection(repository.CustomersCollection)
	usersCollection := repository.Collection(repository.UsersCollection)
	agentsCollection := repository.Collection(repository.AgentsCollection)

	// 将客户ID转换为ObjectID
	customerObjID, err := primitive.ObjectIDFromHex(customerId)
	if err != nil {
		return err, http.StatusBadRequest, gin.H{"error": "无效的客户ID格式"}
	}

	// 查询客户
	var customer models.Customer
	err = customersCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return err, http.StatusNotFound, gin.H{"error": "客户不存在"}
		} else {
			utils.HandleError(c, err)
			return err, http.StatusInternalServerError, gin.H{"error": "分配客户失败"}
		}
	}

	// 查询销售信息
	salesObjID, err := primitive.ObjectIDFromHex(assignRequest.SalesId)
	if err != nil {
		return err, http.StatusBadRequest, gin.H{"error": "无效的销售ID格式"}
	}

	var salesUser models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": salesObjID}).Decode(&salesUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return err, http.StatusNotFound, gin.H{"error": "指定的销售人员不存在"}
		} else {
			utils.HandleError(c, err)
			return err, http.StatusInternalServerError, gin.H{"error": "分配客户失败"}
		}
	}

	// 准备更新数据
	progress := models.CustomerProgressInitialContact
	had, err := HasProjects(ctx, customerObjID)
	if err != nil {
		utils.HandleError(c, err)
		return err, http.StatusInternalServerError, gin.H{"error": "获取客户项目状态失败"}
	}
	if had {
		progress = models.CustomerProgressNormal
	}
	updateData := bson.M{
		"relatedSalesId":   assignRequest.SalesId,
		"relatedSalesName": salesUser.Username,
		"lastupdatetime":   time.Now(),
		"updatedAt":        time.Now(),
		"isInPublicPool":   false,
		"progress":         progress,
	}
	if progress == models.CustomerProgressInitialContact {
		updateData = bson.M{
			"relatedSalesId":     assignRequest.SalesId,
			"relatedSalesName":   salesUser.Username,
			"lastupdatetime":     time.Now(),
			"updatedAt":          time.Now(),
			"isInPublicPool":     false,
			"progress":           progress,
			"initialContactTime": time.Now(),
		}
	}

	// 处理代理商信息
	var agentInfo map[string]string
	if assignRequest.AgentId != "" {
		agentObjID, err := primitive.ObjectIDFromHex(assignRequest.AgentId)
		if err != nil {
			return err, http.StatusBadRequest, gin.H{"error": "无效的代理商ID格式"}
		}
		var agent models.Agent
		err = agentsCollection.FindOne(ctx, bson.M{"_id": agentObjID}).Decode(&agent)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return err, http.StatusNotFound, gin.H{"error": "指定的代理商不存在"}
			} else {
				utils.HandleError(c, err)
				return err, http.StatusInternalServerError, gin.H{"error": "分配客户失败"}
			}
		}

		updateData["relatedAgentId"] = assignRequest.AgentId
		updateData["relatedAgentName"] = agent.CompanyName

		agentInfo = map[string]string{
			"id":   assignRequest.AgentId,
			"name": agent.CompanyName,
		}
	} else if assignRequest.AgentId == "" {
		// 清空代理商关联
		updateData["relatedAgentId"] = nil
		updateData["relatedAgentName"] = nil
	}

	// 更新客户数据
	result, err := customersCollection.UpdateOne(
		ctx,
		bson.M{"_id": customerObjID},
		bson.M{"$set": updateData},
	)

	if err != nil {
		utils.HandleError(c, err)
		return err, http.StatusInternalServerError, gin.H{"error": "分配客户失败"}
	}
	if progress == models.CustomerProgressNormal {
		// 更改其他客户状态
		err1 := UpdateCustomerProgressByName(ctx, customer.Name, models.CustomerProgressDisabled)
		if err1 != nil {
			utils.HandleError(c, err1)
			return err1, http.StatusInternalServerError, gin.H{"error": "客户为正常推进状态，修改其他同名客户信息报错"}
		}
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("result.MatchedCount == 0"), http.StatusNotFound, gin.H{"error": "客户不存在或已被修改"}
	}

	// 检查是否需要记录分配历史
	salesChanged := assignRequest.SalesId != customer.RelatedSalesID
	agentChanged := assignRequest.AgentId != customer.RelatedAgentID

	// 只在有变化时记录历史
	if salesChanged || agentChanged {
		operationType := "分配"
		// 判断是否为认领：当前用户是被分配的销售或代理商
		if (user.Role == "FACTORY_SALES" && user.ID == assignRequest.SalesId) ||
			(user.Role == "AGENT" && user.ID == assignRequest.AgentId) {
			operationType = "认领"
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

	return nil, http.StatusOK, gin.H{
		"message": "客户分配成功",
		"data": gin.H{
			"salesId":   assignRequest.SalesId,
			"salesName": salesUser.Username,
			"agentId":   assignRequest.AgentId,
			"agentName": agentInfo["name"],
		},
	}
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
