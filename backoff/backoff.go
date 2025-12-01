package backoff

import (
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v5"
	"time"
)

type BackoffWrapper struct {
	ctx       context.Context
	operation backoff.Operation[any]
	options   []backoff.RetryOption
}

func NewBackoff(ctx context.Context, initialInterval time.Duration, randomizationFactor float64, multiplier float64, maxTries uint) *BackoffWrapper {
	exponentialBackOff := backoff.NewExponentialBackOff()

	// リトライの初期間隔
	exponentialBackOff.InitialInterval = initialInterval * time.Second
	// リトライ間隔を決めるランダム値
	exponentialBackOff.RandomizationFactor = randomizationFactor
	// リトライ間隔を決める乗数
	exponentialBackOff.Multiplier = multiplier

	// v5の場合、設定された最大回数の-1回まで実行される。それ以前の場合、同じ回数分実行される。
	options := []backoff.RetryOption{backoff.WithBackOff(exponentialBackOff), backoff.WithMaxTries(maxTries)}

	return &BackoffWrapper{
		ctx:     ctx,
		options: options,
	}
}

func (b *BackoffWrapper) SetDoOperation(o backoff.Operation[any]) {
	b.operation = o
}

func (b *BackoffWrapper) SetNotify(n backoff.Notify) {
	b.options = append(b.options, backoff.WithNotify(n))
}

func (b *BackoffWrapper) Exec() {
	_, err := backoff.Retry(b.ctx, b.operation, b.options...)
	if err != nil {
		fmt.Println("処理失敗")
	} else {
		fmt.Println("処理成功")
	}
}
