package srv

import (
	"context"
	"net/http"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// CentrifugeManager 定义Centrifuge管理器接口，避免循环导入
type CentrifugeManager interface {
	PublishMessage(channel string, data []byte) error
	PublishJSON(channel string, data interface{}) error
	HandleWebSocket(w http.ResponseWriter, r *http.Request) error
	Shutdown(ctx context.Context) error

	// Tower兼容方法
	PublishStreamMessage(topic string, eventType types.WsEventType, data interface{}) error
	PublishStreamMessageWithSubject(topic string, subject string, eventType types.WsEventType, data interface{}) error
	PublishMessageMeta(topic string, eventType types.WsEventType, data interface{}) error
	RegisterStreamSignal(sessionID string, closeFunc func()) func()
	NewCloseChatStreamSignal(sessionID string) error
	PublishSessionReName(topic string, sessionID, name string) error
}

// CentrifugeSetupFunc 定义创建Centrifuge管理器的函数类型
type CentrifugeSetupFunc func() (CentrifugeManager, error)

func ApplyCentrifuge(setupFunc CentrifugeSetupFunc) ApplyFunc {
	return func(s *Srv) {
		var err error
		if s.centrifuge, err = setupFunc(); err != nil {
			panic(err)
		}
	}
}
