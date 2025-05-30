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
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
)

// GetDashboardStats 获取数据看板统计信息
func GetDashboardStats(c *gin.Context) {
	// 获取当前用户
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 获取查询参数
	timeRange := c.Query("timeRange")
	if timeRange == "" {
		timeRange = "month"
	}
	startDateParam := c.Query("startDate")
	endDateParam := c.Query("endDate")

	// 记录API请求
	utils.LogApiRequest("GET", "/api/dashboard-stats", nil, nil, map[string]string{
		"timeRange": timeRange,
		"startDate": startDateParam,
		"endDate":   endDateParam,
	})

	utils.LogInfo(map[string]interface{}{
		"username":  user.Username,
		"timeRange": timeRange,
	}, "获取数据看板统计信息")

	// 构建基本查询条件
	customerQuery := bson.M{}

	// 基于用户角色构建查询条件
	if user.Role == string(models.UserRoleFACTORY_SALES) {
		// 销售只能看到关联销售为自己的客户
		customerQuery["relatedSalesId"] = user.ID
	} else if user.Role == string(models.UserRoleAGENT) {
		// 代理商只能看到关联代理商为自己的客户
		customerQuery["relatedAgentId"] = user.ID
	}

	// 根据时间范围参数设置日期筛选
	dateFilter := bson.M{}
	today := time.Now()

	if timeRange == "custom" && startDateParam != "" && endDateParam != "" {
		// 自定义日期范围
		startDate, err := time.Parse(time.RFC3339, startDateParam+"T00:00:00Z")
		if err != nil {
			utils.HandleError(c, fmt.Errorf("解析开始日期失败: %w", err))
			return
		}

		endDate, err := time.Parse(time.RFC3339, endDateParam+"T23:59:59Z")
		if err != nil {
			utils.HandleError(c, fmt.Errorf("解析结束日期失败: %w", err))
			return
		}

		dateFilter = bson.M{
			"createdAt": bson.M{
				"$gte": startDate,
				"$lte": endDate,
			},
		}
	} else if timeRange == "week" {
		// 近7天
		sevenDaysAgo := today.AddDate(0, 0, -7)
		dateFilter = bson.M{
			"createdAt": bson.M{
				"$gte": sevenDaysAgo,
			},
		}
	} else if timeRange == "current_week" {
		// 本周（从周一到今天）
		currentDay := int(today.Weekday())
		daysSinceMonday := currentDay
		if currentDay == 0 {
			daysSinceMonday = 6 // 周日是一周的第7天
		} else {
			daysSinceMonday = currentDay - 1 // 周一=1, 周二=2, ...
		}

		thisWeekMonday := today.AddDate(0, 0, -daysSinceMonday)
		thisWeekMonday = time.Date(thisWeekMonday.Year(), thisWeekMonday.Month(), thisWeekMonday.Day(), 0, 0, 0, 0, thisWeekMonday.Location())

		dateFilter = bson.M{
			"createdAt": bson.M{
				"$gte": thisWeekMonday,
			},
		}
	} else if timeRange == "month" {
		// 近30天
		thirtyDaysAgo := today.AddDate(0, 0, -30)
		dateFilter = bson.M{
			"createdAt": bson.M{
				"$gte": thirtyDaysAgo,
			},
		}
	} else if timeRange == "quarter" {
		// 近3个月
		threeMonthsAgo := today.AddDate(0, -3, 0)
		dateFilter = bson.M{
			"createdAt": bson.M{
				"$gte": threeMonthsAgo,
			},
		}
	} else if timeRange == "year" {
		// 近1年
		oneYearAgo := today.AddDate(-1, 0, 0)
		dateFilter = bson.M{
			"createdAt": bson.M{
				"$gte": oneYearAgo,
			},
		}
	}

	// 合并日期筛选条件到客户查询
	for k, v := range dateFilter {
		if k == "createdAt" {
			customerQuery["createdat"] = v
		}
	}

	// 收集统计数据
	customersCollection := repository.Collection(repository.CustomersCollection)
	agentsCollection := repository.Collection(repository.AgentsCollection)
	productsCollection := repository.Collection(repository.ProductsCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 计算客户总数
	customerCount, err := customersCollection.CountDocuments(ctx, customerQuery)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("计算客户总数失败: %w", err))
		return
	}
	customerQueryStr, _ := json.Marshal(customerQuery)
	utils.LogInfo(map[string]interface{}{
		"customerQuery": string(customerQueryStr),
		"customerCount": customerCount,
	}, "customerQuery")
	// 产品总数
	productCount, err := productsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		utils.HandleError(c, fmt.Errorf("计算产品总数失败: %w", err))
		return
	}

	// 代理商数量
	var agentCount int64
	if user.Role == string(models.UserRoleSUPER_ADMIN) {
		// 超级管理员可以看到所有代理商
		agentQuery := bson.M{"status": "approved"}
		for k, v := range dateFilter {
			agentQuery[k] = v
		}
		agentCount, err = agentsCollection.CountDocuments(ctx, agentQuery)
		if err != nil {
			utils.HandleError(c, fmt.Errorf("计算代理商数量失败: %w", err))
			return
		}
	} else if user.Role == string(models.UserRoleFACTORY_SALES) {
		// 销售只能看到关联销售为自己的代理商
		agentQuery := bson.M{"relatedSalesId": user.ID, "status": "approved"}
		for k, v := range dateFilter {
			agentQuery[k] = v
		}
		agentCount, err = agentsCollection.CountDocuments(ctx, agentQuery)
		if err != nil {
			utils.HandleError(c, fmt.Errorf("计算代理商数量失败: %w", err))
			return
		}
	} else {
		agentCount = 0
	}

	// 获取客户重要程度分布
	importanceDistribution, err := getChartDataAggregation(ctx, customersCollection, customerQuery, "$importance")
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取客户重要程度分布失败: %w", err))
		return
	}

	// 获取客户进展状态分布
	progressDistribution, err := getChartDataAggregation(ctx, customersCollection, customerQuery, "$progress")
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取客户进展状态分布失败: %w", err))
		return
	}

	// 获取客户性质分布
	natureDistribution, err := getChartDataAggregation(ctx, customersCollection, customerQuery, "$nature")
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取客户性质分布失败: %w", err))
		return
	}

	// 获取产品包装类型分布
	packageTypeDistribution, err := getChartDataAggregation(ctx, productsCollection, bson.M{}, "$packageType")
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取产品包装类型分布失败: %w", err))
		return
	}

	// 获取产品库存等级分布
	stockLevels := []map[string]interface{}{
		{"level": "库存不足", "min": 0, "max": 100},
		{"level": "库存适中", "min": 101, "max": 500},
		{"level": "库存充足", "min": 501, "max": 1000000},
	}

	stockLevelDistribution := []models.ChartDataItem{}
	for _, level := range stockLevels {
		query := bson.M{
			"stock": bson.M{
				"$gte": level["min"],
				"$lte": level["max"],
			},
		}
		count, err := productsCollection.CountDocuments(ctx, query)
		if err != nil {
			utils.HandleError(c, fmt.Errorf("获取库存等级分布失败: %w", err))
			return
		}

		if count > 0 {
			stockLevelDistribution = append(stockLevelDistribution, models.ChartDataItem{
				Name:  level["level"].(string),
				Value: int(count),
			})
		}
	}

	// 获取产品客户关联数量分布
	productCustomerRelation, err := getProductCustomerRelation(ctx, customerQuery)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取产品客户关联数量分布失败: %w", err))
		return
	}

	// 获取产品按客户进展阶段分布
	productProgressDistribution, err := getProductProgressDistribution(ctx, customerQuery)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取产品按客户进展阶段分布失败: %w", err))
		return
	}

	// 构建响应数据
	responseData := models.DashboardDataResponse{
		CustomerCount:               int(customerCount),
		ProductCount:                int(productCount),
		AgentCount:                  int(agentCount),
		CustomerImportance:          importanceDistribution,
		CustomerProgress:            progressDistribution,
		CustomerNature:              natureDistribution,
		ProductPackageType:          packageTypeDistribution,
		ProductStockLevel:           stockLevelDistribution,
		ProductCustomerRelation:     productCustomerRelation,
		ProductProgressDistribution: productProgressDistribution,
	}

	utils.SuccessResponse(c, responseData, "成功")
}

