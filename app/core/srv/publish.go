package srv

import (
	"encoding/json"
	"log/slog"

	"github.com/holdno/firetower/protocol"
	fireprotocol "github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/quka-ai/quka-ai/pkg/socket/firetower"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type Tower struct {
	pusher *firetower.SelfPusher[PublishData]
	tower.Manager[PublishData]
	systemEventRegistry *EventRegistry
}

type PublishData struct {
	Subject string            `json:"subject"`
	Version string            `json:"version"`
	Type    types.WsEventType `json:"type"`
	Data    any               `json:"data"`
}

func (c *PublishData) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte(""), nil
	}
	return json.Marshal(c)
}

func (c *PublishData) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == `""` {
		return nil
	}
	return json.Unmarshal(data, c)
}

func SetupSocketSrv() (*Tower, error) {
	tower, pusher, err := firetower.SetupFiretower[PublishData]()
	if err != nil {
		return nil, err
	}

	return &Tower{
		pusher:              pusher,
		Manager:             tower,
		systemEventRegistry: newEventRegistry(),
	}, nil
}

func ApplyTower() ApplyFunc {
	return func(s *Srv) {
		var err error
		if s.tower, err = SetupSocketSrv(); err != nil {
			panic(err)
		}
	}
}

func (t *Tower) NewMessage(imtopic string, _type fireprotocol.FireOperation, data PublishData) *fireprotocol.FireInfo[PublishData] {
	fire := t.NewFire(fireprotocol.SourceSystem, t.pusher)
	fire.Message.Topic = imtopic
	fire.Message.Type = _type
	fire.Message.Data = data
	return fire
}

func (t *Tower) PublishMessageMeta(topic string, logic types.WsEventType, data *types.MessageMeta) error {
	return t.publish(topic, fireprotocol.PublishOperation, PublishData{
		Subject: "on_message_init",
		Version: "v1",
		Type:    logic,
		Data:    data,
	})
}

func (t *Tower) PublishStreamMessage(topic string, logic types.WsEventType, data any) error {
	return t.publish(topic, fireprotocol.PublishOperation, PublishData{
		Subject: "on_message",
		Version: "v1",
		Type:    logic,
		Data:    data,
	})
}

func (t *Tower) PublishStreamMessageWithSubject(topic string, subject string, logic types.WsEventType, data any) error {
	return t.publish(topic, fireprotocol.PublishOperation, PublishData{
		Subject: subject,
		Version: "v1",
		Type:    logic,
		Data:    data,
	})
}

func (t *Tower) PublishSessionReName(topic string, sessionID, name string) error {
	return t.publish(topic, fireprotocol.PublishOperation, PublishData{
		Subject: "session_rename",
		Version: "v1",
		Type:    types.WS_EVENT_OTHERS,
		Data: map[string]string{
			"session_id": sessionID,
			"name":       name,
		},
	})
}

func (t *Tower) publish(imtopic string, _type fireprotocol.FireOperation, data PublishData) error {
	fire := t.NewMessage(imtopic, _type, data)
	return t.Publish(fire)
}

func (t *Tower) RegisterServerSideTopic() {
	serverSideTower := t.BuildServerSideTower(utils.RandomStr(32))
	fire := t.NewFire(fireprotocol.SourceSystem, t.pusher)
	serverSideTower.Subscribe(fire.Context, []string{ // 订阅事件
		types.TOWER_EVENT_CLOSE_CHAT_STREAM,
	})
	serverSideTower.SetReceivedHandler(func(fi fireprotocol.ReadOnlyFire[PublishData]) (ignore bool) {
		slog.Debug("new signal", slog.String("topic", fi.GetMessage().Topic))
		switch fi.GetMessage().Topic {
		case types.TOWER_EVENT_CLOSE_CHAT_STREAM:
			// 关闭GPT消息回复状态
			closeFunc, exist := t.systemEventRegistry.ChatStreamSignal.Get(fi.GetMessage().Data.Subject)
			if exist {
				closeFunc()
			}
		default:
			slog.Warn("got unknown handler signal", slog.String("topic", fi.GetMessage().Topic))
		}
		return
	})
}

type EventRegistry struct {
	ChatStreamSignal cmap.ConcurrentMap[string, func()]
}

func newEventRegistry() *EventRegistry {
	return &EventRegistry{
		ChatStreamSignal: cmap.New[func()](),
	}
}

func (e *Tower) RegisterStreamSignal(msgID string, closeFunc func()) (removeFunc func()) {
	e.systemEventRegistry.ChatStreamSignal.Set(msgID, closeFunc)
	return func() {
		e.systemEventRegistry.ChatStreamSignal.Remove(msgID)
	}
}

func (t *Tower) NewCloseChatStreamSignal(msgID string) error {
	fire := t.NewFire(protocol.SourceSystem, t.pusher)
	fire.Message.Topic = types.TOWER_EVENT_CLOSE_CHAT_STREAM
	fire.Message.Data = PublishData{
		Subject: msgID,
		Version: "v1",
		Type:    types.WS_EVENT_OTHERS,
	}

	return t.Publish(fire)
}
