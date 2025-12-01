package compressor

type NoneCompressor struct{}

// Compress 圧縮
func (NoneCompressor) Compress(src []byte) ([]byte, error) {
	return src, nil
}

// Decompress 解凍
func (NoneCompressor) Decompress(src []byte) ([]byte, error) {
	return src, nil
}
