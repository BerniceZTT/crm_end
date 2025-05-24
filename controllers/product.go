package controllers

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

func GetProductList(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"userRole": user.Role,
		"query":    c.Request.URL.Query(),
	}, "产品列表请求")

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	// skip := (page - 1) * limit

	// 搜索条件
	searchQuery := bson.M{}
	keyword := c.Query("keyword")
	if keyword != "" {
		searchQuery["$or"] = []bson.M{
			{"modelName": bson.M{"$regex": keyword, "$options": "i"}},
			{"packageType": bson.M{"$regex": keyword, "$options": "i"}},
		}
	}

	collection := repository.Collection(repository.ProductsCollection)

	// 获取总数
	totalCount, err := collection.CountDocuments(context.Background(), searchQuery)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"totalCount":  totalCount,
		"searchQuery": searchQuery,
	}, fmt.Sprintf("产品总数: %d", totalCount))

	// 检查集合中是否有数据
	if totalCount == 0 {
		utils.LogInfo(nil, "产品集合中没有数据")

		// 检查集合是否存在
		ok, err := repository.CollectionExists(repository.ProductsCollection)
		if err != nil {
			utils.HandleError(c, err)
			return
		}

		if !ok {
			utils.LogInfo(nil, "产品集合不存在")
		}
	}

	// 查询数据
	findOptions := options.Find().
		SetSort(bson.D{{"modelName", 1}, {"packageType", 1}})
		// .SetSkip(int64(skip)).
		// SetLimit(int64(limit))

	cursor, err := collection.Find(context.Background(), searchQuery, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(context.Background())

	var products []models.Product
	if err := cursor.All(context.Background(), &products); err != nil {
		utils.HandleError(c, err)
		return
	}

	// 详细记录查询结果
	var firstProduct string
	var productIds []string
	if len(products) > 0 {
		firstProductBytes, _ := bson.Marshal(products[0])
		firstProduct = string(firstProductBytes)

		for i := 0; i < len(products) && i < 5; i++ {
			productIds = append(productIds, products[i].ID.Hex())
		}
	}

	utils.LogInfo(map[string]interface{}{
		"firstProduct": firstProduct,
		"productIds":   productIds,
	}, fmt.Sprintf("查询到 %d 个产品", len(products)))

	// 检查数据字段是否符合前端期望
	if len(products) > 0 {
		sampleProduct := products[0]
		requiredFields := []string{"_id", "modelName", "packageType", "stock", "pricing"}
		var missingFields []string

		for _, field := range requiredFields {
			switch field {
			case "_id":
				if sampleProduct.ID.IsZero() {
					missingFields = append(missingFields, field)
				}
			case "modelName":
				if sampleProduct.ModelName == "" {
					missingFields = append(missingFields, field)
				}
			case "packageType":
				if sampleProduct.PackageType == "" {
					missingFields = append(missingFields, field)
				}
			case "pricing":
				if sampleProduct.Pricing == nil {
					missingFields = append(missingFields, field)
				} else {
					if len(sampleProduct.Pricing) != 7 {
						utils.LogInfo(map[string]interface{}{
							"pricingLength": len(sampleProduct.Pricing),
						}, "pricing数组长度不是7")
					}
				}
			}
		}

		if len(missingFields) > 0 {
			utils.LogInfo(map[string]interface{}{
				"sampleProduct": sampleProduct,
			}, fmt.Sprintf("产品数据缺少必要字段: %s", strings.Join(missingFields, ", ")))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"products": products,
		"pagination": gin.H{
			"total": totalCount,
			"page":  page,
			"limit": limit,
			"pages": (totalCount + int64(limit) - 1) / int64(limit),
		},
	})
}

func GetProduct(c *gin.Context) {
	id := c.Param("id")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.ErrorResponse(c, "无效的产品ID", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)

	var product models.Product
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		utils.ErrorResponse(c, "产品不存在", http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"product": product,
	})
}

