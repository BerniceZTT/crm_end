package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/service"
	"github.com/BerniceZTT/crm_end/utils"
)

// 全局变量
var (
	validate *validator.Validate
)

func init() {
	validate = validator.New()
}

// 1. 获取所有项目列表
func GetAllProjects(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username := currentUser.Username
	role := currentUser.Role

	log.Printf("[项目路由] 获取项目列表 - 用户: %s, 角色: %s", username, role)

	// 检查用户是否有权限访问项目管理
	if !(role == string(models.UserRoleSUPER_ADMIN) || role == string(models.UserRoleFACTORY_SALES) || role == string(models.UserRoleAGENT)) {
		log.Printf("[项目路由] 用户 %s 无权访问项目管理", username)
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问项目管理"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := repository.Collection(repository.ProjectsCollection)

	// 获取用户可访问的客户ID列表
	customerIds, err := getUserAccessibleCustomerIds(currentUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if role == string(models.UserRoleFACTORY_SALES) && len(customerIds) == 0 {
		c.JSON(http.StatusOK, models.ProjectListResponse{Projects: []models.ProjectResponse{}})
		return
	}

	// 构建基础查询条件
	filter := bson.M{"webHidden": false}
	if role != string(models.UserRoleSUPER_ADMIN) && len(customerIds) > 0 {
		filter["customerId"] = bson.M{"$in": customerIds}
	}

	// 第一次查询：获取基础项目列表
	var projects []models.Project
	opts := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetLimit(1000).
		SetProjection(bson.M{
			"smallBatchAttachments":     0,
			"massProductionAttachments": 0,
		})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取项目列表失败"})
		return
	}
	if err = cursor.All(ctx, &projects); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析项目列表失败"})
		return
	}

	// 提取所有关联的客户ID
	customerIDMap := make(map[primitive.ObjectID]bool)
	for _, p := range projects {
		customerIDMap[p.CustomerID] = true
	}

	// 批量查询客户信息
	customerInfos := make(map[primitive.ObjectID]models.Customer)
	if len(customerIDMap) > 0 {
		customerIDs := make([]primitive.ObjectID, 0, len(customerIDMap))
		for id := range customerIDMap {
			customerIDs = append(customerIDs, id)
		}

		customerFilter := bson.M{"_id": bson.M{"$in": customerIDs}}
		customerCursor, err := repository.Collection(repository.CustomersCollection).Find(ctx, customerFilter)
		if err == nil {
			var customers []models.Customer
			if err = customerCursor.All(ctx, &customers); err == nil {
				for _, cust := range customers {
					customerInfos[cust.ID] = cust
				}
			}
		}
	}

	// 构建响应数据
	normalizedProjects := make([]models.ProjectResponse, len(projects))
	for i, p := range projects {
		resp := models.ConvertProjectToResponse(p)

		// 添加客户关联信息
		if cust, ok := customerInfos[p.CustomerID]; ok {
			resp.RelatedAgentName = cust.RelatedAgentName
			resp.RelatedSalesName = cust.RelatedSalesName
		}

		normalizedProjects[i] = resp
	}

	c.JSON(http.StatusOK, models.ProjectListResponse{Projects: normalizedProjects})
}

