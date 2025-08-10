package types

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/quka-ai/quka-ai/pkg/object-storage/s3"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
)

type ChatMessage struct {
	ID        string            `db:"id" json:"id"`
	SpaceID   string            `db:"space_id" json:"space_id"`
	SessionID string            `db:"session_id" json:"session_id"`
	UserID    string            `db:"user_id" json:"user_id"`
	Role      MessageUserRole   `db:"role" json:"role"`
	Message   string            `db:"message" json:"message"`
	MsgType   MessageType       `db:"msg_type" json:"msg_type"`
	IsEncrypt int               `db:"is_encrypt" json:"is_encrypt"`
	SendTime  int64             `db:"send_time" json:"send_time"`
	Complete  MessageProgress   `db:"complete" json:"complete"`
	Sequence  int64             `db:"sequence" json:"sequence"`
	MsgBlock  int64             `db:"msg_block" json:"msg_block"`
	Attach    ChatMessageAttach `db:"attach" json:"attach"`
}

type ChatMessageAttach []ChatAttach

func (s ChatMessageAttach) ToMultiContent(text string, fileReader interface {
	DownloadFile(ctx context.Context, filePath string) (*s3.GetObjectResult, error)
}) []openai.ChatMessagePart {
	return lo.Map(s, func(item ChatAttach, _ int) openai.ChatMessagePart {
		var (
			base64Image string
			err         error
		)
		if item.SignURL != "" {
			base64Image, err = utils.FileToBase64(lo.If(item.SignURL != "", item.SignURL).Else(item.URL))
			if err != nil {
				slog.Error("Failed to convert file to base64", "error", err, "file", lo.If(item.SignURL != "", item.SignURL).Else(item.URL))
				return openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: fmt.Sprintf("Failed to download file: %s", item.URL),
				}
			}
		} else {
			if fileReader != nil {
				res, err := fileReader.DownloadFile(context.Background(), item.URL)
				if err != nil {
					slog.Error("Failed to download file", "error", err, "file", item.URL)
					return openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: fmt.Sprintf("Failed to download file: %s", item.URL),
					}
				}

				if base64Image, err = utils.FileBytesToBase64(res.File, res.FileType); err != nil {
					slog.Error("Failed to convert file bytes to base64", "error", err, "file", item.URL)
					return openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: fmt.Sprintf("Failed to convert file bytes to base64: %s", item.URL),
					}
				}
			} else {
				base64Image, err = utils.FileToBase64(item.URL)
				if err != nil {
					slog.Error("Failed to convert file to base64", "error", err, "file", item.URL)
					return openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: fmt.Sprintf("Failed to convert file to base64: %s", item.URL),
					}
				}
			}
		}
		return openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			Text: text,
			ImageURL: &openai.ChatMessageImageURL{
				URL: base64Image,
			},
		}
	})
}

func (s *ChatMessageAttach) String() string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func (a *ChatMessageAttach) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		return a.scanBytes(src)
	case string:
		return a.scanBytes([]byte(src))
	case nil:
		*a = nil
		return nil
	}

	return fmt.Errorf("pq: cannot convert %T to ChatMessageAttach", src)
}

func (a *ChatMessageAttach) scanBytes(src []byte) error {
	if len(src) == 0 {
		a = &ChatMessageAttach{}
		return nil
	}
	return json.Unmarshal(src, a)
}

const (
	MESSAGE_IS_ENCRYPT  int = 1
	MESSAGE_NOT_ENCRYPT int = 0
)

type RAGDocs struct {
	Refs []QueryResult
	Docs []*PassageInfo
}

type PassageInfo struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Resource string `json:"resource"`
	DateTime string `json:"date_time"`
	SW       Undo   `json:"-"`
}

type Undo interface {
	Undo(string) string
	Map() map[string]string
}

type CreateChatMessageArgs struct {
	ID             string
	Message        string
	MsgType        MessageType
	SendTime       int64
	Agent          string
	ChatAttach     []ChatAttach
	EnableThinking bool
	EnableSearch   bool
}

type ChatAttach struct {
	Type          string `json:"type"`
	URL           string `json:"url"`
	SignURL       string `json:"-"`
	AIDescription string `json:"ai_desc"`
}

type MessageUserRole int8

