package compressor

import "github.com/cockroachdb/errors"

// Compresser 圧縮系のインナーフェース
type Compresser interface {
	Compress(src []byte) ([]byte, error)
	Decompress(src []byte) ([]byte, error)
}

var ErrIncompressible = errors.New("compress error")

// ErrNotShrunk 圧縮でサイズが小さくならなかった場合のエラー
var ErrNotShrunk = errors.New("compressed size not reduced")
