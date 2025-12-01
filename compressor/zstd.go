package compressor

import (
	ddzstd "github.com/DataDog/zstd"
	"github.com/klauspost/compress/zstd"
	"log"
)

// ZstdCompressor zstd用のコンプレッサー
type ZstdCompressor struct{}

// CompressWithDdzstd 圧縮
func (z *ZstdCompressor) CompressWithDdzstd(src []byte) ([]byte, error) {
	buf := make([]byte, ddzstd.CompressBound(len(src))) // 圧縮後の最大サイズを求めてバッファを確保

	return ddzstd.CompressLevel(buf, src, ddzstd.DefaultCompression)
}

// DecompressWithDdzstd 解凍
func (z *ZstdCompressor) DecompressWithDdzstd(src []byte) ([]byte, error) {
	var decodedSize int // 圧縮時得られたサイズを別途保存しておく
	out := make([]byte, decodedSize)

	return ddzstd.Decompress(out, src)
}

// Compress 圧縮
func (z *ZstdCompressor) Compress(src []byte) ([]byte, error) {
	enc, err := zstd.NewWriter(nil) // nilだと内部バッファを持つエンコーダー
	if err != nil {
		log.Fatalf("zstd encoder create error: %v", err)
		return nil, ErrIncompressible
	}

	// EncodeAll: src を一気に圧縮して []byte を返す
	compressed := enc.EncodeAll(src, nil)
	//fmt.Printf("original size: %d bytes\n", len(src))
	//fmt.Printf("compressed size: %d bytes\n", len(compressed))

	// 圧縮後サイズが元データと同じか大きい場合はエラーを返す
	if len(compressed) >= len(src) {
		return nil, ErrNotShrunk
	}

	return compressed, nil
}

// Decompress 解凍
func (z *ZstdCompressor) Decompress(src []byte) ([]byte, error) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		log.Fatalf("zstd decoder create error: %v", err)
		return nil, err
	}
	// DecodeAll: 圧縮されたデータを一気に展開
	decompressed, err := dec.DecodeAll(src, nil)
	if err != nil {
		log.Fatalf("zstd decode error: %v", err)
		return nil, err
	}
	return decompressed, nil
}
