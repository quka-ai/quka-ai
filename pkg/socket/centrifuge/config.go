package centrifuge

import (
	"fmt"
	"os"
	"strings"
)

// Config Centrifuge配置结构
type Config struct {
	// 基础配置
	MaxConnections    int `toml:"max_connections"`
	HeartbeatInterval int `toml:"heartbeat_interval"`

	// 部署模式
	DeploymentMode string `toml:"deployment_mode"` // "single" | "distributed"

	// 分布式配置 (可选)
	RedisURL      string `toml:"redis_url"`
	RedisPassword string `toml:"redis_password"`
	RedisCluster  bool   `toml:"redis_cluster"`

	// 功能开关
	EnablePresence bool `toml:"enable_presence"`
	EnableHistory  bool `toml:"enable_history"` // 默认false，使用MySQL
	EnableRecovery bool `toml:"enable_recovery"`

	// 网络配置
	AllowedOrigins   []string `toml:"allowed_origins"`
	MaxChannelLength int      `toml:"max_channel_length"`
	MaxMessageSize   int      `toml:"max_message_size"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxConnections:    10000,
		HeartbeatInterval: 25,

		DeploymentMode: "single", // 默认单实例

		EnablePresence: true,
		EnableHistory:  false, // 默认不启用，使用MySQL
		EnableRecovery: true,

		AllowedOrigins:   []string{"*"},
		MaxChannelLength: 255,
		MaxMessageSize:   65536,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.DeploymentMode == "distributed" && c.RedisURL == "" {
		return fmt.Errorf("redis_url is required for distributed mode")
	}

	if c.MaxConnections <= 0 {
		c.MaxConnections = 10000
	}

	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 25
	}

	return nil
}

// ResolveEnvVars 解析环境变量
func (c *Config) ResolveEnvVars() {
	c.RedisURL = resolveEnvVar(c.RedisURL)
}

func resolveEnvVar(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		}
	}
	return value
}
