package controllers

import (
	"encoding/csv"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"server2/models"
	"server2/repository"
	"server2/utils"
)

// GetAllAgents 获取所有代理商
func GetAllAgents(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 获取代理商集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	ctx := repository.GetContext()

	// 构建查询条件
	query := bson.M{}
	// 如果是原厂销售，只能看到与自己关联的代理商
	if user.Role == "FACTORY_SALES" {
		query = bson.M{"relatedSalesId": user.ID}
	}

	// 查询代理商列表，排除密码字段
	findOptions := options.Find().SetProjection(bson.M{"password": 0})
	cursor, err := agentsCollection.Find(ctx, query, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理商列表失败"})
		return
	}
	defer cursor.Close(ctx)

	// 解析查询结果
	var agents []models.Agent
	if err = cursor.All(ctx, &agents); err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析代理商数据失败"})
		return
	}

	// 如果有代理商数据，关联查询销售人员信息
	if len(agents) > 0 {
		// 获取需要查询的销售ID列表
		var salesIds []primitive.ObjectID
		for _, agent := range agents {
			if agent.RelatedSalesID != "" {
				objID, err := primitive.ObjectIDFromHex(agent.RelatedSalesID)
				if err == nil {
					salesIds = append(salesIds, objID)
				}
			}
		}

		// 如果有关联销售，查询销售人员信息
		if len(salesIds) > 0 {
			usersCollection := repository.Collection(repository.UsersCollection)
			salesQuery := bson.M{"_id": bson.M{"$in": salesIds}}
			salesOptions := options.Find().SetProjection(bson.M{"_id": 1, "username": 1})

			salesCursor, err := usersCollection.Find(ctx, salesQuery, salesOptions)
			if err == nil {
				defer salesCursor.Close(ctx)

				var salesUsers []struct {
					ID       primitive.ObjectID `bson:"_id"`
					Username string             `bson:"username"`
				}

				if err = salesCursor.All(ctx, &salesUsers); err == nil {
					// 构建销售ID到名称的映射
					salesMap := make(map[string]string)
					for _, user := range salesUsers {
						salesMap[user.ID.Hex()] = user.Username
					}

					// 将销售名称添加到代理商数据
					for i := range agents {
						if agents[i].RelatedSalesID != "" {
							if name, ok := salesMap[agents[i].RelatedSalesID]; ok {
								agents[i].RelatedSalesName = name
							}
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// CreateAgent 创建代理商
func CreateAgent(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 解析请求数据
	var agentData models.Agent
	if err := c.ShouldBindJSON(&agentData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式不正确"})
		return
	}

	// 调试日志 - 打印接收到的数据
	utils.Logger.Info().
		Str("companyName", agentData.CompanyName).
		Str("contactPerson", agentData.ContactPerson).
		Str("phone", agentData.Phone).
		Str("password", agentData.Password). // 重点检查这个字段
		Msg("接收到的代理商数据")

	// 验证必填字段
	if agentData.CompanyName == "" || len(agentData.CompanyName) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公司名至少2个字符"})
		return
	}
	if agentData.Password == "" || len(agentData.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少6个字符"})
		return
	}
	if agentData.ContactPerson == "" || len(agentData.ContactPerson) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "联系人名至少2个字符"})
		return
	}
	if agentData.Phone == "" || !utils.IsValidPhone(agentData.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的手机号"})
		return
	}

	// 获取代理商集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	ctx := repository.GetContext()

	// 检查公司名称是否重复
	existingAgent := agentsCollection.FindOne(ctx, bson.M{"companyName": agentData.CompanyName})
	if existingAgent.Err() == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公司名称已存在"})
		return
	} else if existingAgent.Err() != mongo.ErrNoDocuments {
		utils.HandleError(c, existingAgent.Err())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建代理商失败"})
		return
	}

	// 根据用户角色设置代理商状态
	if user.Role == "FACTORY_SALES" {
		// 原厂销售创建时，关联销售ID设为自己，状态为待审批
		agentData.RelatedSalesID = user.ID
		agentData.Status = "pending" // 设为待审批状态
	} else if user.Role == "SUPER_ADMIN" {
		// 超级管理员创建的代理商直接审核通过
		agentData.Status = "approved"
	}

	// 对密码进行加密
	agentData.Password = utils.HashPassword(agentData.Password)

	// 设置创建时间
	agentData.CreatedAt = time.Now()

	// 插入代理商数据
	result, err := agentsCollection.InsertOne(ctx, agentData)
	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建代理商失败"})
		return
	}

	// 设置返回消息
	message := "代理商创建成功"
	if user.Role == "FACTORY_SALES" {
		message = "代理商创建成功，请等待管理员审批"
	}

	// 返回响应
	agentData.ID = result.InsertedID.(primitive.ObjectID)
	agentData.Password = "" // 不返回密码
	c.JSON(http.StatusCreated, gin.H{
		"message": message,
		"agent":   agentData,
	})
}

