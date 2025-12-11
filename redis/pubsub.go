package redis

import (
	"encoding/json"
	"log"
)

type PubSubService struct {
	rdb *RedisClient
}

func NewPubSubService(rdb *RedisClient) *PubSubService {
	return &PubSubService{
		rdb: rdb,
	}
}

// PublishEvent パブリッシャーの実装
func (ps *PubSubService) PublishEvent(channel string, event interface{}) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return ps.rdb.client.Publish(ps.rdb.ctx, channel, eventData).Err()
}

// SubscribeToEvents サブスクライバーの実装
func (ps *PubSubService) SubscribeToEvents(channel string, readyChan chan<- interface{}, handler func([]byte) error) error {
	pubsub := ps.rdb.client.Subscribe(ps.rdb.ctx, channel)
	defer pubsub.Close()
	// サブスクリプション確認
	_, err := pubsub.Receive(ps.rdb.ctx)
	if err != nil {
		return err
	}

	// ここで「購読開始できたよ」通知
	readyChan <- true

	ch := pubsub.Channel()
	for msg := range ch {
		log.Printf("Received message: %s", msg.Payload)
		if err := handler([]byte(msg.Payload)); err != nil {
			log.Printf("Error handling message: %v", err)
		}
	}
	return nil
}
