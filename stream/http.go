package stream

import (
	"io"
	"net/http"
)

// Deprecated 代わりに copyBody を使用してください。
func readAllBody(body io.Reader, w http.ResponseWriter) {
	b, err := io.ReadAll(body)
	if err != nil {
		panic(err)
	}
	_, err = w.Write(b)
	if err != nil {
		return
	}
}

// copyBody 固定サイズバッファでのループ読み書きする。
func copyBody(body io.Reader, w http.ResponseWriter) {
	_, err := io.Copy(w, body)
	if err != nil {
		panic(err)
	}
}
