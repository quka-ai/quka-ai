package types

import (
	"encoding/json"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const (
	AGENT_TYPE_NONE    = ""
	AGENT_TYPE_NORMAL  = "rag"
	AGENT_TYPE_JOURNAL = "journal"
	AGENT_TYPE_BUTLER  = "butler"
)

var registeredAgents = map[string][]string{
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
