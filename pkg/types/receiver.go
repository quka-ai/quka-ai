package types

type ReceiveFunc func(startAt int32, msg MessageContent, progressStatus MessageProgress) error
type DoneFunc func(startAt int32) error

// websocket 推送实现
type Messager interface {
	PublishMessage(_type WsEventType, data any) error
}

type Receiver interface {
	IsStream() bool
	GetReceiveFunc() ReceiveFunc
	GetDoneFunc(callback func(msg *ChatMessage)) DoneFunc
	RecvMessageInit(userReqMsg *ChatMessage, msgID string, seqID int64, ext ChatMessageExt) error
}
