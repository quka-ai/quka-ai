package centrifuge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/centrifugal/centrifuge"
	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// Manager Centrifuge管理器
type Manager struct {
	node   *centrifuge.Node
	config *Config

	// 流信号管理 (用于模拟Tower的流控制功能)
	streamSignals cmap.ConcurrentMap[string, func()]
	mu            sync.RWMutex
}

// ChannelStats 频道统计信息
type ChannelStats struct {
	NumUsers   int `json:"num_users"`
	NumClients int `json:"num_clients"`
}

// NewManager 创建新的Centrifuge管理器
func NewManager(cfg *Config, authorStore Author) (*Manager, error) {
	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// 解析环境变量
	cfg.ResolveEnvVars()

	// 创建Centrifuge节点
	nodeConfig := centrifuge.Config{
		LogLevel: centrifuge.LogLevelInfo,
	}

	node, err := centrifuge.New(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Centrifuge node: %w", err)
	}

	manager := &Manager{
		node:          node,
		config:        cfg,
		streamSignals: cmap.New[func()](),
	}

	// 根据部署模式配置Broker和PresenceManager
	switch cfg.DeploymentMode {
	case "distributed":
		slog.Info("使用分布式模式 (Redis Broker)", "redis_url", cfg.RedisURL)
		if err := manager.setupRedisBroker(cfg.RedisURL, cfg.RedisCluster); err != nil {
			return nil, fmt.Errorf("failed to setup Redis broker: %w", err)
		}
	default:
		slog.Info("使用单实例模式 (内存Broker)")
	}

	// 设置认证处理器 - 使用临时的简化认证
	authHandler := NewSimpleJWTAuthHandler(authorStore)
	node.OnConnecting(func(ctx context.Context, ce centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
		reply, err := authHandler.OnConnecting(ctx, ce)
		if err != nil {
			slog.Error("WebSocket connection rejected", "error", err)
		}
		return reply, err
	})

	// 设置连接处理器
	node.OnConnect(func(client *centrifuge.Client) {
		client.OnSubscribe(func(e centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
			// 复用认证处理器的订阅验证逻辑
			reply, err := authHandler.OnSubscribe(context.Background(), client, e)
			cb(reply, err)
		})

		client.OnUnsubscribe(func(e centrifuge.UnsubscribeEvent) {
			slog.Debug("client unsubscribed", "user_id", client.UserID(), "channel", e.Channel)
		})

		client.OnDisconnect(func(e centrifuge.DisconnectEvent) {
			slog.Debug("client disconnected", "user_id", client.UserID(), "reason", e.Reason)
		})
	})

	// 启动节点
	if err := node.Run(); err != nil {
		return nil, fmt.Errorf("failed to run Centrifuge node: %w", err)
	}

	return manager, nil
}

// Node 返回Centrifuge节点
func (m *Manager) Node() *centrifuge.Node {
	return m.node
}

// Config 返回配置
func (m *Manager) Config() *Config {
	return m.config
}

// PublishMessage 发布消息到频道
func (m *Manager) PublishMessage(channel string, data []byte) error {
	_, err := m.node.Publish(channel, data)
	return err
}

// PublishJSON 发布JSON消息到频道
func (m *Manager) PublishJSON(channel string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return m.PublishMessage(channel, jsonData)
}

// Tower兼容方法 - 发布流消息
func (m *Manager) PublishStreamMessage(topic string, eventType types.WsEventType, data interface{}) error {
	return m.PublishStreamMessageWithSubject(topic, "on_message", eventType, data)
}

// Tower兼容方法 - 发布带主题的流消息
func (m *Manager) PublishStreamMessageWithSubject(topic string, subject string, eventType types.WsEventType, data interface{}) error {
	message := map[string]interface{}{
		"subject": subject,
		"version": "v1",
		"type":    strconv.Itoa(int(eventType)),
		"data":    data,
	}
	return m.PublishJSON(topic, message)
}

// Tower兼容方法 - 发布消息元数据
func (m *Manager) PublishMessageMeta(topic string, eventType types.WsEventType, data interface{}) error {
	return m.PublishStreamMessageWithSubject(topic, "on_message_init", eventType, data)
}

// Tower兼容方法 - 注册流信号
func (m *Manager) RegisterStreamSignal(sessionID string, closeFunc func()) func() {
	m.streamSignals.Set(sessionID, closeFunc)
	return func() {
		m.streamSignals.Remove(sessionID)
	}
}

// Tower兼容方法 - 创建关闭聊天流信号
func (m *Manager) NewCloseChatStreamSignal(sessionID string) error {
	closeFunc, exists := m.streamSignals.Get(sessionID)
	if exists && closeFunc != nil {
		// 在独立的goroutine中调用以避免阻塞
		go closeFunc()
	}
	return nil
}

// Tower兼容方法 - 发布会话重命名
func (m *Manager) PublishSessionReName(topic string, sessionID, name string) error {
	message := map[string]interface{}{
		"subject": "session_rename",
		"version": "v1",
		"type":    "others",
		"data": map[string]string{
			"session_id": sessionID,
			"name":       name,
		},
	}
	return m.PublishJSON(topic, message)
}

// setupRedisBroker 设置Redis Broker
func (m *Manager) setupRedisBroker(redisURL string, isCluster bool) error {
	// 创建Redis分片配置
	redisShardConfigs := []centrifuge.RedisShardConfig{
		{Address: redisURL},
	}

	// 创建Redis分片
	var redisShards []*centrifuge.RedisShard
	for _, redisConf := range redisShardConfigs {
		redisShard, err := centrifuge.NewRedisShard(m.node, redisConf)
		if err != nil {
			return fmt.Errorf("failed to create Redis shard: %w", err)
		}
		redisShards = append(redisShards, redisShard)
	}

	// 创建并设置Redis broker
	broker, err := centrifuge.NewRedisBroker(m.node, centrifuge.RedisBrokerConfig{
		Shards: redisShards,
	})
	if err != nil {
		return fmt.Errorf("failed to create Redis broker: %w", err)
	}
	m.node.SetBroker(broker)

	// 如果启用了Presence，设置Redis presence manager
	if m.config.EnablePresence {
		presenceManager, err := centrifuge.NewRedisPresenceManager(m.node, centrifuge.RedisPresenceManagerConfig{
			Shards: redisShards,
		})
		if err != nil {
			return fmt.Errorf("failed to create Redis presence manager: %w", err)
		}
		m.node.SetPresenceManager(presenceManager)
	}

	return nil
}

// GetChannelStats 获取频道统计信息
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

// HandleWebSocket 处理WebSocket连接
func (m *Manager) HandleWebSocket(w http.ResponseWriter, r *http.Request) error {
	// 创建WebSocket处理器
	wsHandler := centrifuge.NewWebsocketHandler(m.node, centrifuge.WebsocketConfig{
		CheckOrigin: func(r *http.Request) bool {
			// 检查来源
			if len(m.config.AllowedOrigins) == 0 {
				return true
			}

			origin := r.Header.Get("Origin")
			for _, allowed := range m.config.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
	})

	// 处理WebSocket连接
	wsHandler.ServeHTTP(w, r)
	return nil
}

// Shutdown 关闭管理器
func (m *Manager) Shutdown(ctx context.Context) error {
	return m.node.Shutdown(ctx)
}
