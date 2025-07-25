package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/service"
	"github.com/BerniceZTT/crm_end/utils"
)

// GetCustomerList 获取客户列表
func GetCustomerList(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	keyword := c.Query("keyword")
	nature := c.Query("nature")
	importance := c.Query("importance")
	isInPublicPool := c.Query("isInPublicPool")
	progress := c.Query("progress")
	relatedSalesId := c.Query("relatedSalesId")
	relatedAgentId := c.Query("relatedAgentId")

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}
	// skip := (page - 1) * limit

	utils.LogInfo(map[string]interface{}{
		"user":           user.Username,
		"page":           page,
		"limit":          limit,
		"keyword":        keyword,
		"nature":         nature,
		"importance":     importance,
		"progress":       progress,
		"isInPublicPool": isInPublicPool,
	}, "获取客户列表")

	filter := bson.M{}

	if keyword != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": keyword, "$options": "i"}},
			{"contactperson": bson.M{"$regex": keyword, "$options": "i"}},
			{"applicationfield": bson.M{"$regex": keyword, "$options": "i"}},
		}
	}

	if nature != "" {
		filter["nature"] = nature
	}

	if importance != "" {
		filter["importance"] = importance
	}

	filter["isInPublicPool"] = isInPublicPool == "true"

	if models.UserRole(user.Role) == models.UserRoleSUPER_ADMIN {
		// 超级管理员可以查看所有客户
	} else if models.UserRole(user.Role) == models.UserRoleFACTORY_SALES {
		if isInPublicPool != "true" {
			filter["relatedSalesId"] = user.ID
		}
	} else if models.UserRole(user.Role) == models.UserRoleAGENT {
		if isInPublicPool != "true" {
			filter["relatedAgentId"] = user.ID
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问客户数据"})
		return
	}
	if relatedSalesId != "" {
		filter["relatedSalesId"] = relatedSalesId

	}
	if relatedAgentId != "" {
		filter["relatedAgentId"] = relatedAgentId
	}

	if progress != "" {
		filter["progress"] = progress
	}

	ctx := repository.GetContext()
	collection := repository.Collection(repository.CustomersCollection)

	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"lastupdatetime": -1})
	// findOptions.SetSkip(int64(skip))
	// findOptions.SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var customers []models.Customer
	if err := cursor.All(ctx, &customers); err != nil {
		utils.HandleError(c, err)
		return
	}

	if len(customers) > 0 && isInPublicPool != "true" {
		salesIds := make(map[string]bool)
		agentIds := make(map[string]bool)

		for _, customer := range customers {
			if customer.OwnerType == string(models.UserRoleFACTORY_SALES) {
				salesIds[customer.OwnerID] = true
			} else if customer.OwnerType == string(models.UserRoleAGENT) {
				agentIds[customer.OwnerID] = true
			}

			if customer.RelatedSalesID != "" {
				salesIds[customer.RelatedSalesID] = true
			}

			if customer.RelatedAgentID != "" {
				agentIds[customer.RelatedAgentID] = true
			}
		}

		salesMap := make(map[string]string)
		if len(salesIds) > 0 {
			salesIdsArray := make([]primitive.ObjectID, 0, len(salesIds))
			for id := range salesIds {
				objectID, err := primitive.ObjectIDFromHex(id)
				if err == nil {
					salesIdsArray = append(salesIdsArray, objectID)
				}
			}

			usersCollection := repository.Collection(repository.UsersCollection)
			usersCursor, err := usersCollection.Find(ctx, bson.M{"_id": bson.M{"$in": salesIdsArray}},
				options.Find().SetProjection(bson.M{"_id": 1, "username": 1}))

			if err == nil {
				defer usersCursor.Close(ctx)
				var salesUsers []struct {
					ID       primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
					Username string             `bson:"username"`
				}
				if err := usersCursor.All(ctx, &salesUsers); err == nil {
					for _, user := range salesUsers {
						salesMap[user.ID.Hex()] = user.Username
					}
				}
			}
		}

		agentMap := make(map[string]string)
		if len(agentIds) > 0 {
			agentIdsArray := make([]primitive.ObjectID, 0, len(agentIds))
			for id := range agentIds {
				objectID, err := primitive.ObjectIDFromHex(id)
				if err == nil {
					agentIdsArray = append(agentIdsArray, objectID)
				}
			}

			agentsCollection := repository.Collection(repository.AgentsCollection)
			agentsCursor, err := agentsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": agentIdsArray}},
				options.Find().SetProjection(bson.M{"_id": 1, "companyName": 1}))

			if err == nil {
				defer agentsCursor.Close(ctx)
				var agentUsers []struct {
					ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
					CompanyName string             `bson:"companyName"`
				}
				if err := agentsCursor.All(ctx, &agentUsers); err == nil {
					for _, agent := range agentUsers {
						agentMap[agent.ID.Hex()] = agent.CompanyName
					}
				}
			}
		}

		for i := range customers {
			if customers[i].OwnerID != "" {
				if customers[i].OwnerType == string(models.UserRoleFACTORY_SALES) {
					customers[i].OwnerName = salesMap[customers[i].OwnerID]
					if customers[i].OwnerName == "" {
						customers[i].OwnerName = "未知"
					}
					customers[i].OwnerTypeDisplay = "原厂销售"
				} else if customers[i].OwnerType == string(models.UserRoleAGENT) {
					customers[i].OwnerName = agentMap[customers[i].OwnerID]
					if customers[i].OwnerName == "" {
						customers[i].OwnerName = "未知"
					}
					customers[i].OwnerTypeDisplay = "代理商"
				}
			}

			if customers[i].RelatedSalesID != "" {
				customers[i].RelatedSalesName = salesMap[customers[i].RelatedSalesID]
				if customers[i].RelatedSalesName == "" {
					customers[i].RelatedSalesName = "未知"
				}
			}

			if customers[i].RelatedAgentID != "" {
				customers[i].RelatedAgentName = agentMap[customers[i].RelatedAgentID]

				if customers[i].RelatedAgentName == "" {
					customers[i].RelatedAgentName = "未知"
				}
			}
		}
	}

	if isInPublicPool == "true" {
		for i := range customers {
			filteredCustomer := models.Customer{
				ID:             customers[i].ID,
				Name:           customers[i].Name,
				Address:        customers[i].Address,
				IsInPublicPool: true,
				CreatedAt:      customers[i].CreatedAt,
			}
			customers[i] = filteredCustomer
		}
	}

	utils.LogInfo(map[string]interface{}{
		"count": len(customers),
		"total": totalCount,
		"page":  page,
		"limit": limit,
	}, "成功获取客户列表")

	c.JSON(http.StatusOK, gin.H{
		"customers": customers,
		"pagination": gin.H{
			"total": totalCount,
			"page":  page,
			"limit": limit,
			"pages": (totalCount + int64(limit) - 1) / int64(limit),
		},
	})
}