// 2. 获取客户的项目列表
func GetCustomerProjects(c *gin.Context) {
	startTime := time.Now()
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username := currentUser.Username
	customerID := c.Param("customerId")

	log.Printf("[项目路由] 获取客户项目列表 - 客户ID: %s, 用户: %s", customerID, username)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 验证客户ID格式
	customerObjID, err := primitive.ObjectIDFromHex(customerID)
	if err != nil {
		log.Printf("无效的客户ID格式: %s", customerID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID格式"})
		return
	}

	// 查询客户信息
	customerCollection := repository.Collection(repository.CustomersCollection)
	var customer models.Customer
	err = customerCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询客户失败"})
		}
		return
	}

	// 权限检查
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		switch models.UserRole(currentUser.Role) {
		case models.UserRoleFACTORY_SALES:
			if customer.RelatedSalesID != currentUser.ID && !customer.IsInPublicPool {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该客户的项目"})
				return
			}
		case models.UserRoleAGENT:
			if customer.RelatedAgentID != currentUser.ID && !customer.IsInPublicPool {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该客户的项目"})
				return
			}
		}
	}

	// 查询项目列表
	projectCollection := repository.Collection(repository.ProjectsCollection)
	opts := options.Find().SetProjection(bson.M{
		"smallBatchAttachments":     0,
		"massProductionAttachments": 0,
	})

	cursor, err := projectCollection.Find(ctx, bson.M{"customerId": customerObjID, "webHidden": false}, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目失败"})
		return
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err = cursor.All(ctx, &projects); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析项目失败"})
		return
	}

	// 标准化项目数据
	normalizedProjects := make([]models.ProjectResponse, len(projects))
	for i, p := range projects {
		normalizedProjects[i] = models.ConvertProjectToResponse(p)
	}

	totalTime := time.Since(startTime).Milliseconds()
	log.Printf("[项目路由] 找到 %d 个项目，总耗时: %dms", len(projects), totalTime)

	c.JSON(http.StatusOK, models.ProjectListResponse{Projects: normalizedProjects})
}

// 3. 文件下载
func DownloadProjectFile(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username := currentUser.Username
	projectID := c.Param("projectId")
	fileID := c.Param("fileId")

	log.Printf("[项目路由] 文件下载请求 - 项目ID: %s, 文件ID: %s, 用户: %s", projectID, fileID, username)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 验证项目ID格式
	projectObjID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID格式"})
		return
	}

	// 查询项目
	projectCollection := repository.Collection(repository.ProjectsCollection)
	var project models.Project
	err = projectCollection.FindOne(ctx, bson.M{"_id": projectObjID}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目失败"})
		}
		return
	}

	// 查询关联客户
	customerCollection := repository.Collection(repository.CustomersCollection)
	customerObjID := project.CustomerID
	var customer models.Customer
	err = customerCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "关联客户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询客户失败"})
		}
		return
	}

	// 权限检查
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		switch models.UserRole(currentUser.Role) {
		case models.UserRoleFACTORY_SALES:
			if customer.RelatedSalesID != currentUser.ID && !customer.IsInPublicPool {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权下载该项目文件"})
				return
			}
		case models.UserRoleAGENT:
			if customer.RelatedAgentID != currentUser.ID && !customer.IsInPublicPool {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权下载该项目文件"})
				return
			}
		}
	}

	// 查找文件
	var targetFile *models.FileAttachment

	// 检查小批量附件
	for _, file := range project.SmallBatchAttachments {
		if file.ID == fileID {
			targetFile = &file
			break
		}
	}

	// 检查批量生产附件
	if targetFile == nil {
		for _, file := range project.MassProductionAttachments {
			if file.ID == fileID {
				targetFile = &file
				break
			}
		}
	}

	if targetFile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	log.Printf("文件下载成功: %s", targetFile.FileName)

	c.JSON(http.StatusOK, models.FileDownloadResponse{
		Success: true,
		File:    *targetFile,
		Message: "文件信息获取成功",
	})
}

