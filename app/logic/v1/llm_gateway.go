package v1

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	goopenai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/singleflight"
)

const (
	LLMGatewayModelAlias    = "quka-hermes"
	llmGatewayMaxBody       = 16 << 20
	llmGatewayModelCacheTTL = 30 * time.Second
)

type LLMGatewayLogic struct {
	ctx    context.Context
	core   *core.Core
	client *http.Client
	UserInfo
}

type LLMGatewayError struct {
	Status  int
	Code    string
	Message string
}

func (e *LLMGatewayError) Error() string {
	return e.Message
}

type LLMGatewayModelsResponse struct {
	Object string            `json:"object"`
	Data   []LLMGatewayModel `json:"data"`
}

type LLMGatewayModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type llmGatewayResolvedModel struct {
	ModelName string
	BaseURL   string
	APIKey    string
}

type llmGatewayUsage struct {
	PromptTokens           int                           `json:"prompt_tokens"`
	CompletionTokens       int                           `json:"completion_tokens"`
	TotalTokens            int                           `json:"total_tokens"`
	PromptTokensDetails    *llmGatewayPromptTokenDetails `json:"prompt_tokens_details"`
	CacheReadInputTokens   int                           `json:"cache_read_input_tokens"`
	PromptCacheHitTokens   int                           `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens  int                           `json:"prompt_cache_miss_tokens"`
	CacheCreationTokens    int                           `json:"cache_creation_input_tokens"`
	CacheCreationInputToks int                           `json:"cache_creation_tokens"`
}

type llmGatewayPromptTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type llmGatewayModelCacheEntry struct {
	model     *llmGatewayResolvedModel
	expiresAt time.Time
}

type llmGatewayModelCacheStore struct {
	mu      sync.RWMutex
	entries map[string]llmGatewayModelCacheEntry
	group   singleflight.Group
}

var defaultLLMGatewayModelCache = newLLMGatewayModelCacheStore()

func newLLMGatewayModelCacheStore() *llmGatewayModelCacheStore {
	return &llmGatewayModelCacheStore{
		entries: make(map[string]llmGatewayModelCacheEntry),
	}
}

func InvalidateLLMGatewayModelCache() {
	defaultLLMGatewayModelCache.Clear()
}

func NewLLMGatewayLogic(ctx context.Context, core *core.Core) *LLMGatewayLogic {
	return &LLMGatewayLogic{
		ctx:      ctx,
		core:     core,
		client:   http.DefaultClient,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

func (l *LLMGatewayLogic) ListModels() (*LLMGatewayModelsResponse, error) {
	_, err := l.resolveActiveChatModel()
	if err != nil {
		return nil, err
	}
	return &LLMGatewayModelsResponse{
		Object: "list",
		Data: []LLMGatewayModel{
			{
				ID:      LLMGatewayModelAlias,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "quka-ai",
			},
		},
	}, nil
}

func (l *LLMGatewayLogic) ProxyChatCompletions(w http.ResponseWriter, r *http.Request) error {
	resolved, err := l.resolveActiveChatModel()
	if err != nil {
		return err
	}

	body, err := readLimitedBody(r.Body, llmGatewayMaxBody)
	if err != nil {
		return gatewayError(http.StatusBadRequest, "invalid_request_error", err.Error())
	}
	defer r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return gatewayError(http.StatusBadRequest, "invalid_json", "Invalid JSON request body")
	}

	stream, err := rewriteChatCompletionPayload(payload, resolved.ModelName)
	if err != nil {
		return err
	}

	upstreamBody, err := json.Marshal(payload)
	if err != nil {
		return gatewayError(http.StatusInternalServerError, "internal_error", "Failed to encode upstream request")
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, chatCompletionsURL(resolved.BaseURL), bytes.NewReader(upstreamBody))
	if err != nil {
		return gatewayError(http.StatusInternalServerError, "internal_error", "Failed to build upstream request")
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+resolved.APIKey)
	upstreamReq.Header.Set("Content-Type", "application/json")
	if accept := r.Header.Get("Accept"); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}
	if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
		upstreamReq.Header.Set("User-Agent", userAgent)
	}

	resp, err := l.client.Do(upstreamReq)
	if err != nil {
		slog.Error("LLM Gateway upstream request failed", slog.String("error", err.Error()))
		return gatewayError(http.StatusBadGateway, "upstream_error", "Failed to call upstream LLM provider")
	}
	defer resp.Body.Close()

	requestID := utils.GenUniqIDStr()
	if stream && resp.StatusCode < http.StatusBadRequest {
		return l.proxyStreamingResponse(w, resp, resolved.ModelName, requestID)
	}
	return l.proxyJSONResponse(w, resp, resolved.ModelName, requestID)
}

func (l *LLMGatewayLogic) resolveActiveChatModel() (*llmGatewayResolvedModel, error) {
	cacheKey := fmt.Sprintf("%p:%s", l.core, types.AI_USAGE_CHAT)
	return defaultLLMGatewayModelCache.Get(cacheKey, l.resolveActiveChatModelUncached)
}

func (l *LLMGatewayLogic) resolveActiveChatModelUncached() (*llmGatewayResolvedModel, error) {
	model, err := l.core.GetActiveModelConfig(l.ctx, types.AI_USAGE_CHAT)
	if err != nil {
		return nil, gatewayError(http.StatusServiceUnavailable, "gateway_not_configured", "LLM Gateway chat model is not configured")
	}
	if model.Status != types.StatusEnabled {
		return nil, gatewayError(http.StatusServiceUnavailable, "gateway_not_configured", "LLM Gateway chat model is disabled")
	}
	if model.Provider == nil {
		return nil, gatewayError(http.StatusServiceUnavailable, "gateway_not_configured", "LLM Gateway provider is not configured")
	}
	if model.Provider.Status != types.StatusEnabled {
		return nil, gatewayError(http.StatusServiceUnavailable, "gateway_not_configured", "LLM Gateway provider is disabled")
	}
	if strings.TrimSpace(model.Provider.ApiUrl) == "" || strings.TrimSpace(model.Provider.ApiKey) == "" {
		return nil, gatewayError(http.StatusServiceUnavailable, "gateway_not_configured", "LLM Gateway provider credentials are incomplete")
	}

	apiKey := model.Provider.ApiKey
	if decrypted, err := l.core.DecryptData([]byte(apiKey)); err == nil {
		apiKey = string(decrypted)
	} else {
		slog.Warn("LLM Gateway failed to decrypt provider API key, using raw value", slog.String("provider_id", model.ProviderID), slog.String("error", err.Error()))
	}

	return &llmGatewayResolvedModel{
		ModelName: model.ModelName,
		BaseURL:   model.Provider.ApiUrl,
		APIKey:    apiKey,
	}, nil
}

func rewriteChatCompletionPayload(payload map[string]any, upstreamModel string) (bool, error) {
	modelValue, hasModel := payload["model"]
	if hasModel {
		modelName, ok := modelValue.(string)
		if !ok {
			return false, gatewayError(http.StatusBadRequest, "invalid_request_error", "model must be a string")
		}
		modelName = strings.TrimSpace(modelName)
		if modelName != "" && modelName != LLMGatewayModelAlias && modelName != upstreamModel {
			return false, gatewayError(http.StatusBadRequest, "model_not_allowed", "Requested model is not available through this gateway")
		}
	}
	payload["model"] = upstreamModel

	stream, _ := payload["stream"].(bool)
	if stream {
		streamOptions, _ := payload["stream_options"].(map[string]any)
		if streamOptions == nil {
			streamOptions = map[string]any{}
		}
		streamOptions["include_usage"] = true
		payload["stream_options"] = streamOptions
	}
	return stream, nil
}

func (l *LLMGatewayLogic) proxyJSONResponse(w http.ResponseWriter, resp *http.Response, modelName, requestID string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return gatewayError(http.StatusBadGateway, "upstream_error", "Failed to read upstream response")
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(body); err != nil {
		return err
	}

	if resp.StatusCode < http.StatusBadRequest {
		if usage := parseUsageFromJSON(body); usage != nil {
			l.recordUsage(modelName, requestID, usage)
		}
	}
	return nil
}

func (l *LLMGatewayLogic) proxyStreamingResponse(w http.ResponseWriter, resp *http.Response, modelName, requestID string) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return gatewayError(http.StatusInternalServerError, "streaming_unsupported", "Streaming is not supported by the server")
	}

	copyResponseHeaders(w.Header(), resp.Header)
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	reader := bufio.NewReader(resp.Body)
	var recorded bool
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if !recorded {
				if usage := parseUsageFromSSELine(line); usage != nil {
					recorded = true
					l.recordUsage(modelName, requestID, usage)
				}
			}
			if _, writeErr := w.Write(line); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	if !recorded {
		claims := l.GetUserInfo()
		slog.Warn("LLM Gateway stream finished without usage",
			slog.String("user_id", claims.User),
			slog.String("model", modelName),
			slog.String("request_id", requestID))
	}
	return nil
}

func (l *LLMGatewayLogic) recordUsage(modelName, requestID string, usage *llmGatewayUsage) {
	if usage == nil || (usage.PromptTokens == 0 && usage.CachedPromptTokens() == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0) {
		return
	}

	claims := l.GetUserInfo()
	resp := process.NewRecordUserChatUsageRequest(modelName, types.USAGE_SUB_TYPE_LLM_GATEWAY, "", claims.User, requestID, &goopenai.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}, usage.CachedPromptTokens())
	if resp == nil {
		slog.Warn("LLM Gateway skipped usage record because KnowledgeProcess is unavailable",
			slog.String("user_id", claims.User),
			slog.String("model", modelName),
			slog.String("request_id", requestID))
	}
}

func parseUsageFromJSON(body []byte) *llmGatewayUsage {
	var response struct {
		Usage *llmGatewayUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}
	return normalizeUsage(response.Usage)
}

func parseUsageFromSSELine(line []byte) *llmGatewayUsage {
	trimmed := strings.TrimSpace(string(line))
	if !strings.HasPrefix(trimmed, "data:") {
		return nil
	}
	data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if data == "" || data == "[DONE]" {
		return nil
	}
	return parseUsageFromJSON([]byte(data))
}

func normalizeUsage(usage *llmGatewayUsage) *llmGatewayUsage {
	if usage == nil {
		return nil
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.PromptTokens == 0 && usage.CachedPromptTokens() == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	return usage
}

func (u *llmGatewayUsage) CachedPromptTokens() int {
	if u == nil {
		return 0
	}

	cached := maxInt(u.CacheReadInputTokens, u.PromptCacheHitTokens)
	if u.PromptTokensDetails != nil {
		cached = maxInt(cached, u.PromptTokensDetails.CachedTokens)
	}
	if cached < 0 {
		return 0
	}
	if u.PromptTokens > 0 && cached > u.PromptTokens {
		return u.PromptTokens
	}
	return cached
}

func (c *llmGatewayModelCacheStore) Get(key string, load func() (*llmGatewayResolvedModel, error)) (*llmGatewayResolvedModel, error) {
	if model := c.getFresh(key, time.Now()); model != nil {
		return model, nil
	}

	value, err, _ := c.group.Do(key, func() (any, error) {
		if model := c.getFresh(key, time.Now()); model != nil {
			return model, nil
		}

		model, err := load()
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.entries[key] = llmGatewayModelCacheEntry{
			model:     model.clone(),
			expiresAt: time.Now().Add(llmGatewayModelCacheTTL),
		}
		c.mu.Unlock()

		return model.clone(), nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*llmGatewayResolvedModel), nil
}

func (c *llmGatewayModelCacheStore) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]llmGatewayModelCacheEntry)
	c.mu.Unlock()
}

func (c *llmGatewayModelCacheStore) getFresh(key string, now time.Time) *llmGatewayResolvedModel {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !now.Before(entry.expiresAt) {
		return nil
	}
	return entry.model.clone()
}

func (m *llmGatewayResolvedModel) clone() *llmGatewayResolvedModel {
	if m == nil {
		return nil
	}
	return &llmGatewayResolvedModel{
		ModelName: m.ModelName,
		BaseURL:   m.BaseURL,
		APIKey:    m.APIKey,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func readLimitedBody(body io.Reader, limit int64) ([]byte, error) {
	limited := io.LimitReader(body, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("request body is too large")
	}
	return data, nil
}

func chatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if shouldSkipResponseHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func shouldSkipResponseHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade", "content-length":
		return true
	default:
		return false
	}
}

func gatewayError(status int, code, message string) *LLMGatewayError {
	return &LLMGatewayError{Status: status, Code: code, Message: message}
}
