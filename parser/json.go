package parser

import "encoding/json"

// JSONParser はjson用のパーサー
type JSONParser struct{}

// Marshal は構造体をbyteに変換する
func (p *JSONParser) Marshal(i any) ([]byte, error) {
	return json.Marshal(i)
}

// Unmarshal は構造体に変換する
func (p *JSONParser) Unmarshal(b []byte, i any) error {
	return json.Unmarshal(b, &i)
}
