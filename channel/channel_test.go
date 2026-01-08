package channel

import (
	"context"
	"testing"
	"time"
)

// Test_Or は Or 関数の動作を検証し、いずれかの入力チャネルが閉じると結合されたチャネルが閉じることを保証します。
func Test_Or(t *testing.T) {
	a := make(chan struct{})
	b := make(chan struct{})
	c := make(chan struct{})

	done := Or(a, b, c)

	// まだ誰も閉じてないので、短時間では閉じないはず
	select {
	case <-done:
		t.Fatal("done should not be closed yet")
	case <-time.After(10 * time.Second):
		// OK
	}

	// どれか閉じたら閉じる
	close(c)
	select {
	case <-done:
		// OK
		close(a)
		close(b)
		t.Logf("done closed after closing c")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for done to close after closing an input")
	}
}

// Test_OrDone は、値が適切に転送され、コンテキストのキャンセルが正しく処理されることを確認するために OrDone 関数をテストします。
func Test_OrDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan int)
	out := OrDone[int](ctx, in)

	// 1) 転送されること（順序も軽く）
	go func() {
		in <- 1
		in <- 2
		// 2) ここでは入力はまだ close しない（後で「詰まり」ケースを作る）
	}()

	select {
	case v := <-out:
		if v != 1 {
			t.Fatalf("expected 1, got %d", v)
		}

		t.Logf("first value received")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: expected first value")
	}

	select {
	case v := <-out:
		if v != 2 {
			t.Fatalf("expected 2, got %d", v)
		}
		t.Logf("second value received")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: expected second value")
	}

	// 3) out を読まない状況で in に値が来ると、OrDone は valStream への送信で詰まる可能性がある。
	//    その状態でも ctx cancel で終了できることを確認する。
	go func() { in <- 999 }()

	// 詰まるチャンスを与える（短めでOK）
	time.Sleep(1 * time.Second)

	// 4) ctx cancel で out が close される
	cancel()

	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("expected out to be closed after ctx cancel")
		}
		t.Logf("out closed after ctx cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: expected out to close after ctx cancel")
	}
}

// TestTee_minimumCoverage は、Tee 関数が入力信号を 2 つの出力チャネルに正しく複製し、適切な閉じ処理が行われることを検証します。
func TestTee_minimumCoverage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan int)
	out1, out2 := Tee[int](ctx, in)

	// 入力を流して閉じる
	go func() {
		defer close(in)
		in <- 10
		in <- 20
		in <- 30
	}()

	// out1/out2 それぞれが同じ列を受け取ること（複製されること）
	expectedAddNum := 3
	got1 := make([]int, 0, expectedAddNum)
	got2 := make([]int, 0, expectedAddNum)

	deadline := time.After(10 * time.Second)
	for len(got1) < 3 || len(got2) < 3 {
		select {
		case v, ok := <-out1:
			if !ok {
				// 先に閉じるのはNG（まだ取り切ってない）
				if len(got1) < expectedAddNum {
					t.Fatalf("out1 closed early: got=%v", got1)
				}
			} else {
				got1 = append(got1, v)
			}
		case v, ok := <-out2:
			if !ok {
				if len(got2) < expectedAddNum {
					t.Fatalf("out2 closed early: got=%v", got2)
				}
			} else {
				got2 = append(got2, v)
			}
		case <-deadline:
			t.Fatalf("timeout: got1=%v got2=%v", got1, got2)
		}
	}

	want := []int{10, 20, 30}
	for i := range want {
		if got1[i] != want[i] {
			t.Fatalf("out1[%d]: want %d, got %d (got1=%v)", i, want[i], got1[i], got1)
		}
		if got2[i] != want[i] {
			t.Fatalf("out2[%d]: want %d, got %d (got2=%v)", i, want[i], got2[i], got2)
		}
	}

	// 入力 close 後に両方の出力が close されること（終了条件の確認）
	// すでにバッファに残っている可能性があるので drain しつつ close を待つ
	waitClosed := func(ch <-chan int, name string) {
		t.Helper()
		select {
		case _, ok := <-ch:
			if ok {
				// ここで値が出る可能性もあるが、このテストでは3個取り切ってるので基本出ない想定
				// 出たら drain して閉じるまで待つ
				for range ch {
				}
			}
		case <-time.After(200 * time.Millisecond):
			// 値が無いなら close 待ちへ
		}

		select {
		case _, ok := <-ch:
			if ok {
				// まだ開いてるなら drain
				for range ch {
				}
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout: %s should close after input closes", name)
		}
	}

	waitClosed(out1, "out1")
	waitClosed(out2, "out2")
}
