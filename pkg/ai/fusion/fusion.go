package fusion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	NAME = "fusion"
)

type Driver struct {
	client     *openai.Client
	httpClient *http.Client
	endpoint   string
	model      string
	token      string
}

func NewClient(token, endpoint string) *openai.Client {
	cfg := openai.DefaultConfig(token)

	if endpoint != "" {
		cfg.BaseURL = endpoint
	}

	return openai.NewClientWithConfig(cfg)
}

func New(token, endpoint string, model string) *Driver {
	cfg := openai.DefaultConfig(token)

	if endpoint != "" {
		cfg.BaseURL = endpoint
	}

	return &Driver{
		client:   openai.NewClientWithConfig(cfg),
		model:    model,
		endpoint: endpoint,
		token:    token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Driver) Lang() string {
	return ai.MODEL_BASE_LANGUAGE_EN
}

func (s *Driver) embedding(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	slog.Debug("Embedding", slog.String("driver", NAME))
	queryReq := openai.EmbeddingRequest{
		Model:      openai.EmbeddingModel(s.model),
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

func (s *Driver) applyBaseHeader(req *http.Request) {
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.token)
}

func (s *Driver) EmbeddingForQuery(ctx context.Context, content []string) (ai.EmbeddingResult, error) {
	return s.embedding(ctx, "", content)
}

func (s *Driver) EmbeddingForDocument(ctx context.Context, title string, content []string) (ai.EmbeddingResult, error) {
	return s.embedding(ctx, title, content)
}

func (s *Driver) NewQuery(ctx context.Context, query []*types.MessageContext) *ai.QueryOptions {
	return ai.NewQueryOptions(ctx, s, s.model, query)
}

func (s *Driver) NewVisionQuery(ctx context.Context, query []*types.MessageContext) *ai.QueryOptions {
	return ai.NewQueryOptions(ctx, s, s.model, query)
}

func (s *Driver) MsgIsOverLimit(msgs []*types.MessageContext) bool {
	tokenNum, err := ai.NumTokens(lo.Map(msgs, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
		return openai.ChatCompletionMessage{
			Role:    item.Role.String(),
			Content: item.Content,
		}
	}), s.model)
	if err != nil {
		slog.Error("Failed to tik request token", slog.String("error", err.Error()), slog.String("driver", NAME), slog.String("model", s.model))
		return false
	}

	return tokenNum > 80000
}

func (s *Driver) QueryStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	resp, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Completion error: %w", err)
	}

	slog.Debug("Query", slog.Any("query_stream", req), slog.String("driver", NAME), slog.String("model", s.model))

	return resp, nil
}

func (s *Driver) Query(ctx context.Context, query []*types.MessageContext) (*openai.ChatCompletionResponse, error) {

	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: lo.Map(query, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
			return openai.ChatCompletionMessage{
				Role:         item.Role.String(),
				Content:      item.Content,
				MultiContent: item.MultiContent,
			}
		}),
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Completion error: %w", err)

	}

	slog.Debug("Query", slog.Any("query", req), slog.String("driver", NAME), slog.String("model", s.model))
	return &resp, nil
}

func (s *Driver) Chat(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Completion error: %w", err)
	}

	slog.Debug("Query", slog.Any("query_stream", req), slog.String("driver", NAME), slog.String("model", s.model))

	return &resp, nil
}

func (s *Driver) ChatStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	resp, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Completion error: %w", err)
	}

	return resp, nil
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
		Model: s.model,
	}
	resp, err := s.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model:    s.model,
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
		Model: s.model,
	}
	resp, err := s.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model:    s.model,
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

type RerankResponse struct {
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Results []RerankResponseItem `json:"results"`
}

type RerankResponseItem struct {
	Index    int `json:"index"`
	Document struct {
		Text string `json:"text"`
	} `json:"document"`
	RelevanceScore float64 `json:"relevance_score"`
}

type RerankRequestBody struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	TopN      int      `json:"top_n"`
	Documents []string `json:"documents"`
}

func (s *Driver) Rerank(ctx context.Context, query string, docs []*ai.RerankDoc) ([]ai.RankDocItem, *ai.Usage, error) {
	slog.Debug("Rerank", slog.String("driver", NAME))
	request := RerankRequestBody{
		Model: s.model,
		Query: query,
		TopN:  len(docs),
		Documents: lo.Map(docs, func(item *ai.RerankDoc, _ int) string {
			return item.Content
		}),
	}

	raw, _ := json.Marshal(request)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint+"/rerank", bytes.NewReader(raw))
	s.applyBaseHeader(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to request fusion reader: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("Failed to request rerank api, %s", string(body))
	}

	var result RerankResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, err
	}

	var rank []ai.RankDocItem

	for _, v := range result.Results {
		item := docs[v.Index]
		rank = append(rank, ai.RankDocItem{
			ID:    item.ID,
			Score: v.RelevanceScore,
		})
	}

	return rank, &ai.Usage{
		Model: s.model,
		Usage: &openai.Usage{
			PromptTokens: result.Usage.TotalTokens,
		},
	}, nil
}