// CheckDuplicateCustomer 查重检查客户
func CheckDuplicateCustomer(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 解析请求体
	var requestBody struct {
		CustomerNames []string `json:"customerNames"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	customerNames := requestBody.CustomerNames

	// 验证输入
	if len(customerNames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户名称不能为空"})
		return
	}

	// 记录日志
	utils.LogInfo(map[string]interface{}{
		"username":      user.Username,
		"customerNames": customerNames,
	}, "批量检查客户重复")

	ctx := repository.GetContext()
	collection := repository.Collection(repository.CustomersCollection)

	// 查询存在的客户
	filter := bson.M{
		"name":     bson.M{"$in": customerNames},
		"progress": models.CustomerProgressNormal,
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var existingCustomers []models.Customer
	if err = cursor.All(ctx, &existingCustomers); err != nil {
		utils.HandleError(c, err)
		return
	}

	if len(existingCustomers) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"exists":     false,
			"duplicates": []string{},
		})
		return
	}

	// 提取重复的客户名称
	duplicateNames := make([]string, 0, len(existingCustomers))
	for _, customer := range existingCustomers {
		duplicateNames = append(duplicateNames, customer.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"exists":     true,
		"duplicates": duplicateNames,
		"customers":  existingCustomers,
	})
}

// CreateCustomer 创建客户
func CreateCustomer(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var requestData models.CustomerCreateRequest
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	if err := validateCustomerData(requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"name":           requestData.Name,
		"nature":         requestData.Nature,
		"relatedSalesId": requestData.RelatedSalesID,
		"relatedAgentId": requestData.RelatedAgentID,
	}, "创建客户")

	ctx := repository.GetContext()
	collection := repository.Collection(repository.CustomersCollection)
	usersCollection := repository.Collection(repository.UsersCollection)
	agentsCollection := repository.Collection(repository.AgentsCollection)

	existingCustomer := struct {
		ID string `json:"_id,omitempty" bson:"_id,omitempty"`
	}{}
	err1 := collection.FindOne(ctx, bson.M{"name": requestData.Name, "progress": models.CustomerProgressNormal}).Decode(&existingCustomer)
	if err1 == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户名称已存在"})
		return
	}

	var salesUser struct {
		ID       primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
		Username string             `bson:"username"`
	}
	var salesUserName string
	if requestData.RelatedSalesID != "" {
		salesID, err := primitive.ObjectIDFromHex(requestData.RelatedSalesID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的关联销售ID"})
			return
		}

		err = usersCollection.FindOne(ctx, bson.M{"_id": salesID}).Decode(&salesUser)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "找不到关联销售"})
			return
		}
		salesUserName = salesUser.Username
	}

	var agentInfo struct {
		ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
		CompanyName string             `bson:"companyName"`
	}
	var agentCompanyName string
	if requestData.RelatedAgentID != "" {
		agentID, err := primitive.ObjectIDFromHex(requestData.RelatedAgentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的关联代理商ID"})
			return
		}

		err = agentsCollection.FindOne(ctx, bson.M{"_id": agentID}).Decode(&agentInfo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "找不到关联代理商"})
			return
		}
		agentCompanyName = agentInfo.CompanyName
	}

	now := time.Now()

	newCustomer := models.Customer{
		ID:                 primitive.NewObjectID(),
		Name:               requestData.Name,
		Nature:             requestData.Nature,
		Importance:         requestData.Importance,
		ApplicationField:   requestData.ApplicationField,
		ProductNeeds:       requestData.ProductNeeds,
		ContactPerson:      requestData.ContactPerson,
		ContactPhone:       requestData.ContactPhone,
		Address:            requestData.Address,
		Progress:           requestData.Progress,
		InitialContactTime: now,
		AnnualDemand:       requestData.AnnualDemand,
		OwnerID:            user.ID,
		OwnerName:          user.Username,
		OwnerType:          user.Role,
		RelatedSalesID:     requestData.RelatedSalesID,
		RelatedSalesName:   salesUserName,
		RelatedAgentID:     requestData.RelatedAgentID,
		RelatedAgentName:   agentCompanyName,
		IsInPublicPool:     false,
		LastUpdateTime:     now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	_, err = collection.InsertOne(ctx, newCustomer)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	operationType := "新建分配"
	if (models.UserRole(user.Role) == models.UserRoleFACTORY_SALES && user.ID == requestData.RelatedSalesID) ||
		(models.UserRole(user.Role) == models.UserRoleAGENT && user.ID == requestData.RelatedAgentID) {
		operationType = "新建认领"
	}

	if requestData.RelatedSalesID != "" || requestData.RelatedAgentID != "" {
		assignmentHistory := models.CustomerAssignmentHistory{
			CustomerID:           newCustomer.ID.Hex(),
			CustomerName:         requestData.Name,
			FromRelatedSalesID:   "",
			FromRelatedSalesName: "",
			ToRelatedSalesID:     requestData.RelatedSalesID,
			ToRelatedSalesName:   salesUserName,
			FromRelatedAgentID:   "",
			FromRelatedAgentName: "",
			ToRelatedAgentID:     requestData.RelatedAgentID,
			ToRelatedAgentName:   agentCompanyName,
			OperatorID:           user.ID,
			OperatorName:         user.Username,
			OperationType:        operationType,
		}

		err = service.AddAssignmentHistory(ctx, assignmentHistory)
		if err != nil {
			utils.HandleError(c, err)
		}
	}

	utils.LogInfo(map[string]interface{}{
		"id":   newCustomer.ID.Hex(),
		"name": newCustomer.Name,
	}, "客户创建成功")

	c.JSON(http.StatusCreated, gin.H{
		"message":  "创建客户成功",
		"customer": newCustomer,
	})
}

func BulkImportCustomers(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未授权: " + err.Error()})
		return
	}

	// 解析请求体
	var requestData struct {
		Customers []struct {
			Name             string   `json:"name"`
			Nature           string   `json:"nature"`
			Importance       string   `json:"importance"`
			ApplicationField string   `json:"applicationField"`
			ProductNeeds     []string `json:"productNeeds"`
			ContactPerson    string   `json:"contactPerson"`
			ContactPhone     string   `json:"contactPhone"`
			Address          string   `json:"address"`
			Progress         string   `json:"progress"`
			AnnualDemand     float64  `json:"annualDemand"`
			RelatedSalesName string   `json:"relatedSalesName"`
			RelatedSalesId   string   `json:"relatedSalesId"`
			RelatedAgentName string   `json:"relatedAgentName"`
			RelatedAgentId   string   `json:"relatedAgentId"`
		} `json:"customers"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	// 验证输入数据
	if requestData.Customers == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供客户数据，请求体缺少'customers'字段"})
		return
	}

	if len(requestData.Customers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "客户列表不能为空"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"username": user.Username,
		"count":    len(requestData.Customers),
	}, "开始批量导入客户")

	ctx := context.Background()
	db := repository.GetDB()
	customersCollection := db.Collection("customers")
	usersCollection := db.Collection("users")
	agentsCollection := db.Collection("agents")
	productsCollection := db.Collection("products")

	// 验证产品需求
	utils.LogInfo(nil, "开始验证产品需求合法性...")

	allProductNames := make(map[string]bool)
	for i, customer := range requestData.Customers {
		if customer.ProductNeeds != nil && len(customer.ProductNeeds) > 0 {
			for _, productName := range customer.ProductNeeds {
				if productName != "" {
					allProductNames[strings.TrimSpace(productName)] = true
				}
			}
		} else {
			utils.LogInfo(map[string]interface{}{
				"index": i + 1,
			}, "客户数据中产品需求字段无效")
		}
	}

	if len(allProductNames) > 0 {
		utils.LogInfo(map[string]interface{}{
			"count": len(allProductNames),
		}, "需要验证的产品名称数量 ")

		productNameArray := make([]string, 0, len(allProductNames))
		for name := range allProductNames {
			productNameArray = append(productNameArray, name)
		}

		cursor, err := productsCollection.Find(ctx, bson.M{"modelName": bson.M{"$in": productNameArray}},
			options.Find().SetProjection(bson.M{"_id": 1, "modelName": 1}))
		if err != nil {
			utils.HandleError(c, err)
			return
		}
		defer cursor.Close(ctx)

		var existingProducts []struct {
			ModelName string `bson:"modelName"`
		}
		if err := cursor.All(ctx, &existingProducts); err != nil {
			utils.HandleError(c, err)
			return
		}

		existingProductNames := make(map[string]bool)
		for _, product := range existingProducts {
			existingProductNames[product.ModelName] = true
		}

		var invalidProducts []string
		for name := range allProductNames {
			if !existingProductNames[name] {
				invalidProducts = append(invalidProducts, name)
			}
		}

		if len(invalidProducts) > 0 {
			utils.LogError(nil, map[string]interface{}{
				"invalidProducts": invalidProducts,
			}, "发现无效的产品名称")

			c.JSON(http.StatusBadRequest, gin.H{
				"success":         false,
				"error":           fmt.Sprintf("以下产品名称在系统中不存在: %s", strings.Join(invalidProducts, ", ")),
				"invalidProducts": invalidProducts,
			})
			return
		}

		utils.LogInfo(nil, "所有产品名称验证通过")
	} else {
		utils.LogInfo(nil, "没有需要验证的产品名称")
	}

	// 检查客户名称是否重复
	customerNames := make([]string, len(requestData.Customers))
	for i, customer := range requestData.Customers {
		customerNames[i] = customer.Name
	}

	cursor, err := customersCollection.Find(ctx, bson.M{"name": bson.M{"$in": customerNames}},
		options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var existingCustomers []struct {
		Name string `bson:"name"`
	}
	if err := cursor.All(ctx, &existingCustomers); err != nil {
		utils.HandleError(c, err)
		return
	}

	if len(existingCustomers) > 0 {
		duplicateNames := make([]string, len(existingCustomers))
		for i, customer := range existingCustomers {
			duplicateNames[i] = customer.Name
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "以下客户名称已存在",
			"duplicateNames": duplicateNames,
		})
		return
	}

	// 将名称转换为ID
	for i := range requestData.Customers {
		customer := &requestData.Customers[i]

		// 将销售名称转换为销售ID
		if customer.RelatedSalesName != "" && customer.RelatedSalesId == "" {
			var salesUser struct {
				ID       primitive.ObjectID `bson:"_id"`
				Username string             `bson:"username"`
			}
			err := usersCollection.FindOne(ctx, bson.M{
				"username": customer.RelatedSalesName,
				"role":     models.UserRoleFACTORY_SALES,
			}).Decode(&salesUser)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": fmt.Sprintf("第%d行数据中销售名称'%s'不存在", i+1, customer.RelatedSalesName),
					})
					return
				}
				utils.HandleError(c, err)
				return
			}
			customer.RelatedSalesId = salesUser.ID.Hex()
			utils.LogInfo(map[string]interface{}{
				"salesName": customer.RelatedSalesName,
				"salesId":   customer.RelatedSalesId,
			}, "销售名称转换为ID")
		}

		// 将代理商名称转换为代理商ID
		if customer.RelatedAgentName != "" && customer.RelatedAgentId == "" {
			var agent struct {
				ID          primitive.ObjectID `bson:"_id"`
				CompanyName string             `bson:"companyName"`
			}
			err := agentsCollection.FindOne(ctx, bson.M{
				"companyName": customer.RelatedAgentName,
			}).Decode(&agent)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": fmt.Sprintf("第%d行数据中代理商名称'%s'不存在", i+1, customer.RelatedAgentName),
					})
					return
				}
				utils.HandleError(c, err)
				return
			}
			customer.RelatedAgentId = agent.ID.Hex()
			utils.LogInfo(map[string]interface{}{
				"agentName": customer.RelatedAgentName,
				"agentId":   customer.RelatedAgentId,
			}, "代理商名称转换为ID")
		}
	}

	// 准备要插入的客户数据
	now := time.Now()
	customersToInsert := make([]interface{}, len(requestData.Customers))
	for i, customer := range requestData.Customers {
		customersToInsert[i] = models.Customer{
			ID:               primitive.NewObjectID(),
			Name:             customer.Name,
			Nature:           customer.Nature,
			Importance:       customer.Importance,
			ApplicationField: customer.ApplicationField,
			ProductNeeds:     customer.ProductNeeds,
			ContactPerson:    customer.ContactPerson,
			ContactPhone:     customer.ContactPhone,
			Address:          customer.Address,
			Progress:         customer.Progress,
			AnnualDemand:     customer.AnnualDemand,
			OwnerID:          user.ID,
			OwnerType:        user.Role,
			OwnerName:        user.Username,
			RelatedSalesID:   customer.RelatedSalesId,
			RelatedSalesName: customer.RelatedSalesName,
			RelatedAgentID:   customer.RelatedAgentId,
			RelatedAgentName: customer.RelatedAgentName,
			IsInPublicPool:   false,
			LastUpdateTime:   now,
			CreatedAt:        now,
		}
	}

	// 批量插入客户数据
	result, err := customersCollection.InsertMany(ctx, customersToInsert)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if len(result.InsertedIDs) != len(requestData.Customers) {
		utils.LogError(nil, map[string]interface{}{
			"insertedCount": len(result.InsertedIDs),
			"expectedCount": len(requestData.Customers),
		}, "批量插入客户失败")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":         "批量导入客户失败，请重试",
			"insertedCount": len(result.InsertedIDs),
			"expectedCount": len(requestData.Customers),
		})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"count": len(result.InsertedIDs),
	}, "成功插入客户数据")

	// 添加分配历史和进展历史
	for i, customer := range requestData.Customers {
		customerId := result.InsertedIDs[i].(primitive.ObjectID).Hex()

		// 如果有分配信息，则添加分配历史
		if customer.RelatedSalesId != "" || customer.RelatedAgentId != "" {
			operationType := "新建分配"
			if (user.Role == string(models.UserRoleFACTORY_SALES) && user.ID == customer.RelatedSalesId) ||
				(user.Role == string(models.UserRoleAGENT) && user.ID == customer.RelatedAgentId) {
				operationType = "新建认领"
			}

			assignmentHistory := models.CustomerAssignmentHistory{
				CustomerID:           customerId,
				CustomerName:         customer.Name,
				FromRelatedSalesID:   "",
				FromRelatedSalesName: "",
				ToRelatedSalesID:     customer.RelatedSalesId,
				ToRelatedSalesName:   customer.RelatedSalesName,
				FromRelatedAgentID:   "",
				FromRelatedAgentName: "",
				ToRelatedAgentID:     customer.RelatedAgentId,
				ToRelatedAgentName:   customer.RelatedAgentName,
				OperatorID:           user.ID,
				OperatorName:         user.Username,
				OperationType:        operationType,
			}

			if err := service.AddAssignmentHistory(ctx, assignmentHistory); err != nil {
				utils.LogError(err, map[string]interface{}{
					"customerId": customerId,
				}, "添加客户分配历史失败")
			}
		}

	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       fmt.Sprintf("成功导入 %d 个客户", len(requestData.Customers)),
		"insertedCount": len(requestData.Customers),
		"success":       true,
	})
}

