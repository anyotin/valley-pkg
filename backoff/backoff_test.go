package backoff

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"sync/atomic"
	"testing"
	"time"
)

// 成功パターンのテスト
func TestBackoffWrapper_Success(t *testing.T) {
	ctx := context.Background()
	counter := int32(0)

	op := func() (any, error) {
		if atomic.AddInt32(&counter, 1) < 3 {
			return nil, errors.New("一時エラー")
		}
		return "ok", nil
	}

	bw := NewBackoff(ctx, 0, 0, 1, 5)
	bw.SetDoOperation(op)

	called := int32(0)
	bw.SetNotify(func(err error, duration time.Duration) {
		fmt.Printf("エラー: %v %d秒後に再試行します...\n", err, duration/time.Second)
		atomic.AddInt32(&called, 1)
	})

	bw.Exec()

	t.Logf("counter=%d", counter)
	t.Logf("called=%d", called)

	if counter != 3 {
		t.Errorf("リトライ回数が想定外です。got=%d, want=3", counter)
	}
	if called != 2 {
		t.Errorf("Notifyの呼ばれた回数が想定外です。got=%d, want=2", called)
	}
}

// 失敗パターンのテスト
func TestBackoffWrapper_Failure(t *testing.T) {
	ctx := context.Background()
	counter := int32(0)

	op := func() (any, error) {
		atomic.AddInt32(&counter, 1)
		return nil, errors.New("常にエラー")
	}

	bw := NewBackoff(ctx, 0, 0, 1, 3)
	bw.SetDoOperation(op)

	var lastErr error
	called := int32(0)
	bw.SetNotify(func(err error, duration time.Duration) {
		fmt.Printf("エラー: %v %d秒後に再試行します...\n", err, duration/time.Second)
		atomic.AddInt32(&called, 1)
		lastErr = err
	})

	bw.Exec()

	if counter != 2 {
		t.Errorf("リトライ回数が想定外です。got=%d, want=2", counter)
	}
	if called != 2 {
		t.Errorf("Notifyの呼ばれた回数が想定外です。got=%d, want=2", called)
	}
	if lastErr == nil || lastErr.Error() != "常にエラー" {
		t.Errorf("Notifyで渡されたエラーが想定外です。got=%v", lastErr)
	}
}
