package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"server2/models"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// IsValidPhone 验证手机号是否有效
func IsValidPhone(phone string) bool {
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

type LoginUser struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Username string `json:"name"`
}

func GetUser(c *gin.Context) (*LoginUser, error) {
	// 获取当前用户信息
	currentUser, exists := c.Get("user")
	c1, _ := json.Marshal(c)
	Logger.Info().
		Str("c1", string(c1)).
		Msg("GetUser")

	if !exists {
		return nil, fmt.Errorf("GetUser 未授权访问")
	}

	// 处理不同类型的 claims
	var claims map[string]interface{}
	switch v := currentUser.(type) {
	case jwt.MapClaims:
		// 如果是 jwt.MapClaims，转换为 map[string]interface{}
		claims = make(map[string]interface{})
		for key, val := range v {
			claims[key] = val
		}
	case map[string]interface{}:
		// 已经是 map[string]interface{} 类型
		claims = v
	case string:
		// 如果是字符串，尝试解析为 JSON
		if err := json.Unmarshal([]byte(v), &claims); err != nil {
			return nil, fmt.Errorf("解析用户信息失败: %v", err)
		}
	default:
		// 尝试通过 JSON 序列化/反序列化转换
		data, err := json.Marshal(currentUser)
		if err != nil {
			return nil, fmt.Errorf("序列化用户信息失败: %v", err)
		}
		if err := json.Unmarshal(data, &claims); err != nil {
			return nil, fmt.Errorf("反序列化用户信息失败: %v", err)
		}
	}

	// 获取用户信息字段
	id, ok := claims["id"].(string)
	if !ok {
		return nil, fmt.Errorf("无效的用户ID")
	}

	role, ok := claims["role"].(string)
	if !ok {
		return nil, fmt.Errorf("无效的用户角色")
	}

	username, ok := claims["username"].(string)
	if !ok {
		// 检查是否有 "name" 字段作为备选
		if name, ok := claims["name"].(string); ok {
			username = name
		} else {
			return nil, fmt.Errorf("无效的用户名")
		}
	}
	// 调试日志 - 打印接收到的数据
	Logger.Info().
		Str("id", id).
		Str("role", role).
		Str("username", username).
		Msg("GetUser")
	return &LoginUser{
		ID:       id,
		Role:     role,
		Username: username,
	}, nil
}

func PaginatedResponse(c *gin.Context, data interface{}, total int64, page int64, limit int64) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
		"pagination": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
			"pages": (total + limit - 1) / limit,
		},
	})
}

func CanAssignPublicPoolCustomer(role string) bool {
	return role == string(models.UserRoleSUPER_ADMIN) ||
		role == string(models.UserRoleFACTORY_SALES) ||
		role == string(models.UserRoleAGENT)
}

func InventoryOperationResponse(c *gin.Context, data interface{}, success bool) {
	status := 200
	if !success {
		status = 202
	}

	if success {
		c.JSON(status, gin.H{
			"success": true,
			"message": "库存操作成功",
			"data":    data,
		})
		return
	}

	// 状态不确定
	errorMsg := "无法确认库存操作是否成功"
	if dataMap, ok := data.(map[string]interface{}); ok {
		if errMsg, exists := dataMap["error"]; exists {
			errorMsg = errMsg.(string)
		}
	}

	c.JSON(status, gin.H{
		"success": false,
		"warning": "库存操作状态不确定，请刷新页面查看最新库存",
		"error":   errorMsg,
	})
}