// GetCustomerDetail 获取单个客户详情
func GetCustomerDetail(c *gin.Context) {
	id := c.Param("id")
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"id":       id,
		"username": user.Username,
	}, "获取客户详情")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID"})
		return
	}

	ctx := repository.GetContext()
	collection := repository.Collection(repository.CustomersCollection)

	var customer models.Customer
	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&customer)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN && !customer.IsInPublicPool {
		canAccess := (models.UserRole(user.Role) == models.UserRoleFACTORY_SALES &&
			(customer.OwnerID == user.ID || customer.RelatedSalesID == user.ID)) ||
			(models.UserRole(user.Role) == models.UserRoleAGENT &&
				(customer.OwnerID == user.ID || customer.RelatedAgentID == user.ID))

		if !canAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该客户"})
			return
		}
	}

	utils.LogInfo(map[string]interface{}{
		"id":   customer.ID.Hex(),
		"name": customer.Name,
	}, "成功获取客户详情")

	c.JSON(http.StatusOK, gin.H{"customer": customer})
}

// UpdateCustomer 更新客户
func UpdateCustomer(c *gin.Context) {
	id := c.Param("id")
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"id":       id,
		"username": user.Username,
		"fields":   getMapKeys(updateData),
	}, "更新客户信息")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID"})
		return
	}

	ctx := repository.GetContext()
	collection := repository.Collection(repository.CustomersCollection)

	var customer models.Customer
	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&customer)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN && !customer.IsInPublicPool {
		canAccess := (models.UserRole(user.Role) == models.UserRoleFACTORY_SALES &&
			(customer.OwnerID == user.ID || customer.RelatedSalesID == user.ID)) ||
			(models.UserRole(user.Role) == models.UserRoleAGENT &&
				(customer.OwnerID == user.ID || customer.RelatedAgentID == user.ID))

		if !canAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权更新该客户"})
			return
		}
	}

	relatedSalesChanged := updateData["relatedSalesId"] != nil &&
		updateData["relatedSalesId"] != customer.RelatedSalesID
	relatedAgentChanged := (updateData["relatedAgentId"] == nil && customer.RelatedAgentID != "") ||
		(updateData["relatedAgentId"] != nil && updateData["relatedAgentId"] != customer.RelatedAgentID)

	if relatedSalesChanged {
		usersCollection := repository.Collection(repository.UsersCollection)
		salesID, err := primitive.ObjectIDFromHex(updateData["relatedSalesId"].(string))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的关联销售ID"})
			return
		}

		var salesUser struct {
			ID       primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
			Username string             `bson:"username"`
		}
		err = usersCollection.FindOne(ctx, bson.M{"_id": salesID}).Decode(&salesUser)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "找不到关联销售"})
			return
		}
		updateData["relatedSalesName"] = salesUser.Username
	}

	if relatedAgentChanged && updateData["relatedAgentId"] != nil && updateData["relatedAgentId"] != "" {
		agentsCollection := repository.Collection(repository.AgentsCollection)
		agentID, err := primitive.ObjectIDFromHex(updateData["relatedAgentId"].(string))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的关联代理商ID"})
			return
		}

		var agent struct {
			ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
			CompanyName string             `bson:"companyName"`
		}
		err = agentsCollection.FindOne(ctx, bson.M{"_id": agentID}).Decode(&agent)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "找不到关联代理商"})
			return
		}
		updateData["relatedAgentName"] = agent.CompanyName
	} else if relatedAgentChanged && (updateData["relatedAgentId"] == nil || updateData["relatedAgentId"] == "") {
		updateData["relatedAgentName"] = ""
		updateData["relatedAgentId"] = ""
	}

	progressChanged := updateData
	// 更新客户数据
	now := time.Now()
	updateData["lastUpdateTime"] = now
	updateData["updatedAt"] = now

	// MongoDB使用$set操作符更新文档
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": updateData},
	)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
		return
	}

	// 记录关联变更历史
	if relatedSalesChanged || relatedAgentChanged {
		// 获取更新后的销售ID和名称
		toRelatedSalesID := customer.RelatedSalesID
		toRelatedSalesName := customer.RelatedSalesName
		if relatedSalesChanged {
			toRelatedSalesID = updateData["relatedSalesId"].(string)
			toRelatedSalesName = updateData["relatedSalesName"].(string)
		}

		// 获取更新后的代理商ID和名称
		toRelatedAgentID := customer.RelatedAgentID
		toRelatedAgentName := customer.RelatedAgentName
		if relatedAgentChanged {
			if updateData["relatedAgentId"] != nil && updateData["relatedAgentId"] != "" {
				toRelatedAgentID = updateData["relatedAgentId"].(string)
				toRelatedAgentName = updateData["relatedAgentName"].(string)
			} else {
				toRelatedAgentID = ""
				toRelatedAgentName = ""
			}
		}

		// 使用统一的方法记录分配历史
		assignmentHistory := models.CustomerAssignmentHistory{
			CustomerID:           id,
			CustomerName:         customer.Name,
			FromRelatedSalesID:   customer.RelatedSalesID,
			FromRelatedSalesName: customer.RelatedSalesName,
			ToRelatedSalesID:     toRelatedSalesID,
			ToRelatedSalesName:   toRelatedSalesName,
			FromRelatedAgentID:   customer.RelatedAgentID,
			FromRelatedAgentName: customer.RelatedAgentName,
			ToRelatedAgentID:     toRelatedAgentID,
			ToRelatedAgentName:   toRelatedAgentName,
			OperatorID:           user.ID,
			OperatorName:         user.Username,
			OperationType:        "分配",
		}

		err = service.AddAssignmentHistory(ctx, assignmentHistory)
		if err != nil {
			utils.HandleError(c, err)
		}
	}

	utils.LogInfo(map[string]interface{}{
		"id":             id,
		"name":           customer.Name,
		"updateCount":    result.ModifiedCount,
		"relatedChange":  relatedSalesChanged || relatedAgentChanged,
		"progressChange": progressChanged,
	}, "客户更新成功")

	c.JSON(http.StatusOK, gin.H{
		"message":     "客户更新成功",
		"updateCount": result.ModifiedCount,
	})
}

