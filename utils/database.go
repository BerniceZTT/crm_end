// utils/database.go
package utils

import (
	"fmt"
	"time"
)

// ExecuteInventoryOperation 执行库存操作，提供幂等性和错误重试
func ExecuteInventoryOperation(checkExists func() (bool, error), operation func() (interface{}, error), retries int) (map[string]interface{}, error) {
	// 检查操作是否已完成
	exists, err := checkExists()
	if err != nil {
		return nil, err
	}

	if exists {
		return map[string]interface{}{
			"success":          true,
			"alreadyCompleted": true,
		}, nil
	}

	// 执行操作，如果失败则重试
	var lastError error
	for i := 0; i <= retries; i++ {
		result, err := operation()
		if err == nil {
			return map[string]interface{}{
				"success": true,
				"result":  result,
			}, nil
		}

		lastError = err
		LogInfo(map[string]interface{}{
			"error":    err.Error(),
			"attempt":  i + 1,
			"maxRetry": retries,
		}, "库存操作失败，准备重试")

		// 最后一次失败不需要等待
		if i < retries {
			time.Sleep(1 * time.Second)
		}
	}

	// 所有重试都失败
	return map[string]interface{}{
		"success":         false,
		"error":           lastError.Error(),
		"statusUncertain": true,
	}, lastError
}

// LogInventoryOperation 记录库存操作日志
func LogInventoryOperation(operation, productID string, quantity int, success bool) {
	status := "成功"
	if !success {
		status = "状态不确定"
	}

	LogInfo(map[string]interface{}{
		"productId": productID,
		"quantity":  quantity,
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
	}, fmt.Sprintf("库存操作: %s", operation))
}
