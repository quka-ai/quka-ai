package srv

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/samber/lo"
	oai "github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/azure_openai"
	"github.com/quka-ai/quka-ai/pkg/ai/deepseek"
	"github.com/quka-ai/quka-ai/pkg/ai/jina"
	"github.com/quka-ai/quka-ai/pkg/ai/ollama"
	"github.com/quka-ai/quka-ai/pkg/ai/openai"
	"github.com/quka-ai/quka-ai/pkg/ai/qwen"
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
	DescribeImage(ctx context.Context, lang, imageURL string) (ai.GenerateResponse, error)
}

type AIConfig struct {
	Gemini   Gemini      `toml:"gemini"`
	Openai   Openai      `toml:"openai"`
	QWen     QWen        `toml:"qwen"`
	DeepSeek DeepSeek    `toml:"deepseek"`
	Jina     Jina        `toml:"jina"`
	Azure    AzureOpenai `toml:"azure_openai"`
	Ollama   Ollama      `toml:"ollama"`
	Agent    AgentDriver `toml:"agent"`
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
	Token          string            `toml:"token"`
	ReaderEndpoint string            `toml:"reader_endpoint"`
	ApiEndpoint    string            `toml:"api_endpoint"`
	Models         map[string]string `toml:"models"`
}

func (cfg *Jina) Install(root *AI) {
	var oai any
	oai = jina.New(cfg.Token, cfg.Models)

	installAI(root, jina.NAME, oai)
}

func (c *Jina) FromENV() {
	c.Token = os.Getenv("BREW_API_AI_JINA_TOKEN")
	c.ReaderEndpoint = os.Getenv("BREW_API_AI_JINA_READER_ENDPOINT")
}

func (c *AIConfig) FromENV() {
	c.Usage = make(map[string]string)
	c.Usage["embedding.query"] = os.Getenv("BREW_API_AI_USAGE_E_QUERY")
	c.Usage["embedding.document"] = os.Getenv("BREW_API_AI_USAGE_E_DOCUMENT")
	c.Usage["query"] = os.Getenv("BREW_API_AI_USAGE_QUERY")
	c.Usage["summarize"] = os.Getenv("BREW_API_AI_USAGE_SUMMARIZE")
	c.Usage["enhance_query"] = os.Getenv("BREW_API_AI_USAGE_ENHANCE_QUERY")
	c.Usage["reader"] = os.Getenv("BREW_API_AI_USAGE_READER")

	c.Gemini.FromENV()
	c.Openai.FromENV()
	c.Azure.FromENV()
	c.QWen.FromENV()
	c.Jina.FromENV()
	c.DeepSeek.FromENV()
}

func (c *DeepSeek) FromENV() {
	c.Token = os.Getenv("BREW_API_AI_DEEPSEEK_TOKEN")
	c.Endpoint = os.Getenv("BREW_API_AI_DEEPSEEK_ENDPOINT")
}

func (c *Gemini) FromENV() {
	c.Token = os.Getenv("BREW_API_AI_GEMINI_TOKEN")
}

func (c *Openai) FromENV() {
	c.Token = os.Getenv("BREW_API_AI_OPENAI_TOKEN")
	c.Endpoint = os.Getenv("BREW_API_AI_OPENAI_ENDPOINT")
}

func (c *AzureOpenai) FromENV() {
	c.Token = os.Getenv("BREW_API_AI_AZURE_OPENAI_TOKEN")
	c.Endpoint = os.Getenv("BREW_API_AI_AZURE_OPENAI_ENDPOINT")
}

func (c *QWen) FromENV() {
	c.Token = os.Getenv("BREW_API_AI_ALI_TOKEN")
	c.Endpoint = os.Getenv("BREW_API_AI_ALI_ENDPOINT")
}

type Gemini struct {
	Token string `toml:"token"`
}

type DeepSeek struct {
	Token          string `toml:"token"`
	Endpoint       string `toml:"endpoint"`
	EmbeddingModel string `toml:"embedding_model"`
	ChatModel      string `toml:"chat_model"`
}

func (cfg *DeepSeek) Install(root *AI) {
	var oai any
	oai = deepseek.New(cfg.Token, cfg.Endpoint, ai.ModelName{
		ChatModel:      cfg.ChatModel,
		EmbeddingModel: cfg.EmbeddingModel,
	})

	installAI(root, strings.ToLower(deepseek.NAME), oai)
}

type Ollama struct {
	Token          string `toml:"token"`
	Endpoint       string `toml:"endpoint"`
	EmbeddingModel string `toml:"embedding_model"`
	ChatModel      string `toml:"chat_model"`
}

func (cfg *Ollama) Install(root *AI) {
	var oai any
	oai = ollama.New(cfg.Token, cfg.Endpoint, ai.ModelName{
		ChatModel:      cfg.ChatModel,
		EmbeddingModel: cfg.EmbeddingModel,
	})

	installAI(root, strings.ToLower(ollama.NAME), oai)
}

type Openai struct {
	Token          string `toml:"token"`
	Endpoint       string `toml:"endpoint"`
	EmbeddingModel string `toml:"embedding_model"`
	ChatModel      string `toml:"chat_model"`
}

