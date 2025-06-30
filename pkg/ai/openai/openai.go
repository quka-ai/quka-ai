package openai

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
	NAME = "openai"
)

type Driver struct {
	client *openai.Client
	model  ai.ModelName
}

func NewClient(token, proxy string) *openai.Client {
	cfg := openai.DefaultConfig(token)
	if proxy != "" {
		cfg.BaseURL = proxy
	}

	return openai.NewClientWithConfig(cfg)
}

func New(token, proxy string, model ai.ModelName) *Driver {
	cfg := openai.DefaultConfig(token)
	if proxy != "" {
		cfg.BaseURL = proxy
	}

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
		ChatTemplateKwargs: map[string]any{
			"enable_thinking": true,
		},
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
				Role:         item.Role.String(),
				Content:      item.Content,
				MultiContent: item.MultiContent,
			}
		}),
	}

	var result ai.GenerateResponse
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
