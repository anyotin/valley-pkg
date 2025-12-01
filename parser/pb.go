package parser

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

// PbParser はprotobuf用のパーサー
type PbParser struct{}

// Marshal 構造体をbyteに変換
func (p *PbParser) Marshal(v any) ([]byte, error) {
	m, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("PbParser.Marshal: value does not implement proto.Message: %T", v)
	}
	return proto.Marshal(m)
}

// Unmarshal byte配列を構造体に変換
func (p *PbParser) Unmarshal(data []byte, v any) error {
	m, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("PbParser.Unmarshal: value does not implement proto.Message: %T", v)
	}
	return proto.Unmarshal(data, m)
}
