package service

import (
	"fmt"
	"log"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 每天指定时间执行任务
func ScheduleDailyTaskAt(hour, min, sec int, task func()) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			duration := next.Sub(now)
			time.Sleep(duration)
			task()
		}
	}()
}

// 处理初始联系客户
func ProcessInitialContactCustomers() {
	now := time.Now()
	log.Printf("开始执行每日初始联系客户检查任务..., time: %v", now)

	ctx := repository.GetContext()

	// 1. 获取所有处于初始联系状态的客户
	customersCollection := repository.Collection(repository.CustomersCollection)
	customersCursor, err := customersCollection.Find(ctx, bson.M{
		"progress": models.CustomerProgressInitialContact,
	})
	if err != nil {
		log.Printf("查询客户失败: %v", err)
		return
	}
	defer customersCursor.Close(ctx)

	var customers []models.Customer
	if err := customersCursor.All(ctx, &customers); err != nil {
		log.Printf("解析客户数据失败: %v", err)
		return
	}

	// 2. 获取自动转移配置
	query := bson.M{
		"configType": "customer_auto_transfer",
		"isEnabled":  true,
	}

	systemConfigsCollection := repository.Collection(repository.SystemConfigsyCollection)
	systemConfigsCursor, err := systemConfigsCollection.Find(ctx, query)
	if err != nil {
		log.Printf("查询系统配置失败: %v", err)
		return
	}

	var configs []models.SystemConfig
	if err := systemConfigsCursor.All(ctx, &configs); err != nil {
		log.Printf("解析系统配置失败: %v", err)
		return
	}

	// 检查是否有有效配置
	if len(configs) == 0 {
		log.Printf("未找到有效的自动转移配置")
		return
	}

	config, err := GetAutoTransferConfig(configs[0])
	if err != nil {
		log.Printf("解析自动转移配置失败: %v", err)
		return
	}

	if config == nil {
		log.Printf("自动转移配置为空")
		return
	}

	log.Printf("自动转移配置: 目标销售ID=%s, 目标销售姓名=%s, 无进展天数=%d",
		config.TargetSalesID, config.TargetSalesName, config.DaysWithoutProgress)

	// 3. 处理每个客户
	for _, customer := range customers {
		// 确定参考时间 (InitialContactTime 或 CreatedAt)
		referenceTime := customer.InitialContactTime
		if referenceTime.IsZero() {
			referenceTime = customer.CreatedAt
		}

		// 计算时间差
		duration := now.Sub(referenceTime)
		daysWithoutProgress := int(duration.Hours() / 24)

		// 判断是否超过配置的天数
		if daysWithoutProgress >= config.DaysWithoutProgress {
			log.Printf("客户 [ID=%s, 名称=%s] 已超过无进展天数限制: %d天 (参考时间: %v)",
				customer.ID.Hex(), customer.Name, daysWithoutProgress, referenceTime)

			assignRequest := AssignRequest{
				SalesId: config.TargetSalesID,
				AgentId: "",
			}

			user := &utils.LoginUser{
				ID:       "68405ea56e4eb64aa7ed3cb3",
				Role:     "SUPER_ADMIN",
				Username: "admin",
			}
			if customer.RelatedAgentID == "" && customer.RelatedSalesID == config.TargetSalesID {
				continue
			}
			err1, _, _ := AssignCustomer(ctx, nil, customer.ID.Hex(), assignRequest, user)
			if err1 != nil {
				log.Printf("客户转移失败，客户:%v, to:%v, %v", customer.Name, config.TargetSalesName, config.TargetSalesID)
			}
		}
	}

	log.Printf("每日初始联系客户检查任务完成, 共检查了 %d 个客户", len(customers))
}

