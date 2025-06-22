package utils

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Logger 全局日志对象
var Logger zerolog.Logger

// InitLogger 初始化日志系统
func InitLogger() {
	// 配置日志输出
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}

	// 创建日志记录器
	Logger = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger().
		Level(zerolog.InfoLevel)

	// 设置日志级别
	if os.Getenv("GIN_MODE") == "debug" {
		Logger = Logger.Level(zerolog.DebugLevel)
	}

	Logger.Info().Msg("日志系统初始化完成")
}

// LogApiRequest 记录API请求
func LogApiRequest(method, url string, params, body interface{}, headers map[string]string) {
	// 过滤敏感信息
	if headers != nil && headers["Authorization"] != "" {
		if len(headers["Authorization"]) > 15 {
			headers["Authorization"] = headers["Authorization"][:15] + "..."
		}
	}

	Logger.Info().
		Str("method", method).
		Str("url", url).
		Interface("params", params).
		Interface("body", body).
		Interface("headers", headers).
		Msg("API请求")
}

// LogApiResponse 记录API响应
func LogApiResponse(method, url string, statusCode int, responseTime time.Duration, responseBody interface{}) {
	event := Logger.Info()
	if statusCode >= 400 {
		event = Logger.Error()
	}
	if strings.Contains(url, "/api/projects/") {
		return
	}
	event.
		Str("method", method).
		Str("url", url).
		Int("statusCode", statusCode).
		Dur("responseTime", responseTime).
		Interface("body", responseBody).
		Msg("API响应")
}

// LogInfo 记录
func LogInfo(context map[string]interface{}, message string) {
	Logger.Info().
		Interface("context", context).
		Msg(message)
}

// LogError 记录错误
func LogError(err error, context map[string]interface{}, message string) {
	Logger.Error().
		Err(err).
		Interface("context", context).
		Msg("错误")
}

func LogError2(message string, err error, context map[string]interface{}) {
	Logger.Error().
		Err(err).
		Interface("context", context).
		Msg(message)
}

// LogDbOperation 记录数据库操作
func LogDbOperation(operation string, collection string, query interface{}, result interface{}) {
	Logger.Debug().
		Str("operation", operation).
		Str("collection", collection).
		Interface("query", query).
		Interface("result", result).
		Msg("数据库操作")
}

// LogInconsistency 记录数据不一致情况
func LogInconsistency(operation string, path string, expected interface{}, actual interface{}) {
	isInventoryOperation := (operation == "库存") ||
		contains(path, "/inventory") ||
		contains(path, "/products")

	event := Logger.Warn()
	if isInventoryOperation {
		event = Logger.Error()
	}

	event.
		Str("operation", operation).
		Str("path", path).
		Interface("expected", expected).
		Interface("actual", actual).
		Bool("isInventoryOperation", isInventoryOperation).
		Msg("数据一致性问题")

	if isInventoryOperation {
		Logger.Error().Msgf(
			"警告: 库存操作可能导致数据不一致! 路径: %s, 操作: %s, 时间: %s",
			path, operation, time.Now().Format(time.RFC3339),
		)
	}
}

// contains 检查字符串是否包含子串
func contains(s string, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
