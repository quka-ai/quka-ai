package jina

// provider for https://jina.ai/
// - reader
// - rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
)

type Driver struct {
	client *http.Client
	token  string
	models map[string]string
}

const (
	NAME = "jina"
)

func New(token string, models map[string]string) *Driver {
	return &Driver{
		client: &http.Client{},
		token:  token,
		models: models,
	}
}

func (s *Driver) applyBaseHeader(req *http.Request) {
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.token)
}

type ReaderResponse struct {
	Code   int             `json:"code"`
	Status int             `json:"status"`
	Data   ai.ReaderResult `json:"data"`
}

func (s *Driver) Reader(ctx context.Context, endpoint string) (*ai.ReaderResult, error) {
	slog.Debug("Reader", slog.String("driver", NAME))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://r.jina.ai/"+endpoint, nil)
	s.applyBaseHeader(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to request jina reader: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to request jina reader, %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result ReaderResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal response, %w", err)
	}

	return &result.Data, nil
}

type RerankRequestBody struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	TopN      int      `json:"top_n"`
	Documents []string `json:"documents"`
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

func (s *Driver) Rerank(ctx context.Context, query string, docs []*ai.RerankDoc) ([]ai.RankDocItem, *ai.Usage, error) {
	slog.Debug("Rerank", slog.String("driver", NAME))
	model := s.models["rerank"]
	request := RerankRequestBody{
		Model: model,
		Query: query,
		TopN:  len(docs),
		Documents: lo.Map(docs, func(item *ai.RerankDoc, _ int) string {
			return item.Content
		}),
	}

	raw, _ := json.Marshal(request)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.jina.ai/v1/rerank", bytes.NewReader(raw))
	s.applyBaseHeader(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to request jina reader: %w", err)
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
		Model: model,
		Usage: &openai.Usage{
			PromptTokens: result.Usage.TotalTokens,
		},
	}, nil
}