func GetAutoTransferConfig(config models.SystemConfig) (*models.AutoTransferConfig, error) {
	// 方法1：尝试直接转换为 bson.D
	if doc, ok := config.ConfigValue.(bson.D); ok {
		return parseFromBSOND(doc)
	}

	// 方法2：尝试转换为 bson.M
	if m, ok := config.ConfigValue.(bson.M); ok {
		return parseFromMap(m)
	}

	// 方法3：尝试转换为 primitive.D
	if doc, ok := config.ConfigValue.(primitive.D); ok {
		return parseFromPrimitiveD(doc)
	}

	// 方法4：尝试转换为 primitive.M
	if m, ok := config.ConfigValue.(primitive.M); ok {
		return parseFromMap(m)
	}

	// 方法5：最通用的处理方式 - 通过 BSON 序列化/反序列化
	data, err := bson.Marshal(config.ConfigValue)
	if err != nil {
		return nil, fmt.Errorf("BSON 序列化失败: %v", err)
	}

	var resultMap map[string]interface{}
	if err := bson.Unmarshal(data, &resultMap); err == nil {
		return parseFromMap(resultMap)
	}

	// 如果所有方法都失败
	return nil, fmt.Errorf("无法解析 ConfigValue，实际类型: %T", config.ConfigValue)
}

// 从 bson.D 解析
func parseFromBSOND(doc bson.D) (*models.AutoTransferConfig, error) {
	result := &models.AutoTransferConfig{}
	for _, elem := range doc {
		switch elem.Key {
		case "targetSalesId":
			result.TargetSalesID = elem.Value.(string)
		case "targetSalesName":
			result.TargetSalesName = elem.Value.(string)
		case "daysWithoutProgress":
			switch v := elem.Value.(type) {
			case int32:
				result.DaysWithoutProgress = int(v)
			case int64:
				result.DaysWithoutProgress = int(v)
			case float64:
				result.DaysWithoutProgress = int(v)
			case int:
				result.DaysWithoutProgress = v
			default:
				return nil, fmt.Errorf("无效的 daysWithoutProgress 类型: %T", v)
			}
		}
	}
	return validateConfig(result)
}

// 从 primitive.D 解析
func parseFromPrimitiveD(doc primitive.D) (*models.AutoTransferConfig, error) {
	result := &models.AutoTransferConfig{}
	for _, elem := range doc {
		switch elem.Key {
		case "targetSalesId":
			result.TargetSalesID = elem.Value.(string)
		case "targetSalesName":
			result.TargetSalesName = elem.Value.(string)
		case "daysWithoutProgress":
			switch v := elem.Value.(type) {
			case int32:
				result.DaysWithoutProgress = int(v)
			case int64:
				result.DaysWithoutProgress = int(v)
			case float64:
				result.DaysWithoutProgress = int(v)
			case int:
				result.DaysWithoutProgress = v
			default:
				return nil, fmt.Errorf("无效的 daysWithoutProgress 类型: %T", v)
			}
		}
	}
	return validateConfig(result)
}

// 从 map 解析
func parseFromMap(m map[string]interface{}) (*models.AutoTransferConfig, error) {
	result := &models.AutoTransferConfig{}

	if val, ok := m["targetSalesId"].(string); ok {
		result.TargetSalesID = val
	}
	if val, ok := m["targetSalesName"].(string); ok {
		result.TargetSalesName = val
	}
	if val, ok := m["daysWithoutProgress"].(int); ok {
		result.DaysWithoutProgress = val
	} else if val, ok := m["daysWithoutProgress"].(int32); ok {
		result.DaysWithoutProgress = int(val)
	} else if val, ok := m["daysWithoutProgress"].(int64); ok {
		result.DaysWithoutProgress = int(val)
	} else if val, ok := m["daysWithoutProgress"].(float64); ok {
		result.DaysWithoutProgress = int(val)
	}

	return validateConfig(result)
}

// 验证配置是否完整
func validateConfig(config *models.AutoTransferConfig) (*models.AutoTransferConfig, error) {
	if config.TargetSalesID == "" {
		return nil, fmt.Errorf("缺少 targetSalesId")
	}
	if config.TargetSalesName == "" {
		return nil, fmt.Errorf("缺少 targetSalesName")
	}
	if config.DaysWithoutProgress <= 0 {
		return nil, fmt.Errorf("daysWithoutProgress 必须大于0")
	}
	return config, nil
}
