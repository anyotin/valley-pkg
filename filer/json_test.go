package filer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"valley-pkg/parser"
)

// JSONの深い比較用ヘルパー関数
func jsonEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

func Test_jsonExporter_Export(t *testing.T) {
	// テスト用の構造体
	type user struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}

	tests := []struct {
		name     string
		filename string
		data     interface{}
		wantErr  bool
	}{
		{
			name:     "valid user struct",
			filename: "test_user.json",
			data:     user{Id: "1", Name: "test"},
			wantErr:  false,
		},
		{
			name:     "empty struct",
			filename: "test_empty.json",
			data:     user{},
			wantErr:  false,
		},
		{
			name:     "map data",
			filename: "test_map.json",
			data:     map[string]interface{}{"key": "value", "number": 42},
			wantErr:  false,
		},
		{
			name:     "slice data",
			filename: "test_slice.json",
			data:     []user{{Id: "1", Name: "Alice"}, {Id: "2", Name: "Bob"}},
			wantErr:  false,
		},
		{
			// 親ディレクトリは作成しない
			name:     "invalid filename",
			filename: "/invalid/path/test.json",
			data:     user{Id: "1", Name: "test"},
			wantErr:  true,
		},
	}

	// テスト用ディレクトリを作成
	testDir := "test_output"
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// テスト後にクリーンアップ
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(testDir)

	j := NewJsonLoader()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(testDir, tt.filename)

			err := j.Save(filePath, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Export() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Export() unexpected error: %v", err)
				return
			}

			// ファイルが存在することを確認
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Export() file was not created: %s", filePath)
				return
			}

			// ファイル内容を読み取り
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("failed to read exported file: %v", err)
				return
			}

			jsonParser := parser.JSONParser{}
			// JSONが正しく書き込まれているかチェック
			var result interface{}
			if err := jsonParser.Unmarshal(content, &result); err != nil {
				t.Errorf("exported content is not valid JSON: %v", err)
				return
			}

			// 元のデータとエクスポートされたデータを比較
			expectedJSON, _ := jsonParser.Marshal(tt.data)
			var expected interface{}
			err = jsonParser.Unmarshal(expectedJSON, &expected)
			if err != nil {
				t.Errorf("failed to unmarshal expected JSON: %v", err)
				return
			}

			if !jsonEqual(expected, result) {
				t.Errorf("Export() content mismatch\nexpected: %s\ngot: %s",
					string(expectedJSON), string(content))
			}
		})
	}
}

func Test_jsonImporter_Import(t *testing.T) {
	// テスト用の構造体を定義
	type TestStruct struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	type TestNestedStruct struct {
		User    TestStruct `json:"user"`
		Active  bool       `json:"active"`
		Numbers []int      `json:"numbers"`
	}

	// テスト用のJSONファイルを作成する関数
	createTestFile := func(t *testing.T, filename, content string) string {
		t.Helper()

		// テスト用の一時ディレクトリ作成。テスト終了後に自動的に消去される。
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		return filePath
	}

	tests := []struct {
		name      string
		setupFile func(t *testing.T) string
		target    any
		wantErr   bool
		validate  func(t *testing.T, target any)
	}{
		{
			name: "正常なJSONファイルの読み込み",
			setupFile: func(t *testing.T) string {
				return createTestFile(t, "test.json", `{"name":"太郎","age":30,"email":"taro@example.com"}`)
			},
			target:  &TestStruct{},
			wantErr: false,
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "太郎" {
					t.Errorf("expected name '太郎', got '%s'", result.Name)
				}
				if result.Age != 30 {
					t.Errorf("expected age 30, got %d", result.Age)
				}
				if result.Email != "taro@example.com" {
					t.Errorf("expected email 'taro@example.com', got '%s'", result.Email)
				}
			},
		},
		{
			name: "ネストしたJSONの読み込み",
			setupFile: func(t *testing.T) string {
				content := `{"user":{"name":"花子","age":25,"email":"hanako@example.com"},"active":true,"numbers":[1,2,3,4,5]}`
				return createTestFile(t, "nested.json", content)
			},
			target:  &TestNestedStruct{},
			wantErr: false,
			validate: func(t *testing.T, target any) {
				result := target.(*TestNestedStruct)
				if result.User.Name != "花子" {
					t.Errorf("expected user name '花子', got '%s'", result.User.Name)
				}
				if !result.Active {
					t.Error("expected active to be true")
				}
				if len(result.Numbers) != 5 {
					t.Errorf("expected 5 numbers, got %d", len(result.Numbers))
				}
			},
		},
		{
			name: "空のJSONファイル",
			setupFile: func(t *testing.T) string {
				return createTestFile(t, "empty.json", `{}`)
			},
			target:  &TestStruct{},
			wantErr: false,
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "" || result.Age != 0 || result.Email != "" {
					t.Error("expected zero values for empty JSON")
				}
			},
		},
		{
			name: "存在しないファイル",
			setupFile: func(t *testing.T) string {
				return "/non/existent/file.json"
			},
			target:   &TestStruct{},
			wantErr:  true,
			validate: nil,
		},
		{
			name: "無効なJSONフォーマット",
			setupFile: func(t *testing.T) string {
				return createTestFile(t, "invalid.json", `{"name":"太郎","age":}`)
			},
			target:   &TestStruct{},
			wantErr:  true,
			validate: nil,
		},
		{
			name: "JSON構造体の不一致（一部フィールドが存在しない）",
			setupFile: func(t *testing.T) string {
				return createTestFile(t, "partial.json", `{"name":"次郎"}`)
			},
			target:  &TestStruct{},
			wantErr: false,
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "次郎" {
					t.Errorf("expected name '次郎', got '%s'", result.Name)
				}
				if result.Age != 0 || result.Email != "" {
					t.Error("expected zero values for missing fields")
				}
			},
		},
		{
			name: "UTF-8エンコーディングのテスト",
			setupFile: func(t *testing.T) string {
				return createTestFile(t, "utf8.json", `{"name":"こんにちは世界","age":42,"email":"test@日本.jp"}`)
			},
			target:  &TestStruct{},
			wantErr: false,
			validate: func(t *testing.T, target any) {
				result := target.(*TestStruct)
				if result.Name != "こんにちは世界" {
					t.Errorf("expected name 'こんにちは世界', got '%s'", result.Name)
				}
				if result.Email != "test@日本.jp" {
					t.Errorf("expected email 'test@日本.jp', got '%s'", result.Email)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := NewJsonLoader()
			filePath := tt.setupFile(t)

			err := j.Load(filePath, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("Import() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, tt.target)
			}
		})
	}
}