func (cfg *Openai) Install(root *AI) {
	var oai any
	oai = openai.New(cfg.Token, cfg.Endpoint, ai.ModelName{
		ChatModel:      cfg.ChatModel,
		EmbeddingModel: cfg.EmbeddingModel,
	})

	installAI(root, strings.ToLower(openai.NAME), oai)
}

type AzureOpenai struct {
	Token          string `toml:"token"`
	Endpoint       string `toml:"endpoint"`
	EmbeddingModel string `toml:"embedding_model"`
	ChatModel      string `toml:"chat_model"`
}

func (cfg *AzureOpenai) Install(root *AI) {
	var oai any
	oai = azure_openai.New(cfg.Token, cfg.Endpoint, ai.ModelName{
		ChatModel:      cfg.ChatModel,
		EmbeddingModel: cfg.EmbeddingModel,
	})

	installAI(root, strings.ToLower(azure_openai.NAME), oai)
}

type QWen struct {
	Token          string `toml:"token"`
	Endpoint       string `toml:"endpoint"`
	EmbeddingModel string `toml:"embedding_model"`
	ChatModel      string `toml:"chat_model"`
}

func (cfg *QWen) Install(root *AI) {
	var oai any
	oai = qwen.New(cfg.Token, cfg.Endpoint, ai.ModelName{
		ChatModel:      cfg.ChatModel,
		EmbeddingModel: cfg.EmbeddingModel,
	})

	installAI(root, strings.ToLower(qwen.NAME), oai)
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

func (s *AI) DescribeImage(ctx context.Context, lang, imageURL string) (ai.GenerateResponse, error) {
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
	return s.rerankDefault.Rerank(ctx, query, docs)
}

func (s *AI) Lang() string {
	if d := s.chatUsage["query"]; d != nil {
		return d.Lang()
	}
	return s.chatDefault.Lang()
}

func (s *AI) EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error) {
	if d := s.embedUsage["embedding.query"]; d != nil {
		return d.EmbeddingForQuery(ctx, content)
	}
	return s.embedDefault.EmbeddingForQuery(ctx, content)
}

func (s *AI) EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	if d := s.embedUsage["embedding.document"]; d != nil {
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

var (
	ERROR_UNSUPPORTED_FEATURE = errors.New("Unsupported feature")
)

// Option Feature
func (s *AI) Reader(ctx context.Context, endpoint string) (*ai.ReaderResult, error) {
	if d := s.readerUsage["reader"]; d != nil {
		return d.Reader(ctx, endpoint)
	}

	if s.readerDefault == nil {
		return nil, ERROR_UNSUPPORTED_FEATURE
	}
	return s.readerDefault.Reader(ctx, endpoint)
}

func installAI(a *AI, name string, driver any) {
	if d, ok := driver.(ChatAI); ok {
		a.chatDrivers[name] = d
	}

	if d, ok := driver.(EmbeddingAI); ok {
		a.embedDrivers[name] = d
	}

	if d, ok := driver.(ai.Enhance); ok {
		a.enhanceDrivers[name] = d
	}

	if d, ok := driver.(ReaderAI); ok {
		a.readerDrivers[name] = d
	}

	if d, ok := driver.(VisionAI); ok {
		a.visionDrivers[name] = d
	}

	if d, ok := driver.(RerankAI); ok {
		a.rerankDrivers[name] = d
	}
}

func SetupAI(cfg AIConfig) (*AI, error) {
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

	cfg.Openai.Install(a)
	cfg.Azure.Install(a)
	cfg.QWen.Install(a)
	cfg.Jina.Install(a)
	cfg.DeepSeek.Install(a)
	cfg.Ollama.Install(a)
	// TODO: Gemini install

	for k, v := range cfg.Usage {
		v = strings.ToLower(v)
		switch k {
		case "reader":
			a.readerUsage[k] = a.readerDrivers[v]
		case "embedding.document", "embedding.query":
			a.embedUsage[k] = a.embedDrivers[v]
		case "enhance_query":
			a.enhanceUsage[k] = a.enhanceDrivers[v]
		case "vision":
			a.visionUsage[k] = a.visionDrivers[v]
		case "rerank":
			a.rerankUsage[k] = a.rerankDrivers[v]
		default:
			a.chatUsage[k] = a.chatDrivers[v]
		}
	}

	for _, v := range a.chatDrivers {
		a.chatDefault = v
		break
	}

	for _, v := range a.embedDrivers {
		a.embedDefault = v
		break
	}

	for _, v := range a.enhanceDrivers {
		a.enhanceDefault = v
		break
	}

	for _, v := range a.readerDrivers {
		a.readerDefault = v
		break
	}

	for _, v := range a.visionDrivers {
		a.visionDefault = v
		break
	}

	for _, v := range a.rerankDrivers {
		a.rerankDefault = v
		break
	}

	if a.chatDefault == nil || a.embedDefault == nil {
		panic("AI driver of chat and embedding must be set")
	}

	return a, nil
}

type ApplyFunc func(s *Srv)

func ApplyAI(cfg AIConfig) ApplyFunc {
	return func(s *Srv) {
		s.ai, _ = SetupAI(cfg)
	}
}