func CreateProduct(c *gin.Context) {
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 验证权限
	if user.Role != string(models.UserRoleSUPER_ADMIN) &&
		user.Role != string(models.UserRoleINVENTORY_MANAGER) {
		utils.ErrorResponse(c, "无权创建产品", http.StatusForbidden)
		return
	}

	var productData models.Product
	if err := c.ShouldBindJSON(&productData); err != nil {
		utils.ErrorResponse(c, "无效的产品数据: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 验证价格阶梯
	if len(productData.Pricing) != 7 {
		utils.ErrorResponse(c, "必须提供7档阶梯定价", http.StatusBadRequest)
		return
	}

	for _, tier := range productData.Pricing {
		if tier.Quantity < 1 {
			utils.ErrorResponse(c, "数量必须大于或等于1", http.StatusBadRequest)
			return
		}
		if tier.Price <= 0 {
			utils.ErrorResponse(c, "价格必须为正数", http.StatusBadRequest)
			return
		}
	}

	utils.LogInfo(map[string]interface{}{
		"modelName":   productData.ModelName,
		"packageType": productData.PackageType,
	}, "开始创建产品")

	collection := repository.Collection(repository.ProductsCollection)

	// 检查产品是否已存在
	count, err := collection.CountDocuments(context.Background(), bson.M{
		"modelName":   productData.ModelName,
		"packageType": productData.PackageType,
	})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if count > 0 {
		utils.ErrorResponse(c, "该型号产品的封装类型已存在", http.StatusBadRequest)
		return
	}

	// 设置创建时间
	now := time.Now()
	productData.CreatedAt = now
	productData.UpdatedAt = now

	// 创建新产品
	result, err := collection.InsertOne(context.Background(), productData)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	productID := result.InsertedID.(primitive.ObjectID)
	utils.LogInfo(map[string]interface{}{
		"productId": productID.Hex(),
	}, "产品成功插入数据库")

	// 如果有初始库存，创建库存记录
	if productData.Stock > 0 {
		recordsCollection := repository.Collection(repository.InventoryRecordsCollection)

		_, err := recordsCollection.InsertOne(context.Background(), models.InventoryRecord{
			ProductID:     productID.Hex(),
			ModelName:     productData.ModelName,
			PackageType:   productData.PackageType,
			OperationType: "in",
			Quantity:      productData.Stock,
			Remark:        "产品初始库存",
			Operator:      user.Username,
			OperatorID:    user.ID,
			OperationTime: now,
		})

		if err != nil {
			// 记录错误但不影响产品创建
			utils.LogInfo(map[string]interface{}{
				"error":     err.Error(),
				"productId": productID.Hex(),
			}, "创建初始库存记录失败，但产品已创建成功")
		} else {
			utils.LogInfo(map[string]interface{}{
				"productId": productID.Hex(),
				"stock":     productData.Stock,
			}, "成功创建初始库存记录")
		}
	}

	// 重新查询产品以获取完整信息
	var newProduct models.Product
	err = collection.FindOne(context.Background(), bson.M{"_id": productID}).Decode(&newProduct)
	if err != nil {
		newProduct = productData
		newProduct.ID = productID
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "创建产品成功",
		"product": newProduct,
	})
}

func BulkImportProducts(c *gin.Context) {
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN &&
		models.UserRole(user.Role) != models.UserRoleINVENTORY_MANAGER {
		utils.ErrorResponse(c, "无权导入产品", http.StatusForbidden)
		return
	}

	var request models.BulkImportProduct
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.ErrorResponse(c, "无效的请求数据: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(request.Products) == 0 {
		utils.ErrorResponse(c, "至少需要一个产品", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)

	// 检查产品是否已存在
	var orConditions []bson.M
	for _, product := range request.Products {
		orConditions = append(orConditions, bson.M{
			"modelName":   product.ModelName,
			"packageType": product.PackageType,
		})
	}

	cursor, err := collection.Find(context.Background(), bson.M{"$or": orConditions})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	var existingProducts []models.Product
	if err := cursor.All(context.Background(), &existingProducts); err != nil {
		utils.HandleError(c, err)
		return
	}

	if len(existingProducts) > 0 {
		var duplicateProducts []string
		for _, p := range existingProducts {
			duplicateProducts = append(duplicateProducts, fmt.Sprintf("%s/%s", p.ModelName, p.PackageType))
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "以下产品型号已存在",
			"duplicateProducts": duplicateProducts,
		})
		return
	}

	// 设置创建时间
	now := time.Now()
	for i := range request.Products {
		request.Products[i].CreatedAt = now
		request.Products[i].UpdatedAt = now
	}

	// 插入所有产品
	var productsToInsert []interface{}
	for _, p := range request.Products {
		productsToInsert = append(productsToInsert, p)
	}

	result, err := collection.InsertMany(context.Background(), productsToInsert)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "批量导入产品成功",
		"insertedCount": len(result.InsertedIDs),
	})
}

func UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN &&
		models.UserRole(user.Role) != models.UserRoleINVENTORY_MANAGER {
		utils.ErrorResponse(c, "无权更新产品", http.StatusForbidden)
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		utils.ErrorResponse(c, "无效的更新数据: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 不允许直接修改库存
	if _, exists := updateData["stock"]; exists {
		utils.ErrorResponse(c, "库存数量只能通过入库/出库操作进行修改", http.StatusForbidden)
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.ErrorResponse(c, "无效的产品ID", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)

	// 检查产品是否存在
	var product models.Product
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		utils.ErrorResponse(c, "产品不存在", http.StatusNotFound)
		return
	}

	// 如果更新型号或封装类型，检查是否已存在
	modelName, hasModelName := updateData["modelName"].(string)
	packageType, hasPackageType := updateData["packageType"].(string)
	if (hasModelName && modelName != product.ModelName) ||
		(hasPackageType && packageType != product.PackageType) {

		newModelName := product.ModelName
		if modelName, exists := updateData["modelName"].(string); exists {
			newModelName = modelName
		}

		newPackageType := product.PackageType
		if packageType, exists := updateData["packageType"].(string); exists {
			newPackageType = packageType
		}

		count, err := collection.CountDocuments(context.Background(), bson.M{
			"modelName":   newModelName,
			"packageType": newPackageType,
			"_id":         bson.M{"$ne": objectID},
		})
		if err != nil {
			utils.HandleError(c, err)
			return
		}

		if count > 0 {
			utils.ErrorResponse(c, "该型号产品的封装类型已存在", http.StatusBadRequest)
			return
		}
	}

	// 设置更新时间
	updateData["updatedAt"] = time.Now()

	// 更新产品
	result, err := collection.UpdateOne(
		context.Background(),
		bson.M{"_id": objectID},
		bson.M{"$set": updateData},
	)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "产品数据未变更",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "更新产品成功",
	})
}

func DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN {
		utils.ErrorResponse(c, "无权删除产品", http.StatusForbidden)
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.ErrorResponse(c, "无效的产品ID", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)

	// 检查产品是否存在
	var product models.Product
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		utils.ErrorResponse(c, "产品不存在", http.StatusNotFound)
		return
	}

	// 检查是否有客户正在使用该产品
	customersCollection := repository.Collection(repository.CustomersCollection)
	customerCount, err := customersCollection.CountDocuments(context.Background(), bson.M{
		"productNeeds": bson.M{
			"$elemMatch": bson.M{
				"$regex": product.ModelName,
			},
		},
	})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if customerCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "该产品有关联客户，无法删除",
			"customerCount": customerCount,
		})
		return
	}

	// 删除产品
	result, err := collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	if result.DeletedCount == 0 {
		utils.ErrorResponse(c, "产品不存在或已被删除", http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除产品成功",
	})
}

