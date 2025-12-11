package redis

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRedis(t *testing.T) {
	// 接続テスト
	ctx := context.Background()
	r, err := NewRedisClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func(r *RedisClient) {
		err := r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(r)

	_, err = r.client.Ping(ctx).Result()

	assert.NoError(t, err)
	t.Logf("Successfully connected to Redis!")
}

func TestRedisClient_Write(t *testing.T) {
	ctx := context.Background()
	r, _ := NewRedisClient(ctx)

	err := r.Set("test-key", "1234567890", 0)
	assert.NoError(t, err)

	result, err := r.Get("test-key")
	assert.Equal(t, "1234567890", result)

	v := map[string]interface{}{
		"name":  "田中太郎",
		"email": "tanaka@example.com",
		"age":   "30",
	}

	err = r.HSet("test-hash", v)
	assert.NoError(t, err)

	result, err = r.HGet("test-hash", "name")
	assert.NoError(t, err)
	assert.Equal(t, "田中太郎", result)

	// 全フィールドを取得
	all, err := r.HGetAll("test-hash")
	if err != nil {
		panic(err)
	}
	fmt.Printf("User profile: %v\n", all)
}
