package controllers

import (
	"net/http"
	"time"

	"server2/models"
	"server2/repository"
	"server2/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAllUsers 获取所有用户
func GetAllUsers(c *gin.Context) {
	utils.Logger.Info().Msg("处理获取用户列表请求...")

	// 检查数据库和集合状态
	collection := repository.Collection(repository.UsersCollection)

	// 检查集合中的总记录数
	totalCount, err := collection.CountDocuments(repository.GetContext(), bson.M{})
	if err != nil {
		utils.Logger.Error().Err(err).Msg("获取用户总数失败")
		utils.ErrorResponse(c, "获取用户列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Logger.Info().Int64("totalCount", totalCount).Msg("集合总记录数")

	if totalCount == 0 {
		utils.Logger.Warn().Msg("用户集合中没有任何记录!")
		utils.SuccessResponse(c, gin.H{"users": []interface{}{}}, "")
		return
	}

	// 设置查询选项，排除密码字段
	findOptions := options.Find().SetProjection(bson.M{"password": 0})

	// 执行查询
	cursor, err := collection.Find(repository.GetContext(), bson.M{}, findOptions)
	if err != nil {
		utils.Logger.Error().Err(err).Msg("查询用户失败")
		utils.ErrorResponse(c, "获取用户列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(repository.GetContext())

	// 解析结果
	var users []models.User
	if err := cursor.All(repository.GetContext(), &users); err != nil {
		utils.Logger.Error().Err(err).Msg("解析用户数据失败")
		utils.ErrorResponse(c, "获取用户列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Logger.Info().Int("count", len(users)).Msg("成功查询到用户记录")

	if len(users) > 0 {
		utils.Logger.Debug().Interface("firstUser", users[0]).Msg("返回的第一个用户数据示例")
	}

	// 检查查询结果
	if int64(len(users)) != totalCount {
		utils.Logger.Warn().
			Int("resultCount", len(users)).
			Int64("totalCount", totalCount).
			Msg("警告: 查询结果数量与集合总记录数不一致")
	}

	utils.Logger.Info().Int("count", len(users)).Msg("获取用户列表成功")
	utils.SuccessResponse(c, gin.H{"users": users}, "")
}

// GetSalesUsers 获取所有销售人员
func GetSalesUsers(c *gin.Context) {
	utils.Logger.Info().Msg("处理获取销售人员列表请求...")

	collection := repository.Collection(repository.UsersCollection)

	// 设置查询条件和选项
	filter := bson.M{
		"role":   models.UserRoleFACTORY_SALES,
		"status": models.UserStatusAPPROVED,
	}
	options := options.Find().SetProjection(bson.M{"password": 0})

	// 执行查询
	cursor, err := collection.Find(repository.GetContext(), filter, options)
	if err != nil {
		utils.Logger.Error().Err(err).Msg("查询销售人员失败")
		utils.ErrorResponse(c, "获取销售人员列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(repository.GetContext())

	// 解析结果
	var users []models.User
	if err := cursor.All(repository.GetContext(), &users); err != nil {
		utils.Logger.Error().Err(err).Msg("解析销售人员数据失败")
		utils.ErrorResponse(c, "获取销售人员列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Logger.Info().Int("count", len(users)).Msg("成功查询到销售人员记录")

	if len(users) > 0 {
		utils.Logger.Debug().Interface("firstUser", users[0]).Msg("返回的第一个销售人员数据示例")
	}

	utils.SuccessResponse(c, gin.H{"users": users}, "")
}

// GetPendingApprovalUsers 获取待审批用户
func GetPendingApprovalUsers(c *gin.Context) {
	utils.Logger.Info().Msg("处理获取待审批用户请求...")

	collection := repository.Collection(repository.UsersCollection)

	// 设置查询条件和选项
	filter := bson.M{"status": models.UserStatusPENDING}
	options := options.Find().SetProjection(bson.M{"password": 0})

	// 执行查询
	cursor, err := collection.Find(repository.GetContext(), filter, options)
	if err != nil {
		utils.Logger.Error().Err(err).Msg("查询待审批用户失败")
		utils.ErrorResponse(c, "获取待审批用户失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(repository.GetContext())

	// 解析结果
	var pendingUsers []models.User
	if err := cursor.All(repository.GetContext(), &pendingUsers); err != nil {
		utils.Logger.Error().Err(err).Msg("解析待审批用户数据失败")
		utils.ErrorResponse(c, "获取待审批用户失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Logger.Info().Int("count", len(pendingUsers)).Msg("找到待审批用户")

	utils.SuccessResponse(c, gin.H{"pendingAccounts": pendingUsers}, "")
}

// ApproveUser 审批用户
func ApproveUser(c *gin.Context) {
	var req models.ApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.Logger.Info().
		Str("id", req.ID).
		Str("type", req.Type).
		Bool("approved", req.Approved).
		Msg("处理账户审批请求")

	// 确定集合名称
	collectionName := repository.UsersCollection
	if req.Type == "agent" {
		collectionName = repository.AgentsCollection
	}

	collection := repository.Collection(collectionName)

	// 检查ID格式
	objectID, err := primitive.ObjectIDFromHex(req.ID)
	if err != nil {
		utils.Logger.Error().Err(err).Str("id", req.ID).Msg("无效的ID格式")
		utils.ErrorResponse(c, "无效的ID格式", http.StatusBadRequest)
		return
	}

	// 检查账户是否存在
	var existingAccount bson.M
	err = collection.FindOne(repository.GetContext(), bson.M{"_id": objectID}).Decode(&existingAccount)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.Logger.Error().Str("id", req.ID).Msg("账户不存在")
			utils.ErrorResponse(c, "账户不存在", http.StatusNotFound)
		} else {
			utils.Logger.Error().Err(err).Str("id", req.ID).Msg("查询账户失败")
			utils.ErrorResponse(c, "查询账户失败: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// 检查是否已经被审批
	if status, ok := existingAccount["status"].(string); ok && status != string(models.UserStatusPENDING) {
		utils.Logger.Error().Str("id", req.ID).Str("status", status).Msg("该账户已经被审批过")
		utils.ErrorResponse(c, "该账户已经被审批过", http.StatusBadRequest)
		return
	}

	// 更新用户状态
	newStatus := models.UserStatusAPPROVED
	if !req.Approved {
		newStatus = models.UserStatusREJECTED
	}

	update := bson.M{
		"$set": bson.M{
			"status":    newStatus,
			"updatedAt": time.Now(),
		},
	}

	// 如果拒绝，添加拒绝原因
	if !req.Approved && req.Reason != "" {
		update["$set"].(bson.M)["rejectionReason"] = req.Reason
	}

	// 更新账户
	result, err := collection.UpdateOne(repository.GetContext(), bson.M{"_id": objectID}, update)
	if err != nil {
		utils.Logger.Error().Err(err).Str("id", req.ID).Msg("更新账户状态失败")
		utils.ErrorResponse(c, "更新账户状态失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result.ModifiedCount == 0 {
		utils.Logger.Error().Str("id", req.ID).Msg("账户状态更新失败")
		utils.ErrorResponse(c, "账户状态更新失败", http.StatusBadRequest)
		return
	}

	message := "已批准账户"
	if !req.Approved {
		message = "已拒绝账户"
	}

	utils.Logger.Info().
		Str("id", req.ID).
		Str("status", string(newStatus)).
		Msg("账户审批成功")

	utils.SuccessResponse(c, nil, message)
}

// CreateUser 创建用户
func CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.Logger.Info().
		Str("username", req.Username).
		Str("role", string(req.Role)).
		Msg("处理创建用户请求")

	collection := repository.Collection(repository.UsersCollection)

	// 检查用户名是否已存在
	var existingUser models.User
	err := collection.FindOne(repository.GetContext(), bson.M{"username": req.Username}).Decode(&existingUser)
	if err == nil {
		utils.Logger.Error().Str("username", req.Username).Msg("用户名已存在")
		utils.ErrorResponse(c, "用户名已存在", http.StatusBadRequest)
		return
	} else if err != mongo.ErrNoDocuments {
		utils.Logger.Error().Err(err).Str("username", req.Username).Msg("检查用户名时发生错误")
		utils.ErrorResponse(c, "创建用户失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 检查是否有超级管理员角色
	if req.Role == models.UserRoleSUPER_ADMIN {
		// 检查是否已存在超级管理员
		var existingAdmin models.User
		err := collection.FindOne(repository.GetContext(), bson.M{"role": models.UserRoleSUPER_ADMIN}).Decode(&existingAdmin)
		if err == nil {
			utils.Logger.Error().Msg("已存在超级管理员")
			utils.ErrorResponse(c, "已存在超级管理员", http.StatusBadRequest)
			return
		} else if err != mongo.ErrNoDocuments {
			utils.Logger.Error().Err(err).Msg("检查超级管理员时发生错误")
			utils.ErrorResponse(c, "创建用户失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// 创建新用户(直接批准)
	now := time.Now()
	newUser := models.User{
		Username:  req.Username,
		Password:  utils.HashPassword(req.Password),
		Phone:     req.Phone,
		Role:      req.Role,
		Status:    models.UserStatusAPPROVED,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 插入用户
	result, err := collection.InsertOne(repository.GetContext(), newUser)
	if err != nil {
		utils.Logger.Error().Err(err).Str("username", req.Username).Msg("插入用户失败")

		// 检查特殊错误 - 重复键
		if mongo.IsDuplicateKeyError(err) {
			utils.ErrorResponse(c, "用户名或其他唯一字段已存在", http.StatusBadRequest)
			return
		}

		utils.ErrorResponse(c, "创建用户失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 验证插入是否成功
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		utils.Logger.Error().Msg("获取插入ID失败")
		utils.ErrorResponse(c, "创建用户成功，但无法获取用户ID", http.StatusInternalServerError)
		return
	}

	newUser.ID = insertedID
	newUser.Password = "" // 不返回密码

	utils.Logger.Info().
		Str("id", insertedID.Hex()).
		Str("username", newUser.Username).
		Msg("用户创建成功")

	utils.SuccessResponse(c, gin.H{"user": newUser}, "创建用户成功", http.StatusCreated)
}

// UpdateUser 更新用户
func UpdateUser(c *gin.Context) {
	// 获取用户ID
	userID := c.Param("id")

	// 验证ID格式
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		utils.Logger.Error().Err(err).Str("id", userID).Msg("无效的ID格式")
		utils.ErrorResponse(c, "无效的ID格式", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.Logger.Info().
		Str("id", userID).
		Interface("updateData", req).
		Msg("处理更新用户请求")

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	collection := repository.Collection(repository.UsersCollection)

	// 检查用户是否存在
	var existingUser models.User
	err = collection.FindOne(repository.GetContext(), bson.M{"_id": objectID}).Decode(&existingUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.Logger.Error().Str("id", userID).Msg("用户不存在")
			utils.ErrorResponse(c, "用户不存在", http.StatusNotFound)
		} else {
			utils.Logger.Error().Err(err).Str("id", userID).Msg("查询用户失败")
			utils.ErrorResponse(c, "更新用户失败: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// 检查是否是超级管理员修改自己
	isSelfUpdate := user.ID == userID

	// 如果是修改自己，不能修改自己的角色
	if isSelfUpdate && req.Role != "" && req.Role != existingUser.Role {
		utils.Logger.Error().Str("id", userID).Msg("不能修改自己的角色")
		utils.ErrorResponse(c, "不能修改自己的角色", http.StatusBadRequest)
		return
	}

	// 如果更改为超级管理员，检查是否已存在
	if req.Role == models.UserRoleSUPER_ADMIN && existingUser.Role != models.UserRoleSUPER_ADMIN {
		var existingAdmin models.User
		err := collection.FindOne(
			repository.GetContext(),
			bson.M{
				"role": models.UserRoleSUPER_ADMIN,
				"_id":  bson.M{"$ne": objectID},
			},
		).Decode(&existingAdmin)

		if err == nil {
			utils.Logger.Error().Msg("已存在超级管理员")
			utils.ErrorResponse(c, "已存在超级管理员", http.StatusBadRequest)
			return
		} else if err != mongo.ErrNoDocuments {
			utils.Logger.Error().Err(err).Msg("检查超级管理员时发生错误")
			utils.ErrorResponse(c, "更新用户失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// 如果修改了用户名，检查是否已存在
	if req.Username != "" && req.Username != existingUser.Username {
		var existingUsername models.User
		err := collection.FindOne(
			repository.GetContext(),
			bson.M{
				"username": req.Username,
				"_id":      bson.M{"$ne": objectID},
			},
		).Decode(&existingUsername)

		if err == nil {
			utils.Logger.Error().Str("username", req.Username).Msg("用户名已存在")
			utils.ErrorResponse(c, "用户名已存在", http.StatusBadRequest)
			return
		} else if err != mongo.ErrNoDocuments {
			utils.Logger.Error().Err(err).Str("username", req.Username).Msg("检查用户名时发生错误")
			utils.ErrorResponse(c, "更新用户失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// 构建更新数据
	updateData := bson.M{"updatedAt": time.Now()}

	if req.Username != "" {
		updateData["username"] = req.Username
	}

	if req.Phone != "" {
		updateData["phone"] = req.Phone
	}

	if req.Role != "" {
		updateData["role"] = req.Role
	}

	if req.Password != "" {
		updateData["password"] = utils.HashPassword(req.Password)
	}

	// 更新用户
	result, err := collection.UpdateOne(
		repository.GetContext(),
		bson.M{"_id": objectID},
		bson.M{"$set": updateData},
	)

	if err != nil {
		utils.Logger.Error().Err(err).Str("id", userID).Msg("更新用户失败")
		utils.ErrorResponse(c, "更新用户失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result.ModifiedCount == 0 {
		utils.Logger.Info().Str("id", userID).Msg("用户数据未变更")
		utils.SuccessResponse(c, nil, "用户数据未变更")
		return
	}

	utils.Logger.Info().Str("id", userID).Msg("更新用户成功")
	utils.SuccessResponse(c, nil, "更新用户成功")
}

// DeleteUser 删除用户
func DeleteUser(c *gin.Context) {
	// 获取用户ID
	userID := c.Param("id")

	// 验证ID格式
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		utils.Logger.Error().Err(err).Str("id", userID).Msg("无效的ID格式")
		utils.ErrorResponse(c, "无效的ID格式", http.StatusBadRequest)
		return
	}

	utils.Logger.Info().Str("id", userID).Msg("处理删除用户请求")

	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 检查是否在删除自己
	if user.ID == userID {
		utils.Logger.Error().Str("id", userID).Msg("不能删除当前登录账户")
		utils.ErrorResponse(c, "不能删除当前登录账户", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.UsersCollection)

	// 检查用户是否存在
	var userToDelete models.User
	err = collection.FindOne(repository.GetContext(), bson.M{"_id": objectID}).Decode(&userToDelete)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.Logger.Error().Str("id", userID).Msg("用户不存在")
			utils.ErrorResponse(c, "用户不存在", http.StatusNotFound)
		} else {
			utils.Logger.Error().Err(err).Str("id", userID).Msg("查询用户失败")
			utils.ErrorResponse(c, "删除用户失败: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// 检查是否删除的是超级管理员
	if userToDelete.Role == models.UserRoleSUPER_ADMIN {
		utils.Logger.Error().Str("id", userID).Msg("不能删除超级管理员账户")
		utils.ErrorResponse(c, "不能删除超级管理员账户", http.StatusBadRequest)
		return
	}

	// 删除用户
	result, err := collection.DeleteOne(repository.GetContext(), bson.M{"_id": objectID})
	if err != nil {
		utils.Logger.Error().Err(err).Str("id", userID).Msg("删除用户失败")
		utils.ErrorResponse(c, "删除用户失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		utils.Logger.Error().Str("id", userID).Msg("用户不存在")
		utils.ErrorResponse(c, "用户不存在", http.StatusNotFound)
		return
	}

	utils.Logger.Info().Str("id", userID).Msg("删除用户成功")
	utils.SuccessResponse(c, nil, "删除用户成功")
}