func StockInProduct(c *gin.Context) {
	id := c.Param("id")
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN &&
		models.UserRole(user.Role) != models.UserRoleINVENTORY_MANAGER {
		utils.ErrorResponse(c, "无权执行入库操作", http.StatusForbidden)
		return
	}

	var operation models.StockOperation
	if err := c.ShouldBindJSON(&operation); err != nil {
		utils.ErrorResponse(c, "无效的操作数据: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"user":   user.Username,
		"userId": user.ID,
		"remark": operation.Remark,
	}, fmt.Sprintf("执行入库操作：产品ID=%s, 数量=%d", id, operation.Quantity))

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.ErrorResponse(c, "无效的产品ID", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)

	// 检查产品是否存在
	var product models.Product
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		utils.ErrorResponse(c, "产品不存在", http.StatusNotFound)
		return
	}

	// 创建操作ID用于幂等性检查
	operationId := fmt.Sprintf("in_%s_%d_%d", id, operation.Quantity, time.Now().UnixNano())

	// 使用增强的库存操作函数
	operationResult, err := utils.ExecuteInventoryOperation(
		// 检查此操作是否已经完成
		func() (bool, error) {
			recordsCollection := repository.Collection(repository.InventoryRecordsCollection)
			count, err := recordsCollection.CountDocuments(context.Background(), bson.M{
				"productId":     id,
				"operationType": "in",
				"quantity":      operation.Quantity,
				"operationId":   operationId,
				"operationTime": bson.M{"$gte": time.Now().Add(-5 * time.Minute)},
			})
			return count > 0, err
		},
		// 实际执行的操作逻辑
		func() (interface{}, error) {
			// 第一步：更新库存
			updateResult, err := collection.UpdateOne(
				context.Background(),
				bson.M{"_id": objectID},
				bson.M{"$inc": bson.M{"stock": operation.Quantity}},
			)
			if err != nil {
				return nil, err
			}

			if updateResult.ModifiedCount == 0 {
				return nil, errors.New("库存更新失败")
			}

			// 第二步：创建入库记录
			recordsCollection := repository.Collection(repository.InventoryRecordsCollection)
			recordResult, err := recordsCollection.InsertOne(context.Background(), models.InventoryRecord{
				ProductID:     id,
				ModelName:     product.ModelName,
				PackageType:   product.PackageType,
				OperationType: "in",
				Quantity:      operation.Quantity,
				Remark:        operation.Remark,
				Operator:      user.Username,
				OperatorID:    user.ID,
				OperationTime: time.Now(),
				OperationID:   operationId,
			})
			if err != nil {
				utils.LogInfo(map[string]interface{}{
					"productId": id,
					"quantity":  operation.Quantity,
				}, "入库记录创建失败，但库存可能已更新")

				return map[string]interface{}{
					"statusUncertain":    true,
					"inventoryOperation": true,
					"message":            "入库操作状态不确定，请刷新页面查看最新库存",
				}, nil
			}

			if recordResult.InsertedID == nil {
				return map[string]interface{}{
					"statusUncertain":    true,
					"inventoryOperation": true,
					"message":            "入库操作状态不确定，请刷新页面查看最新库存",
				}, nil
			}

			// 第三步：重新查询更新后的库存
			var updatedProduct models.Product
			err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&updatedProduct)
			if err != nil {
				return nil, err
			}

			// 记录成功的库存操作
			utils.LogInventoryOperation("入库", id, operation.Quantity, true)

			return map[string]interface{}{
				"success":       true,
				"message":       "入库操作成功",
				"newStock":      updatedProduct.Stock,
				"expectedStock": product.Stock + operation.Quantity,
			}, nil
		},
		3, // 重试次数
	)

	if err != nil {
		utils.LogInfo(map[string]interface{}{
			"error":     err.Error(),
			"productId": id,
			"quantity":  operation.Quantity,
		}, "入库操作出错")

		utils.InventoryOperationResponse(c, map[string]interface{}{
			"error":     err.Error(),
			"productId": id,
		}, false)
		return
	}

	// 判断操作结果状态
	if statusUncertain, exists := operationResult["statusUncertain"]; exists && statusUncertain.(bool) {
		utils.InventoryOperationResponse(c, map[string]interface{}{
			"message":   "入库操作状态不确定，请刷新页面查看最新库存",
			"productId": id,
		}, false)
		return
	} else if success, exists := operationResult["success"]; exists && success.(bool) {
		utils.SuccessResponse(c, map[string]interface{}{
			"message":  "入库操作成功",
			"newStock": operationResult["result"].(map[string]interface{})["newStock"],
		}, "入库操作成功", http.StatusOK)
		return
	} else if alreadyCompleted, exists := operationResult["alreadyCompleted"]; exists && alreadyCompleted.(bool) {
		utils.SuccessResponse(c, map[string]interface{}{
			"message": "入库操作已完成",
			"warning": "此操作可能是重复提交",
		}, "入库操作已完成", http.StatusOK)
		return
	}

	utils.ErrorResponse(c, "入库操作失败：未知原因", http.StatusInternalServerError)
}

