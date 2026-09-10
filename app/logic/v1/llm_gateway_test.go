package v1

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestResponseBodyReaderDecodesGzip(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": []string{"gzip"},
		},
		Body: io.NopCloser(bytes.NewReader(gzipBytes(t, []byte(`{"ok":true}`)))),
	}

	reader, decoded, err := responseBodyReader(resp)
	require.NoError(t, err)
	require.True(t, decoded)
	defer reader.Close()

	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, string(body))
}

func TestCopyResponseHeadersDropsContentEncodingAfterDecode(t *testing.T) {
	src := http.Header{
		"Content-Type":     []string{"application/json"},
		"Content-Encoding": []string{"gzip"},
		"Content-Length":   []string{"128"},
	}
	dst := http.Header{}

	copyResponseHeaders(dst, src, true)

	require.Equal(t, "application/json", dst.Get("Content-Type"))
	require.Empty(t, dst.Get("Content-Encoding"))
	require.Empty(t, dst.Get("Content-Length"))
}

func TestProxyJSONResponseDecodesGzip(t *testing.T) {
	logic := &LLMGatewayLogic{}
	recorder := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":     []string{"application/json"},
			"Content-Encoding": []string{"gzip"},
		},
		Body: io.NopCloser(bytes.NewReader(gzipBytes(t, []byte(`{"id":"chatcmpl-test","choices":[]}`)))),
	}

	err := logic.proxyJSONResponse(recorder, resp, "gpt-test", "request-test")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.Empty(t, recorder.Header().Get("Content-Encoding"))
	require.Equal(t, `{"id":"chatcmpl-test","choices":[]}`, recorder.Body.String())
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buf.Bytes()
}
