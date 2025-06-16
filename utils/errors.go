package utils

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ApiError 自定义API错误
type ApiError struct {
	StatusCode int
	Message    string
	ErrorCode  string
}

// Error 实现error接口
func (e *ApiError) Error() string {
	return e.Message
}

// NewApiError 创建API错误
func NewApiError(message string, statusCode int, errorCode string) *ApiError {
	return &ApiError{
		StatusCode: statusCode,
		Message:    message,
		ErrorCode:  errorCode,
	}
}

// CreateNotFoundError 创建资源不存在错误
func CreateNotFoundError(resource string) *ApiError {
	return NewApiError(resource+"不存在", http.StatusNotFound, "RESOURCE_NOT_FOUND")
}

// CreateUnauthorizedError 创建未授权错误
func CreateUnauthorizedError() *ApiError {
	return NewApiError("未授权访问", http.StatusUnauthorized, "UNAUTHORIZED")
}

// CreateForbiddenError 创建权限不足错误
func CreateForbiddenError() *ApiError {
	return NewApiError("权限不足", http.StatusForbidden, "FORBIDDEN")
}

// CreateBadRequestError 创建错误请求错误
func CreateBadRequestError(message string) *ApiError {
	return NewApiError(message, http.StatusBadRequest, "BAD_REQUEST")
}

// CreateUncertainOperationError 创建操作结果不确定错误
func CreateUncertainOperationError() *ApiError {
	return NewApiError(
		"操作状态不确定，请刷新页面查看最新状态",
		http.StatusInternalServerError,
		"UNCERTAIN_OPERATION",
	)
}

// HandleError 处理错误并返回适当的响应
func HandleError(c *gin.Context, err error) {
	if c == nil {
		return
	}
	// 记录错误
	errorMessage := err.Error()
	Logger.Error().Str("path", c.Request.URL.Path).Str("method", c.Request.Method).Msg("API错误: " + errorMessage)

	// 记录详细错误信息
	LogError(err, map[string]interface{}{
		"path":   c.Request.URL.Path,
		"method": c.Request.Method,
	}, errorMessage)

	// 处理API错误
	if apiErr, ok := err.(*ApiError); ok {
		response := gin.H{"error": apiErr.Message}
		if apiErr.ErrorCode != "" {
			response["code"] = apiErr.ErrorCode
		}
		c.JSON(apiErr.StatusCode, response)
		return
	}

	// 其他未预期的错误
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   errorMessage,
		"success": false,
	})
}

// SuccessResponse 成功响应
func SuccessResponse(c *gin.Context, data interface{}, message string, statusCode ...int) {
	code := http.StatusOK
	if len(statusCode) > 0 {
		code = statusCode[0]
	}

	response := gin.H{"success": true}
	if data != nil {
		response["data"] = data
	}
	if message != "" {
		response["message"] = message
	}

	c.JSON(code, response)
}

// ErrorResponse 错误响应
func ErrorResponse(c *gin.Context, message string, statusCode int) {
	c.JSON(statusCode, gin.H{
		"success": false,
		"error":   message,
	})
}

// AppError 应用错误类型
type AppError struct {
	Message    string
	StatusCode int
	Err        error
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap 返回底层错误
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError 创建新的应用错误
func NewAppError(message string, statusCode int, err error) *AppError {
	return &AppError{
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
	}
}