func StockOutProduct(c *gin.Context) {
	id := c.Param("id")
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN &&
		models.UserRole(user.Role) != models.UserRoleINVENTORY_MANAGER {
		utils.ErrorResponse(c, "无权执行出库操作", http.StatusForbidden)
		return
	}

	var operation models.StockOperation
	if err := c.ShouldBindJSON(&operation); err != nil {
		utils.ErrorResponse(c, "无效的操作数据: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"user":   user.Username,
		"userId": user.ID,
		"remark": operation.Remark,
	}, fmt.Sprintf("执行出库操作：产品ID=%s, 数量=%d", id, operation.Quantity))

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.ErrorResponse(c, "无效的产品ID", http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)

	// 检查产品是否存在
	var product models.Product
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		utils.ErrorResponse(c, "产品不存在", http.StatusNotFound)
		return
	}

	// 检查库存是否充足
	if product.Stock < operation.Quantity {
		utils.ErrorResponse(c, fmt.Sprintf("库存不足，当前库存: %d", product.Stock), http.StatusBadRequest)
		return
	}

	// 创建操作ID用于幂等性检查
	operationId := fmt.Sprintf("out_%s_%d_%d", id, operation.Quantity, time.Now().UnixNano())

	// 使用增强的库存操作函数
	operationResult, err := utils.ExecuteInventoryOperation(
		// 检查此操作是否已经完成
		func() (bool, error) {
			recordsCollection := repository.Collection(repository.InventoryRecordsCollection)
			count, err := recordsCollection.CountDocuments(context.Background(), bson.M{
				"productId":     id,
				"operationType": "out",
				"quantity":      operation.Quantity,
				"operationId":   operationId,
				"operationTime": bson.M{"$gte": time.Now().Add(-5 * time.Minute)},
			})
			return count > 0, err
		},
		// 实际执行的操作逻辑
		func() (interface{}, error) {
			// 第一步：更新库存
			updateResult, err := collection.UpdateOne(
				context.Background(),
				bson.M{
					"_id":   objectID,
					"stock": bson.M{"$gte": operation.Quantity},
				},
				bson.M{"$inc": bson.M{"stock": -operation.Quantity}},
			)
			if err != nil {
				return nil, err
			}

			if updateResult.ModifiedCount == 0 {
				// 再次检查是否因为库存不足导致的更新失败
				var currentProduct models.Product
				err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&currentProduct)
				if err != nil {
					return nil, err
				}

				if currentProduct.Stock < operation.Quantity {
					return nil, fmt.Errorf("库存不足，当前库存: %d", currentProduct.Stock)
				} else {
					return nil, errors.New("出库操作失败")
				}
			}

			// 第二步：创建出库记录
			recordsCollection := repository.Collection(repository.InventoryRecordsCollection)

			recordResult, err := recordsCollection.InsertOne(context.Background(), models.InventoryRecord{
				ProductID:     id,
				ModelName:     product.ModelName,
				PackageType:   product.PackageType,
				OperationType: "out",
				Quantity:      operation.Quantity,
				Remark:        operation.Remark,
				Operator:      user.Username,
				OperatorID:    user.ID,
				OperationTime: time.Now(),
				OperationID:   operationId,
			})
			if err != nil {
				utils.LogInfo(map[string]interface{}{
					"productId": id,
					"quantity":  operation.Quantity,
				}, "出库记录创建失败，但库存可能已更新")

				return map[string]interface{}{
					"statusUncertain":    true,
					"inventoryOperation": true,
					"message":            "出库操作状态不确定，请刷新页面查看最新库存",
				}, nil
			}

			if recordResult.InsertedID == nil {
				return map[string]interface{}{
					"statusUncertain":    true,
					"inventoryOperation": true,
					"message":            "出库操作状态不确定，请刷新页面查看最新库存",
				}, nil
			}

			// 第三步：重新查询更新后的库存
			var updatedProduct models.Product
			err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&updatedProduct)
			if err != nil {
				return nil, err
			}

			// 记录成功的库存操作
			utils.LogInventoryOperation("出库", id, operation.Quantity, true)

			return map[string]interface{}{
				"success":       true,
				"message":       "出库操作成功",
				"newStock":      updatedProduct.Stock,
				"expectedStock": product.Stock - operation.Quantity,
			}, nil
		},
		3, // 重试次数
	)

	if err != nil {
		utils.LogInfo(map[string]interface{}{
			"error":     err.Error(),
			"productId": id,
			"quantity":  operation.Quantity,
		}, "出库操作出错")

		utils.InventoryOperationResponse(c, map[string]interface{}{
			"error":     err.Error(),
			"productId": id,
		}, false)
		return
	}

	// 判断操作结果状态
	if statusUncertain, exists := operationResult["statusUncertain"]; exists && statusUncertain.(bool) {
		utils.InventoryOperationResponse(c, map[string]interface{}{
			"message":   "出库操作状态不确定，请刷新页面查看最新库存",
			"productId": id,
		}, false)
		return
	} else if success, exists := operationResult["success"]; exists && success.(bool) {
		utils.SuccessResponse(c, map[string]interface{}{
			"message":  "出库操作成功",
			"newStock": operationResult["result"].(map[string]interface{})["newStock"],
		}, "出库操作成功", http.StatusOK)
		return
	} else if alreadyCompleted, exists := operationResult["alreadyCompleted"]; exists && alreadyCompleted.(bool) {
		utils.SuccessResponse(c, map[string]interface{}{
			"message": "出库操作已完成",
			"warning": "此操作可能是重复提交",
		}, "出库操作已完成", http.StatusOK)
		return
	}

	utils.ErrorResponse(c, "出库操作失败：未知原因", http.StatusInternalServerError)
}

