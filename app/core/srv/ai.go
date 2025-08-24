package srv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
	oai "github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/fusion"
	"github.com/quka-ai/quka-ai/pkg/ai/jina"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type EmbeddingAI interface {
	EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error)
	EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error)
}

type SummarizeAI interface {
	Summarize(ctx context.Context, doc *string) (*ai.SummarizeResult, error)
	Chunk(ctx context.Context, doc *string) (*ai.ChunkResult, error)
}

type ReaderAI interface {
	Reader(ctx context.Context, endpoint string) (*ai.ReaderResult, error)
}

type RerankAI interface {
	Rerank(ctx context.Context, query string, docs []*ai.RerankDoc) ([]ai.RankDocItem, *ai.Usage, error)
}

type AIDriver interface {
	EmbeddingAI
	SummarizeAI
	ReaderAI
	RerankAI
	Lang() string
	DescribeImage(ctx context.Context, lang, imageURL string) (*DescribeImageResult, error)
	MsgIsOverLimit(msgs []*types.MessageContext) bool
	GetConfig(modelType string) types.ModelConfig
	GetChatAI(needsThinking bool) types.ChatModel
	GetVisionAI() types.ChatModel
	GetEnhanceAI() types.ChatModel
}

type AIConfig struct {
	Agent AgentDriver `toml:"agent"`
	// Usage list
	// embedding.query
	// embedding.document
	// query
	// summarize
	// enhance_query
	// reader
	Usage map[string]string `toml:"usage"`
}

type AgentDriver struct {
	Token    string `toml:"token"`
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
	VlModel  string `toml:"vl_model"`
}

type Jina struct {
	Token       string            `toml:"token"`
	ApiEndpoint string            `toml:"api_endpoint"`
	Models      map[string]string `toml:"models"`
}

type AI struct {
	chatDrivers         map[string]types.ChatModel
	chatThinkingDrivers map[string]types.ChatModel
	embedDrivers        map[string]EmbeddingAI
	enhanceDrivers      map[string]types.ChatModel
	visionDrivers       map[string]types.ChatModel
	readerDrivers       map[string]ReaderAI
	rerankDrivers       map[string]RerankAI

	summarize SummarizeAI

	chatDefault         types.ChatModel
	chatThinkingDefault types.ChatModel // 思考聊天模型
	enhanceDefault      types.ChatModel
	embedDefault        EmbeddingAI
	readerDefault       ReaderAI
	visionDefault       types.ChatModel
	rerankDefault       RerankAI

	allModels map[string]types.ModelConfig
	usage     Usage
}

func (s *AI) MsgIsOverLimit(msgs []*types.MessageContext) bool {
	tokenNum, err := ai.NumTokens(lo.Map(msgs, func(item *types.MessageContext, _ int) oai.ChatCompletionMessage {
		return oai.ChatCompletionMessage{
			Role:    item.Role.String(),
			Content: item.Content,
		}
	}), "")
	if err != nil {
		slog.Error("Failed to tik request token", slog.String("error", err.Error()))
		return false
	}

	return tokenNum > 80000
}

func (s *AI) GetConfig(id string) types.ModelConfig {
	return s.allModels[id]
}

// GetChatConfig 根据思考需求获取聊天模型配置
func (s *AI) GetChatConfig(needsThinking bool) types.ModelConfig {
	if needsThinking {
		// 如果需要思考，优先返回思考模型
		if s.usage.ChatThinking != "" {
			if config, exists := s.allModels[s.usage.ChatThinking]; exists {
				return config
			}
		}

		// 如果没有配置专用的思考模型，检查默认chat模型是否支持思考
		if s.usage.Chat != "" {
			if config, exists := s.allModels[s.usage.Chat]; exists {
				if config.ThinkingSupport == types.ThinkingSupportOptional ||
					config.ThinkingSupport == types.ThinkingSupportForced {
					return config
				}
			}
		}
	} else {
		// 如果不需要思考，优先返回普通chat模型
		if s.usage.Chat != "" {
			if config, exists := s.allModels[s.usage.Chat]; exists {
				if config.ThinkingSupport != types.ThinkingSupportForced {
					return config
				}
			}
		}

		// 如果普通chat模型强制思考，尝试思考模型（如果它支持关闭思考）
		if s.usage.ChatThinking != "" {
			if config, exists := s.allModels[s.usage.ChatThinking]; exists {
				if config.ThinkingSupport == types.ThinkingSupportOptional {
					return config
				}
			}
		}
	}

	// 兜底返回默认chat配置
	return s.GetConfig(types.MODEL_TYPE_CHAT)
}

