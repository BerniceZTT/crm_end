package controllers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
)

// GetInventoryRecords 获取库存操作记录
func GetInventoryRecords(c *gin.Context) {
	// 验证用户权限
	// 获取当前用户信息
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	if user.Role != string(models.UserRoleSUPER_ADMIN) && user.Role != string(models.UserRoleINVENTORY_MANAGER) {
		utils.ErrorResponse(c, "无权查看库存记录", http.StatusForbidden)
		return
	}

	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.ParseInt(pageStr, 10, 64)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// skip := (page - 1) * limit

	// 筛选条件
	searchQuery := bson.M{}

	// 按产品ID搜索
	if productID := c.Query("productId"); productID != "" {
		searchQuery["productId"] = productID
	}

	// 按产品型号搜索
	if modelName := c.Query("modelName"); modelName != "" {
		searchQuery["modelName"] = bson.M{"$regex": modelName, "$options": "i"}
	}

	// 按操作类型筛选
	if operationType := c.Query("operationType"); operationType != "" && operationType != "all" {
		searchQuery["operationType"] = operationType
	}

	// 时间范围筛选
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	if startDateStr != "" && endDateStr != "" {
		// 转换为日期对象
		startDate, err := time.Parse(time.RFC3339, startDateStr+"T00:00:00Z")
		if err != nil {
			utils.HandleError(c, err)
			return
		}

		// 结束日期设置为当天的23:59:59
		endDate, err := time.Parse(time.RFC3339, endDateStr+"T23:59:59Z")
		if err != nil {
			utils.HandleError(c, err)
			return
		}

		searchQuery["operationTime"] = bson.M{"$gte": startDate, "$lte": endDate}
		utils.LogInfo(map[string]interface{}{
			"startDate": startDate.Format(time.RFC3339),
			"endDate":   endDate.Format(time.RFC3339),
		}, "使用自定义日期范围")
	} else {
		// 使用天数
		days := 30
		if daysStr := c.Query("days"); daysStr != "" {
			if d, err := strconv.Atoi(daysStr); err == nil {
				days = d
			}
		}

		fromDate := time.Now().AddDate(0, 0, -days)
		searchQuery["operationTime"] = bson.M{"$gte": fromDate}
		utils.LogInfo(map[string]interface{}{
			"days": days,
		}, "使用最近天数筛选")
	}

	utils.LogInfo(map[string]interface{}{
		"query": searchQuery,
	}, "库存记录查询条件")

	// 查询数据库
	collection := repository.Collection(repository.InventoryRecordsCollection)

	// 获取总数
	ctx := context.Background()
	total, err := collection.CountDocuments(ctx, searchQuery)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 设置排序和分页
	findOptions := options.Find().
		SetSort(bson.M{"operationTime": -1})
		// .SetSkip(skip).
		// SetLimit(limit)

	// 查询数据
	cursor, err := collection.Find(ctx, searchQuery, findOptions)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer cursor.Close(ctx)

	var records []models.InventoryRecord
	if err := cursor.All(ctx, &records); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.LogInfo(map[string]interface{}{
		"recordsCount": len(records),
		"totalRecords": total,
	}, "查询库存记录")

	// 返回结果
	utils.PaginatedResponse(c, records, total, page, limit)
}

// GetInventoryStats 获取库存统计信息
func GetInventoryStats(c *gin.Context) {
	user, err := utils.GetUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	if user.Role != string(models.UserRoleSUPER_ADMIN) && user.Role != string(models.UserRoleINVENTORY_MANAGER) {
		utils.ErrorResponse(c, "无权查看库存统计", http.StatusForbidden)
		return
	}

	ctx := context.Background()
	productsCollection := repository.Collection(repository.ProductsCollection)

	// 获取总产品数
	totalProducts, err := productsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 获取低库存产品数
	lowStockProducts, err := productsCollection.CountDocuments(ctx, bson.M{"stock": bson.M{"$lt": 50}})
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	// 获取总库存量
	var stockResult []bson.M
	stockPipeline := mongo.Pipeline{
		{{"$group", bson.M{"_id": nil, "totalStock": bson.M{"$sum": "$stock"}}}},
	}

	stockCursor, err := productsCollection.Aggregate(ctx, stockPipeline)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer stockCursor.Close(ctx)

	if err := stockCursor.All(ctx, &stockResult); err != nil {
		utils.HandleError(c, err)
		return
	}

	totalStock := int64(0)
	if len(stockResult) > 0 {
		if total, ok := stockResult[0]["totalStock"].(int64); ok {
			totalStock = total
		}
	}

	// 获取最近30天的库存变动
	recordsCollection := repository.Collection(repository.InventoryRecordsCollection)
	fromDate := time.Now().AddDate(0, 0, -30)

	// 获取入库操作总量
	var inOperations []bson.M
	inPipeline := mongo.Pipeline{
		{{"$match", bson.M{"operationType": "in", "operationTime": bson.M{"$gte": fromDate}}}},
		{{"$group", bson.M{"_id": nil, "total": bson.M{"$sum": "$quantity"}}}},
	}

	inCursor, err := recordsCollection.Aggregate(ctx, inPipeline)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer inCursor.Close(ctx)

	if err := inCursor.All(ctx, &inOperations); err != nil {
		utils.HandleError(c, err)
		return
	}

	// 获取出库操作总量
	var outOperations []bson.M
	outPipeline := mongo.Pipeline{
		{{"$match", bson.M{"operationType": "out", "operationTime": bson.M{"$gte": fromDate}}}},
		{{"$group", bson.M{"_id": nil, "total": bson.M{"$sum": "$quantity"}}}},
	}

	outCursor, err := recordsCollection.Aggregate(ctx, outPipeline)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer outCursor.Close(ctx)

	if err := outCursor.All(ctx, &outOperations); err != nil {
		utils.HandleError(c, err)
		return
	}

	totalIn := int64(0)
	if len(inOperations) > 0 {
		if total, ok := inOperations[0]["total"].(int64); ok {
			totalIn = total
		}
	}

	totalOut := int64(0)
	if len(outOperations) > 0 {
		if total, ok := outOperations[0]["total"].(int64); ok {
			totalOut = total
		}
	}

	// 返回结果
	stats := models.InventoryStats{
		TotalProducts:    totalProducts,
		LowStockProducts: lowStockProducts,
		TotalStock:       totalStock,
		RecentChanges: models.RecentChanges{
			In:  totalIn,
			Out: totalOut,
			Net: totalIn - totalOut,
		},
	}

	utils.SuccessResponse(c, stats, "", http.StatusOK)
}
