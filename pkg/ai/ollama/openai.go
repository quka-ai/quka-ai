package ollama

import (
	"context"
	"fmt"
	"log/slog"

	openai "github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/pkg/ai"
)

const (
	NAME = "ollama"
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
