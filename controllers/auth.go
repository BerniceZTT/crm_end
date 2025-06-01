package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Login 用户登录
func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogApiRequest("POST", "/api/auth/login", nil, gin.H{
		"username": req.Username,
		"password": "******",
		"isAgent":  req.IsAgent,
	}, nil)

	utils.Logger.Info().
		Str("username", req.Username).
		Bool("isAgent", req.IsAgent).
		Msg("登录尝试")

	var user *models.User
	var agent *models.Agent

	// 先查询用户表
	usersCollection := repository.Collection(repository.UsersCollection)
	var userResult models.User
	err := usersCollection.FindOne(repository.GetContext(), bson.M{"username": req.Username}).Decode(&userResult)
	if err == nil {
		user = &userResult
	} else if err != mongo.ErrNoDocuments {
		utils.Logger.Error().Err(err).Msg("查询用户出错")
		utils.ErrorResponse(c, "登录失败: 数据库错误", http.StatusInternalServerError)
		return
	}

	// 如果没找到，也尝试查询代理商表
	if user == nil {
		agentsCollection := repository.Collection(repository.AgentsCollection)
		var agentResult models.Agent
		err := agentsCollection.FindOne(repository.GetContext(), bson.M{"companyName": req.Username}).Decode(&agentResult)
		if err == nil {
			agent = &agentResult
		} else if err != mongo.ErrNoDocuments {
			utils.Logger.Error().Err(err).Msg("查询代理商出错")
			utils.ErrorResponse(c, "登录失败: 数据库错误", http.StatusInternalServerError)
			return
		}
	}

	// 处理未找到用户或代理商的情况
	if user == nil && agent == nil {
		utils.Logger.Info().Str("username", req.Username).Msg("登录失败: 用户名不存在")
		utils.ErrorResponse(c, "用户名不存在，请检查用户名或注册新账号", http.StatusUnauthorized)
		return
	}

	// 处理代理商登录
	if agent != nil {
		// 检查代理商状态
		if agent.Status == models.UserStatusPENDING {
			utils.Logger.Info().Str("username", req.Username).Msg("代理商登录失败: 账户待审核")
			utils.ErrorResponse(c, "账户正在审核中，请等待审核通过", http.StatusForbidden)
			return
		} else if agent.Status == models.UserStatusREJECTED {
			utils.Logger.Info().Str("username", req.Username).Msg("代理商登录失败: 账户已被拒绝")
			utils.ErrorResponse(c, "账户已被拒绝", http.StatusForbidden)
			return
		} else if agent.Status != models.UserStatusAPPROVED {
			utils.ErrorResponse(c, "账户状态异常", http.StatusForbidden)
			return
		}

		// 验证密码
		if !utils.VerifyPassword(req.Password, agent.Password) {
			utils.Logger.Info().Str("username", req.Username).Msg("代理商登录失败: 密码错误")
			utils.ErrorResponse(c, "用户名或密码错误", http.StatusUnauthorized)
			return
		}

		// 生成JWT令牌
		token, err := utils.GenerateToken(*agent)
		if err != nil {
			utils.Logger.Error().Err(err).Msg("生成代理商token失败")
			utils.ErrorResponse(c, "生成登录令牌失败，请重试", http.StatusInternalServerError)
			return
		}

		// 构造代理商用户对象(用于前端统一处理)
		agentUser := gin.H{
			"_id":              agent.ID.Hex(),
			"username":         agent.CompanyName,
			"role":             models.UserRoleAGENT,
			"contactPerson":    agent.ContactPerson,
			"phone":            agent.Phone,
			"relatedSalesId":   agent.RelatedSalesID,
			"relatedSalesName": agent.RelatedSalesName,
		}

		utils.Logger.Info().Str("username", agent.CompanyName).Msg("代理商登录成功")
		utils.SuccessResponse(c, gin.H{
			"token": token,
			"user":  agentUser,
		}, "")
		return
	}

	// 处理用户登录
	if user != nil {
		// 检查用户状态
		if user.Status == models.UserStatusPENDING {
			utils.Logger.Info().Str("username", req.Username).Msg("登录失败: 用户账户待审核")
			utils.ErrorResponse(c, "账户正在审核中，请等待审核通过", http.StatusForbidden)
			return
		} else if user.Status == models.UserStatusREJECTED {
			reason := "未提供"
			if user.RejectionReason != "" {
				reason = user.RejectionReason
			}
			utils.Logger.Info().Str("username", req.Username).Msg("登录失败: 用户账户已被拒绝")
			utils.ErrorResponse(c, "账户已被拒绝，原因: "+reason, http.StatusForbidden)
			return
		} else if user.Status != models.UserStatusAPPROVED {
			utils.ErrorResponse(c, "账户状态异常", http.StatusForbidden)
			return
		}

		// 验证密码
		if !utils.VerifyPassword(req.Password, user.Password) {
			utils.Logger.Info().Str("username", req.Username).Msg("登录失败: 密码错误")
			utils.ErrorResponse(c, "用户名或密码错误", http.StatusUnauthorized)
			return
		}

		// 生成JWT令牌
		token, err := utils.GenerateToken(*user)
		if err != nil {
			utils.Logger.Error().Err(err).Msg("生成token失败")
			utils.ErrorResponse(c, "生成登录令牌失败，请重试", http.StatusInternalServerError)
			return
		}

		// 构造用户对象(不包含密码)
		userWithoutPassword := *user
		userWithoutPassword.Password = ""

		utils.Logger.Info().Str("username", user.Username).Msg("用户登录成功")
		utils.SuccessResponse(c, gin.H{
			"token": token,
			"user":  userWithoutPassword,
		}, "")
		return
	}

	// 如果执行到这里，说明有逻辑漏洞
	utils.Logger.Error().Str("username", req.Username).Msg("登录处理逻辑异常: 用户/代理商检查完毕后无法确定身份")
	utils.ErrorResponse(c, "登录处理失败，请重试", http.StatusInternalServerError)
}

