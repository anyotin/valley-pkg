package tcp

//go:generate enumer -type ParserType -json
type ParserType int8

const (
	// Undefined
	_ ParserType = iota

	JSON

	PROTOBUF
)
