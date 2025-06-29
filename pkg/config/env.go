package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EnvConfig 环境变量配置管理
type EnvConfig struct {
	// 数据库配置
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	// MinIO 配置
	MinioRootUser     string
	MinioRootPassword string
	MinioBucket       string
	MinioEndpoint     string

	// 应用配置
	AppS3AccessKey string
	AppS3SecretKey string
	AppS3Bucket    string
	AppS3Endpoint  string
}

// LoadEnvConfig 加载环境变量配置
func LoadEnvConfig() *EnvConfig {
	return &EnvConfig{
		// 数据库配置
		PostgresUser:     getEnv("POSTGRES_USER", "quka"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "quka123"),
		PostgresDB:       getEnv("POSTGRES_DB", "quka"),

		// MinIO 配置
		MinioRootUser:     getEnv("MINIO_ROOT_USER", "minioadmin"),
		MinioRootPassword: getEnv("MINIO_ROOT_PASSWORD", "minioadmin123"),
		MinioBucket:       getEnv("MINIO_BUCKET", "quka-bucket"),
		MinioEndpoint:     getEnv("MINIO_ENDPOINT", "http://localhost:9000"),

		// 应用配置（自动使用 MinIO 配置）
		AppS3AccessKey: getEnv("APP_S3_ACCESS_KEY", getEnv("MINIO_ROOT_USER", "minioadmin")),
		AppS3SecretKey: getEnv("APP_S3_SECRET_KEY", getEnv("MINIO_ROOT_PASSWORD", "minioadmin123")),
		AppS3Bucket:    getEnv("APP_S3_BUCKET", getEnv("MINIO_BUCKET", "quka-bucket")),
		AppS3Endpoint:  getEnv("APP_S3_ENDPOINT", "http://localhost:9000"),
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool 获取布尔类型环境变量
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// getEnvInt 获取整数类型环境变量
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

// ToEnvFile 将配置导出为环境变量文件格式
func (c *EnvConfig) ToEnvFile() string {
	var builder strings.Builder

	builder.WriteString("# 数据库配置\n")
	builder.WriteString(fmt.Sprintf("POSTGRES_USER=%s\n", c.PostgresUser))
	builder.WriteString(fmt.Sprintf("POSTGRES_PASSWORD=%s\n", c.PostgresPassword))
	builder.WriteString(fmt.Sprintf("POSTGRES_DB=%s\n", c.PostgresDB))
	builder.WriteString("\n")

	builder.WriteString("# MinIO 配置\n")
	builder.WriteString(fmt.Sprintf("MINIO_ROOT_USER=%s\n", c.MinioRootUser))
	builder.WriteString(fmt.Sprintf("MINIO_ROOT_PASSWORD=%s\n", c.MinioRootPassword))
	builder.WriteString(fmt.Sprintf("MINIO_BUCKET=%s\n", c.MinioBucket))
	builder.WriteString(fmt.Sprintf("MINIO_ENDPOINT=%s\n", c.MinioEndpoint))
	builder.WriteString("\n")

	builder.WriteString("# 应用配置\n")
	builder.WriteString(fmt.Sprintf("APP_S3_ACCESS_KEY=%s\n", c.AppS3AccessKey))
	builder.WriteString(fmt.Sprintf("APP_S3_SECRET_KEY=%s\n", c.AppS3SecretKey))
	builder.WriteString(fmt.Sprintf("APP_S3_BUCKET=%s\n", c.AppS3Bucket))
	builder.WriteString(fmt.Sprintf("APP_S3_ENDPOINT=%s\n", c.AppS3Endpoint))

	return builder.String()
}

// SetMinioCredentials 设置MinIO凭证，同时更新应用S3配置
func (c *EnvConfig) SetMinioCredentials(user, password string) {
	c.MinioRootUser = user
	c.MinioRootPassword = password
	c.AppS3AccessKey = user
	c.AppS3SecretKey = password
}

// GetS3Config 获取S3配置用于应用
func (c *EnvConfig) GetS3Config() map[string]string {
	return map[string]string{
		"endpoint":   c.AppS3Endpoint,
		"bucket":     c.AppS3Bucket,
		"access_key": c.AppS3AccessKey,
		"secret_key": c.AppS3SecretKey,
		"region":     "us-east-1",
	}
}
