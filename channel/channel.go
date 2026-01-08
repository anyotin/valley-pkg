package channel

import (
	"context"
)

// Or 複数のチャンネルを1つに結合し、最初の入力チャンネルが閉じられた際に結果のチャンネルを閉じます。
// 値を扱わず、どれかのシグナルに通知が来たらチャネルをCloseするので any ではなくメモリコストが0の struct{} を使用している。
func Or(channels ...<-chan struct{}) <-chan struct{} {
	switch len(channels) {
	case 0:
		// untyped nil は、 「chan / map / func / pointer / slice / interface」のような
		//「nil を値として持てる型」に そのまま代入可能。
		return nil
	case 1:
		return channels[0]
	}

	orDone := make(chan struct{})
	go func() {
		defer close(orDone)

		switch len(channels) {
		case 2:
			select {
			case <-channels[0]:
			case <-channels[1]:
			}
		default:
			select {
			case <-channels[1]:
			case <-channels[2]:
			case <-Or(append(channels[3:], orDone)...):
			}
		}
	}()

	return orDone
}

// OrDone は入力チャネル`c`からの値を転送するチャネルを返します。これは`done`チャネルが閉じられるまで続きます。
func OrDone[T any](ctx context.Context, c <-chan T) <-chan T {
	valStream := make(chan T)
	go func() {
		defer close(valStream)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-c:
				if ok == false {
					return
				}
				select {
				case valStream <- v:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return valStream
}

// Tee は入力チャネルを2つの出力チャネルに分割し、各項目を両方に複製し、コンテキストキャンセルを遵守します。
func Tee[T any](ctx context.Context, in <-chan T) (<-chan T, <-chan T) {
	// 外側で受信できない状態の可能性もあるので一応バッファを持っておく。
	out1 := make(chan T, 1)
	out2 := make(chan T, 1)

	go func() {
		defer close(out1)
		defer close(out2)

		for {
			var v T
			var ok bool

			select {
			case <-ctx.Done():
				return
			case v, ok = <-in:
				if !ok {
					return
				}
			}

			// それぞれに1回ずつ送る
			o1, o2 := out1, out2
			for i := 0; i < 2; i++ {
				select {
				case <-ctx.Done():
					return
				case o1 <- v:
					o1 = nil // 次はこっちに送らない
				case o2 <- v:
					o2 = nil
				}
			}
		}
	}()

	return out1, out2
}

// Bridge はコンテキストキャンセルを尊重しつつ、複数のチャネルストリームからの値を単一の出力チャネルに多重化します。
func Bridge[T any](ctx context.Context, chanStream <-chan <-chan T) <-chan T {
	valStream := make(chan T)

	go func() {
		defer close(valStream)
		for {
			var stream <-chan T
			select {
			case maybeStream, ok := <-chanStream:
				if !ok {
					return
				}
				stream = maybeStream
			case <-ctx.Done():
				return
			}
			for val := range OrDone(ctx, stream) {
				valStream <- val
			}
		}
	}()

	return valStream
}