// DeleteCustomer 删除客户
func DeleteCustomer(c *gin.Context) {
	id := c.Param("id")
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"id":       id,
		"username": user.Username,
	}, "删除客户")

	// 将id转换为ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID"})
		return
	}

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取客户集合
	collection := repository.Collection(repository.CustomersCollection)

	// 查询客户
	var customer models.Customer
	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&customer)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN {
		// 检查是否是自己的客户或关联的客户
		canDelete := (models.UserRole(user.Role) == models.UserRoleFACTORY_SALES &&
			(customer.OwnerID == user.ID || customer.RelatedSalesID == user.ID)) ||
			(models.UserRole(user.Role) == models.UserRoleAGENT &&
				(customer.OwnerID == user.ID || customer.RelatedAgentID == user.ID))

		if !canDelete || customer.IsInPublicPool {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该客户"})
			return
		}
	}

	// 删除客户相关的跟进记录
	followUpCollection := repository.Collection(repository.FollowUpCollection)
	_, err = followUpCollection.DeleteMany(ctx, bson.M{"customerid": id})
	if err != nil {
		utils.HandleError(c, err)
	}

	// 删除客户
	result, err := collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在或已被删除"})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"id":   id,
		"name": customer.Name,
	}, "客户删除成功")

	c.JSON(http.StatusOK, gin.H{"message": "客户删除成功"})
}

