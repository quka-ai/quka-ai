package srv

import (
	"encoding/json"

	fireprotocol "github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"

	"github.com/quka-ai/quka-ai/pkg/socket/firetower"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type Tower struct {
	pusher *firetower.SelfPusher[PublishData]
	tower.Manager[PublishData]
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
		pusher:  pusher,
		Manager: tower,
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
