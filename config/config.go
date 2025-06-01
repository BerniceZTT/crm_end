package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	Port     int
	MongoURI string
	MongoDB  string
	JWTKey   string
	Debug    bool
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	return &Config{
		Port:     port,
		MongoURI: fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?authSource=%s", "qianxin", "QianXin123", "127.0.0.1", "27017", "crm", "admin"),
		MongoDB:  getEnv("MONGO_DB", "crm"),
		JWTKey:   getEnv("JWT_KEY", "your-secret-key"), // 实际环境应替换为安全密钥
		Debug:    getEnv("GIN_MODE", "debug") == "debug",
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