// 4. 获取单个项目详情
func GetProjectDetail(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	projectID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 验证项目ID格式
	projectObjID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Printf("无效的项目ID格式: %s, 错误: %v", projectID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID格式"})
		return
	}

	// 查询项目详情
	projectCollection := repository.Collection(repository.ProjectsCollection)
	var project models.Project
	err = projectCollection.FindOne(ctx, bson.M{"_id": projectObjID}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("项目不存在: %s", projectID)
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		} else {
			log.Printf("查询项目失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目失败"})
		}
		return
	}

	// 查询关联客户
	customerCollection := repository.Collection(repository.CustomersCollection)
	customerObjID := project.CustomerID
	var customer models.Customer
	err = customerCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("项目关联客户不存在: %s", customerObjID.Hex())
			c.JSON(http.StatusNotFound, gin.H{"error": "关联客户不存在"})
		} else {
			log.Printf("查询客户失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询客户失败"})
		}
		return
	}

	// 权限验证
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		switch models.UserRole(currentUser.Role) {
		case models.UserRoleFACTORY_SALES:
			if customer.RelatedSalesID != currentUser.ID && !customer.IsInPublicPool {
				log.Printf("销售权限不足: 用户%s, 客户销售%s", currentUser.ID, customer.RelatedSalesID)
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目"})
				return
			}
		case models.UserRoleAGENT:
			if customer.RelatedAgentID != currentUser.ID && !customer.IsInPublicPool {
				log.Printf("代理权限不足: 用户%s, 客户代理%s", currentUser.ID, customer.RelatedAgentID)
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目"})
				return
			}
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目"})
			return
		}
	}

	c.JSON(http.StatusOK, models.ProjectDetailResponse{
		Success: true,
		Project: project,
	})
}