// UpdateAgent 更新代理商
func UpdateAgent(c *gin.Context) {
	// 获取代理商ID
	id := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的代理商ID"})
		return
	}

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 解析请求数据
	var updateData struct {
		CompanyName    string `json:"companyName"`
		ContactPerson  string `json:"contactPerson"`
		Phone          string `json:"phone"`
		RelatedSalesID string `json:"relatedSalesId"`
		Password       string `json:"password"`
	}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式不正确"})
		return
	}

	// 验证字段
	if updateData.CompanyName != "" && len(updateData.CompanyName) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "公司名至少2个字符"})
		return
	}
	if updateData.ContactPerson != "" && len(updateData.ContactPerson) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "联系人名至少2个字符"})
		return
	}
	if updateData.Phone != "" && !utils.IsValidPhone(updateData.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的手机号"})
		return
	}
	if updateData.Password != "" && len(updateData.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少6个字符"})
		return
	}

	// 获取代理商集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	ctx := repository.GetContext()

	// 查找要更新的代理商
	var existingAgent models.Agent
	err = agentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existingAgent)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "代理商不存在"})
		} else {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新代理商失败"})
		}
		return
	}

	// 检查权限
	if user.Role == "FACTORY_SALES" {
		// 原厂销售只能修改自己创建的代理商
		if existingAgent.RelatedSalesID != user.ID && existingAgent.Status != "pending" {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权修改此代理商"})
			return
		}

		// 原厂销售可以修改关联销售，但只能是自己
		if updateData.RelatedSalesID != "" && updateData.RelatedSalesID != user.ID {
			updateData.RelatedSalesID = user.ID // 强制设置为自己
		}
	}

	// 构建更新数据
	update := bson.M{}
	if updateData.CompanyName != "" {
		// 检查公司名称是否重复
		if updateData.CompanyName != existingAgent.CompanyName {
			count, err := agentsCollection.CountDocuments(ctx, bson.M{
				"companyName": updateData.CompanyName,
				"_id":         bson.M{"$ne": objID},
			})
			if err != nil {
				utils.HandleError(c, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新代理商失败"})
				return
			}
			if count > 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "公司名称已存在"})
				return
			}
		}
		update["companyName"] = updateData.CompanyName
	}

	if updateData.ContactPerson != "" {
		update["contactPerson"] = updateData.ContactPerson
	}

	if updateData.Phone != "" {
		update["phone"] = updateData.Phone
	}

	if updateData.RelatedSalesID != "" {
		update["relatedSalesId"] = updateData.RelatedSalesID
	}

	if updateData.Password != "" {
		update["password"] = utils.HashPassword(updateData.Password)
	}

	// 设置更新时间
	update["updatedat"] = time.Now()

	// 执行更新操作
	result, err := agentsCollection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": update})
	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新代理商失败"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "代理商不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "代理商更新成功",
		"modifiedCount": result.ModifiedCount,
	})
}

// DeleteAgent 删除代理商
func DeleteAgent(c *gin.Context) {
	// 获取代理商ID
	id := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的代理商ID"})
		return
	}

	// 获取代理商和客户集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	customersCollection := repository.Collection(repository.CustomersCollection)
	ctx := repository.GetContext()

	// 首先检查代理商是否存在
	var agent models.Agent
	err = agentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&agent)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "代理商不存在"})
		} else {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除代理商失败"})
		}
		return
	}

	// 查询该代理商是否有关联客户
	customerCount, err := customersCollection.CountDocuments(ctx, bson.M{
		"ownerid":   id,
		"ownertype": "AGENT",
	})
	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除代理商失败"})
		return
	}

	if customerCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "该代理商有关联客户，无法删除",
			"customerCount": customerCount,
		})
		return
	}

	// 删除代理商
	result, err := agentsCollection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除代理商失败"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "代理商不存在或已被删除"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除代理商成功"})
}

// GetAgentsBySalesId 获取特定销售的代理商列表
func GetAgentsBySalesId(c *gin.Context) {
	// 获取销售ID
	salesId := c.Param("salesId")

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 权限检查
	if user.Role != "SUPER_ADMIN" && user.Role != "FACTORY_SALES" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该数据"})
		return
	}

	// 销售只能查看自己关联的代理商
	if user.Role == "FACTORY_SALES" && user.ID != salesId {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能查看自己关联的代理商"})
		return
	}

	// 获取代理商集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	ctx := repository.GetContext()

	// 查询代理商列表
	findOptions := options.Find().SetProjection(bson.M{"password": 0})
	cursor, err := agentsCollection.Find(ctx, bson.M{
		"relatedSalesId": salesId,
		"status":         "approved",
	}, findOptions)

	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理商列表失败"})
		return
	}
	defer cursor.Close(ctx)

	// 解析查询结果
	var agents []models.Agent
	if err = cursor.All(ctx, &agents); err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析代理商数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// ExportAgentsToCSV 导出代理商列表为CSV