// MoveCustomerToPublic 将客户移入公海池
func MoveCustomerToPublic(c *gin.Context) {
	id := c.Param("id")
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"id":       id,
		"username": user.Username,
	}, "将客户移入公海池")

	// 将id转换为ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID"})
		return
	}

	// 获取数据库上下文
	ctx := repository.GetContext()

	// 获取客户集合
	collection := repository.Collection(repository.CustomersCollection)

	// 查询客户
	var customer models.Customer
	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&customer)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN {
		// 检查是否是自己的客户或关联的客户
		canMove := (models.UserRole(user.Role) == models.UserRoleFACTORY_SALES &&
			(customer.OwnerID == user.ID || customer.RelatedSalesID == user.ID)) ||
			(models.UserRole(user.Role) == models.UserRoleAGENT &&
				(customer.OwnerID == user.ID || customer.RelatedAgentID == user.ID))

		if !canMove || customer.IsInPublicPool {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权将该客户移入公海"})
			return
		}
	}

	// 更新客户状态为公海
	now := time.Now()
	updateResult, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{
			"isInPublicPool":   true,
			"progress":         models.CustomerProgressPublicPool,
			"relatedSalesId":   nil,
			"relatedSalesName": nil,
			"relatedAgentId":   nil,
			"relatedAgentName": nil,
			"contactperson":    "",
			"contactphone":     "",
			"lastupdatetime":   now,
			"updatedAt":        now,
		}},
	)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if updateResult.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在或已被修改"})
		return
	}

	// 使用统一的方法记录分配历史
	assignmentHistory := models.CustomerAssignmentHistory{
		CustomerID:           id,
		CustomerName:         customer.Name,
		FromRelatedSalesID:   customer.RelatedSalesID,
		FromRelatedSalesName: customer.RelatedSalesName,
		ToRelatedSalesID:     "",
		ToRelatedSalesName:   "",
		FromRelatedAgentID:   customer.RelatedAgentID,
		FromRelatedAgentName: customer.RelatedAgentName,
		ToRelatedAgentID:     "",
		ToRelatedAgentName:   "",
		OperatorID:           user.ID,
		OperatorName:         user.Username,
		OperationType:        "移入公海池",
	}

	err = service.AddAssignmentHistory(ctx, assignmentHistory)
	if err != nil {
		utils.HandleError(c, err)
	}

	utils.LogInfo(map[string]interface{}{
		"id":   id,
		"name": customer.Name,
	}, "客户已成功移入公海")

	// 客户下项目全部变成前端不可展示
	err = UpdateWebHiddenByCustomerID(ctx, objectID)
	if err != nil {
		utils.LogError(err, map[string]interface{}{
			"id": objectID,
		}, "客户下所有项目不可见更新失败")
	}

	c.JSON(http.StatusOK, gin.H{"message": "客户已成功移入公海"})
}