// 5. 创建项目
func CreateProject(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username := currentUser.Username

	var req struct {
		ProjectName               string                  `json:"projectName" binding:"required"`
		CustomerID                string                  `json:"customerId" binding:"required"`
		ProductID                 string                  `json:"productId" binding:"required"`
		BatchNumber               string                  `json:"batchNumber" binding:"required"`
		ProjectProgress           string                  `json:"projectProgress" binding:"required"`
		Remark                    string                  `json:"remark,omitempty"`
		SmallBatchPrice           float64                 `json:"smallBatchPrice,omitempty"`
		SmallBatchQuantity        int                     `json:"smallBatchQuantity,omitempty"`
		SmallBatchTotal           float64                 `json:"smallBatchTotal,omitempty"`
		SmallBatchAttachments     []models.FileAttachment `json:"smallBatchAttachments,omitempty"`
		MassProductionPrice       float64                 `json:"massProductionPrice,omitempty"`
		MassProductionQuantity    int                     `json:"massProductionQuantity,omitempty"`
		MassProductionTotal       float64                 `json:"massProductionTotal,omitempty"`
		PaymentTerm               string                  `json:"paymentTerm,omitempty"`
		MassProductionAttachments []models.FileAttachment `json:"massProductionAttachments,omitempty"`
		StartDate                 time.Time               `json:"startDate" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 转换客户ID
	customerObjID, err := primitive.ObjectIDFromHex(req.CustomerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的客户ID格式"})
		return
	}

	// 查询客户
	customerCollection := repository.Collection(repository.CustomersCollection)
	var customer models.Customer
	err = customerCollection.FindOne(ctx, bson.M{"_id": customerObjID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "客户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询客户失败"})
		}
		return
	}

	// 权限检查
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		switch models.UserRole(currentUser.Role) {
		case models.UserRoleFACTORY_SALES:
			if customer.RelatedSalesID != currentUser.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权为该客户创建项目"})
				return
			}
		case models.UserRoleAGENT:
			if customer.RelatedAgentID != currentUser.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权为该客户创建项目"})
				return
			}
		}
	}

	// 转换产品ID
	productObjID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的产品ID格式"})
		return
	}

	// 查询产品
	productCollection := repository.Collection(repository.ProductsCollection)
	var product models.Product
	err = productCollection.FindOne(ctx, bson.M{"_id": productObjID}).Decode(&product)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "产品不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询产品失败"})
		}
		return
	}

	// 构建项目
	now := time.Now()
	userObjID, _ := primitive.ObjectIDFromHex(currentUser.ID)

	newProject := models.Project{
		ProjectName:     req.ProjectName,
		CustomerID:      customerObjID,
		CustomerName:    customer.Name,
		CreatorID:       userObjID,
		CreatorName:     username,
		UpdaterID:       userObjID,
		UpdaterName:     username,
		ProductID:       productObjID,
		ProductName:     fmt.Sprintf("%s - %s", product.ModelName, product.PackageType),
		BatchNumber:     req.BatchNumber,
		ProjectProgress: models.ProjectProgress(req.ProjectProgress),
		Remark:          req.Remark,
		CreatedAt:       now,
		UpdatedAt:       now,
		StartDate:       req.StartDate,

		SmallBatchPrice:       req.SmallBatchPrice,
		SmallBatchQuantity:    req.SmallBatchQuantity,
		SmallBatchTotal:       req.SmallBatchTotal,
		SmallBatchAttachments: req.SmallBatchAttachments,

		MassProductionPrice:       req.MassProductionPrice,
		MassProductionQuantity:    req.MassProductionQuantity,
		MassProductionTotal:       req.MassProductionTotal,
		MassProductionAttachments: req.MassProductionAttachments,

		WebHidden: false,
	}

	// 插入项目
	projectCollection := repository.Collection(repository.ProjectsCollection)
	result, err := projectCollection.InsertOne(ctx, newProject)
	if err != nil {
		log.Printf("创建项目失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建项目失败: %v", err)})
		return
	}

	insertedID := result.InsertedID.(primitive.ObjectID)
	log.Printf("项目创建成功, ID: %s", insertedID.Hex())

	// 记录项目进展历史
	input := models.ProjectProgressHistory{
		ProjectID:    insertedID.Hex(),
		ProjectName:  req.ProjectName,
		FromProgress: "无",
		ToProgress:   string(newProject.ProjectProgress),
		OperatorID:   currentUser.ID,
		OperatorName: currentUser.Username,
		Remark:       "新建项目",
	}
	_, err = AddProjectProgressHistoryFn(context.Background(), input)
	if err != nil {
		log.Printf("项目创建成功, AddProjectProgressHistoryFn err: %s", err.Error())
	}

	// 修改客户状态
	err1 := service.UpdateCustomerProgress(ctx, customerObjID, models.CustomerProgressNormal)
	if err1 != nil {
		log.Printf("项目创建成功, UpdateCustomerProgress customerObjID: %v, err: %s", customerObjID, err.Error())
	}
	err2 := service.UpdateCustomerProgressByName(ctx, customer.Name, models.CustomerProgressDisabled)
	if err2 != nil {
		utils.HandleError(c, err2)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "客户为正常推进状态，修改其他同名客户信息报错"})
	}
	// 更改其他客户状态
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "项目创建成功",
		"project": gin.H{
			"_id": insertedID.Hex(),
		},
	})
}

// 6. 更新项目
func UpdateProject(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username := currentUser.Username
	projectID := c.Param("id")

	var req struct {
		ProjectName               string                  `json:"projectName,omitempty"`
		ProductID                 string                  `json:"productId,omitempty"`
		BatchNumber               string                  `json:"batchNumber,omitempty"`
		ProjectProgress           models.ProjectProgress  `json:"projectProgress,omitempty"`
		SmallBatchPrice           *float64                `json:"smallBatchPrice,omitempty"`
		SmallBatchQuantity        *int                    `json:"smallBatchQuantity,omitempty"`
		SmallBatchAttachments     []models.FileAttachment `json:"smallBatchAttachments,omitempty"`
		MassProductionPrice       *float64                `json:"massProductionPrice,omitempty"`
		MassProductionQuantity    *int                    `json:"massProductionQuantity,omitempty"`
		PaymentTerm               string                  `json:"paymentTerm,omitempty"`
		MassProductionAttachments []models.FileAttachment `json:"massProductionAttachments,omitempty"`
		Remark                    string                  `json:"remark,omitempty"`
		StartDate                 time.Time               `json:"startDate,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 验证项目ID格式
	projectObjID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Printf("无效的项目ID格式: %s", projectID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID格式"})
		return
	}

	// 查询现有项目
	projectCollection := repository.Collection(repository.ProjectsCollection)
	var existingProject models.Project
	err = projectCollection.FindOne(ctx, bson.M{"_id": projectObjID}).Decode(&existingProject)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目失败"})
		}
		return
	}

	// 查询关联客户
	customerCollection := repository.Collection(repository.CustomersCollection)
	var customer models.Customer
	err = customerCollection.FindOne(ctx, bson.M{"_id": existingProject.CustomerID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "关联客户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询客户失败"})
		}
		return
	}

	// 权限检查
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		switch models.UserRole(currentUser.Role) {
		case models.UserRoleFACTORY_SALES:
			if customer.RelatedSalesID != currentUser.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权更新该项目"})
				return
			}
		case models.UserRoleAGENT:
			if customer.RelatedAgentID != currentUser.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权更新该项目"})
				return
			}
		}
	}

	// 构建更新数据
	update := bson.M{
		"updaterId":   currentUser.ID,
		"updaterName": username,
		"updatedAt":   time.Now(),
		"startDate":   req.StartDate,
	}

	if req.ProjectName != "" {
		update["projectName"] = req.ProjectName
	}
	if req.BatchNumber != "" {
		update["batchNumber"] = req.BatchNumber
	}
	if req.ProjectProgress != "" {
		update["projectProgress"] = req.ProjectProgress
	}
	if req.Remark != "" {
		update["remark"] = req.Remark
	}
	if req.PaymentTerm != "" {
		update["paymentTerm"] = req.PaymentTerm
	}

	// 处理产品更新
	if req.ProductID != "" {
		productObjID, err := primitive.ObjectIDFromHex(req.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的产品ID格式"})
			return
		}

		// 查询新产品
		productCollection := repository.Collection(repository.ProductsCollection)
		var product models.Product
		err = productCollection.FindOne(ctx, bson.M{"_id": productObjID}).Decode(&product)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, gin.H{"error": "产品不存在"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "查询产品失败"})
			}
			return
		}

		update["productId"] = productObjID
		update["productName"] = fmt.Sprintf("%s - %s", product.ModelName, product.PackageType)
	}

	// 处理小批量附件
	if req.SmallBatchAttachments != nil {
		log.Printf("更新小批量附件: %d 个文件", len(req.SmallBatchAttachments))
		update["smallBatchAttachments"] = req.SmallBatchAttachments
	}

	// 处理批量生产附件
	if req.MassProductionAttachments != nil {
		log.Printf("更新批量出货附件: %d 个文件", len(req.MassProductionAttachments))
		update["massProductionAttachments"] = req.MassProductionAttachments
	}

	// 计算总额
	if req.SmallBatchPrice != nil || req.SmallBatchQuantity != nil {
		price := existingProject.SmallBatchPrice
		if req.SmallBatchPrice != nil {
			price = *req.SmallBatchPrice
		}

		quantity := existingProject.SmallBatchQuantity
		if req.SmallBatchQuantity != nil {
			quantity = *req.SmallBatchQuantity
		}

		update["smallBatchTotal"] = (price * float64(quantity)) / 10000
	}

	if req.MassProductionPrice != nil || req.MassProductionQuantity != nil {
		price := existingProject.MassProductionPrice
		if req.MassProductionPrice != nil {
			price = *req.MassProductionPrice
		}

		quantity := existingProject.MassProductionQuantity
		if req.MassProductionQuantity != nil {
			quantity = *req.MassProductionQuantity
		}

		update["massProductionTotal"] = (price * float64(quantity)) / 10000
	}

	// 检查项目进展是否变化
	progressChanged := req.ProjectProgress != "" && req.ProjectProgress != existingProject.ProjectProgress

	// 执行更新
	result, err := projectCollection.UpdateOne(
		ctx,
		bson.M{"_id": projectObjID},
		bson.M{"$set": update},
	)
	if err != nil {
		log.Printf("更新项目失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新项目失败"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 记录进展历史
	if progressChanged {
		log.Printf("项目进展历史记录已添加: 从 %s 到 %s",
			existingProject.ProjectProgress, req.ProjectProgress)
		input := models.ProjectProgressHistory{
			ProjectID:    projectID,
			ProjectName:  req.ProjectName,
			FromProgress: string(existingProject.ProjectProgress),
			ToProgress:   string(req.ProjectProgress),
			OperatorID:   currentUser.ID,
			OperatorName: currentUser.Username,
			Remark:       "更新项目",
		}
		AddProjectProgressHistoryFn(context.Background(), input)
	}

	if result.ModifiedCount > 0 {
		log.Printf("项目更新成功")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "项目更新成功"})
	} else {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "项目信息无变化"})
	}
}

