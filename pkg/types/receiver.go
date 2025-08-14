package types

import "github.com/quka-ai/quka-ai/pkg/mark"

type ReceiveFunc func(msg MessageContent, progressStatus MessageProgress) error
type DoneFunc func(err error) error

// websocket 推送实现
type Messager interface {
	PublishMessage(_type WsEventType, data any) error
}

type Receiver interface {
	IsStream() bool
	Copy() Receiver
	GetReceiveFunc() ReceiveFunc
	GetDoneFunc(callback func(msg *ChatMessage)) DoneFunc
	RecvMessageInit(ext ChatMessageExt) error
	MessageID() string

	VariableHandler() mark.VariableHandler
}
