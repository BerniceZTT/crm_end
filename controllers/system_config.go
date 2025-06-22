package controllers

import (
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

// GetAllConfigs 获取所有配置列表
// GET /api/system-configs
func GetAllConfigs(c *gin.Context) {
	utils.Logger.Info().Msg("[配置管理] 获取配置列表")

	// 获取查询参数
	configType := c.Query("configType")
	isEnabled := c.Query("isEnabled")

	// 构建查询条件
	query := bson.M{}
	if configType != "" {
		query["configType"] = configType
	}
	if isEnabled != "" {
		query["isEnabled"] = isEnabled == "true"
	}

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 查询配置列表
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := collection.Find(ctx, query, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Msg("[配置管理] 获取配置列表失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置列表失败"})
		return
	}
	defer cursor.Close(ctx)

	// 解析结果
	var configs []models.SystemConfig
	if err := cursor.All(ctx, &configs); err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Msg("[配置管理] 解析配置列表失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析配置列表失败"})
		return
	}

	utils.Logger.Info().Int("count", len(configs)).Msg("[配置管理] 查询到配置记录")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"configs": configs,
			"total":   len(configs),
		},
	})
}

// GetConfigsByType 根据配置类型获取配置
// GET /api/system-configs/type/:configType
func GetConfigsByType(c *gin.Context) {
	configType := c.Param("configType")
	utils.Logger.Info().Str("configType", configType).Msg("[配置管理] 获取配置类型")

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 查询配置列表
	query := bson.M{
		"configType": configType,
		"isEnabled":  true,
	}
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := collection.Find(ctx, query, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Str("configType", configType).Msg("[配置管理] 获取指定类型配置失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败"})
		return
	}
	defer cursor.Close(ctx)

	// 解析结果
	var configs []models.SystemConfig
	if err := cursor.All(ctx, &configs); err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Str("configType", configType).Msg("[配置管理] 解析指定类型配置失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"configs":    configs,
			"configType": configType,
		},
	})
}

// GetConfigDetail 获取配置详情
// GET /api/system-configs/:id
func GetConfigDetail(c *gin.Context) {
	configID := c.Param("id")
	utils.Logger.Info().Str("configId", configID).Msg("[配置管理] 获取配置详情")

	// 转换ID为ObjectID
	objID, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置ID"})
		return
	}

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 查询配置详情
	var config models.SystemConfig
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&config)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.Logger.Warn().Str("configId", configID).Msg("[配置管理] 配置不存在")
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		} else {
			utils.HandleError(c, err)
			utils.Logger.Error().Err(err).Str("configId", configID).Msg("[配置管理] 获取配置详情失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置详情失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// CreateConfig 创建新配置
// POST /api/system-configs
func CreateConfig(c *gin.Context) {
	// 获取当前用户
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 解析请求数据
	var requestData models.CreateConfigRequest
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式不正确"})
		return
	}

	utils.Logger.Info().Interface("requestData", requestData).Msg("[配置管理] 创建新配置")

	// 验证必填字段
	if requestData.ConfigType == "" || requestData.ConfigKey == "" || requestData.ConfigValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置类型、配置键和配置值为必填项"})
		return
	}

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 检查配置键是否已存在
	existingConfig := collection.FindOne(ctx, bson.M{
		"configType": requestData.ConfigType,
		"configKey":  requestData.ConfigKey,
	})

	if existingConfig.Err() == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该配置键已存在，请使用不同的配置键"})
		return
	} else if existingConfig.Err() != mongo.ErrNoDocuments {
		utils.HandleError(c, existingConfig.Err())
		utils.Logger.Error().Err(existingConfig.Err()).Msg("[配置管理] 检查配置键是否存在失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败"})
		return
	}

	// 构建配置对象
	configData := models.SystemConfig{
		ConfigType:  requestData.ConfigType,
		ConfigKey:   requestData.ConfigKey,
		ConfigValue: requestData.ConfigValue,
		Description: requestData.Description,
		IsEnabled:   utils.BoolPtr(requestData.IsEnabled, true),
		CreatorID:   user.ID,
		CreatorName: user.Username,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 插入配置
	result, err := collection.InsertOne(ctx, configData)
	if err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Msg("[配置管理] 创建配置失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败"})
		return
	}

	utils.Logger.Info().Str("configId", result.InsertedID.(primitive.ObjectID).Hex()).Msg("[配置管理] 配置创建成功")

	// 返回响应
	configData.ID = result.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "配置创建成功",
		"data":    configData,
	})
}