// 辅助函数：验证客户数据
func validateCustomerData(data models.CustomerCreateRequest) error {
	if data.Name == "" {
		return &utils.AppError{Message: "客户名称不能为空", StatusCode: http.StatusBadRequest}
	}
	if data.Nature == "" {
		return &utils.AppError{Message: "客户性质不能为空", StatusCode: http.StatusBadRequest}
	}
	if data.Importance == "" {
		return &utils.AppError{Message: "客户重要程度不能为空", StatusCode: http.StatusBadRequest}
	}
	if data.Progress == "" {
		return &utils.AppError{Message: "客户进展状态不能为空", StatusCode: http.StatusBadRequest}
	}
	if data.RelatedSalesID == "" {
		return &utils.AppError{Message: "关联销售不能为空", StatusCode: http.StatusBadRequest}
	}
	// 可以添加其他验证逻辑
	return nil
}

// 辅助函数：获取map的所有键
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// 阿里云API配置
const (
	AliCloudURL   = "https://comserver.market.alicloudapi.com/searchCompany"
	AliCloudAppID = "b5849b8dac554e7e93af538180a24382"
)

// CompanyInfo 公司信息结构体
type CompanyInfo struct {
	RegNumber      string `json:"regNumber"`
	RegType        string `json:"regType"`
	CompanyName    string `json:"companyName"`
	CompanyType    string `json:"companyType"`
	RegMoney       string `json:"regMoney"`
	FaRen          string `json:"faRen"`
	IssueTime      string `json:"issueTime"`
	CreditCode     string `json:"creditCode"`
	ProvinceName   string `json:"provinceName"`
	BusinessStatus string `json:"businessStatus"`
}

