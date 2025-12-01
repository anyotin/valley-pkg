package parser

import (
	"fmt"
)

// ErrTypeAssert はデータの型がおかしい場合のエラー
var ErrTypeAssert = fmt.Errorf("type assert error")

// Parser パーサー用のインターフェース
type Parser interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, any) error
}
