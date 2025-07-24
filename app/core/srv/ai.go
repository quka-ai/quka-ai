package srv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/samber/lo"
	oai "github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/fusion"
	"github.com/quka-ai/quka-ai/pkg/ai/jina"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type ChatAI interface {
	Summarize(ctx context.Context, doc *string) (ai.SummarizeResult, error)
	Chunk(ctx context.Context, doc *string) (ai.ChunkResult, error)
	MsgIsOverLimit(msgs []*types.MessageContext) bool
	NewQuery(ctx context.Context, msgs []*types.MessageContext) *ai.QueryOptions
	Lang() string
}

type EnhanceAI interface {
	NewEnhance(ctx context.Context) *ai.EnhanceOptions
}

type EmbeddingAI interface {
	EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error)
	EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error)
}

type ReaderAI interface {
	Reader(ctx context.Context, endpoint string) (*ai.ReaderResult, error)
}

type VisionAI interface {
	NewVisionQuery(ctx context.Context, msgs []*types.MessageContext) *ai.QueryOptions
}

type RerankAI interface {
	Rerank(ctx context.Context, query string, docs []*ai.RerankDoc) ([]ai.RankDocItem, *ai.Usage, error)
}

type AIDriver interface {
	EmbeddingAI
	EnhanceAI
	ChatAI
	ReaderAI
	VisionAI
	RerankAI
	DescribeImage(ctx context.Context, lang, imageURL string) (*oai.ChatCompletionResponse, error)
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
	chatDrivers    map[string]ChatAI
	embedDrivers   map[string]EmbeddingAI
	enhanceDrivers map[string]ai.Enhance
	visionDrivers  map[string]VisionAI
	readerDrivers  map[string]ReaderAI
	rerankDrivers  map[string]RerankAI

	chatUsage    map[string]ChatAI
	enhanceUsage map[string]ai.Enhance
	embedUsage   map[string]EmbeddingAI
	readerUsage  map[string]ReaderAI
	visionUsage  map[string]VisionAI
	rerankUsage  map[string]RerankAI

	chatDefault    ChatAI
	enhanceDefault ai.Enhance
	embedDefault   EmbeddingAI
	readerDefault  ReaderAI
	visionDefault  VisionAI
	rerankDefault  RerankAI
}

func (s *AI) DescribeImage(ctx context.Context, lang, imageURL string) (*oai.ChatCompletionResponse, error) {
	opts := s.NewVisionQuery(ctx, []*types.MessageContext{
		{
			Role: types.USER_ROLE_USER,
			MultiContent: []oai.ChatMessagePart{
				{
					Type: oai.ChatMessagePartTypeImageURL,
					ImageURL: &oai.ChatMessageImageURL{
						URL: imageURL,
					},
				},
			},
		},
	})

	opts.WithPrompt(lo.If(s.Lang() == ai.MODEL_BASE_LANGUAGE_CN, ai.IMAGE_GENERATE_PROMPT_CN).Else(ai.IMAGE_GENERATE_PROMPT_EN))
	opts.WithVar("${lang}", lang)
	resp, err := opts.Query()
	if err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *AI) NewQuery(ctx context.Context, query []*types.MessageContext) *ai.QueryOptions {
	if d := s.chatUsage["query"]; d != nil {
		return d.NewQuery(ctx, query)
	}
	return s.chatDefault.NewQuery(ctx, query)
}

func (s *AI) NewVisionQuery(ctx context.Context, query []*types.MessageContext) *ai.QueryOptions {
	if d := s.visionUsage["vision"]; d != nil {
		return d.NewVisionQuery(ctx, query)
	}
	return s.chatDefault.NewQuery(ctx, query)
}

func (s *AI) Rerank(ctx context.Context, query string, docs []*ai.RerankDoc) ([]ai.RankDocItem, *ai.Usage, error) {
	if d := s.rerankUsage["rerank"]; d != nil {
		return d.Rerank(ctx, query, docs)
	}

	if s.rerankDefault == nil {
		return nil, nil, errors.ERROR_UNSUPPORTED_FEATURE
	}
	return s.rerankDefault.Rerank(ctx, query, docs)
}

func (s *AI) Lang() string {
	if d := s.chatUsage["query"]; d != nil {
		return d.Lang()
	}
	return s.chatDefault.Lang()
}

func (s *AI) EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error) {
	if d := s.embedUsage["embedding"]; d != nil {
		return d.EmbeddingForQuery(ctx, content)
	}
	return s.embedDefault.EmbeddingForQuery(ctx, content)
}

