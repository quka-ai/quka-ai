package types

const (
	NO_PAGINATION = 0

	NOT_DELETE = 0
	DELETED    = 1
)

type WsEventType int32

const (
	WS_EVENT_UNKNOWN            WsEventType = 0
	WS_EVENT_ASSISTANT_INIT     WsEventType = 1   // bot消息载体已创建
	WS_EVENT_ASSISTANT_CONTINUE WsEventType = 2   // bot 回复中
	WS_EVENT_ASSISTANT_DONE     WsEventType = 3   // bot 回复完成
	WS_EVENT_ASSISTANT_FAILED   WsEventType = 4   // bot 请求失败
	WS_EVENT_TOOL_INIT          WsEventType = 5   // bot 工具调用初始化
	WS_EVENT_TOOL_CONTINUE      WsEventType = 6   // bot 工具调用
	WS_EVENT_TOOL_DONE          WsEventType = 7   // bot 工具调用结束
	WS_EVENT_TOOL_FAILED        WsEventType = 8   // bot 工具调用失败
	WS_EVENT_MESSAGE_PUBLISH    WsEventType = 100 // 新消息推送
	WS_EVENT_SYSTEM_ONSUBSCRIBE WsEventType = 300 // IMTopic 成功订阅
	WS_EVENT_SYSTEM_UNSUBSCRIBE WsEventType = 301 // IMTopic 取消订阅
	WS_EVENT_OTHERS             WsEventType = 400 // 其他未定义事件
)

type SystemContextGenConditionType uint8
type RequestAssistantMode uint8

const (
	GEN_SUMMARY_ONLY SystemContextGenConditionType = 1
	GEN_CONTEXT      SystemContextGenConditionType = 2

	GEN_MODE_NORMAL RequestAssistantMode = 1
	GEN_MODE_REGEN  RequestAssistantMode = 2
)

const (
	LANGUAGE_EN_KEY = "en"
	LANGUAGE_CN_KEY = "zh-CN"
)

const (
	// 系统内部频道（服务端订阅，用于跨实例通信）
	SYSTEM_CHANNEL_STOP_CHAT_STREAM = "system:stop_chat_stream"

	// 已废弃：FireTower 事件常量（保留用于向后兼容，实际不再使用）
	TOWER_EVENT_CLOSE_CHAT_STREAM = "/qukaai/event/chat/close_stream"

	FIXED_S3_UPLOAD_PATH_PREFIX   = "/assets/s3/"
	DEFAULT_APPID                 = "quka"
)

// StopChatStreamMessage 停止聊天流信号消息（用于跨实例通信）
type StopChatStreamMessage struct {
	SessionID string `json:"session_id"`
	Timestamp int64  `json:"timestamp"`
}
