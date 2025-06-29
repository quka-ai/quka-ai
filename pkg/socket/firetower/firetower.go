package firetower

import (
	"github.com/holdno/firetower/config"
	"github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
	"github.com/holdno/firetower/utils"
)

type SelfPusher[T any] struct {
	ip      string
	channel chan *protocol.FireInfo[T]
}

func (s SelfPusher[T]) Publish(fire *protocol.FireInfo[T]) error {
	s.channel <- fire
	return nil
}

func (s *SelfPusher[T]) Receive() chan *protocol.FireInfo[T] {
	return s.channel
}

func (s *SelfPusher[T]) UserID() string {
	return "system"
}

func (s *SelfPusher[T]) ClientID() string {
	return s.ip
}

func SetupFiretower[T any]() (tower.Manager[T], *SelfPusher[T], error) {
	msgChan := make(chan *protocol.FireInfo[T], 10000)

	localIP, err := utils.GetIP()
	if err != nil {
		localIP = "localhost"
	}
	pusher := &SelfPusher[T]{
		ip:      localIP,
		channel: msgChan,
	}

	// 全局唯一id生成器
	tm, err := tower.Setup[T](config.FireTowerConfig{
		ReadChanLens:  5,
		WriteChanLens: 1000,
		Heartbeat:     60,
		ServiceMode:   config.SingleMode,
		Bucket: config.BucketConfig{
			Num:              4,
			CentralChanCount: 1000,
			BuffChanCount:    1000,
			ConsumerNum:      1, // 为了保证消费顺序，需要将consumerNum设置为1
		},
	}, tower.BuildWithPusher[T](pusher))
	if err != nil {
		return nil, nil, err
	}

	return tm, pusher, nil
}
