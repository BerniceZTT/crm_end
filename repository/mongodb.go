package repository

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	// 集合名
	UsersCollection                  = "users"
	AgentsCollection                 = "agents"
	ProductsCollection               = "products"
	CustomersCollection              = "customers"
	InventoryCollection              = "inventory"
	FollowUpCollection               = "followUpRecords"
	CustAssignCollection             = "customerAssignmentHistory"
	CustomerProgressCollection       = "customer_progress"
	InventoryRecordsCollection       = "inventory_records"
	ApiOperationLogsCollection       = "apiOperationLogs"
	ProjectsCollection               = "projects"
	ProjectFollowUpRecordsCollection = "projectFollowUpRecord"
	ProjectProgressHistoryCollection = "projectProgressHistory"
)

var (
	client *mongo.Client
	db     *mongo.Database
	ctx    = context.Background()
	once   sync.Once
)

// InitMongoDB 初始化MongoDB连接
func InitMongoDB(uri, dbName string) error {
	// 设置连接超时
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 创建客户端
	var err error
	clientOptions := options.Client().ApplyURI(uri)
	client, err = mongo.Connect(connectCtx, clientOptions)
	if err != nil {
		return fmt.Errorf("连接MongoDB失败: %w", err)
	}

	// 检查连接
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return fmt.Errorf("ping MongoDB失败: %w", err)
	}

	// 选择数据库
	db = client.Database(dbName)
	utils.Logger.Info().Str("database", dbName).Msg("已连接到MongoDB")

	return nil
}

// CloseMongoDB 关闭MongoDB连接
func CloseMongoDB() {
	if client != nil {
		if err := client.Disconnect(ctx); err != nil {
			utils.Logger.Error().Err(err).Msg("断开MongoDB连接失败")
			return
		}
		utils.Logger.Info().Msg("已断开MongoDB连接")
	}
}

// ExecuteDbOperation 执行数据库操作，提供错误处理和重试机制
func ExecuteDbOperation(operation func() (interface{}, error), retries int) (interface{}, error) {
	if retries <= 0 {
		retries = 3
	}

	var lastErr error
	for i := 0; i < retries; i++ {
		result, err := operation()
		if err == nil {
			return result, nil
		}

		lastErr = err
		utils.Logger.Error().Err(err).Msgf("数据库操作失败，重试 (%d/%d)", i+1, retries)

		// 如果是不可重试的错误，立即返回
		if !isRetryableError(err) {
			break
		}

		// 延迟后重试
		time.Sleep(time.Duration(500*(i+1)) * time.Millisecond)
	}

	return nil, lastErr
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// MongoDB可重试错误代码
	retryableCodes := map[int]bool{
		6:     true, // HostUnreachable
		7:     true, // HostNotFound
		89:    true, // NetworkTimeout
		91:    true, // ShutdownInProgress
		189:   true, // PrimarySteppedDown
		10107: true, // NotMaster
		13436: true, // NotMasterNoSlaveOk
		11600: true, // InterruptedAtShutdown
		11602: true, // InterruptedDueToReplStateChange
		10058: true, // ConnectionReset
	}

	if cmdErr, ok := err.(mongo.CommandError); ok {
		return retryableCodes[int(cmdErr.Code)]
	}

	// 检查常见网络错误
	return isNetworkError(err)
}

// isNetworkError 检查是否是网络错误
func isNetworkError(err error) bool {
	errMsg := err.Error()
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection closed",
		"no reachable servers",
		"timeout",
		"context deadline exceeded",
		"server selection error",
	}

	for _, ne := range networkErrors {
		if containsIgnoreCase(errMsg, ne) {
			return true
		}
	}

	return false
}

// containsIgnoreCase 判断字符串是否包含子串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	s, substr = toLowerCase(s), toLowerCase(substr)
	return s != "" && substr != "" && contains(s, substr)
}

