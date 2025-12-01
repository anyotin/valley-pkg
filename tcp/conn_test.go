package tcp

import (
	"google.golang.org/protobuf/types/known/wrapperspb"
	"net"
	"testing"
	"valley-pkg/crypter"
	"valley-pkg/rand"
)

// テスト用にフォーマット名を固定（実装に合わせて変えてください）
const testFormat = "TNN"

// server 側で ReadMessage するヘルパー
type readResult struct {
	msg *TcpMessage
	err error
}

func TestWriteReadMessage_RoundTrip(t *testing.T) {
	// 127.0.0.1:0 で Listen して OS にポートを選ばせる
	ln, err := ListenTCP("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenTCP error: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)

	resultCh := make(chan readResult, 1)

	aesKey, _ := rand.GenerateRandomBytes(32)
	aseIv, _ := rand.GenerateRandomBytes(16)
	aes, _ := crypter.NewAes(aesKey, aseIv)

	// server goroutine
	go func() {
		conn, err := ln.AcceptTCP()
		if err != nil {
			resultCh <- readResult{nil, err}
			return
		}
		defer conn.Close()

		// server 側の Conn
		serverConn := NewConn(conn, testFormat)
		serverConn.SetParser(DefaultParser)
		serverConn.SetCompressor(DefaultCompressor)
		serverConn.SetCrypter(aes)

		// 読み込み
		msg, err := serverConn.ReadMessage()
		resultCh <- readResult{msg, err}
	}()

	// client 側の接続
	clientTCP, err := DialTCP(addr.String())
	if err != nil {
		t.Fatalf("DialTCP error: %v", err)
	}
	defer clientTCP.Close()

	clientConn := NewConn(clientTCP, testFormat)
	clientConn.SetParser(DefaultParser)
	clientConn.SetCompressor(DefaultCompressor)
	clientConn.SetCrypter(aes)

	// 送る payload（適当な proto.Message）
	payload := &wrapperspb.StringValue{Value: "hello world"}
	const kind int8 = 1

	// WriteMessage 実行
	if err := clientConn.WriteMessage(kind, payload); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	// server 側結果待ち
	res := <-resultCh
	if res.err != nil {
		t.Fatalf("server ReadMessage error: %v", res.err)
	}
	if res.msg == nil {
		t.Fatalf("server ReadMessage returned nil message")
	}

	gotPayload := &wrapperspb.StringValue{}

	err = res.msg.UnpackReadBody(gotPayload)
	if err != nil {
		t.Fatalf("unpack error: %v", err)
	}

	if gotPayload.GetValue() != payload.GetValue() {
		t.Fatalf("message payload mismatch.\n got=%v\nwant=%v", gotPayload.GetValue(), payload.GetValue())
	}
}
