package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"time"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient(ctx context.Context) (*RedisClient, error) {
	// Redisクライアントの初期化
	client := redis.NewClient(&redis.Options{
		Addr:         "localhost:16379", // Redis サーバーのアドレス
		Password:     "",                // パスワード（必要な場合）
		DB:           0,                 // 使用するデータベース番号
		DialTimeout:  10 * time.Second,  // Redisサーバーへの新規接続時のタイムアウト
		ReadTimeout:  30 * time.Second,  // Redisサーバーからレスポンスを読み取る時のタイムアウト
		WriteTimeout: 30 * time.Second,  // Redisサーバーにコマンドを書き込む（送信する）時のタイムアウト
		PoolSize:     10,                // コネクションプールの最大コネクション数
		PoolTimeout:  30 * time.Second,  // コネクションプールがいっぱいの場合、新しいコネクションが利用可能になるまで最大どれだけ待機する
	})

	// 接続テスト
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %v", err)
	}

	return &RedisClient{client, ctx}, nil
}

// Close クライアントのクローズ処理
func (rc *RedisClient) Close() error {
	log.Println("Close Redis Client")
	return rc.client.Close()
}

func (rc *RedisClient) Set(key string, value string, expire time.Duration) error {
	// 文字列の設定
	err := rc.client.Set(rc.ctx, key, value, expire).Err()
	if err != nil {
		return err
	}

	return nil
}

// HSet 複数フィールドセット
func (rc *RedisClient) HSet(key string, values map[string]interface{}) error {
	var args []interface{}
	for k, v := range values {
		args = append(args, k, v)
	}

	err := rc.client.HSet(rc.ctx, key, args...).Err()
	if err != nil {
		return err
	}

	return nil
}

func (rc *RedisClient) Get(key string) (string, error) {
	// 文字列の設定
	result, err := rc.client.Get(rc.ctx, key).Result()
	if err != nil {
		return "", err
	}

	return result, nil
}

// HGet ハッシュから指定されたフィールドの値を取得
func (rc *RedisClient) HGet(key, value string) (string, error) {
	result, err := rc.client.HGet(rc.ctx, key, value).Result()
	if err != nil {
		return "", err
	}

	return result, nil
}

// HGetAll ハッシュから全てのフィールドの値を取得
func (rc *RedisClient) HGetAll(key string) (map[string]string, error) {
	result, err := rc.client.HGetAll(rc.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return result, nil
}
