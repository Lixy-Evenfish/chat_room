package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv" //终端输入  go get github.com/joho/godotenv   加载第三方库
)

var Conf struct {
	JWTSecret     string
	JWTExpireHour int
	ServerPort    string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

func LoadConfig() {
	// 尝试加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables or defaults")
	}

	Conf.JWTSecret = getEnv("JWT_SECRET", "your-secret-key")
	Conf.JWTExpireHour = getEnvInt("JWT_EXPIRE_HOUR", 24)
	Conf.ServerPort = getEnv("SERVER_PORT", ":8084")
	Conf.DBHost = getEnv("DB_HOST", "localhost")
	Conf.DBPort = getEnv("DB_PORT", "3306")
	Conf.DBUser = getEnv("DB_USER", "root")
	Conf.DBPassword = getEnv("DB_PASSWORD", "@1919810ysxB")
	Conf.DBName = getEnv("DB_NAME", "chat_app")
	Conf.RedisAddr = getEnv("REDIS_ADDR", "localhost:6379")
	Conf.RedisPassword = getEnv("REDIS_PASSWORD", "")
	Conf.RedisDB = getEnvInt("REDIS_DB", 0)
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		log.Println("Using environment variable:", key, "=", value)
		return value
	}
	log.Println("Failed to get environment variable:", key, "=", defaultValue)
	return defaultValue
}

// getEnvInt 获取环境变量并转换为整数，如果不存在则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
