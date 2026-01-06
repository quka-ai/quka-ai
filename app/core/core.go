package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/app/store/sqlstore"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

type Core struct {
	cfg       CoreConfig
	cfgReader io.Reader
	srv       *srv.Srv

	prompt        Prompt
	promptManager *ai.PromptManager

	stores           func() *sqlstore.Provider
	redisClient      redis.UniversalClient
	asynqClient      *asynq.Client
	httpClient       *http.Client
	httpEngine       *gin.Engine
	semaphoreManager *SemaphoreManager

	metrics *Metrics
	Plugins
}

func MustSetupCore(cfg CoreConfig) *Core {
	{
		var writer io.Writer = os.Stdout
		if cfg.Log.Path != "" {
			writer = &lumberjack.Logger{
				Filename:   cfg.Log.Path,
				MaxSize:    500, // megabytes
				MaxBackups: 3,
				MaxAge:     28,   //days
				Compress:   true, // disabled by default
			}
		}
		l := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: cfg.Log.SlogLevel(),
		}))
		slog.SetDefault(l)
	}

	// 初始化 PromptManager
	promptConfig := &ai.PromptConfig{
		Header:       cfg.Prompt.Base,
		ChatSummary:  cfg.Prompt.ChatSummary,
		EnhanceQuery: cfg.Prompt.EnhanceQuery,
		SessionName:  cfg.Prompt.SessionName,
	}
	promptManager := ai.NewPromptManager(promptConfig, ai.MODEL_BASE_LANGUAGE_CN)

	core := &Core{
		cfg:           cfg,
		httpClient:    &http.Client{Timeout: time.Second * 3},
		metrics:       NewMetrics("quka", "core"),
		httpEngine:    gin.New(),
		prompt:        cfg.Prompt,
		promptManager: promptManager,
	}
	editorjs.SetupGlobalEditorJS(cfg.ObjectStorage.StaticDomain)

	// setup store
	setupSqlStore(core)

	// setup redis
	setupRedis(core)

	return core
}

// loadAIConfigFromDB 从数据库加载AI配置的公共方法
func (s *Core) loadAIConfigFromDB(ctx context.Context) ([]types.ModelConfig, []types.ModelProvider, srv.Usage, error) {
	statusEnabled := types.StatusEnabled

	// 1. 从数据库获取启用的模型配置
	models, err := s.Store().ModelConfigStore().ListWithProvider(ctx, types.ListModelConfigOptions{
		Status: &statusEnabled,
	})
	if err != nil {
		return nil, nil, srv.Usage{}, err
	}

	for _, v := range models {
		if v.Provider == nil {
			continue
		}
		result, err := s.DecryptData([]byte(v.Provider.ApiKey))
		if err != nil {
			// maybe unencrypted data
			slog.Warn("Decrypt model(provider) api key failed, maybe unencrypted data", "model_display_name", v.DisplayName, "error", err)
			continue
		}
		v.Provider.ApiKey = string(result)
	}

	// 2. 获取启用的模型提供商配置
	modelProviders, err := s.Store().ModelProviderStore().List(ctx, types.ListModelProviderOptions{
		Status: &statusEnabled,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, nil, srv.Usage{}, err
	}

	for i := range modelProviders {
		result, err := s.DecryptData([]byte(modelProviders[i].ApiKey))
		if err != nil {
			// maybe unencrypted data
			slog.Warn("Decrypt model provider api key failed, maybe unencrypted data", "provider", modelProviders[i].Name, "error", err)
			continue
		}
		modelProviders[i].ApiKey = string(result)
	}

	// 3. 获取使用配置
	usage, err := s.loadAIUsageFromDB(ctx)
	if err != nil {
		return nil, nil, srv.Usage{}, err
	}

	// 转换模型配置
	modelConfigs := lo.Map(models, func(item *types.ModelConfig, _ int) types.ModelConfig {
		return *item
	})

	return modelConfigs, modelProviders, usage, nil
}

// loadInitialAIConfig 系统启动时加载AI配置
func (s *Core) loadInitialAIConfig() srv.ApplyFunc {
	ctx := context.Background()

	models, providers, usage, err := s.loadAIConfigFromDB(ctx)
	if err != nil {
		// 如果加载失败，返回空配置而不是nil
		return srv.ApplyAI([]types.ModelConfig{}, []types.ModelProvider{}, srv.Usage{})
	}

	return srv.ApplyAI(models, providers, usage)
}

// TODO: gen with redis
type sg struct {
	msgStore store.ChatMessageStore
}

func (s *sg) GetChatMessageSequence(ctx context.Context, spaceID, sessionID string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	latestChat, err := s.msgStore.GetSessionLatestMessage(ctx, spaceID, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if latestChat == nil {
		return 1, nil
	}
	return latestChat.Sequence + 1, nil
}

func (s *Core) Cfg() CoreConfig {
	return s.cfg
}

func (s *Core) Prompt() Prompt {
	return s.prompt
}

func (s *Core) UpdatePrompt(p Prompt) {
	s.prompt = p
}

// PromptManager 获取 prompt 管理器
func (s *Core) PromptManager() *ai.PromptManager {
	return s.promptManager
}

func (s *Core) HttpEngine() *gin.Engine {
	return s.httpEngine
}

func (s *Core) Metrics() *Metrics {
	return s.metrics
}

func setupSqlStore(core *Core) {
	core.stores = sqlstore.MustSetup(core.cfg.Postgres)
	// 执行数据库表初始化
	if err := core.stores().Install(); err != nil {
		panic(err)
	}
	fmt.Println("setupSqlStore done")
}

func (s *Core) Store() *sqlstore.Provider {
	return s.stores()
}

func (s *Core) Redis() redis.UniversalClient {
	return s.redisClient
}

func (s *Core) Cache() types.Cache {
	return &Cache{
		redis: s.redisClient,
	}
}

func (s *Core) Asynq() *asynq.Client {
	return s.asynqClient
}

func (s *Core) Srv() *srv.Srv {
	return s.srv
}

// Semaphore 获取信号量管理器
func (c *Core) Semaphore() *SemaphoreManager {
	if c.semaphoreManager == nil {
		c.semaphoreManager = NewSemaphoreManager(c)
	}
	return c.semaphoreManager
}

func setupRedis(core *Core) {
	cfg := core.cfg.Redis

	// 设置默认值
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = 10
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 5
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 3
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 3
	}

	// 判断是否为集群模式
	if cfg.Cluster {
		slog.Info("Initializing Redis Cluster",
			slog.Int("node_count", len(cfg.ClusterAddrs)))

		// Redis 集群模式
		core.redisClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.ClusterAddrs,
			Password:     cfg.ClusterPasswd,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			MaxRetries:   cfg.MaxRetries,
			DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		})
	} else {
		// 单机模式
		addr := cfg.Addr
		slog.Info("Initializing Redis (standalone mode)",
			slog.String("addr", addr),
			slog.Int("db", cfg.DB))

		core.redisClient = redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			MaxRetries:   cfg.MaxRetries,
			DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		})
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := core.redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("Redis connection test failed", slog.String("error", err.Error()))
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}

	slog.Info("Redis connected successfully",
		slog.Bool("cluster_mode", cfg.Cluster))

	// 初始化 Asynq client
	setupAsynqClient(core)
}

