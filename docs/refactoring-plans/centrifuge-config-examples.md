# Centrifuge 配置示例

## 设计理念

基于你的建议，我们采用**简化配置**的设计理念：

1. **无强制依赖**: 单实例部署不需要Redis
2. **渐进式扩展**: 可随时从单实例升级到分布式
3. **复用现有架构**: 历史消息继续使用MySQL
4. **最小化配置**: 只暴露必要的配置项

## 配置结构

```go
type CentrifugeConfig struct {
    // 基础配置 (移除TokenSecret)
    MaxConnections   int    `toml:"max_connections"`
    HeartbeatInterval int   `toml:"heartbeat_interval"`
    
    // 部署模式
    DeploymentMode   string `toml:"deployment_mode"` // "single" | "distributed"
    
    // 分布式配置 (可选)
    RedisURL         string `toml:"redis_url,omitempty"`
    RedisCluster     bool   `toml:"redis_cluster,omitempty"`
    
    // 功能开关
    EnablePresence   bool   `toml:"enable_presence"`
    EnableHistory    bool   `toml:"enable_history"`    // 默认false
    EnableRecovery   bool   `toml:"enable_recovery"`
    
    // 网络配置
    AllowedOrigins   []string `toml:"allowed_origins"`
    MaxChannelLength int      `toml:"max_channel_length"`
    MaxMessageSize   int      `toml:"max_message_size"`
}
```

## 配置示例

### 1. 开发环境 (单实例)

```toml
# cmd/service/etc/service-dev.toml
[websocket]
enable = true
allow_origins = ["*"]

[centrifuge]
# 基础配置 (移除token_secret，复用现有JWT)
max_connections = 1000
heartbeat_interval = 25

# 单实例模式 - 无需外部依赖
deployment_mode = "single"

# 功能配置
enable_presence = true      # 在线用户统计
enable_history = false      # 历史消息由MySQL管理
enable_recovery = true      # 断线重连

# 网络配置
allowed_origins = ["http://localhost:3000", "http://localhost:8080"]
max_channel_length = 255
max_message_size = 65536
```

### 2. 生产环境 (单实例)

```toml
# cmd/service/etc/service-prod.toml
[websocket]
enable = true
allow_origins = ["https://yourdomain.com"]

[centrifuge]
# 基础配置 (移除token_secret，使用现有JWT认证)
max_connections = 10000
heartbeat_interval = 30

# 单实例模式 - 适合中小型应用
deployment_mode = "single"

# 功能配置
enable_presence = true
enable_history = false      # 继续使用MySQL
enable_recovery = true

# 安全配置
allowed_origins = ["https://yourdomain.com", "https://app.yourdomain.com"]
max_channel_length = 255
max_message_size = 65536
```

### 3. 大规模生产环境 (分布式)

```toml
# cmd/service/etc/service-cluster.toml
[websocket]
enable = true
allow_origins = ["https://yourdomain.com"]

[centrifuge]
# 基础配置 (复用现有JWT，无需额外token配置)
max_connections = 50000
heartbeat_interval = 30

# 分布式模式 - 支持水平扩展
deployment_mode = "distributed"
redis_url = "redis://redis-cluster:6379"
redis_cluster = false

# 功能配置
enable_presence = true      # 跨节点在线统计
enable_history = false      # 仍使用MySQL
enable_recovery = true

# 生产配置
allowed_origins = ["https://yourdomain.com"]
max_channel_length = 255
max_message_size = 65536
```

## 实现代码

### 配置加载

```go
// pkg/socket/centrifuge/config.go
package centrifuge

import (
    "fmt"
    "os"
    "strings"
)

type Config struct {
    TokenSecret      string   `toml:"token_secret"`
    MaxConnections   int      `toml:"max_connections"`
    HeartbeatInterval int     `toml:"heartbeat_interval"`
    
    DeploymentMode   string   `toml:"deployment_mode"`
    
    RedisURL         string   `toml:"redis_url"`
    RedisCluster     bool     `toml:"redis_cluster"`
    
    EnablePresence   bool     `toml:"enable_presence"`
    EnableHistory    bool     `toml:"enable_history"`
    EnableRecovery   bool     `toml:"enable_recovery"`
    
    AllowedOrigins   []string `toml:"allowed_origins"`
    MaxChannelLength int      `toml:"max_channel_length"`
    MaxMessageSize   int      `toml:"max_message_size"`
}

// 默认配置 (移除TokenSecret相关)
func DefaultConfig() *Config {
    return &Config{
        MaxConnections:   10000,
        HeartbeatInterval: 25,
        
        DeploymentMode:   "single",  // 默认单实例
        
        EnablePresence:   true,
        EnableHistory:    false,     // 默认不启用，使用MySQL
        EnableRecovery:   true,
        
        AllowedOrigins:   []string{"*"},
        MaxChannelLength: 255,
        MaxMessageSize:   65536,
    }
}

// 验证配置 (移除token_secret验证)
func (c *Config) Validate() error {
    if c.DeploymentMode != "single" && c.DeploymentMode != "distributed" {
        return fmt.Errorf("deployment_mode must be 'single' or 'distributed'")
    }
    
    if c.DeploymentMode == "distributed" && c.RedisURL == "" {
        return fmt.Errorf("redis_url is required for distributed mode")
    }
    
    return nil
}

// 支持环境变量替换 (移除TokenSecret)
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
```

