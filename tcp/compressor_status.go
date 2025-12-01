package tcp

// go:generate go run github.com/dmarkham/enumer@latest -type=Compressor -json
//
//go:generate enumer -type CompressorType -json
type CompressorType int8

const (
	_ CompressorType = iota

	None

	// ZSTD zstd
	ZSTD
)