const (
	USER_ROLE_UNKNOWN   MessageUserRole = 0
	USER_ROLE_USER      MessageUserRole = 1 // 用户
	USER_ROLE_ASSISTANT MessageUserRole = 2 // bot
	USER_ROLE_SYSTEM    MessageUserRole = 3
	USER_ROLE_TOOL      MessageUserRole = 4 // 工具
)

func (s MessageUserRole) String() string {
	return GetMessageUserRoleStr(s)
}

func GetMessageUserRoleStr(r MessageUserRole) string {
	switch r {
	case USER_ROLE_ASSISTANT:
		return "assistant"
	case USER_ROLE_USER:
		return "user"
	case USER_ROLE_SYSTEM:
		return "system"
	case USER_ROLE_TOOL:
		return "tool"
	default:
		return "unknown"
	}
}

func GetMessageUserRole(r string) MessageUserRole {
	switch r {
	case "assistant":
		return USER_ROLE_ASSISTANT
	case "user":
		return USER_ROLE_USER
	case "system":
		return USER_ROLE_SYSTEM
	case "tool":
		return USER_ROLE_TOOL
	default:
		return USER_ROLE_UNKNOWN
	}
}

type MessageProgress int8

const (
	MESSAGE_PROGRESS_UNKNOWN         MessageProgress = 0
	MESSAGE_PROGRESS_COMPLETE        MessageProgress = 1
	MESSAGE_PROGRESS_UNCOMPLETE      MessageProgress = 2
	MESSAGE_PROGRESS_GENERATING      MessageProgress = 3
	MESSAGE_PROGRESS_FAILED          MessageProgress = 4
	MESSAGE_PROGRESS_CANCELED        MessageProgress = 5
	MESSAGE_PROGRESS_INTERCEPTED     MessageProgress = 6
	MESSAGE_PROGRESS_REQUEST_TIMEOUT MessageProgress = 7
)

type MessageType int8

const (
	MESSAGE_TYPE_UNKNOWN   MessageType = 0
	MESSAGE_TYPE_TEXT      MessageType = 1
	MESSAGE_TYPE_TOOL_TIPS MessageType = 2
)

type EvaluateType int8
type GenerationStatusType int8

const (
	EVALUATE_TYPE_UNKNOWN EvaluateType = 0
	EVALUATE_TYPE_LIKE    EvaluateType = 1 // 喜欢
	EVALUATE_TYPE_DISLIKE EvaluateType = 2 // 不喜欢

	GENERATE_STATUS_UNKNOWN    GenerationStatusType = 0 // 未发生过交互
	GENERATE_STATUS_PAUSE      GenerationStatusType = 1 // 暂停生成
	GENERATE_STATUS_REGENERATE GenerationStatusType = 2 // 已重新生成
)

type InterceptAnswers []string

func (i InterceptAnswers) String() string {
	if len(i) == 0 {
		return ""
	}
	return i[utils.Random(0, len(i)-1)]
}

type MessageMeta struct {
	MsgID       string          `json:"message_id"`
	SeqID       int64           `json:"sequence"`
	SendTime    int64           `json:"send_time"`
	Role        MessageUserRole `json:"role"`
	UserID      string          `json:"user_id"`
	SessionID   string          `json:"session_id"`
	SpaceID     string          `json:"space_id"`
	Complete    MessageProgress `json:"complete"`
	MessageType MessageType     `json:"message_type"`
	Message     MessageTypeImpl `json:"message"`
	Attach      []ChatAttach    `json:"attach"`
}

type MessageTypeImpl struct {
	Text string `json:"text"`
}

type MessageDetail struct {
	Meta *MessageMeta `json:"meta"`
	Ext  *MessageExt  `json:"ext"`
}
type MessageExt struct {
	IsRead           []string     `json:"is_read"`
	RelDocs          []string     `json:"rel_docs"`
	Evaluate         EvaluateType `json:"evaluate"`
	ToolName         string       `json:"tool_name"`
	ToolArgs         string       `json:"tool_args"`
	IsEvaluateEnable bool         `json:"is_evaluate_enable"`
}

type StreamMessage struct {
	MessageID string          `json:"message_id"`
	SessionID string          `json:"session_id"`
	Message   string          `json:"message,omitempty"`
	ToolTips  json.RawMessage `json:"tool_tips,omitempty"`
	StartAt   int             `json:"start_at"`
	Complete  int32           `json:"complete"`
	MsgType   MessageType     `json:"msg_type"`
}