// AgentLogin 代理商登录 - 兼容旧接口
func AgentLogin(c *gin.Context) {
	var req struct {
		CompanyName string `json:"companyName" binding:"required"`
		Password    string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogApiRequest("POST", "/api/auth/agent/login", nil, gin.H{
		"companyName": req.CompanyName,
		"password":    "******",
	}, nil)

	utils.Logger.Info().Str("companyName", req.CompanyName).Msg("代理商登录接口调用，重定向至统一登录")

	// 构造统一登录格式请求
	loginReq := models.LoginRequest{
		Username: req.CompanyName,
		Password: req.Password,
		IsAgent:  true,
	}

	// 创建新的上下文
	c.Set("originalRequest", req)

	// 调用统一登录函数
	c.Set("loginRequest", loginReq)
	Login(c)
}

// Register 用户注册
func Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogApiRequest("POST", "/api/auth/register", nil, gin.H{
		"username": req.Username,
		"password": "******",
		"phone":    req.Phone,
		"role":     req.Role,
	}, nil)

	utils.Logger.Info().
		Str("username", req.Username).
		Str("role", string(req.Role)).
		Msg("用户注册请求")

	// 检查用户名是否已存在
	collection := repository.Collection(repository.UsersCollection)
	var existingUser models.User
	err := collection.FindOne(repository.GetContext(), bson.M{"username": req.Username}).Decode(&existingUser)
	if err == nil {
		utils.Logger.Info().Str("username", req.Username).Msg("注册失败: 用户名已存在")
		utils.ErrorResponse(c, "用户名已存在", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		utils.Logger.Error().Err(err).Msg("检查用户名是否存在时出错")
		utils.ErrorResponse(c, "注册失败: 数据库错误", http.StatusInternalServerError)
		return
	}

	// 创建新用户(待审批状态)
	now := time.Now()
	newUser := models.User{
		Username:  req.Username,
		Password:  utils.HashPassword(req.Password),
		Phone:     req.Phone,
		Role:      req.Role,
		Status:    models.UserStatusPENDING,
		CreatedAt: now,
		UpdatedAt: now,
	}

	utils.Logger.Info().
		Str("username", newUser.Username).
		Str("role", string(newUser.Role)).
		Msg("准备创建新用户")

	// 插入用户
	result, err := collection.InsertOne(repository.GetContext(), newUser)
	if err != nil {
		utils.Logger.Error().Err(err).Msg("创建用户失败")

		// 检查特殊错误 - 重复键
		if mongo.IsDuplicateKeyError(err) {
			utils.ErrorResponse(c, "用户名或其他唯一字段已存在", http.StatusConflict)
			return
		}

		utils.ErrorResponse(c, "注册失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 验证是否成功插入
	utils.Logger.Info().
		Interface("insertedId", result.InsertedID).
		Msg("用户创建结果")

	// 额外验证：检查用户是否真的被插入
	var verifyUser models.User
	err = collection.FindOne(repository.GetContext(), bson.M{"username": req.Username}).Decode(&verifyUser)
	if err != nil {
		utils.Logger.Error().Err(err).Msg("用户创建验证失败")
		utils.ErrorResponse(c, "注册失败: 无法验证用户是否已创建", http.StatusInternalServerError)
		return
	}

	utils.Logger.Info().
		Str("username", verifyUser.Username).
		Str("id", verifyUser.ID.Hex()).
		Msg("验证成功: 用户已成功插入数据库")

	utils.SuccessResponse(c, nil, "注册申请已提交，请等待管理员审批", http.StatusCreated)
}

// AgentRegister 代理商注册
func AgentRegister(c *gin.Context) {
	var req models.AgentRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, "无效的请求参数: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogApiRequest("POST", "/api/auth/agent/register", nil, gin.H{
		"companyName":   req.CompanyName,
		"contactPerson": req.ContactPerson,
		"password":      "******",
		"phone":         req.Phone,
	}, nil)

	utils.Logger.Info().
		Str("companyName", req.CompanyName).
		Str("contactPerson", req.ContactPerson).
		Msg("代理商注册请求")

	// 检查公司名是否已存在
	collection := repository.Collection(repository.AgentsCollection)
	var existingAgent models.Agent
	err := collection.FindOne(repository.GetContext(), bson.M{"companyName": req.CompanyName}).Decode(&existingAgent)
	if err == nil {
		utils.Logger.Info().Str("companyName", req.CompanyName).Msg("注册失败: 代理商公司名已存在")
		utils.ErrorResponse(c, "代理商公司名已存在", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		utils.Logger.Error().Err(err).Msg("检查代理商公司名是否存在时出错")
		utils.ErrorResponse(c, "注册失败: 数据库错误", http.StatusInternalServerError)
		return
	}

	// 创建新代理商(待审批状态)
	now := time.Now()
	newAgent := models.Agent{
		CompanyName:   req.CompanyName,
		ContactPerson: req.ContactPerson,
		Password:      utils.HashPassword(req.Password),
		Phone:         req.Phone,
		Status:        models.UserStatusPENDING,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	utils.Logger.Info().
		Str("companyName", newAgent.CompanyName).
		Str("contactPerson", newAgent.ContactPerson).
		Msg("准备创建新代理商")

	// 插入代理商
	result, err := collection.InsertOne(repository.GetContext(), newAgent)
	if err != nil {
		utils.Logger.Error().Err(err).Msg("创建代理商失败")

		// 检查特殊错误 - 重复键
		if mongo.IsDuplicateKeyError(err) {
			utils.ErrorResponse(c, "代理商公司名或其他唯一字段已存在", http.StatusConflict)
			return
		}

		utils.ErrorResponse(c, "注册失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Logger.Info().
		Interface("insertedId", result.InsertedID).
		Msg("代理商创建结果")

	utils.SuccessResponse(c, nil, "代理商注册申请已提交，请等待审批", http.StatusCreated)
}

// ValidateToken 验证Token
func ValidateToken(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 检查是否有必要的字段
	if user.ID == "" || user.Username == "" || user.Role == "" {
		userBody, _ := json.Marshal(user)
		utils.Logger.Info().Str("user", string(userBody)).Msg("Token验证失败: 用户信息不完整")
		utils.ErrorResponse(c, "无效的token: 用户信息不完整", http.StatusUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(user.ID)
	if err != nil {
		utils.Logger.Error().Err(err).Str("id", user.ID).Msg("无效的ID格式")
		utils.ErrorResponse(c, "无效的ID格式", http.StatusBadRequest)
		return
	}

	if user.Role != string(models.UserRoleAGENT) {
		// 检查账户是否存在
		var modelsUser models.User
		collection := repository.Collection(repository.UsersCollection)
		err = collection.FindOne(repository.GetContext(), bson.M{"_id": userID}).Decode(&modelsUser)
		if err != nil {
			utils.ErrorResponse(c, "查询用户失败", http.StatusBadRequest)
			return
		}
		utils.SuccessResponse(c, gin.H{"user": map[string]interface{}{
			"_id":              modelsUser.ID.Hex(),
			"id":               modelsUser.ID.Hex(),
			"companyName":      "",
			"username":         modelsUser.Username,
			"role":             user.Role,
			"status":           modelsUser.Status,
			"rejectionReason":  modelsUser.RejectionReason,
			"relatedSalesId":   modelsUser.RelatedSalesID,
			"relatedSalesName": modelsUser.RelatedSalesName,
		}}, "")
	} else {
		// 检查代理商是否存在
		var modelsAgent models.Agent
		collection := repository.Collection(repository.AgentsCollection)
		err = collection.FindOne(repository.GetContext(), bson.M{"_id": userID}).Decode(&modelsAgent)
		if err != nil {
			utils.ErrorResponse(c, "查询代理商失败", http.StatusBadRequest)
			return
		}
		utils.SuccessResponse(c, gin.H{"user": map[string]interface{}{
			"_id":              modelsAgent.ID.Hex(),
			"id":               modelsAgent.ID.Hex(),
			"companyName":      modelsAgent.CompanyName,
			"username":         modelsAgent.CompanyName,
			"role":             user.Role,
			"status":           modelsAgent.Status,
			"rejectionReason":  "",
			"relatedSalesId":   modelsAgent.RelatedSalesID,
			"relatedSalesName": modelsAgent.RelatedSalesName,
		}}, "")
	}
}
