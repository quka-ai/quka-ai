package types

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

const (
	AGENT_TYPE_NONE    = ""
	AGENT_TYPE_NORMAL  = "rag"
	AGENT_TYPE_JOURNAL = "journal"
	AGENT_TYPE_BUTLER  = "butler"
	AGENT_TYPE_AUTO    = "auto"
)

var registeredAgents = map[string][]string{
	AGENT_TYPE_AUTO:    {"Quka", "QukaAI"},
	AGENT_TYPE_NORMAL:  {"Quka", "QukaAI"},
	AGENT_TYPE_JOURNAL: {"Journal", "工作助理"},
	AGENT_TYPE_BUTLER:  {"Butler", "管家"},
	AGENT_TYPE_NONE:    {},
}

type AICallOptions struct {
	GenMode        RequestAssistantMode
	Docs           *RAGDocs
	GetDocsFunc    func() (RAGDocs, error)
	Model          string
	EnableThinking bool
	EnableSearch   bool
}

func FilterAgent(userQuery string) string {
	for agentType, keywords := range registeredAgents {
		for _, keyword := range keywords {
			if strings.Contains(userQuery, "@"+keyword) {
				return agentType
			}
		}
	}
	return AGENT_TYPE_NONE
}

const AssistantFailedMessage = "Sorry, I'm wrong"

type ChatMessagePart struct {
	Type     openai.ChatMessagePartType  `json:"type,omitempty"`
	Text     string                      `json:"text,omitempty"`
	ImageURL *openai.ChatMessageImageURL `json:"image_url,omitempty"`
}

type MessageContext struct {
	Role         MessageUserRole `json:"role"`
	Content      string          `json:"content"`
	MultiContent []openai.ChatMessagePart
}

type ResponseChoice struct {
	ID           string
	Message      string
	FinishReason string
	Error        error
}

type MessageContent interface {
	Bytes() json.RawMessage
	Type() MessageType
}

type TextMessage struct {
	Text string `json:"text"`
}

func (t *TextMessage) Bytes() json.RawMessage {
	return json.RawMessage(t.Text)
}

func (t *TextMessage) Type() MessageType {
	return MESSAGE_TYPE_TEXT
}

const (
	TOOL_STATUS_NONE = iota
	TOOL_STATUS_RUNNING
	TOOL_STATUS_SUCCESS
	TOOL_STATUS_FAILED
)

type ToolTips struct {
	ID       string `json:"id"`
	ToolName string `json:"tool_name"`
	Status   int    `json:"status"`
	Content  string `json:"content"`
}

func (t *ToolTips) Bytes() json.RawMessage {
	raw, _ := json.Marshal(t)
	return raw
}

func (t *ToolTips) Type() MessageType {
	return MESSAGE_TYPE_TOOL_TIPS
}

// AgentContext 包含了创建和使用 AI Agent 所需的所有上下文信息
// 实现了 context.Context 接口，可以直接作为 context 使用
type AgentContext struct {
	context.Context

	// 业务标识信息
	SpaceID   string
	UserID    string
	SessionID string
	MessageID string

	// Agent 行为控制标志
	EnableThinking  bool // 是否开启思考模式
	EnableWebSearch bool // 是否开启联网搜索
}

// NewAgentContext 创建一个新的 AgentContext
func NewAgentContext(ctx context.Context, spaceID, userID, sessionID, messageID string) *AgentContext {
	return &AgentContext{
		Context:   ctx,
		SpaceID:   spaceID,
		UserID:    userID,
		SessionID: sessionID,
		MessageID: messageID,
		// 默认配置
		EnableThinking:  false,
		EnableWebSearch: false,
	}
}

// NewAgentContextWithOptions 创建一个带有自定义选项的 AgentContext
func NewAgentContextWithOptions(ctx context.Context, spaceID, userID, sessionID, messageID string, enableThinking, enableWebSearch bool) *AgentContext {
	return &AgentContext{
		Context:         ctx,
		SpaceID:         spaceID,
		UserID:          userID,
		SessionID:       sessionID,
		MessageID:       messageID,
		EnableThinking:  enableThinking,
		EnableWebSearch: enableWebSearch,
	}
}

// WithThinking 设置思考模式
func (ac *AgentContext) WithThinking(enable bool) *AgentContext {
	newCtx := *ac
	newCtx.EnableThinking = enable
	return &newCtx
}

// WithWebSearch 设置联网搜索
func (ac *AgentContext) WithWebSearch(enable bool) *AgentContext {
	newCtx := *ac
	newCtx.EnableWebSearch = enable
	return &newCtx
}

// WithDeadline 创建一个带有截止时间的新 AgentContext
func (ac *AgentContext) WithDeadline(deadline time.Time) (*AgentContext, context.CancelFunc) {
	ctx, cancel := context.WithDeadline(ac.Context, deadline)
	newCtx := *ac
	newCtx.Context = ctx
	return &newCtx, cancel
}

// WithTimeout 创建一个带有超时的新 AgentContext
func (ac *AgentContext) WithTimeout(timeout time.Duration) (*AgentContext, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ac.Context, timeout)
	newCtx := *ac
	newCtx.Context = ctx
	return &newCtx, cancel
}

// WithCancel 创建一个可取消的新 AgentContext
func (ac *AgentContext) WithCancel() (*AgentContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ac.Context)
	newCtx := *ac
	newCtx.Context = ctx
	return &newCtx, cancel
}