// toLowerCase 将字符串转为小写
func toLowerCase(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// InitializeCollections 初始化数据库集合
func InitializeCollections() error {
	collections := []string{
		UsersCollection,
		AgentsCollection,
		ProductsCollection,
		CustomersCollection,
		InventoryCollection,
		FollowUpCollection,
		CustAssignCollection,
		CustomerProgressCollection,
		InventoryRecordsCollection,
		ApiOperationLogsCollection,
		ProjectsCollection,
		ProjectFollowUpRecordsCollection,
		ProjectProgressHistoryCollection,
	}

	for _, collName := range collections {
		// 检查集合是否存在
		collExists, err := CollectionExists(collName)
		if err != nil {
			return fmt.Errorf("检查集合失败: %w", err)
		}

		// 如果不存在则创建
		if !collExists {
			if err := createCollection(collName); err != nil {
				return fmt.Errorf("创建集合失败: %w", err)
			}
			utils.Logger.Info().Str("collection", collName).Msg("创建集合成功")
		} else {
			utils.Logger.Info().Str("collection", collName).Msg("集合已存在")
		}
	}

	return nil
}

// CollectionExists 检查集合是否存在
func CollectionExists(collName string) (bool, error) {
	collections, err := db.ListCollectionNames(ctx, bson.M{"name": collName})
	if err != nil {
		return false, err
	}

	for _, name := range collections {
		if name == collName {
			return true, nil
		}
	}

	return false, nil
}

// createCollection 创建集合
func createCollection(collName string) error {
	return db.CreateCollection(ctx, collName)
}

// InitializeAdminAccount 初始化管理员账户
func InitializeAdminAccount() error {
	// 检查是否已存在超级管理员
	usersCollection := db.Collection(UsersCollection)

	count, err := usersCollection.CountDocuments(ctx, bson.M{"role": models.UserRoleSUPER_ADMIN})
	if err != nil {
		return fmt.Errorf("检查管理员账户失败: %w", err)
	}

	// 如果已存在，则不创建
	if count > 0 {
		utils.Logger.Info().Msg("超级管理员账户已存在，跳过创建")
		return nil
	}

	// 创建默认管理员
	adminUser := models.User{
		Username:  "admin",
		Password:  utils.HashPassword("admin123"),
		Phone:     "13800000000",
		Role:      models.UserRoleSUPER_ADMIN,
		Status:    models.UserStatusAPPROVED,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = usersCollection.InsertOne(ctx, adminUser)
	if err != nil {
		return fmt.Errorf("创建管理员账户失败: %w", err)
	}

	utils.Logger.Info().Msg("已创建默认超级管理员账户")
	return nil
}

// GetDatabaseStatus 获取数据库状态
func GetDatabaseStatus() (map[string]interface{}, error) {
	collections := []string{
		UsersCollection,
		AgentsCollection,
		ProductsCollection,
		CustomersCollection,
		InventoryCollection,
		FollowUpCollection,
		CustAssignCollection,
		CustomerProgressCollection,
		InventoryRecordsCollection,
		ApiOperationLogsCollection,
		ProjectsCollection,
		ProjectFollowUpRecordsCollection,
		ProjectProgressHistoryCollection,
	}

	result := make(map[string]interface{})

	for _, collName := range collections {
		coll := db.Collection(collName)
		count, err := coll.CountDocuments(ctx, bson.M{})
		if err != nil {
			utils.Logger.Error().Err(err).Str("collection", collName).Msg("获取集合计数失败")
			result[collName] = map[string]interface{}{
				"count": 0,
				"error": err.Error(),
			}
		} else {
			result[collName] = map[string]interface{}{
				"count": count,
			}

			// 获取一个样本文档
			if count > 0 {
				var sample bson.M
				err := coll.FindOne(ctx, bson.M{}).Decode(&sample)
				if err == nil {
					// 移除敏感字段
					delete(sample, "password")
					result[collName].(map[string]interface{})["sample"] = sample
				}
			}
		}
	}

	return result, nil
}

// FindUserByID 根据ID查找用户
func FindUserByID(id string) (*models.User, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("无效的ID格式: %w", err)
	}

	var user models.User
	err = db.Collection(UsersCollection).FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, err
	}

	return &user, nil
}

// GetDB 返回MongoDB数据库实例
func GetDB() *mongo.Database {
	once.Do(func() {
		// 创建一个可取消的上下文，用于MongoDB操作
		ctx = context.Background()

		// 设置连接选项
		clientOptions := options.Client().ApplyURI(
			"mongodb://qianxin:QianXin123@47.113.230.108:27017/crm?authSource=admin",
		)

		// 连接到MongoDB
		var err error
		client, err = mongo.Connect(ctx, clientOptions)
		if err != nil {
			log.Fatal("MongoDB连接失败:", err)
		}

		// 设置连接超时
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// 测试连接
		err = client.Ping(pingCtx, nil)
		if err != nil {
			log.Fatal("MongoDB ping失败:", err)
		}

		// 选择数据库
		db = client.Database("crm")

		log.Println("MongoDB连接成功")
	})

	return db
}

// GetContext 返回MongoDB操作的上下文
func GetContext() context.Context {
	if ctx == nil {
		GetDB() // 确保初始化
	}
	return ctx
}

// Collection 返回指定名称的集合
func Collection(name string) *mongo.Collection {
	return GetDB().Collection(name)
}

// Close 关闭MongoDB连接
func Close() {
	if client != nil {
		if err := client.Disconnect(ctx); err != nil {
			log.Println("MongoDB断开连接失败:", err)
		}
	}
}
