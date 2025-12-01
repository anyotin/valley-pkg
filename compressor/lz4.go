package compressor

import (
	"bytes"
	"github.com/pierrec/lz4"
)

type Lz4Compressor struct{}

// Compress は引数のバイト列を LZ4 で圧縮して返す
func (Lz4Compressor) Compress(src []byte) ([]byte, error) {
	// 圧縮後の最大サイズを見積もってバッファ確保
	// LZ4 は「ちょっと多め」ぐらいの余裕が必要
	maxDstSize := lz4.CompressBlockBound(len(src))
	dst := make([]byte, maxDstSize)

	n, err := lz4.CompressBlock(src, dst, nil)
	if err != nil {
		return nil, ErrIncompressible
	}
	if n == 0 {
		// 圧縮しても大きくならない場合は 0 が返る仕様なので、
		// そのときは非圧縮で返すなどのポリシーを決める必要がある
		return src, nil
	}

	return dst[:n], nil
}

// Decompress は LZ4 圧縮されたバイト列を解凍する
func (Lz4Compressor) Decompress(src []byte) ([]byte, error) {
	r := lz4.NewReader(bytes.NewReader(src))

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
