package controllers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
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

// safeNumber 安全转换数字
func safeNumber(value interface{}) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}

// safeAdd 安全的加法运算，避免浮点数精度问题
func safeAdd(a, b float64) float64 {
	const precision = 1000000
	return float64(int64(a*precision)+int64(b*precision)) / precision
}

// GetDashboardStats 获取数据看板统计信息
func GetDashboardStats(c *gin.Context) {
	// 获取当前用户
	currentUser, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	// 获取查询参数
	timeRange := c.Query("timeRange")
	if timeRange == "" {
		timeRange = "month"
	}
	startDateParam := c.Query("startDate")
	endDateParam := c.Query("endDate")

	utils.LogInfo(map[string]interface{}{
		"user":      currentUser.Username,
		"timeRange": timeRange,
		"startDate": startDateParam,
		"endDate":   endDateParam,
	}, "[数据看板] 获取统计信息")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 基于用户角色构建查询条件
	customerQuery := bson.M{}
	baseProjectQuery := bson.M{}
	dateFilter := bson.M{}

	today := time.Now()

	// 处理日期范围
	if timeRange == "custom" && startDateParam != "" && endDateParam != "" {
		startDate, err := time.Parse(time.RFC3339, startDateParam+"T00:00:00Z")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的开始日期"})
			return
		}

		endDate, err := time.Parse(time.RFC3339, endDateParam+"T23:59:59Z")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的结束日期"})
			return
		}

		dateFilter["createdAt"] = bson.M{
			"$gte": startDate,
			"$lte": endDate,
		}
	} else {
		switch timeRange {
		case "week":
			sevenDaysAgo := today.AddDate(0, 0, -7)
			dateFilter["createdAt"] = bson.M{"$gte": sevenDaysAgo}
		case "current_week":
			daysSinceMonday := int(today.Weekday())
			if daysSinceMonday == 0 {
				daysSinceMonday = 6
			} else {
				daysSinceMonday--
			}
			thisWeekMonday := today.AddDate(0, 0, -daysSinceMonday)
			thisWeekMonday = time.Date(thisWeekMonday.Year(), thisWeekMonday.Month(), thisWeekMonday.Day(), 0, 0, 0, 0, time.UTC)
			dateFilter["createdAt"] = bson.M{"$gte": thisWeekMonday}
		case "month":
			thirtyDaysAgo := today.AddDate(0, 0, -30)
			dateFilter["createdAt"] = bson.M{"$gte": thirtyDaysAgo}
		case "quarter":
			threeMonthsAgo := today.AddDate(0, -3, 0)
			dateFilter["createdAt"] = bson.M{"$gte": threeMonthsAgo}
		case "year":
			oneYearAgo := today.AddDate(-1, 0, 0)
			dateFilter["createdAt"] = bson.M{"$gte": oneYearAgo}
		}
	}

	// 合并日期筛选条件
	for k, v := range dateFilter {
		if k == "createdAt" {
			customerQuery["createdat"] = v
		}
		baseProjectQuery[k] = v
	}

	// 根据用户角色添加权限过滤
	if currentUser.Role == string(models.UserRoleFACTORY_SALES) {
		customerQuery["relatedSalesId"] = currentUser.ID
	} else if currentUser.Role == string(models.UserRoleAGENT) {
		customerQuery["relatedAgentId"] = currentUser.ID
	}

	// 获取数据库集合
	customersCollection := repository.Collection(repository.CustomersCollection)
	agentsCollection := repository.Collection(repository.AgentsCollection)
	productsCollection := repository.Collection(repository.ProductsCollection)
	projectsCollection := repository.Collection(repository.ProjectsCollection)

	// 如果用户有权限限制，需要根据客户权限筛选项目
	if currentUser.Role != string(models.UserRoleSUPER_ADMIN) {
		cursor, err := customersCollection.Find(ctx, customerQuery, options.Find().SetProjection(bson.M{"_id": 1}))
		if err != nil {
			utils.HandleError(c, fmt.Errorf("查询可访问客户失败: %w", err))
			return
		}

		var accessibleCustomers []bson.M
		if err = cursor.All(ctx, &accessibleCustomers); err != nil {
			utils.HandleError(c, fmt.Errorf("解析可访问客户失败: %w", err))
			return
		}

		customerIDs := make([]string, 0, len(accessibleCustomers))
		for _, customer := range accessibleCustomers {
			if id, ok := customer["_id"].(primitive.ObjectID); ok {
				customerIDs = append(customerIDs, id.Hex())
			}
		}

		if len(customerIDs) > 0 {
			baseProjectQuery["customerId"] = bson.M{"$in": customerIDs}
		} else {
			// 如果没有可访问客户，则设置一个不可能的条件
			baseProjectQuery["customerId"] = bson.M{"$in": []string{"000000000000000000000000"}}
		}
	}

	// 基础统计数据
	customerCount, err := customersCollection.CountDocuments(ctx, customerQuery)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计客户数量失败: %w", err))
		return
	}

	productCount, err := productsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计产品数量失败: %w", err))
		return
	}

	projectCount, err := projectsCollection.CountDocuments(ctx, baseProjectQuery)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计项目数量失败: %w", err))
		return
	}

	// 代理商数量
	var agentCount int64
	if currentUser.Role == string(models.UserRoleSUPER_ADMIN) {
		agentQuery := bson.M{"status": "approved"}
		for k, v := range dateFilter {
			agentQuery[k] = v
		}
		agentCount, err = agentsCollection.CountDocuments(ctx, agentQuery)
		if err != nil {
			utils.HandleError(c, fmt.Errorf("统计代理商数量失败: %w", err))
			return
		}
	} else if currentUser.Role == string(models.UserRoleFACTORY_SALES) {
		agentQuery := bson.M{
			"relatedSalesId": currentUser.ID,
			"status":         "approved",
		}
		for k, v := range dateFilter {
			agentQuery[k] = v
		}
		agentCount, err = agentsCollection.CountDocuments(ctx, agentQuery)
		if err != nil {
			utils.HandleError(c, fmt.Errorf("统计代理商数量失败: %w", err))
			return
		}
	}

	// 客户分布统计
	// 定义聚合结果的结构体
	type aggResult struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	// 客户重要性分布
	importanceAgg, err := customersCollection.Aggregate(ctx, []bson.M{
		{"$match": customerQuery},
		{"$group": bson.M{"_id": "$importance", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计客户重要性分布失败: %w", err))
		return
	}
	defer importanceAgg.Close(ctx)

	var importanceAggResults []aggResult
	if err = importanceAgg.All(ctx, &importanceAggResults); err != nil {
		utils.HandleError(c, fmt.Errorf("解析客户重要性分布失败: %w", err))
		return
	}

	var importanceDistribution []models.ChartDataItem
	for _, res := range importanceAggResults {
		importanceDistribution = append(importanceDistribution, models.ChartDataItem{
			Name:  res.ID,
			Value: res.Count,
		})
	}

	// 客户性质分布
	natureAgg, err := customersCollection.Aggregate(ctx, []bson.M{
		{"$match": customerQuery},
		{"$group": bson.M{"_id": "$nature", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计客户性质分布失败: %w", err))
		return
	}
	defer natureAgg.Close(ctx)

	var natureAggResults []aggResult
	if err = natureAgg.All(ctx, &natureAggResults); err != nil {
		utils.HandleError(c, fmt.Errorf("解析客户性质分布失败: %w", err))
		return
	}

	var natureDistribution []models.ChartDataItem
	for _, res := range natureAggResults {
		natureDistribution = append(natureDistribution, models.ChartDataItem{
			Name:  res.ID,
			Value: res.Count,
		})
	}

	// 产品分布统计
	packageTypeAgg, err := productsCollection.Aggregate(ctx, []bson.M{
		{"$group": bson.M{"_id": "$packageType", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计产品包装类型分布失败: %w", err))
		return
	}
	defer packageTypeAgg.Close(ctx)

	var packageTypeDistributionAggResults []aggResult
	if err = packageTypeAgg.All(ctx, &packageTypeDistributionAggResults); err != nil {
		utils.HandleError(c, fmt.Errorf("解析产品包装类型分布失败: %w", err))
		return
	}

	var packageTypeDistribution []models.ChartDataItem
	for _, res := range packageTypeDistributionAggResults {
		packageTypeDistribution = append(packageTypeDistribution, models.ChartDataItem{
			Name:  res.ID,
			Value: res.Count,
		})
	}

	// 产品库存等级分布
	stockLevels := []struct {
		level string
		min   int
		max   int
	}{
		{"库存不足(0-5k)", 0, 5000},
		{"库存适中(5k-3w)", 5000, 30000},
		{"库存充足(3w+)", 30001, math.MaxInt32},
	}

	var stockLevelDistribution []models.ChartDataItem
	for _, level := range stockLevels {
		count, err := productsCollection.CountDocuments(ctx, bson.M{
			"stock": bson.M{"$gte": level.min, "$lte": level.max},
		})
		if err != nil {
			utils.HandleError(c, fmt.Errorf("统计产品库存等级失败: %w", err))
			return
		}

		if count > 0 {
			stockLevelDistribution = append(stockLevelDistribution, models.ChartDataItem{
				Name:  level.level,
				Value: int(count),
			})
		}
	}

	// 产品客户关联数量分布
	productCustomerRelation, err := getProductCustomerRelation(ctx, baseProjectQuery, productsCollection, projectsCollection)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取产品客户关联失败: %w", err))
		return
	}

	// 产品项目关联数量分布
	productProjectRelation, err := getProductProjectRelation(ctx, baseProjectQuery, productsCollection, projectsCollection)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取产品项目关联失败: %w", err))
		return
	}

	// 项目进展分布
	projectProgressAgg, err := projectsCollection.Aggregate(ctx, []bson.M{
		{"$match": baseProjectQuery},
		{"$group": bson.M{"_id": "$projectProgress", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		utils.HandleError(c, fmt.Errorf("统计项目进展分布失败: %w", err))
		return
	}
	defer projectProgressAgg.Close(ctx)

	var projectProgressDistributionAggResults []aggResult
	if err = projectProgressAgg.All(ctx, &projectProgressDistributionAggResults); err != nil {
		utils.HandleError(c, fmt.Errorf("解析项目进展分布失败: %w", err))
		return
	}
	var projectProgressDistribution []models.ChartDataItem
	for _, res := range projectProgressDistributionAggResults {
		projectProgressDistribution = append(projectProgressDistribution, models.ChartDataItem{
			Name:  res.ID,
			Value: res.Count,
		})
	}

	// 批量总额统计
	projectBatchTotalStats, err := getProjectBatchTotalStats(ctx, baseProjectQuery, projectsCollection)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取批量总额统计失败: %w", err))
		return
	}

	// 小批量总额统计
	projectSmallBatchTotalStats, err := getProjectSmallBatchTotalStats(ctx, baseProjectQuery, projectsCollection)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取小批量总额统计失败: %w", err))
		return
	}

	// 项目月度统计
	projectMonthlyStats, err := getProjectMonthlyStats(ctx, baseProjectQuery, projectsCollection)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取项目月度统计失败: %w", err))
		return
	}

	// 项目价值排行Top10
	topProjectsByValue, err := getTopProjectsByValue(ctx, baseProjectQuery, projectsCollection, productsCollection)
	if err != nil {
		utils.HandleError(c, fmt.Errorf("获取项目价值排行失败: %w", err))
		return
	}

	// 构建响应数据
	responseData := models.DashboardDataResponse{
		CustomerCount:               int(customerCount),
		ProductCount:                int(productCount),
		AgentCount:                  int(agentCount),
		ProjectCount:                int(projectCount),
		CustomerImportance:          importanceDistribution,
		CustomerNature:              natureDistribution,
		ProductPackageType:          packageTypeDistribution,
		ProductStockLevel:           stockLevelDistribution,
		ProductCustomerRelation:     productCustomerRelation,
		ProductProjectRelation:      productProjectRelation,
		ProjectProgressDistribution: projectProgressDistribution,
		ProjectBatchTotalStats:      projectBatchTotalStats,
		ProjectSmallBatchTotalStats: projectSmallBatchTotalStats,
		ProjectMonthlyStats:         projectMonthlyStats,
		TopProjectsByValue:          topProjectsByValue,
	}

	utils.LogInfo(map[string]interface{}{
		"customerCount": customerCount,
		"productCount":  productCount,
		"agentCount":    agentCount,
		"projectCount":  projectCount,
		"batchTotal":    projectBatchTotalStats.TotalAmount,
		"smallBatch":    projectSmallBatchTotalStats.TotalAmount,
	}, "[数据看板] 统计完成")

	c.JSON(http.StatusOK, responseData)
}

// getProductCustomerRelation 获取产品客户关联数量分布
func getProductCustomerRelation(ctx context.Context, baseProjectQuery bson.M,
	productsCollection, projectsCollection *mongo.Collection) ([]models.ChartDataItem, error) {

	cursor, err := productsCollection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"modelName":   1,
		"packageType": 1,
	}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var products []models.Product
	if err = cursor.All(ctx, &products); err != nil {
		return nil, err
	}

	var productRelationData []models.ChartDataItem
	for _, product := range products {
		distinctCustomers, err := projectsCollection.Distinct(ctx, "customerId", bson.M{
			"productId": product.ID,
			"$and":      []bson.M{baseProjectQuery},
		})

		if err != nil {
			return nil, err
		}

		customerCount := len(distinctCustomers)
		if customerCount > 0 {
			productName := product.ModelName
			if len(productName) > 6 {
				productName = productName[:6] + "..."
			}

			productRelationData = append(productRelationData, models.ChartDataItem{
				Name:  productName,
				Value: customerCount,
			})
		}
	}

	// 排序并取前10项
	sort.Slice(productRelationData, func(i, j int) bool {
		return productRelationData[i].Value > productRelationData[j].Value
	})

	if len(productRelationData) > 10 {
		productRelationData = productRelationData[:10]
	}

	return productRelationData, nil
}

// getProductProjectRelation 获取产品项目关联数量分布
func getProductProjectRelation(ctx context.Context, baseProjectQuery bson.M,
	productsCollection, projectsCollection *mongo.Collection) ([]models.ChartDataItem, error) {

	cursor, err := productsCollection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"modelName":   1,
		"packageType": 1,
	}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var products []models.Product
	if err = cursor.All(ctx, &products); err != nil {
		return nil, err
	}

	var productProjectData []models.ChartDataItem

	for _, product := range products {
		count, err := projectsCollection.CountDocuments(ctx, bson.M{
			"productId": product.ID,
			"$and":      []bson.M{baseProjectQuery},
		})
		if err != nil {
			return nil, err
		}

		if count > 0 {
			productName := product.ModelName
			if len(productName) > 8 {
				productName = productName[:8] + "..."
			}

			productProjectData = append(productProjectData, models.ChartDataItem{
				Name:  productName,
				Value: int(count),
			})
		}
	}

	// 排序并取前10项
	sort.Slice(productProjectData, func(i, j int) bool {
		return productProjectData[i].Value > productProjectData[j].Value
	})

	if len(productProjectData) > 10 {
		productProjectData = productProjectData[:10]
	}

	return productProjectData, nil
}

// getProjectBatchTotalStats 获取批量总额统计
func getProjectBatchTotalStats(ctx context.Context, baseProjectQuery bson.M,
	projectsCollection *mongo.Collection) (models.ProjectBatchStats, error) {

	query := bson.M{
		"massProductionTotal": bson.M{"$gt": 0},
		"$and":                []bson.M{baseProjectQuery},
	}

	cursor, err := projectsCollection.Find(ctx, query)
	if err != nil {
		return models.ProjectBatchStats{}, err
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err = cursor.All(ctx, &projects); err != nil {
		return models.ProjectBatchStats{}, err
	}

	if len(projects) == 0 {
		return models.ProjectBatchStats{
			TotalAmount:   0,
			TotalProjects: 0,
			MaxAmount:     0,
			MinAmount:     0,
		}, nil
	}

	totalAmount := 0.0
	maxAmount := 0.0
	minAmount := math.MaxFloat64
	amounts := make([]float64, 0, len(projects))

	for _, project := range projects {
		amount := safeNumber(project.MassProductionTotal)
		amounts = append(amounts, amount)
		totalAmount = safeAdd(totalAmount, amount)

		if amount > maxAmount {
			maxAmount = amount
		}
		if amount < minAmount {
			minAmount = amount
		}
	}

	return models.ProjectBatchStats{
		TotalAmount:   totalAmount,
		TotalProjects: len(projects),
		MaxAmount:     maxAmount,
		MinAmount:     minAmount,
	}, nil
}

// getProjectSmallBatchTotalStats 获取小批量总额统计
func getProjectSmallBatchTotalStats(ctx context.Context, baseProjectQuery bson.M,
	projectsCollection *mongo.Collection) (models.ProjectBatchStats, error) {

	query := bson.M{
		"smallBatchTotal": bson.M{"$gt": 0},
		"$and":            []bson.M{baseProjectQuery},
	}

	cursor, err := projectsCollection.Find(ctx, query)
	if err != nil {
		return models.ProjectBatchStats{}, err
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err = cursor.All(ctx, &projects); err != nil {
		return models.ProjectBatchStats{}, err
	}

	if len(projects) == 0 {
		return models.ProjectBatchStats{
			TotalAmount:   0,
			TotalProjects: 0,
			MaxAmount:     0,
			MinAmount:     0,
		}, nil
	}

	totalAmount := 0.0
	maxAmount := 0.0
	minAmount := math.MaxFloat64
	amounts := make([]float64, 0, len(projects))

	for _, project := range projects {
		amount := safeNumber(project.SmallBatchTotal)
		amounts = append(amounts, amount)
		totalAmount = safeAdd(totalAmount, amount)

		if amount > maxAmount {
			maxAmount = amount
		}
		if amount < minAmount {
			minAmount = amount
		}
	}

	return models.ProjectBatchStats{
		TotalAmount:   totalAmount,
		TotalProjects: len(projects),
		MaxAmount:     maxAmount,
		MinAmount:     minAmount,
	}, nil
}

// getProjectMonthlyStats 获取项目月度统计
func getProjectMonthlyStats(ctx context.Context, baseProjectQuery bson.M,
	projectsCollection *mongo.Collection) ([]models.ProjectMonthlyStats, error) {

	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	query := bson.M{
		"createdAt": bson.M{"$gte": oneYearAgo},
		"$and":      []bson.M{baseProjectQuery},
	}

	pipeline := []bson.M{
		{"$match": query},
		{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$createdAt"},
				"month": bson.M{"$month": "$createdAt"},
			},
			"projectCount":    bson.M{"$sum": 1},
			"totalBatch":      bson.M{"$sum": "$massProductionTotal"},
			"totalSmallBatch": bson.M{"$sum": "$smallBatchTotal"},
		}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
	}

	cursor, err := projectsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type monthlyAggResult struct {
		ID struct {
			Year  int `bson:"year"`
			Month int `bson:"month"`
		} `bson:"_id"`
		ProjectCount    int     `bson:"projectCount"`
		TotalBatch      float64 `bson:"totalBatch"`
		TotalSmallBatch float64 `bson:"totalSmallBatch"`
	}

	var monthlyData []monthlyAggResult
	if err = cursor.All(ctx, &monthlyData); err != nil {
		return nil, err
	}

	// 创建月份到数据的映射
	monthMap := make(map[string]monthlyAggResult)
	for _, data := range monthlyData {
		monthKey := fmt.Sprintf("%d.%02d", data.ID.Year, data.ID.Month)
		monthMap[monthKey] = data
	}

	// 生成近12个月的完整数据
	now := time.Now()
	completeMonthlyData := make([]models.ProjectMonthlyStats, 0, 12)

	for i := 11; i >= 0; i-- {
		targetDate := now.AddDate(0, -i, 0)
		targetKey := fmt.Sprintf("%d.%02d", targetDate.Year(), targetDate.Month())

		if data, exists := monthMap[targetKey]; exists {
			totalAmount := safeAdd(data.TotalBatch, data.TotalSmallBatch)
			completeMonthlyData = append(completeMonthlyData, models.ProjectMonthlyStats{
				Month:            targetKey,
				ProjectCount:     data.ProjectCount,
				TotalAmount:      totalAmount,
				BatchAmount:      data.TotalBatch,
				SmallBatchAmount: data.TotalSmallBatch,
			})
		} else {
			completeMonthlyData = append(completeMonthlyData, models.ProjectMonthlyStats{
				Month:            targetKey,
				ProjectCount:     0,
				TotalAmount:      0,
				BatchAmount:      0,
				SmallBatchAmount: 0,
			})
		}
	}

	return completeMonthlyData, nil
}

// getTopProjectsByValue 获取项目价值排行Top10
func getTopProjectsByValue(ctx context.Context, baseProjectQuery bson.M,
	projectsCollection, productsCollection *mongo.Collection) ([]models.ProjectValueItem, error) {

	cursor, err := projectsCollection.Find(ctx, baseProjectQuery)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err = cursor.All(ctx, &projects); err != nil {
		return nil, err
	}

	// 计算每个项目的总价值并排序
	projectsWithValue := make([]models.ProjectValueItem, 0, len(projects))
	for _, project := range projects {
		batchTotal := safeNumber(project.MassProductionTotal)
		smallBatchTotal := safeNumber(project.SmallBatchTotal)
		totalValue := safeAdd(batchTotal, smallBatchTotal)

		if totalValue > 0 {
			projectsWithValue = append(projectsWithValue, models.ProjectValueItem{
				ProjectID:           project.ID.Hex(),
				ProjectName:         project.ProjectName,
				CustomerName:        project.CustomerName,
				ProductID:           project.ProductID.Hex(),
				SmallBatchTotal:     smallBatchTotal,
				MassProductionTotal: batchTotal,
				TotalValue:          totalValue,
				Progress:            string(project.ProjectProgress),
			})
		}
	}

	// 按总价值降序排序
	sort.Slice(projectsWithValue, func(i, j int) bool {
		return projectsWithValue[i].TotalValue > projectsWithValue[j].TotalValue
	})

	// 只取前10个
	if len(projectsWithValue) > 10 {
		projectsWithValue = projectsWithValue[:10]
	}

	// 获取产品信息
	productIDs := make([]primitive.ObjectID, 0, len(projectsWithValue))
	for _, project := range projectsWithValue {
		if id, err := primitive.ObjectIDFromHex(project.ProductID); err == nil {
			productIDs = append(productIDs, id)
		}
	}

	productMap := make(map[string]models.Product)
	if len(productIDs) > 0 {
		cursor, err := productsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": productIDs}})
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)

		var products []models.Product
		if err = cursor.All(ctx, &products); err != nil {
			return nil, err
		}

		for _, product := range products {
			productMap[product.ID.Hex()] = product
		}
	}

	// 添加产品名称
	for i := range projectsWithValue {
		if product, exists := productMap[projectsWithValue[i].ProductID]; exists {
			projectsWithValue[i].ProductName = fmt.Sprintf("%s/%s", product.ModelName, product.PackageType)
		} else {
			projectsWithValue[i].ProductName = "未知产品"
		}
	}

	return projectsWithValue, nil
}