### Centrifuge初始化

```go
// pkg/socket/centrifuge/manager.go
package centrifuge

import (
    "github.com/centrifugal/centrifuge"
    "log/slog"
)

type Manager struct {
    node   *centrifuge.Node
    config *Config
}

func NewManager(cfg *Config) (*Manager, error) {
    // 验证配置
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    
    // 解析环境变量
    cfg.ResolveEnvVars()
    
    // 选择引擎
    var engine centrifuge.Engine
    var err error
    
    switch cfg.DeploymentMode {
    case "single":
        slog.Info("使用单实例模式 (内存引擎)")
        engine = centrifuge.NewMemoryEngine()
        
    case "distributed":
        slog.Info("使用分布式模式 (Redis引擎)", "redis_url", cfg.RedisURL)
        redisConfig := centrifuge.RedisEngineConfig{
            Address:    cfg.RedisURL,
            IsCluster:  cfg.RedisCluster,
        }
        engine, err = centrifuge.NewRedisEngine(redisConfig)
        if err != nil {
            return nil, fmt.Errorf("failed to create Redis engine: %w", err)
        }
        
    default:
        return nil, fmt.Errorf("unsupported deployment mode: %s", cfg.DeploymentMode)
    }
    
    // 创建节点
    nodeConfig := centrifuge.Config{
        Engine: engine,
        ChannelOptionsFunc: func(channel string) (centrifuge.ChannelOptions, bool, error) {
            return centrifuge.ChannelOptions{
                Presence:  cfg.EnablePresence,
                HistorySize: 0,  // 不使用内置历史
                HistoryTTL:  0,  // 不使用内置历史
                Recover:   cfg.EnableRecovery,
            }, true, nil
        },
    }
    
    node, err := centrifuge.New(nodeConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create Centrifuge node: %w", err)
    }
    
    return &Manager{
        node:   node,
        config: cfg,
    }, nil
}

func (m *Manager) Node() *centrifuge.Node {
    return m.node
}

func (m *Manager) Config() *Config {
    return m.config
}

func (m *Manager) GetChannelStats(channel string) (*ChannelStats, error) {
    if !m.config.EnablePresence {
        return &ChannelStats{}, nil
    }
    
    stats, err := m.node.PresenceStats(channel)
    if err != nil {
        return nil, err
    }
    
    return &ChannelStats{
        NumUsers:   stats.NumUsers,
        NumClients: stats.NumClients,
    }, nil
}

type ChannelStats struct {
    NumUsers   int `json:"num_users"`
    NumClients int `json:"num_clients"`
}
```

## 迁移策略

### 阶段1: 开发环境测试
```bash
# 使用单实例配置测试
deployment_mode = "single"
enable_history = false
```

### 阶段2: 生产环境部署
```bash
# 先使用单实例上线
deployment_mode = "single"
max_connections = 10000
```

### 阶段3: 根据需要扩展
```bash
# 如果需要多节点，切换到分布式
deployment_mode = "distributed"
redis_url = "redis://cluster:6379"
```

## 优势总结

### 相比原方案的优势

1. **降低门槛**: 
   - 单实例无需Redis
   - 配置项减少80%
   - 开箱即用

2. **保持灵活性**:
   - 可随时升级到分布式
   - 历史消息策略可控
   - 功能模块化开关

3. **简化运维**:
   - 减少外部依赖
   - 配置更清晰
   - 错误排查更容易

4. **成本优化**:
   - 小型部署无需Redis集群
   - 减少服务器资源
   - 降低维护成本

### 性能对比

| 部署模式 | 连接数 | 外部依赖 | 适用场景 |
|----------|--------|----------|----------|
| 单实例 | 1万-5万 | 无 | 中小型应用 |
| 分布式 | 10万+ | Redis | 大型应用 |

这种设计既满足了简化需求，又保持了扩展能力，是一个很好的平衡方案。