// GetChatAI 根据思考需求获取预组装的ChatAI实例
func (s *AI) GetChatAI(needsThinking bool) types.ChatModel {
	raw, _ := json.Marshal(s.usage)
	fmt.Println(111, string(raw))
	if needsThinking {
		impl, exist := s.chatThinkingDrivers[s.usage.ChatThinking]
		if exist {
			return impl
		}
		// 如果需要思考且有专用的思考模型
		if s.chatThinkingDefault != nil {
			return s.chatThinkingDefault
		}
	}

	impl, exist := s.chatDrivers[s.usage.Chat]
	if exist {
		return impl
	}

	return s.chatDefault
}

// GetEnhanceAI 获取增强AI模型
func (s *AI) GetEnhanceAI() types.ChatModel {
	if d := s.enhanceDrivers[s.usage.Enhance]; d != nil {
		return d
	}
	return s.enhanceDefault
}

// GetEnhanceAI 获取增强AI模型
func (s *AI) GetVisionAI() types.ChatModel {
	if d := s.visionDrivers[s.usage.Vision]; d != nil {
		return d
	}
	return s.visionDefault
}

type DescribeImageResult struct {
	*schema.Message
	Model string
}

func (s *AI) DescribeImage(ctx context.Context, lang, imageURL string) (*DescribeImageResult, error) {
	impl, exist := s.visionDrivers[s.usage.Vision]
	if !exist {
		return nil, fmt.Errorf("not support vision model")
	}

	prompt := strings.ReplaceAll(ai.IMAGE_GENERATE_PROMPT_CN, "${lang}", lang)

	result, err := impl.Generate(ctx, []*schema.Message{
		schema.SystemMessage(prompt),
		{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL: imageURL,
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return &DescribeImageResult{
		Message: result,
		Model:   impl.Config().ModelName,
	}, nil
}

func (s *AI) Rerank(ctx context.Context, query string, docs []*ai.RerankDoc) ([]ai.RankDocItem, *ai.Usage, error) {
	if d := s.rerankDrivers[s.usage.Rerank]; d != nil {
		return d.Rerank(ctx, query, docs)
	}

	if s.rerankDefault == nil {
		return nil, nil, errors.ERROR_UNSUPPORTED_FEATURE
	}
	return s.rerankDefault.Rerank(ctx, query, docs)
}

func (s *AI) EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error) {
	if d := s.embedDrivers[s.usage.Embedding]; d != nil {
		return d.EmbeddingForQuery(ctx, content)
	}
	return s.embedDefault.EmbeddingForQuery(ctx, content)
}

func (s *AI) EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	if d := s.embedDrivers[s.usage.Embedding]; d != nil {
		return d.EmbeddingForDocument(ctx, title, content)
	}
	return s.embedDefault.EmbeddingForQuery(ctx, content)
}

func (s *AI) Summarize(ctx context.Context, doc *string) (*ai.SummarizeResult, error) {
	// 使用 enhance driver 进行摘要
	enhanceAI := s.GetEnhanceAI()
	if enhanceAI == nil {
		return nil, fmt.Errorf("enhance AI not available")
	}

	// 定义工具的参数 schema
	params := map[string]*schema.ParameterInfo{
		"tags": {
			Type:     schema.Array,
			Desc:     "从用户描述中提取相关关键词或技术标签，以帮助用户稍后对相关内容进行分类。将这些值以数组格式组织。",
			Required: true,
		},
		"title": {
			Type:     schema.String,
			Desc:     "为用户提供的内容生成标题并填入此字段。",
			Required: true,
		},
		"summary": {
			Type:     schema.String,
			Desc:     "处理后的摘要内容。",
			Required: true,
		},
		"date_time": {
			Type:     schema.String,
			Desc:     "用户内容中提到的时间，格式为'年-月-日 时:分'。如果无法提取时间，则留空。",
			Required: false,
		},
	}

	// 创建工具信息
	toolInfo := &schema.ToolInfo{
		Name:        "summarize",
		Desc:        "处理后的摘要内容。",
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}

	// 构建消息
	messages := []*schema.Message{
		schema.SystemMessage(ai.ReplaceVarCN(ai.PROMPT_PROCESS_CONTENT_EN)),
		schema.UserMessage(*doc),
	}

	// 执行带工具调用的生成
	response, err := enhanceAI.Generate(ctx, messages, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		return nil, fmt.Errorf("failed to generate with tools: %w", err)
	}

	result := ai.SummarizeResult{
		Model: enhanceAI.Config().ModelName,
	}

	// 处理工具调用结果
	if len(response.ToolCalls) > 0 {
		for _, toolCall := range response.ToolCalls {
			if toolCall.Function.Name == "summarize" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &result); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
				}
				result.Model = enhanceAI.Config().ModelName
				return &result, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to summarize knowledge, unexcept ai response %v", response)
}

func (s *AI) Chunk(ctx context.Context, doc *string) (*ai.ChunkResult, error) {
	// 使用 enhance driver 进行分块
	enhanceAI := s.GetEnhanceAI()
	if enhanceAI == nil {
		return nil, fmt.Errorf("enhance AI not available")
	}

	// 定义工具的参数 schema
	params := map[string]*schema.ParameterInfo{
		"tags": {
			Type:     schema.Array,
			Desc:     "从用户描述中提取相关关键词或技术标签，以帮助用户稍后对相关内容进行分类。将这些值以数组格式组织。",
			Required: true,
		},
		"title": {
			Type:     schema.String,
			Desc:     "为用户提供的内容生成标题并填入此字段。",
			Required: true,
		},
		"chunks": {
			Type:     schema.Array,
			Desc:     "处理后的分块内容。",
			Required: true,
		},
		"date_time": {
			Type:     schema.String,
			Desc:     "用户内容中提到的时间，格式为'年-月-日 时:分'。如果无法提取时间，则留空。",
			Required: false,
		},
	}

	// 创建工具信息
	toolInfo := &schema.ToolInfo{
		Name:        "chunk",
		Desc:        "处理后的分块内容。",
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}

	// 构建消息
	messages := []*schema.Message{
		schema.SystemMessage(ai.ReplaceVarCN(ai.PROMPT_CHUNK_CONTENT_EN)),
		schema.UserMessage(strings.ReplaceAll(*doc, "\n", "")),
	}

	// 执行带工具调用的生成
	response, err := enhanceAI.Generate(ctx, messages, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		return nil, fmt.Errorf("failed to generate with tools: %w", err)
	}

	result := ai.ChunkResult{}

	// 处理工具调用结果
	if len(response.ToolCalls) > 0 {
		for _, toolCall := range response.ToolCalls {
			if toolCall.Function.Name == "chunk" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &result); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
				}
				result.Model = enhanceAI.Config().ModelName
				return &result, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to chunk knowledge, unexcept ai response %v", response)
}

func (s *AI) NewEnhance(ctx context.Context) *ai.EnhanceOptions {
	if d := s.enhanceDrivers[s.usage.Enhance]; d != nil {
		return ai.NewEnhance(ctx, d)
	}
	return ai.NewEnhance(ctx, s.enhanceDefault)
}

type ReaderProvider interface {
	Match(endpoint string) bool
	Reader(ctx context.Context, endpoint string) (*ai.ReaderResult, error)
}

type ReaderProviderRegistry struct {
	providers []ReaderProvider
}

var rpr = &ReaderProviderRegistry{}

func RegisterReaderProvider(provider ReaderProvider) {
	rpr.providers = append(rpr.providers, provider)
}

// Option Feature
func (s *AI) Reader(ctx context.Context, endpoint string) (*ai.ReaderResult, error) {
	for _, v := range rpr.providers {
		if v.Match(endpoint) {
			return v.Reader(ctx, endpoint)
		}
	}

	if d := s.readerDrivers[s.usage.Reader]; d != nil {
		return d.Reader(ctx, endpoint)
	}

	if s.readerDefault == nil {
		return nil, errors.ERROR_UNSUPPORTED_FEATURE
	}
	return s.readerDefault.Reader(ctx, endpoint)
}

type Usage struct {
	// 模型级别配置（指向model_id）
	Chat         string `json:"chat"`
	ChatThinking string `json:"chat_thinking"` // 思考聊天模型ID
	Embedding    string `json:"embedding"`
	Vision       string `json:"vision"`
	Rerank       string `json:"rerank"`
	Enhance      string `json:"enhance"`

	// 提供商级别配置（指向provider_id）
	Reader string `json:"reader"`
}

func SetupReader(s *AI, providers []types.ModelProvider) error {
	for _, v := range providers {
		var providerConfig types.ModelProviderConfig
		if err := json.Unmarshal(v.Config, &providerConfig); err != nil {
			slog.Error("Failed to unmarshal provider config for SetupReader", slog.String("provider_id", v.ID), slog.Any("error", err))
			continue // 如果配置解析失败，跳过该提供商
		}

		// 只有配置了is_reader为true的提供商才会设置Reader功能
		if !providerConfig.IsReader {
			continue
		}

		switch strings.ToLower(v.Name) {
		case strings.ToLower(jina.NAME):
			driver := jina.New(v.ApiKey, v.ApiUrl)
			// 使用provider_id作为key，不是固定的"jina"
			s.readerDrivers[v.ID] = driver
			if s.readerDefault == nil {
				s.readerDefault = driver
			}
		}
	}
	return nil
}

func SetupAI(models []types.ModelConfig, modelProviders []types.ModelProvider, usage Usage) (*AI, error) {
	a := &AI{
		chatDrivers:         make(map[string]types.ChatModel),
		chatThinkingDrivers: make(map[string]types.ChatModel),
		enhanceDrivers:      make(map[string]types.ChatModel),
		embedDrivers:        make(map[string]EmbeddingAI),
		readerDrivers:       make(map[string]ReaderAI),
		visionDrivers:       make(map[string]types.ChatModel),
		rerankDrivers:       make(map[string]RerankAI),

		allModels: lo.SliceToMap(models, func(item types.ModelConfig) (string, types.ModelConfig) {
			return item.ID, item
		}),
		usage: usage,
	}

	// 设置模型配置
	for _, v := range models {
		if v.Provider == nil {
			continue
		}

		switch v.ModelType {
		case types.MODEL_TYPE_CHAT:
			d, err := SetupAIDriver(context.Background(), v)
			if err != nil {
				return nil, err
			}

			if v.ThinkingSupport == types.ThinkingSupportForced {
				a.chatThinkingDrivers[v.ID] = d
				a.chatThinkingDefault = d
			} else {
				a.chatDrivers[v.ID] = d
			}
			a.chatDefault = d
		case types.MODEL_TYPE_ENHANCE:
			d, err := SetupAIDriver(context.Background(), v)
			if err != nil {
				return nil, err
			}
			a.enhanceDrivers[v.ID] = d
			a.enhanceDefault = d
		case types.MODEL_TYPE_EMBEDDING:
			d := fusion.New(v.Provider.ApiKey, v.Provider.ApiUrl, v.ModelName)
			a.embedDrivers[v.ID] = d
			a.embedDefault = d
		case types.MODEL_TYPE_RERANK:
			d := fusion.New(v.Provider.ApiKey, v.Provider.ApiUrl, v.ModelName)
			a.rerankDrivers[v.ID] = d
			a.rerankDefault = d
		case types.MODEL_TYPE_VISION:
			d, err := SetupAIDriver(context.Background(), v)
			if err != nil {
				return nil, err
			}
			a.visionDrivers[v.ID] = d
			a.visionDefault = d
		}
	}
	// 设置提供商级别的Reader配置
	if err := SetupReader(a, modelProviders); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *AI) Lang() string {
	return types.LANGUAGE_CN_KEY
}

func SetupAIDriver(ctx context.Context, modelConfig types.ModelConfig) (types.ChatModel, error) {
	a := &types.CommonAIWithMeta{
		Cfg: modelConfig,
	}
	boolPoint := modelConfig.ThinkingSupport == types.ThinkingSupportForced
	if strings.HasPrefix(strings.ToLower(modelConfig.ModelName), "qwen") {
		// 创建 Qwen 模型
		// chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		// 	APIKey:         modelConfig.Provider.ApiKey,
		// 	BaseURL:        modelConfig.Provider.ApiUrl,
		// 	Model:          modelConfig.ModelName,
		// 	Timeout:        5 * time.Minute,
		// 	EnableThinking: &boolPoint,
		// })
		chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  modelConfig.Provider.ApiKey,
			BaseURL: modelConfig.Provider.ApiUrl,
			Model:   modelConfig.ModelName,
			Timeout: 5 * time.Minute,
			ExtraFields: map[string]any{
				"enable_thinking": &boolPoint,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create qwen chat model: %w", err)
		}

		a.ToolCallingChatModel = chatModel
		return a, nil
	} else if strings.Contains(strings.ToLower(modelConfig.ModelName), "deepseek") {
		chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			APIKey:  modelConfig.Provider.ApiKey,
			BaseURL: modelConfig.Provider.ApiUrl,
			Model:   modelConfig.ModelName,
			Timeout: 5 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create deepseek chat model: %w", err)
		}
		a.ToolCallingChatModel = chatModel
		return a, nil
	}

	// 创建 OpenAI 模型
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  modelConfig.Provider.ApiKey,
		BaseURL: modelConfig.Provider.ApiUrl,
		Model:   modelConfig.ModelName,
		Timeout: 5 * time.Minute,
		ExtraFields: map[string]any{
			"enable_thinking": &boolPoint,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create openai chat model: %w", err)
	}

	a.ToolCallingChatModel = chatModel
	return a, nil
}

type ApplyFunc func(s *Srv)

func ApplyAI(providers []types.ModelConfig, modelProviders []types.ModelProvider, usage Usage) ApplyFunc {
	return func(s *Srv) {
		s.ai, _ = SetupAI(providers, modelProviders, usage)
	}
}

// 配置热重载方法
func (s *AI) ReloadFromProviders(providers []types.ModelConfig, modelProviders []types.ModelProvider, usage Usage) error {
	newAI, err := SetupAI(providers, modelProviders, usage)
	if err != nil {
		return err
	}

	// 原子性替换AI配置
	*s = *newAI
	return nil
}