// AliCloudResponse 阿里云API响应结构体
type AliCloudResponse struct {
	ErrorCode int             `json:"error_code"`
	Reason    string          `json:"reason"`
	Result    json.RawMessage `json:"result"` // 使用 json.RawMessage 延迟解析
}

// APIResponse API响应结构体
type APIResponse struct {
	ErrorCode int         `json:"error_code"`
	Reason    string      `json:"reason"`
	Result    interface{} `json:"result"`
	OrderSign string      `json:"ordersign,omitempty"`
}

// ResultWithData 包含 total 和 data 的结构体
type ResultWithData struct {
	Total int           `json:"total"`
	Data  []CompanyInfo `json:"data"`
}

func CompleteCompanyNamesHandler(c *gin.Context) {
	log.Println("[API] 获取公司名称补充请求")

	// 获取查询参数
	prefix := c.Query("prefix")

	// 验证参数
	if prefix == "" || strings.TrimSpace(prefix) == "" {
		log.Println("[API] 缺少prefix参数")
		c.JSON(http.StatusBadRequest, APIResponse{
			ErrorCode: 1,
			Reason:    "请提供公司名称前缀",
			Result:    nil,
		})
		return
	}

	log.Printf("[API] 查询公司名称前缀: %s", prefix)

	// 调用阿里云API
	companyList, err := callAliCloudAPI(prefix)
	if err != nil {
		log.Printf("[API] 调用阿里云API失败: %v", err)
		errorMsg := "服务器内部错误"
		if strings.Contains(err.Error(), "阿里云API请求失败") {
			errorMsg = "公司信息查询服务暂时不可用"
		} else if strings.Contains(err.Error(), "网络连接失败") {
			errorMsg = "网络连接失败，请稍后重试"
		}

		c.JSON(http.StatusInternalServerError, APIResponse{
			ErrorCode: 1,
			Reason:    errorMsg,
			Result:    nil,
		})
		return
	}

	// 过滤掉空的公司名称
	filteredCompanyList := []CompanyInfo{}
	for _, company := range companyList {
		if company.CompanyName != "" && strings.TrimSpace(company.CompanyName) != "" {
			filteredCompanyList = append(filteredCompanyList, company)
		}
	}

	log.Printf("[API] 处理前公司数量: %d, 过滤后公司数量: %d", len(companyList), len(filteredCompanyList))

	// 生成订单签名
	rand.Seed(time.Now().UnixNano())
	orderSign := fmt.Sprintf("%d%d", time.Now().Unix(), rand.Intn(1000000000000000))

	c.JSON(http.StatusOK, APIResponse{
		ErrorCode: 0,
		Reason:    "查询成功",
		Result: ResultWithData{
			Total: len(filteredCompanyList),
			Data:  filteredCompanyList,
		},
		OrderSign: orderSign,
	})
}

