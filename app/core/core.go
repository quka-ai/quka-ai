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
	"github.com/samber/lo"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/app/store/sqlstore"
	centrifugeManager "github.com/quka-ai/quka-ai/pkg/socket/centrifuge"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

type Core struct {
	cfg       CoreConfig
	cfgReader io.Reader
	srv       *srv.Srv

	prompt Prompt

	stores     func() *sqlstore.Provider
	httpClient *http.Client
	httpEngine *gin.Engine

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

	core := &Core{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: time.Second * 3},
		metrics:    NewMetrics("quka", "core"),
		httpEngine: gin.New(),
		prompt:     cfg.Prompt,
	}
	editorjs.SetupGlobalEditorJS(cfg.ObjectStorage.StaticDomain)

	// setup store
	setupSqlStore(core)

	// 初始化时加载AI配置
	aiApplyFunc := core.loadInitialAIConfig()

	// 创建Centrifuge设置函数
	centrifugeSetupFunc := func() (srv.CentrifugeManager, error) {
		// 从配置文件创建Centrifuge配置
		config := &centrifugeManager.Config{
			MaxConnections:    core.cfg.Centrifuge.MaxConnections,
			HeartbeatInterval: core.cfg.Centrifuge.HeartbeatInterval,
			DeploymentMode:    core.cfg.Centrifuge.DeploymentMode,
			RedisURL:          core.cfg.Centrifuge.RedisURL,
			RedisCluster:      core.cfg.Centrifuge.RedisCluster,
			EnablePresence:    core.cfg.Centrifuge.EnablePresence,
			EnableHistory:     core.cfg.Centrifuge.EnableHistory,
			EnableRecovery:    core.cfg.Centrifuge.EnableRecovery,
			AllowedOrigins:    core.cfg.Centrifuge.AllowedOrigins,
			MaxChannelLength:  core.cfg.Centrifuge.MaxChannelLength,
			MaxMessageSize:    core.cfg.Centrifuge.MaxMessageSize,
		}
		manager, err := centrifugeManager.NewManager(config, core.Store())
		if err != nil {
			return nil, err
		}
		// 返回接口类型
		return manager, nil
	}

	core.srv = srv.SetupSrvs(aiApplyFunc, // ai provider select
		// web socket
		srv.ApplyTower(),
		// centrifuge websocket
		srv.ApplyCentrifuge(centrifugeSetupFunc))

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

	// 2. 获取启用的模型提供商配置
	modelProviders, err := s.Store().ModelProviderStore().List(ctx, types.ListModelProviderOptions{
		Status: &statusEnabled,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, nil, srv.Usage{}, err
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

func (s *Core) Srv() *srv.Srv {
	return s.srv
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
