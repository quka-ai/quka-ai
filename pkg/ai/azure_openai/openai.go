package azure_openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/samber/lo"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	NAME = "azure_openai"
)

type Driver struct {
	client *openai.Client
	model  ai.ModelName
}

func New(token, proxy string, model ai.ModelName) *Driver {
	cfg := openai.DefaultConfig(token)
	if proxy != "" {
		cfg.BaseURL = proxy
	}

	cfg.APIType = openai.APITypeAzure
	cfg.APIVersion = "2024-02-15-preview"

	if model.ChatModel == "" {
		model.ChatModel = openai.GPT4oMini
	}
	if model.EmbeddingModel == "" {
		model.EmbeddingModel = string(openai.LargeEmbedding3)
	}

	return &Driver{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

func (s *Driver) Lang() string {
	return ai.MODEL_BASE_LANGUAGE_EN
}

func (s *Driver) embedding(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	slog.Debug("Embedding", slog.String("driver", NAME))
	queryReq := openai.EmbeddingRequest{
		Model:      openai.EmbeddingModel(s.model.EmbeddingModel),
		Dimensions: 1024,
	}

	var (
		groups   [][]string
		result   [][]float32
		batchMax = 6
	)

	for i, v := range content {
		if i%batchMax == 0 {
			groups = append(groups, []string{})
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], v)
	}
	r := ai.EmbeddingResult{
		Usage: &openai.Usage{},
	}
	for _, v := range groups {
		// Create an embedding for the user query
		queryReq.Input = v
		resp, err := s.client.CreateEmbeddings(ctx, queryReq)
		if err != nil {
			return r, fmt.Errorf("Error creating embedding: %w", err)
		}
		for _, v := range resp.Data {
			result = append(result, v.Embedding)
		}

		r.Usage.CompletionTokens += resp.Usage.CompletionTokens
		r.Usage.PromptTokens += resp.Usage.PromptTokens
		r.Usage.TotalTokens += resp.Usage.TotalTokens
		r.Model = string(resp.Model)
	}
	r.Data = result

	return r, nil
}

func (s *Driver) EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error) {
	return s.embedding(ctx, "", content)
}

func (s *Driver) EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	return s.embedding(ctx, title, content)
}

func (s *Driver) NewQuery(ctx context.Context, query []*types.MessageContext) *ai.QueryOptions {
	return ai.NewQueryOptions(ctx, s, query)
}

func (s *Driver) NewEnhance(ctx context.Context) *ai.EnhanceOptions {
	return ai.NewEnhance(ctx, s)
}

func (s *Driver) MsgIsOverLimit(msgs []*types.MessageContext) bool {
	tokenNum, err := ai.NumTokens(lo.Map(msgs, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
		return openai.ChatCompletionMessage{
			Role:    item.Role.String(),
			Content: item.Content,
		}
	}), s.model.ChatModel)
	if err != nil {
		slog.Error("Failed to tik request token", slog.String("error", err.Error()), slog.String("driver", NAME), slog.String("model", s.model.ChatModel))
		return false
	}

	return tokenNum > 8000
}

func (s *Driver) EnhanceQuery(ctx context.Context, messages []openai.ChatCompletionMessage) (ai.EnhanceQueryResult, error) {
	slog.Debug("EnhanceQuery", slog.String("driver", NAME))

	req := openai.ChatCompletionRequest{
		Model:       s.model.ChatModel,
		Messages:    messages,
		Temperature: 0.1,
		MaxTokens:   200,
	}

	var (
		result ai.EnhanceQueryResult
	)

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil || len(resp.Choices) != 1 {
		return result, fmt.Errorf("Completion error: err:%v len(choices):%v\n", err,
			len(resp.Choices))
	}

	var enhanceQuerys []string
	if err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &enhanceQuerys); err != nil {
		return result, fmt.Errorf("failed to unmarshal query enhance result, %w", err)
	}

	result.News = enhanceQuerys
	result.Model = resp.Model
	result.Usage = &resp.Usage
	return result, nil
}

func (s *Driver) QueryStream(ctx context.Context, query []*types.MessageContext) (*openai.ChatCompletionStream, error) {

	req := openai.ChatCompletionRequest{
		Model:  s.model.ChatModel,
		Stream: true,
		Messages: lo.Map(query, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
			return openai.ChatCompletionMessage{
				Role:    item.Role.String(),
				Content: item.Content,
			}
		}),
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}

	resp, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Completion error: %w", err)
	}

	slog.Debug("Query", slog.Any("query_stream", req), slog.String("driver", NAME), slog.String("model", s.model.ChatModel))

	return resp, nil
}

func (s *Driver) Query(ctx context.Context, query []*types.MessageContext) (ai.GenerateResponse, error) {

	req := openai.ChatCompletionRequest{
		Model: s.model.ChatModel,
		Messages: lo.Map(query, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
			return openai.ChatCompletionMessage{
				Role:    item.Role.String(),
				Content: item.Content,
			}
		}),
	}

	result := ai.GenerateResponse{
		Model: s.model.ChatModel,
	}
	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return result, fmt.Errorf("Completion error: %w", err)

	}

	slog.Debug("Query", slog.Any("query", req), slog.String("driver", NAME), slog.String("model", s.model.ChatModel))

	result.Received = append(result.Received, resp.Choices[0].Message.Content)
	result.Usage = &resp.Usage

	return result, nil
}

const SummarizeFuncName = "summarize"

