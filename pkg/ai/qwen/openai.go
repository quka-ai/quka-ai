package qwen

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
	NAME = "qwen"
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

	if model.ChatModel == "" {
		model.ChatModel = "qwen-plus"
	}
	if model.EmbeddingModel == "" {
		model.EmbeddingModel = "text-embedding-v3"
	}

	return &Driver{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

func (s *Driver) Lang() string {
	return ai.MODEL_BASE_LANGUAGE_CN
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
			return ai.EmbeddingResult{}, fmt.Errorf("Error creating embedding: %w", err)
		}
		for _, v := range resp.Data {
			result = append(result, v.Embedding)
		}
		r.Usage.PromptTokens += resp.Usage.PromptTokens
		r.Usage.CompletionTokens += resp.Usage.CompletionTokens
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

func (s *Driver) NewQuery(ctx context.Context, model string, query []*types.MessageContext) *ai.QueryOptions {
	opts := ai.NewQueryOptions(ctx, s, model, query)
	return opts
}

func (s *Driver) NewVisionQuery(ctx context.Context, model string, query []*types.MessageContext) *ai.QueryOptions {
	opts := ai.NewQueryOptions(ctx, s, model, query)
	return opts
}

func (s *Driver) NewEnhance(ctx context.Context) *ai.EnhanceOptions {
	return ai.NewEnhance(ctx, s)
}

func convertModelToVLModel(model string) string {
	if strings.Contains(model, "vl") {
		return model
	}
	switch model {
	case "qwen-plus":
		return "qwen-vl-plus-latest"
	case "qwen-max":
		return "qwen-vl-max"
	default:
		return "qwen-vl-plus-latest"
	}
}

func (s *Driver) QueryStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	needToSwitchVL := false
	for _, v := range req.Messages {
		if len(v.MultiContent) > 0 {
			needToSwitchVL = true
			break
		}
	}

	if needToSwitchVL {
		req.Model = convertModelToVLModel(req.Model)
	}

	resp, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Completion error: %w", err)
	}

	slog.Debug("Query", slog.Any("query_stream", req), slog.String("driver", NAME))

	return resp, nil
}

func (s *Driver) Query(ctx context.Context, query []*types.MessageContext) (ai.GenerateResponse, error) {
	needToSwitchVL := false
	messages := lo.Map(query, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
		if len(item.MultiContent) > 0 {
			needToSwitchVL = true
		}
		return openai.ChatCompletionMessage{
			Role:         item.Role.String(),
			Content:      item.Content,
			MultiContent: item.MultiContent,
		}
	})
	model := lo.If(needToSwitchVL, convertModelToVLModel(s.model.ChatModel)).Else(s.model.ChatModel)
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	slog.Debug("Query", slog.Any("query", req), slog.String("driver", NAME))

	var result ai.GenerateResponse
	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return result, fmt.Errorf("Completion error: %w", err)
	}

	result.Received = append(result.Received, resp.Choices[0].Message.Content)
	result.Usage = &resp.Usage
	result.Model = resp.Model

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
				Description: "你从用户描述内容中分析出对应关键内容或关键技术的标签，以便用户后续归类相关的内容，需要以数组的形式组织该字段的值",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"title": {
				Type:        jsonschema.String,
				Description: "为用户提供的内容自动生成标题填入该字段",
			},
			"summary": {
				Type:        jsonschema.String,
				Description: "请将处理后的总结内容填入该字段中",
			},
			"date_time": {
				Type:        jsonschema.String,
				Description: "用户内容中提到的时间，时间格式为 year-month-day hour:minute，如果无法提取时间，请留空",
			},
		},
		Required: []string{"tags", "title", "summary"},
	}

	f := openai.FunctionDefinition{
		Name:        SummarizeFuncName,
		Description: "对文本内容的预处理结果",
		Parameters:  params,
	}
	t := openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &f,
	}

	// simulate user asking a question that requires the function
	dialogue := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ReplaceVarCN(ai.PROMPT_PROCESS_CONTENT_CN)},
		{Role: openai.ChatMessageRoleUser, Content: *doc},
	}
	var result ai.SummarizeResult
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
	result.Model = resp.Model
	return result, nil
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