func ExportAgentsToCSV(c *gin.Context) {
	// 获取代理商集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	ctx := repository.GetContext()

	// 查询代理商列表
	findOptions := options.Find().SetProjection(bson.M{"password": 0})
	cursor, err := agentsCollection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出代理商失败"})
		return
	}
	defer cursor.Close(ctx)

	// 解析查询结果
	var agents []models.Agent
	if err = cursor.All(ctx, &agents); err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析代理商数据失败"})
		return
	}

	// 获取关联销售ID集合
	var salesIds []primitive.ObjectID
	for _, agent := range agents {
		if agent.RelatedSalesID != "" {
			objID, err := primitive.ObjectIDFromHex(agent.RelatedSalesID)
			if err == nil {
				salesIds = append(salesIds, objID)
			}
		}
	}

	// 查询销售人员信息
	salesMap := make(map[string]string)
	if len(salesIds) > 0 {
		usersCollection := repository.Collection(repository.UsersCollection)
		salesCursor, err := usersCollection.Find(ctx, bson.M{
			"_id": bson.M{"$in": salesIds},
		}, options.Find().SetProjection(bson.M{"_id": 1, "username": 1}))

		if err == nil {
			defer salesCursor.Close(ctx)

			var salesUsers []struct {
				ID       primitive.ObjectID `bson:"_id"`
				Username string             `bson:"username"`
			}

			if err = salesCursor.All(ctx, &salesUsers); err == nil {
				for _, user := range salesUsers {
					salesMap[user.ID.Hex()] = user.Username
				}
			}
		}
	}

	// 设置CSV响应头
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=agents.csv")

	// 创建CSV Writer
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// 写入CSV头
	headers := []string{"代理商公司名称", "联系人", "联系电话", "关联销售", "创建时间", "状态"}
	if err := writer.Write(headers); err != nil {
		utils.HandleError(c, err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// 状态映射
	statusMap := map[string]string{
		"approved": "已批准",
		"pending":  "待审批",
		"rejected": "已拒绝",
	}

	// 写入数据行
	for _, agent := range agents {
		// 获取销售名称
		salesName := "无"
		if agent.RelatedSalesID != "" {
			if name, ok := salesMap[agent.RelatedSalesID]; ok {
				salesName = name
			} else {
				salesName = "未知"
			}
		}

		// 获取状态显示文本
		status := statusMap[string(agent.Status)]
		if status == "" {
			status = string(agent.Status)
		}

		// 构建CSV行
		row := []string{
			agent.CompanyName,
			agent.ContactPerson,
			agent.Phone,
			salesName,
			agent.CreatedAt.Format("2006-01-02 15:04:05"),
			status,
		}

		// 写入行
		if err := writer.Write(row); err != nil {
			utils.HandleError(c, err)
			c.Status(http.StatusInternalServerError)
			return
		}
	}
}

// GetAssignableAgents 获取可分配的代理商列表
func GetAssignableAgents(c *gin.Context) {
	// 获取当前用户信息
	user, err1 := utils.GetUser(c)
	if err1 != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err1.Error()})
		return
	}

	// 获取代理商集合
	agentsCollection := repository.Collection(repository.AgentsCollection)
	ctx := repository.GetContext()

	// 构建查询条件和投影
	query := bson.M{"status": "approved"}
	projection := bson.M{
		"_id":            1,
		"companyName":    1,
		"contactPerson":  1,
		"phone":          1,
		"relatedSalesId": 1,
	}

	var agents []models.Agent
	var err error

	// 根据不同角色处理查询逻辑
	if user.Role == "SUPER_ADMIN" {
		// 超级管理员可以看到所有已批准的代理商
		cursor, err := agentsCollection.Find(ctx, query, options.Find().SetProjection(projection))
		if err != nil {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取可分配代理商列表失败"})
			return
		}
		defer cursor.Close(ctx)

		err = cursor.All(ctx, &agents)
	} else if user.Role == "FACTORY_SALES" {
		// 销售只能看到关联的代理商
		query["relatedSalesId"] = user.ID
		cursor, err := agentsCollection.Find(ctx, query, options.Find().SetProjection(projection))
		if err != nil {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取可分配代理商列表失败"})
			return
		}
		defer cursor.Close(ctx)

		err = cursor.All(ctx, &agents)
	} else if user.Role == "AGENT" {
		// 代理商只能看到自己
		objID, err := primitive.ObjectIDFromHex(user.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
			return
		}

		query["_id"] = objID
		cursor, err := agentsCollection.Find(ctx, query, options.Find().SetProjection(projection))
		if err != nil {
			utils.HandleError(c, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取可分配代理商列表失败"})
			return
		}
		defer cursor.Close(ctx)

		err = cursor.All(ctx, &agents)
	} else {
		// 其他角色无权访问
		c.JSON(http.StatusForbidden, gin.H{"error": "无权获取代理商列表"})
		return
	}

	if err != nil {
		utils.HandleError(c, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取可分配代理商列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}