func callAliCloudAPI(prefix string) ([]CompanyInfo, error) {
	// 构建查询参数
	params := url.Values{}
	params.Set("com", strings.TrimSpace(prefix))
	params.Set("page", "1")
	params.Set("query", "all")

	apiURL := fmt.Sprintf("%s?%s", AliCloudURL, params.Encode())
	log.Printf("[API] 调用阿里云API: %s", apiURL)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "APPCODE "+AliCloudAppID)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络连接失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("阿里云API请求失败: %s", resp.Status)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	log.Printf("[API] 阿里云API原始响应: %s", string(body))

	// 解析响应
	return parseAliCloudResponse(body)
}

func parseAliCloudResponse(body []byte) ([]CompanyInfo, error) {
	var response AliCloudResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// 检查错误码
	if response.ErrorCode != 0 {
		// 如果是查询无结果的情况，返回空切片和 nil 错误
		if response.ErrorCode == 50002 && response.Reason == "查询无结果" {
			return []CompanyInfo{}, nil
		}
		// 其他错误情况，返回错误信息
		return nil, errors.New(response.Reason)
	}

	// 尝试解析 Result 为 ResultWithData
	var resultWithData ResultWithData
	if err := json.Unmarshal(response.Result, &resultWithData); err == nil {
		return resultWithData.Data, nil
	}

	// 尝试解析 Result 为空数组
	var emptyArray []interface{}
	if err := json.Unmarshal(response.Result, &emptyArray); err == nil {
		return []CompanyInfo{}, nil
	}

	// 如果无法解析，返回错误
	return nil, errors.New("无法解析 Result 字段")
}