// func (s *Driver) EnhanceQuery(ctx context.Context, prompt, query string) (ai.EnhanceQueryResult, error) {
// 	slog.Debug("EnhanceQuery", slog.String("driver", NAME))
// 	// describe the function & its inputs
// 	params := jsonschema.Definition{
// 		Type: jsonschema.Object,
// 		Properties: map[string]jsonschema.Definition{
// 			"querys": {
// 				Type:        jsonschema.Array,
// 				Description: "将用户可能的查询问题列出在该字段中",
// 				Items: &jsonschema.Definition{
// 					Type: jsonschema.String,
// 				},
// 			},
// 		},
// 		Required: []string{"querys"},
// 	}

// 	f := openai.FunctionDefinition{
// 		Name:        "enhance_query",
// 		Description: "增强用户提问的信息，获取更多相同的提问方式",
// 		Parameters:  params,
// 	}
// 	t := openai.Tool{
// 		Type:     openai.ToolTypeFunction,
// 		Function: &f,
// 	}

// 	req := openai.ChatCompletionRequest{
// 		Model: s.model.ChatModel,
// 		Messages: []openai.ChatCompletionMessage{
// 			{
// 				Role:    types.USER_ROLE_SYSTEM.String(),
// 				Content: prompt,
// 			},
// 			{
// 				Role:    types.USER_ROLE_USER.String(),
// 				Content: query,
// 			},
// 		},
// 		Tools:     []openai.Tool{t},
// 		MaxTokens: 200,
// 	}

// 	var (
// 		funcCallResult EnhanceQueryResult
// 		result         ai.EnhanceQueryResult
// 	)

// 	resp, err := s.client.CreateChatCompletion(ctx, req)
// 	if err != nil || len(resp.Choices) != 1 {
// 		return result, fmt.Errorf("Completion error: err:%v len(choices):%v\n", err,
// 			len(resp.Choices))
// 	}

// 	for _, v := range resp.Choices[0].Message.ToolCalls {
// 		if v.Function.Name != "enhance_query" {
// 			continue
// 		}
// 		if err = json.Unmarshal([]byte(v.Function.Arguments), &funcCallResult); err != nil {
// 			return result, fmt.Errorf("failed to unmarshal func call arguments of ChunkResult, %w", err)
// 		}

// 		result.News = funcCallResult.Querys
// 	}

// 	result.Original = query
// 	result.Model = resp.Model
// 	result.Usage = &resp.Usage
// 	return result, nil
// }

func (s *Driver) Chunk(ctx context.Context, doc *string) (ai.ChunkResult, error) {
	slog.Debug("Chunk", slog.String("driver", NAME))
	// describe the function & its inputs
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"tags": {
				Type:        jsonschema.Array,
				Description: "你从用户描述内容中分析出对应关键内容或关键技术的标签，以便用户后续归类相关的内容，需要以数组的形式组织该字段的值",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"title": {
				Type:        jsonschema.String,
				Description: "为用户提供的内容自动生成标题填入该字段",
			},
			"chunks": {
				Type:        jsonschema.Array,
				Description: "分类好的内容块填入该字段",
				Items: &jsonschema.Definition{
					Type: jsonschema.String,
				},
			},
			"date_time": {
				Type:        jsonschema.String,
				Description: "分析用户内容中提到的时间，时间格式为 year-month-day hour:minute，如果你认为用户提供的内容中没有关于时间的描述，请留空",
			},
		},
		Required: []string{"tags", "title", "chunks"},
	}

	f := openai.FunctionDefinition{
		Name:        "chunk",
		Description: "对文本内容的分块处理结果",
		Parameters:  params,
	}
	t := openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &f,
	}
	// simulate user asking a question that requires the function
	dialogue := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ReplaceVarCN(ai.PROMPT_CHUNK_CONTENT_CN)},
		{Role: openai.ChatMessageRoleUser, Content: strings.ReplaceAll(*doc, "\n", "")},
	}
	var result ai.ChunkResult
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
			return result, fmt.Errorf("failed to unmarshal func call arguments of ChunkResult, %w", err)
		}
	}

	result.Model = resp.Model
	result.Usage = &resp.Usage
	return result, nil
}