func BulkStockProduct(c *gin.Context) {
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 验证权限
	if models.UserRole(user.Role) != models.UserRoleSUPER_ADMIN &&
		models.UserRole(user.Role) != models.UserRoleINVENTORY_MANAGER {
		utils.ErrorResponse(c, "无权执行批量库存操作", http.StatusForbidden)
		return
	}

	var request models.BulkStockOperation
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.ErrorResponse(c, "无效的请求数据: "+err.Error(), http.StatusBadRequest)
		return
	}

	collection := repository.Collection(repository.ProductsCollection)
	recordsCollection := repository.Collection(repository.InventoryRecordsCollection)

	for _, operation := range request.Operations {
		objectID, err := primitive.ObjectIDFromHex(operation.ProductID)
		if err != nil {
			utils.ErrorResponse(c, fmt.Sprintf("无效的产品ID: %s", operation.ProductID), http.StatusBadRequest)
			return
		}

		var product models.Product
		err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&product)
		if err != nil {
			utils.ErrorResponse(c, fmt.Sprintf("产品 %s 不存在", operation.ProductID), http.StatusNotFound)
			return
		}

		var result *mongo.UpdateResult
		if operation.Type == "in" {
			result, err = collection.UpdateOne(
				context.Background(),
				bson.M{"_id": objectID},
				bson.M{"$inc": bson.M{"stock": operation.Quantity}},
			)
		} else if operation.Type == "out" {
			if product.Stock < operation.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        fmt.Sprintf("产品 %s 库存不足", operation.ProductID),
					"currentStock": product.Stock,
				})
				return
			}

			result, err = collection.UpdateOne(
				context.Background(),
				bson.M{"_id": objectID},
				bson.M{"$inc": bson.M{"stock": -operation.Quantity}},
			)
		}

		if err != nil {
			utils.HandleError(c, err)
			return
		}

		if result.ModifiedCount == 0 {
			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("操作 %s 未生效", operation.ProductID),
			})
			return
		}

		_, err = recordsCollection.InsertOne(context.Background(), models.InventoryRecord{
			ProductID:     operation.ProductID,
			ModelName:     product.ModelName,
			PackageType:   product.PackageType,
			OperationType: operation.Type,
			Quantity:      operation.Quantity,
			Operator:      user.Username,
			OperatorID:    user.ID,
			OperationTime: time.Now(),
		})
		if err != nil {
			utils.HandleError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量库存操作成功",
	})
}

