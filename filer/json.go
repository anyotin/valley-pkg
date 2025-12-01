package filer

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
)

// JsonFiler ファイル入出力用のインターフェース
type JsonFiler interface {
	Save(name string, i any) error
	Load(name string, in any) error
}

type jsonFiler struct{}

// NewJsonLoader json形式版
func NewJsonLoader() JsonFiler {
	return &jsonFiler{}
}

// Save データをjson形式にしてファイル出力
// サイズが大きい場合はストリーム方式が推奨
func (e jsonFiler) Save(name string, i any) error {
	b, err := json.Marshal(i)
	if err != nil {
		return errors.Errorf("failed to json marshal: %w", err)
	}

	// - 書き込み専用
	// - ファイルが存在しない場合、新規ファイル作成
	// - ファイルが存在する場合、ファイルサイズを0にリセット（内容を全削除）します
	if err := os.WriteFile(name, b, 0o644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", name, err)
	}

	return nil
}

// Load ファイルから読み込んだjsonを任意の構造体に変換
// 数 MB〜数十 MB 程度が対象かな。
func (e jsonFiler) Load(name string, in any) error {
	b, err := os.ReadFile(name)
	if err != nil {
		return errors.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(b, in); err != nil {
		return errors.Errorf("failed to json unmarshal: %w", err)
	}

	return nil
}
