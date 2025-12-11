package redis

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type DistributedLock struct {
	redis  *RedisClient
	key    string
	value  string
	expiry time.Duration
}

func NewDistributedLock(rc *RedisClient, key string) *DistributedLock {
	return &DistributedLock{
		redis:  rc,
		key:    fmt.Sprintf("lock:%s", key),
		value:  uuid.New().String(),
		expiry: 30 * time.Second,
	}
}

// Acquire ロックの取得
func (dl *DistributedLock) Acquire() (bool, error) {
	return dl.redis.client.SetNX(dl.redis.ctx, dl.key, dl.value, dl.expiry).Result()
}

// Release　ロックの解放（自分が取得したロックのみ解放可能）1回のコマンド実行で「Get」と「Del」が実行されるので割り込みが発生しない。
func (dl *DistributedLock) Release() error {
	// Luaスクリプトを使用して、アトミックに確認と削除を行う
	script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
	result, err := dl.redis.client.Eval(dl.redis.ctx, script, []string{dl.key}, dl.value).Result()
	if err != nil {
		return err
	}
	if result.(int64) == 0 {
		return fmt.Errorf("lock not owned")
	}
	return nil
}