func (s *AI) EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	if d := s.embedUsage["embedding"]; d != nil {
		return d.EmbeddingForDocument(ctx, title, content)
	}
	return s.embedDefault.EmbeddingForQuery(ctx, content)
}

func (s *AI) Summarize(ctx context.Context, doc *string) (ai.SummarizeResult, error) {
	if d := s.chatUsage["summarize"]; d != nil {
		return d.Summarize(ctx, doc)
	}
	return s.chatDefault.Summarize(ctx, doc)
}

func (s *AI) Chunk(ctx context.Context, doc *string) (ai.ChunkResult, error) {
	if d := s.chatUsage["summarize"]; d != nil {
		return d.Chunk(ctx, doc)
	}
	return s.chatDefault.Chunk(ctx, doc)
}

func (s *AI) NewEnhance(ctx context.Context) *ai.EnhanceOptions {
	if d := s.enhanceUsage["enhance_query"]; d != nil {
		return ai.NewEnhance(ctx, d)
	}
	return ai.NewEnhance(ctx, s.enhanceDefault)
}

func (s *AI) MsgIsOverLimit(msgs []*types.MessageContext) bool {
	// TODO
	return false
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

	if d := s.readerUsage["reader"]; d != nil {
		return d.Reader(ctx, endpoint)
	}

	if s.readerDefault == nil {
		return nil, errors.ERROR_UNSUPPORTED_FEATURE
	}
	return s.readerDefault.Reader(ctx, endpoint)
}

type Usage struct {
	// 模型级别配置（指向model_id）
	Chat      string `json:"chat"`
	Embedding string `json:"embedding"`
	Vision    string `json:"vision"`
	Rerank    string `json:"rerank"`
	Enhance   string `json:"enhance"`

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
			fmt.Println("init jina reader driver", v.ID, v.ApiKey, v.ApiUrl)
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
		chatDrivers:    make(map[string]ChatAI),
		chatUsage:      make(map[string]ChatAI),
		enhanceDrivers: make(map[string]ai.Enhance),
		enhanceUsage:   make(map[string]ai.Enhance),
		embedDrivers:   make(map[string]EmbeddingAI),
		embedUsage:     make(map[string]EmbeddingAI),
		readerDrivers:  make(map[string]ReaderAI),
		readerUsage:    make(map[string]ReaderAI),
		visionDrivers:  make(map[string]VisionAI),
		visionUsage:    make(map[string]VisionAI),
		rerankDrivers:  make(map[string]RerankAI),
		rerankUsage:    make(map[string]RerankAI),
	}

	// 设置模型配置
	for _, v := range models {
		if v.Provider == nil {
			continue
		}

		d := fusion.New(v.Provider.ApiKey, v.Provider.ApiUrl, v.ModelName)
		switch v.ModelType {
		case "chat":
			a.chatDrivers[v.ID] = d
			a.chatDefault = d
			a.enhanceDrivers[v.ID] = d
			a.enhanceDefault = d
		case "embedding":
			a.embedDrivers[v.ID] = d
			a.embedDefault = d
		case "rerank":
			a.rerankDrivers[v.ID] = d
			a.rerankDefault = d
		case "vision":
			a.visionDrivers[v.ID] = d
			a.visionDefault = d
		}
	}
	// 设置提供商级别的Reader配置
	if err := SetupReader(a, modelProviders); err != nil {
		return nil, err
	}

	a.embedUsage["embedding"] = a.embedDrivers[usage.Embedding]
	if a.embedDefault == nil {
		a.embedDefault = a.embedDrivers[usage.Embedding]
	}

	a.enhanceUsage["enhance_query"] = a.enhanceDrivers[usage.Enhance]
	if a.enhanceDefault == nil {
		a.enhanceDefault = a.enhanceDrivers[usage.Enhance]
	}

	a.visionUsage["vision"] = a.visionDrivers[usage.Vision]
	if a.visionDefault == nil {
		a.visionDefault = a.visionDrivers[usage.Vision]
	}

	a.rerankUsage["rerank"] = a.rerankDrivers[usage.Rerank]
	if a.rerankDefault == nil {
		a.rerankDefault = a.rerankDrivers[usage.Rerank]
	}

	a.chatUsage["query"] = a.chatDrivers[usage.Chat]
	if a.chatDefault == nil {
		a.chatDefault = a.chatDrivers[usage.Chat]
	}

	// 设置Reader usage配置（使用provider_id）
	if usage.Reader != "" {
		a.readerUsage["reader"] = a.readerDrivers[usage.Reader]
		if a.readerDefault == nil {
			a.readerDefault = a.readerDrivers[usage.Reader]
		}
	}

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