func (s *Driver) Summarize(ctx context.Context, doc *string) (ai.SummarizeResult, error) {
	slog.Debug("Summarize", slog.String("driver", NAME))
	// describe the function & its inputs
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"tags": {
				Type:        jsonschema.Array,
				Description: "Extract relevant keywords or technical tags from the user's description to assist the user in categorizing related content later. Organize these values in an array format.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"title": {
				Type:        jsonschema.String,
				Description: "Generate a title for the content provided by the user and fill in this field.",
			},
			"summary": {
				Type:        jsonschema.String,
				Description: "Processed summary content.",
			},
			"date_time": {
				Type:        jsonschema.String,
				Description: "The time mentioned in the user content, formatted as 'year-month-day hour:minute'. If no time can be extracted, leave it empty.",
			},
		},
		Required: []string{"tags", "title", "summary"},
	}

	f := openai.FunctionDefinition{
		Name:        SummarizeFuncName,
		Description: "Processed summary content.",
		Parameters:  params,
	}
	t := openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &f,
	}

	// simulate user asking a question that requires the function
	dialogue := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ReplaceVarCN(ai.PROMPT_PROCESS_CONTENT_EN)},
		{Role: openai.ChatMessageRoleUser, Content: *doc},
	}
	result := ai.SummarizeResult{
		Model: s.model.ChatModel,
	}
	resp, err := s.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model:    s.model.ChatModel,
			Messages: dialogue,
			Tools:    []openai.Tool{t},
		},
	)
	if err != nil || len(resp.Choices) != 1 {
		return result, fmt.Errorf("Completion error: err:%v len(choices):%v\n", err,
			len(resp.Choices))
	}
	for _, v := range resp.Choices[0].Message.ToolCalls {
		if v.Function.Name != SummarizeFuncName {
			continue
		}
		if err = json.Unmarshal([]byte(v.Function.Arguments), &result); err != nil {
			return result, fmt.Errorf("failed to unmarshal func call arguments of SummarizeResult, %w", err)
		}
	}

	result.Usage = &resp.Usage
	return result, nil
}

const PROMPT_CHUNK_CONTENT_CN = `
你是一位RAG技术专家，你需要将用户提供的内容进行分块处理(chunk)，你只对用户提供的内容做分块处理，用户并不是在与你聊天。
将内容分块的原因是期望embedding的匹配度能够更高，如果你认为用户提供的内容已经足够精简，则可以直接使用原文作为一个块。
请结合文章整体内容从主题来对用户内容进行分块，一定不能疏漏与块相关的上下文信息，例如时间点、节日、日期等。
注意：分块一定不能缺乏上下文信息，不能出现主语不明确的语句。
至少生成1个块，至多生成10个块。
至多提取5个标签。

### 分块处理过程

1. **解析内容**：首先，理解整个文本的上下文和结构。
2. **识别关键概念**：找出文本中的重要术语、方法、流程等。
3. **划分主题**：基于识别的关键概念，将文本划分为不同的主题或部分。
4. **生成描述**：为每个分块提供详细的描述，说明其在整体内容中的位置和意义。

### 错误的例子
"将这些事情做完"，这样的结果丢失了上下文，用户会不清楚"这些"指的是什么。

### 检查
分块结束后，重新检查所有分块，是否与用户所描述内容相关，若不相关则删除该分块。
`

func (s *Driver) Chunk(ctx context.Context, doc *string) (ai.ChunkResult, error) {
	slog.Debug("Chunk", slog.String("driver", NAME))
	// describe the function & its inputs
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"tags": {
				Type:        jsonschema.Array,
				Description: "Extract relevant keywords or technical tags from the user's description to assist the user in categorizing related content later. Organize these values in an array format.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"title": {
				Type:        jsonschema.String,
				Description: "Generate a title for the content provided by the user and fill in this field.",
			},
			"chunks": {
				Type:        jsonschema.Array,
				Description: "Processed chunks content.",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"date_time": {
				Type:        jsonschema.String,
				Description: "The time mentioned in the user content, formatted as 'year-month-day hour:minute'. If no time can be extracted, leave it empty.",
			},
		},
		Required: []string{"tags", "title", "chunks"},
	}

	f := openai.FunctionDefinition{
		Name:        "chunk",
		Description: "Processed chunks content.",
		Parameters:  params,
	}
	t := openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &f,
	}
	// simulate user asking a question that requires the function
	dialogue := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ReplaceVarCN(ai.PROMPT_CHUNK_CONTENT_EN)},
		{Role: openai.ChatMessageRoleUser, Content: strings.ReplaceAll(*doc, "\n", "")},
	}
	result := ai.ChunkResult{
		Model: s.model.ChatModel,
	}
	resp, err := s.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model:    s.model.ChatModel,
			Messages: dialogue,
			Tools:    []openai.Tool{t},
		},
	)
	if err != nil || len(resp.Choices) != 1 {
		return result, fmt.Errorf("Completion error: err:%v len(choices):%v\n", err,
			len(resp.Choices))
	}

	for _, v := range resp.Choices[0].Message.ToolCalls {
		if v.Function.Name != "chunk" {
			continue
		}
		if err = json.Unmarshal([]byte(v.Function.Arguments), &result); err != nil {
			return result, fmt.Errorf("failed to unmarshal func call arguments of SummarizeResult, %w", err)
		}
	}

	result.Usage = &resp.Usage
	return result, nil
}