// getChartDataAggregation 执行图表数据聚合
func getChartDataAggregation(ctx context.Context, collection *mongo.Collection, query bson.M, groupField string) ([]models.ChartDataItem, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: query}},
		{{Key: "$group", Value: bson.M{"_id": groupField, "count": bson.M{"$sum": 1}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID    string `json:"_id,omitempty" bson:"_id,omitempty"`
		Count int    `bson:"count"`
	}

	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	var chartData []models.ChartDataItem
	for _, result := range results {
		chartData = append(chartData, models.ChartDataItem{
			Name:  result.ID,
			Value: result.Count,
		})
	}

	return chartData, nil
}

// getProductCustomerRelation 获取产品客户关联数量分布
func getProductCustomerRelation(ctx context.Context, customerQuery bson.M) ([]models.ChartDataItem, error) {
	customersCollection := repository.Collection(repository.CustomersCollection)
	productsCollection := repository.Collection(repository.ProductsCollection)

	// 获取所有产品
	cursor, err := productsCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var products []struct {
		ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
		ModelName   string             `json:"modelName" bson:"modelName"`
		PackageType string             `json:"packageType" bson:"packageType"`
	}

	if err = cursor.All(ctx, &products); err != nil {
		return nil, err
	}

	// 计算每个产品关联的客户数量
	var productRelationData []models.ChartDataItem

	for _, product := range products {
		query := customerQuery
		query["productneeds"] = bson.M{"$elemMatch": bson.M{"$regex": product.ID.Hex()}}
		count, err := customersCollection.CountDocuments(ctx, query)
		if err != nil {
			return nil, err
		}

		if count > 0 {
			productName := fmt.Sprintf("%s/%s", product.ModelName, product.PackageType)
			productRelationData = append(productRelationData, models.ChartDataItem{
				Name:  productName,
				Value: int(count),
			})
		}
	}
	// 排序并取前10项
	// 注意：这里需要自己实现排序，或者使用Go的sort包
	if len(productRelationData) > 10 {
		// 简单冒泡排序
		for i := 0; i < len(productRelationData)-1; i++ {
			for j := 0; j < len(productRelationData)-i-1; j++ {
				if productRelationData[j].Value < productRelationData[j+1].Value {
					productRelationData[j], productRelationData[j+1] = productRelationData[j+1], productRelationData[j]
				}
			}
		}
		productRelationData = productRelationData[:10]
	}

	return productRelationData, nil
}

// getProductProgressDistribution 获取产品按客户进展阶段分布
func getProductProgressDistribution(ctx context.Context, customerQuery bson.M) ([]models.ProductProgressDistribution, error) {
	customersCollection := repository.Collection(repository.CustomersCollection)
	productsCollection := repository.Collection(repository.ProductsCollection)

	// 获取前5个产品
	limit := int64(5)
	cursor, err := productsCollection.Find(ctx, bson.M{}, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var products []struct {
		ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
		ModelName   string             `bson:"modelName"`
		PackageType string             `bson:"packageType"`
	}

	if err = cursor.All(ctx, &products); err != nil {
		return nil, err
	}

	var result []models.ProductProgressDistribution

	for _, product := range products {
		productID := product.ID.Hex()
		productName := fmt.Sprintf("%s/%s", product.ModelName, product.PackageType)

		// 对于每个进展阶段，计算关联了该产品的客户数量
		query := bson.M{}
		for k, v := range customerQuery {
			query[k] = v
		}
		query["productneeds"] = bson.M{"$elemMatch": bson.M{"$regex": productID}}

		sampleQuery := query
		sampleQuery["progress"] = string(models.CustomerProgressSampleEvaluation)
		sampleCount, err := customersCollection.CountDocuments(ctx, sampleQuery)
		if err != nil {
			return nil, err
		}

		testingQuery := query
		testingQuery["progress"] = string(models.CustomerProgressTesting)
		testingCount, err := customersCollection.CountDocuments(ctx, testingQuery)
		if err != nil {
			return nil, err
		}

		smallBatchQuery := query
		smallBatchQuery["progress"] = string(models.CustomerProgressSmallBatch)
		smallBatchCount, err := customersCollection.CountDocuments(ctx, smallBatchQuery)
		if err != nil {
			return nil, err
		}

		massProductionQuery := query
		massProductionQuery["progress"] = string(models.CustomerProgressMassProduction)
		massProductionCount, err := customersCollection.CountDocuments(ctx, massProductionQuery)
		if err != nil {
			return nil, err
		}

		result = append(result, models.ProductProgressDistribution{
			ProductName:    productName,
			Sample:         int(sampleCount),
			Testing:        int(testingCount),
			SmallBatch:     int(smallBatchCount),
			MassProduction: int(massProductionCount),
		})
	}

	return result, nil
}