// 7. 删除项目
func DeleteProject(c *gin.Context) {
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	username := currentUser.Username
	projectID := c.Param("id")

	log.Printf("删除项目请求 - 项目ID: %s, 用户: %s", projectID, username)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 验证项目ID格式
	projectObjID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Printf("无效的项目ID格式: %s", projectID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID格式"})
		return
	}

	// 查询项目
	projectCollection := repository.Collection(repository.ProjectsCollection)
	var project models.Project
	err = projectCollection.FindOne(ctx, bson.M{"_id": projectObjID}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目失败"})
		}
		return
	}

	// 查询关联客户
	customerCollection := repository.Collection(repository.CustomersCollection)
	var customer models.Customer
	err = customerCollection.FindOne(ctx, bson.M{"_id": project.CustomerID}).Decode(&customer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "关联客户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询客户失败"})
		}
		return
	}

	// 权限检查
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		switch models.UserRole(currentUser.Role) {
		case models.UserRoleFACTORY_SALES:
			if customer.RelatedSalesID != currentUser.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该项目"})
				return
			}
		case models.UserRoleAGENT:
			if customer.RelatedAgentID != currentUser.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该项目"})
				return
			}
		}
	}

	// 删除项目
	result, err := projectCollection.DeleteOne(ctx, bson.M{"_id": projectObjID})
	if err != nil {
		log.Printf("删除项目失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除项目失败"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	log.Printf("项目删除成功")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "项目删除成功"})
}