func ExportProduct(c *gin.Context) {
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	utils.LogInfo(map[string]interface{}{
		"user":   user.Username,
		"userId": user.ID,
	}, "请求产品数据导出 - CSV格式")

	collection := repository.Collection(repository.ProductsCollection)

	cursor, err := collection.Find(context.Background(), bson.M{}, options.Find().SetSort(bson.D{{"modelName", 1}, {"packageType", 1}}))
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(context.Background())

	var products []models.Product
	if err := cursor.All(context.Background(), &products); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{}, fmt.Sprintf("准备导出 %d 条产品记录", len(products)))

	// 设置CSV文件头
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="products_export_%s.csv"`, time.Now().Format("2006-01-02")))

	// 创建CSV写入器
	writer := csv.NewWriter(c.Writer)

	// 写入CSV头
	headers := []string{
		"产品型号", "封装型号", "库存数量",
		"0-1k价格", "1k-10k价格", "10k-50k价格", "50k-100k价格", "100k-500k价格", "500k-1M价格", "大于1M价格",
		"0-1k数量", "1k-10k数量", "10k-50k数量", "50k-100k数量", "100k-500k数量", "500k-1M数量", "大于1M数量",
		"创建时间", "最后更新时间",
	}
	if err := writer.Write(headers); err != nil {
		utils.HandleError(c, err)
		return
	}

	// 标准化阶梯数量对应值
	standardTiers := []int{0, 1000, 10000, 50000, 100000, 500000, 1000000}

	// 添加每条产品数据
	for _, product := range products {
		// 标准化阶梯价格信息
		pricing := make([]models.PricingTier, 7)
		for i := 0; i < 7; i++ {
			pricing[i] = models.PricingTier{Quantity: 0, Price: 0}
		}

		if len(product.Pricing) > 0 {
			// 将产品定价映射到标准阶梯
			for i := 0; i < 7; i++ {
				// 尝试找到精确匹配的阶梯
				found := false
				for _, p := range product.Pricing {
					if p.Quantity == standardTiers[i] {
						pricing[i] = p
						found = true
						break
					}
				}

				// 如果没有精确匹配但有数据，使用现有数据
				if !found && i < len(product.Pricing) {
					pricing[i] = product.Pricing[i]
				}
			}
		}

		row := []string{
			product.ModelName,
			product.PackageType,
			strconv.Itoa(product.Stock),
		}

		// 添加价格信息
		for i := 0; i < 7; i++ {
			row = append(row, strconv.FormatFloat(pricing[i].Price, 'f', 2, 64))
		}

		// 添加数量信息
		for i := 0; i < 7; i++ {
			quantity := pricing[i].Quantity
			if quantity == 0 {
				quantity = standardTiers[i]
			}
			row = append(row, strconv.Itoa(quantity))
		}

		// 添加时间信息
		createdAt := ""
		if !product.CreatedAt.IsZero() {
			createdAt = product.CreatedAt.Format(time.RFC3339)
		}

		updatedAt := ""
		if !product.UpdatedAt.IsZero() {
			updatedAt = product.UpdatedAt.Format(time.RFC3339)
		}

		row = append(row, createdAt, updatedAt)

		if err := writer.Write(row); err != nil {
			utils.HandleError(c, err)
			return
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		utils.HandleError(c, err)
		return
	}
}
