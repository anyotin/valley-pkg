package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"
)

type UserEvent struct {
	Type      string    `json:"type"`
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}

func TestPubSubService_SubscribeToEvents(t *testing.T) {
	ctx := context.Background()
	rdb, err := NewRedisClient(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer func(r *RedisClient) {
		err := r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(rdb)

	pubSubService := &PubSubService{
		rdb: rdb,
	}

	ready := make(chan interface{})

	// サブスクライバーの起動（別のゴルーチンで）
	go func() {
		err := pubSubService.SubscribeToEvents("user-events", ready, func(data []byte) error {
			var event UserEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}
			fmt.Printf("Received event: %+v\n", event)
			return nil
		})
		if err != nil {
			log.Printf("Subscription error: %v", err)
		}
	}()

	<-ready

	// イベントのパブリッシュ
	event := UserEvent{
		Type:      "user_created",
		UserID:    "123",
		Timestamp: time.Now(),
	}
	err = pubSubService.PublishEvent("user-events", event)
	if err != nil {
		log.Printf("Failed to publish event: %v", err)
	}
}