// 辅助函数：获取用户可访问的客户ID列表
func getUserAccessibleCustomerIds(user *utils.LoginUser) ([]primitive.ObjectID, error) {
	startTime := time.Now()
	log.Printf("开始获取用户 %s 的可访问客户列表", user.Username)

	if user.Role == string(models.UserRoleSUPER_ADMIN) {
		log.Printf("超级管理员，可访问所有客户，耗时: %dms", time.Since(startTime).Milliseconds())
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	customerCollection := repository.Collection(repository.CustomersCollection)

	var filter bson.M
	if user.Role == string(models.UserRoleFACTORY_SALES) {
		filter = bson.M{
			"$or": []bson.M{
				{"relatedSalesId": user.ID},
				{"isInPublicPool": true},
			},
		}
	} else if user.Role == string(models.UserRoleAGENT) {
		filter = bson.M{
			"$or": []bson.M{
				{"relatedAgentId": user.ID},
				{"isInPublicPool": true},
			},
		}
	} else {
		return nil, errors.New("用户角色不支持")
	}

	cursor, err := customerCollection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, fmt.Errorf("查询客户失败: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("解析客户ID失败: %w", err)
	}

	customerIds := make([]primitive.ObjectID, len(results))
	for i, r := range results {
		customerIds[i] = r.ID
	}

	log.Printf("用户 %s 可访问 %d 个客户，耗时: %dms",
		user.Username, len(customerIds), time.Since(startTime).Milliseconds())

	return customerIds, nil
}

func UpdateWebHiddenByCustomerID(ctx context.Context, customerObjID primitive.ObjectID) error {
	// 获取 projects 集合
	projectCollection := repository.Collection(repository.ProjectsCollection)
	filter := bson.M{"customerId": customerObjID, "webHidden": false}
	update := bson.M{"$set": bson.M{"webHidden": true}}
	// 执行更新操作
	_, err := projectCollection.UpdateMany(ctx, filter, update)
	return err
}
