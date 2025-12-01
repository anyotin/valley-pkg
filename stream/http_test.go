package stream

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"testing"
)

var image []byte

func init() {
	body, _ := os.Open("./image.jpg")
	image, _ = io.ReadAll(body)
	body.Close()
}

func BenchmarkReadAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := bytes.NewReader(image)
		readAllBody(r, w)
	}

	// go test -bench=BenchmarkReadAll -benchmem
	// 1回の操作あたりの平均実行時間: 528.9 ns/op
	// 1回あたりの確保されるメモリのバイト数: 1504 B/op
	// 1回あたりの割り当て回数: 10 allocs/op
}

func BenchmarkCopy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := bytes.NewReader(image)
		copyBody(r, w)
	}

	// go test -bench=BenchmarkCopy -benchmem
	// 1回の操作あたりの平均実行時間: 75.90 ns/op
	// 1回あたりの確保されるメモリのバイト数: 208 B/op
	// 1回あたりの割り当て回数: 4 allocs/op
}
