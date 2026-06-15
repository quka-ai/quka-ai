package v1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteChatCompletionPayload(t *testing.T) {
	payload := map[string]any{
		"model":  LLMGatewayModelAlias,
		"stream": true,
		"tools":  []any{map[string]any{"type": "function"}},
	}

	stream, err := rewriteChatCompletionPayload(payload, "gpt-test")
	require.NoError(t, err)
	require.True(t, stream)
	require.Equal(t, "gpt-test", payload["model"])
	require.Equal(t, []any{map[string]any{"type": "function"}}, payload["tools"])

	streamOptions, ok := payload["stream_options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, streamOptions["include_usage"])
}

func TestRewriteChatCompletionPayloadRejectsUnknownModel(t *testing.T) {
	payload := map[string]any{
		"model": "other-model",
	}

	_, err := rewriteChatCompletionPayload(payload, "gpt-test")
	require.Error(t, err)
	var gatewayErr *LLMGatewayError
	require.ErrorAs(t, err, &gatewayErr)
	require.Equal(t, "model_not_allowed", gatewayErr.Code)
}

func TestParseUsageFromSSELine(t *testing.T) {
	usage := parseUsageFromSSELine([]byte(`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19,"prompt_tokens_details":{"cached_tokens":5}}}` + "\n"))
	require.NotNil(t, usage)
	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 5, usage.CachedPromptTokens())
	require.Equal(t, 7, usage.CompletionTokens)
	require.Equal(t, 19, usage.TotalTokens)

	require.Nil(t, parseUsageFromSSELine([]byte("data: [DONE]\n")))
	require.Nil(t, parseUsageFromSSELine([]byte(`data: {"choices":[{"delta":{"content":"hi"}}],"usage":null}`+"\n")))
}

func TestLLMGatewayUsageCachedPromptTokens(t *testing.T) {
	require.Equal(t, 4, (&llmGatewayUsage{
		PromptTokens:         10,
		CacheReadInputTokens: 4,
	}).CachedPromptTokens())
	require.Equal(t, 6, (&llmGatewayUsage{
		PromptTokens:         10,
		CacheReadInputTokens: 4,
		PromptTokensDetails: &llmGatewayPromptTokenDetails{
			CachedTokens: 6,
		},
	}).CachedPromptTokens())
	require.Equal(t, 10, (&llmGatewayUsage{
		PromptTokens: 10,
		PromptTokensDetails: &llmGatewayPromptTokenDetails{
			CachedTokens: 12,
		},
	}).CachedPromptTokens())
}

func TestLLMGatewayModelCacheStore(t *testing.T) {
	cache := newLLMGatewayModelCacheStore()
	loadCount := 0
	load := func() (*llmGatewayResolvedModel, error) {
		loadCount++
		return &llmGatewayResolvedModel{
			ModelName: "gpt-test",
			BaseURL:   "https://example.com/v1",
			APIKey:    "sk-test",
		}, nil
	}

	first, err := cache.Get("core:chat", load)
	require.NoError(t, err)
	require.Equal(t, "gpt-test", first.ModelName)

	first.ModelName = "mutated"
	second, err := cache.Get("core:chat", load)
	require.NoError(t, err)
	require.Equal(t, "gpt-test", second.ModelName)
	require.Equal(t, 1, loadCount)

	cache.Clear()
	_, err = cache.Get("core:chat", load)
	require.NoError(t, err)
	require.Equal(t, 2, loadCount)
}

func TestLLMGatewayModelCacheStoreDoesNotCacheErrors(t *testing.T) {
	cache := newLLMGatewayModelCacheStore()
	loadCount := 0
	loadErr := errors.New("not configured")
	_, err := cache.Get("core:chat", func() (*llmGatewayResolvedModel, error) {
		loadCount++
		return nil, loadErr
	})
	require.ErrorIs(t, err, loadErr)

	_, err = cache.Get("core:chat", func() (*llmGatewayResolvedModel, error) {
		loadCount++
		return &llmGatewayResolvedModel{ModelName: "gpt-test"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, loadCount)
}

func TestChatCompletionsURL(t *testing.T) {
	require.Equal(t, "https://example.com/v1/chat/completions", chatCompletionsURL("https://example.com/v1"))
	require.Equal(t, "https://example.com/v1/chat/completions", chatCompletionsURL("https://example.com/v1/"))
	require.Equal(t, "https://example.com/v1/chat/completions", chatCompletionsURL("https://example.com/v1/chat/completions"))
}