// UpdateConfig 更新配置
// PUT /api/system-configs/:id
func UpdateConfig(c *gin.Context) {
	// 获取当前用户
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 获取配置ID
	configID := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置ID"})
		return
	}

	utils.Logger.Info().Str("configId", configID).Msg("[配置管理] 更新配置")

	// 解析请求数据
	var requestData models.UpdateConfigRequest
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式不正确"})
		return
	}

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 检查配置是否存在
	var existingConfig models.SystemConfig
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existingConfig)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		} else {
			utils.HandleError(c, err)
			utils.Logger.Error().Err(err).Str("configId", configID).Msg("[配置管理] 检查配置是否存在失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		}
		return
	}

	// 构建更新数据
	updateData := bson.M{
		"updatedAt":   time.Now(),
		"updaterId":   user.ID,
		"updaterName": user.Username,
	}

	if requestData.ConfigValue != "" {
		updateData["configValue"] = requestData.ConfigValue
	}
	if requestData.Description != "" {
		updateData["description"] = requestData.Description
	}
	if requestData.IsEnabled != nil {
		updateData["isEnabled"] = *requestData.IsEnabled
	}

	// 执行更新
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": updateData},
	)
	if err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Str("configId", configID).Msg("[配置管理] 更新配置失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	utils.Logger.Info().Str("configId", configID).Msg("[配置管理] 配置更新成功")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置更新成功",
	})
}

// DeleteConfig 删除配置
// DELETE /api/system-configs/:id
func DeleteConfig(c *gin.Context) {
	// 获取配置ID
	configID := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置ID"})
		return
	}

	utils.Logger.Info().Str("configId", configID).Msg("[配置管理] 删除配置")

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 执行删除
	result, err := collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Str("configId", configID).Msg("[配置管理] 删除配置失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	utils.Logger.Info().Str("configId", configID).Msg("[配置管理] 配置删除成功")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置删除成功",
	})
}

// ToggleConfigStatus 切换配置启用状态
// PATCH /api/system-configs/:id/toggle
func ToggleConfigStatus(c *gin.Context) {
	// 获取当前用户
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 获取配置ID
	configID := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置ID"})
		return
	}

	utils.Logger.Info().Str("configId", configID).Msg("[配置管理] 切换配置状态")

	// 获取集合
	collection := repository.Collection(repository.SystemConfigsCollection)
	ctx := repository.GetContext()

	// 获取当前配置
	var existingConfig models.SystemConfig
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existingConfig)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		} else {
			utils.HandleError(c, err)
			utils.Logger.Error().Err(err).Str("configId", configID).Msg("[配置管理] 获取配置详情失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "切换配置状态失败"})
		}
		return
	}

	// 切换启用状态
	newStatus := !existingConfig.IsEnabled

	// 执行更新
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{
			"$set": bson.M{
				"isEnabled":   newStatus,
				"updatedAt":   time.Now(),
				"updaterId":   user.ID,
				"updaterName": user.Username,
			},
		},
	)
	if err != nil {
		utils.HandleError(c, err)
		utils.Logger.Error().Err(err).Str("configId", configID).Msg("[配置管理] 切换配置状态失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "切换配置状态失败"})
		return
	}

	statusText := "启用"
	if !newStatus {
		statusText = "禁用"
	}

	utils.Logger.Info().
		Str("configId", configID).
		Bool("newStatus", newStatus).
		Msg("[配置管理] 配置状态切换成功")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置已" + statusText,
		"data": gin.H{
			"isEnabled": newStatus,
		},
	})
}