// ReloadAI 从数据库重新加载AI配置
func (s *Core) ReloadAI(ctx context.Context) error {
	models, providers, usage, err := s.loadAIConfigFromDB(ctx)
	if err != nil {
		return err
	}

	// 热重载AI配置
	return s.srv.ReloadAI(models, providers, usage)
}

// loadAIUsageFromDB 从数据库加载使用配置
func (s *Core) loadAIUsageFromDB(ctx context.Context) (srv.Usage, error) {
	statusEnabled := types.StatusEnabled
	configs, err := s.Store().CustomConfigStore().List(ctx, types.ListCustomConfigOptions{
		Category: types.AI_USAGE_CATEGORY,
		Status:   &statusEnabled,
	}, 0, 0)
	if err != nil {
		return srv.Usage{}, err
	}

	usage := srv.Usage{}
	for _, config := range configs {
		var modelID string
		if err := json.Unmarshal(config.Value, &modelID); err != nil {
			continue
		}

		switch config.Name {
		case types.AI_USAGE_CHAT:
			usage.Chat = modelID
		case types.AI_USAGE_CHAT_THINKING:
			usage.ChatThinking = modelID
		case types.AI_USAGE_EMBEDDING:
			usage.Embedding = modelID
		case types.AI_USAGE_VISION:
			usage.Vision = modelID
		case types.AI_USAGE_RERANK:
			usage.Rerank = modelID
		case types.AI_USAGE_ENHANCE:
			usage.Enhance = modelID
		case types.AI_USAGE_READER:
			// Reader配置存储的是provider_id，不是model_id
			usage.Reader = modelID
		case types.AI_USAGE_OCR:
			// OCR配置存储的是provider_id，不是model_id
			usage.OCR = modelID
		}
	}

	return usage, nil
}

// GetAIStatus 获取AI系统状态
func (s *Core) GetAIStatus() map[string]interface{} {
	return s.srv.GetAIStatus()
}

func (s *Core) GetActiveModelConfig(ctx context.Context, modelType string) (*types.ModelConfig, error) {
	// Get model ID from custom_config
	modelConfig, err := s.Store().CustomConfigStore().Get(ctx, modelType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s model ID from custom_config: %w", modelType, err)
	}

	if modelConfig == nil || len(modelConfig.Value) == 0 {
		return nil, fmt.Errorf("%s model not configured in custom_config", modelType)
	}

	var modelID string
	if err := json.Unmarshal(modelConfig.Value, &modelID); err != nil {
		return nil, fmt.Errorf("failed to parse %s model ID: %w", modelType, err)
	}

	// Fetch model configuration
	model, err := s.Store().ModelConfigStore().Get(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s model details: %w", modelType, err)
	}

	if model == nil {
		return nil, fmt.Errorf("%s model not found: %s", modelType, modelID)
	}

	// Fetch provider information
	provider, err := s.Store().ModelProviderStore().Get(ctx, model.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s model provider: %w", modelType, err)
	}

	model.Provider = provider
	return model, nil
}

// setupAsynqClient 初始化 Asynq 客户端
func setupAsynqClient(core *Core) {
	cfg := core.cfg.Redis

	// 设置默认的 key prefix
	keyPrefix := cfg.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = "quka" // 默认前缀
	}

	var redisOpt asynq.RedisConnOpt

	if cfg.Cluster {
		// 集群模式
		redisOpt = asynq.RedisClusterClientOpt{
			Addrs:    cfg.ClusterAddrs,
			Password: cfg.ClusterPasswd,
		}
		slog.Info("Asynq client initialized with Redis Cluster",
			slog.String("key_prefix", keyPrefix),
			slog.Int("node_count", len(cfg.ClusterAddrs)))
	} else {
		// 单机模式
		redisOpt = asynq.RedisClientOpt{
			Network:  "tcp",
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
		slog.Info("Asynq client initialized with standalone Redis",
			slog.String("key_prefix", keyPrefix),
			slog.String("addr", cfg.Addr))
	}

	core.asynqClient = asynq.NewClient(redisOpt)
}
