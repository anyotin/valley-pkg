package parser

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestJSONParser_Marshal(t *testing.T) {
	type testDate struct {
		name    string
		input   interface{}
		want    []byte
		wantErr bool
	}

	tests := []testDate{
		{
			name: "正常系: 構造体をJSONに変換",
			input: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "田中太郎",
				Age:  30,
			},
			want:    []byte(`{"name":"田中太郎","age":30}`),
			wantErr: false,
		},
		{
			name:    "正常系: nilをJSONに変換",
			input:   nil,
			want:    []byte(`null`),
			wantErr: false,
		},
		{
			name:    "異常系: JSONに変換できない値",
			input:   func() {}, // 関数はJSONに変換できない
			want:    nil,
			wantErr: true,
		},
	}

	parser := &JSONParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Marshal(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// JSONとして有効かも確認
			var v interface{}
			err = json.Unmarshal(got, &v)
			assert.NoError(t, err)
		})
	}
}

func TestJSONParser_Unmarshal(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   []byte
		target  interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:   "正常系: JSONから構造体への変換",
			input:  []byte(`{"name":"山田花子","age":25}`),
			target: &testStruct{},
			want: &testStruct{
				Name: "山田花子",
				Age:  25,
			},
			wantErr: false,
		},
		{
			name:    "正常系: 空のJSONオブジェクト",
			input:   []byte(`{}`),
			target:  &testStruct{},
			want:    &testStruct{},
			wantErr: false,
		},
		{
			name:    "異常系: 不正なJSON",
			input:   []byte(`{"name":"山田花子","age":25`), // JSONが閉じていない
			target:  &testStruct{},
			want:    &testStruct{},
			wantErr: true,
		},
		{
			name:    "異常系: 型が不一致",
			input:   []byte(`{"name":123,"age":"invalid"}`), // 型が不正
			target:  &testStruct{},
			want:    &testStruct{},
			wantErr: true,
		},
	}

	parser := &JSONParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.Unmarshal(tt.input, tt.target)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, tt.target)
		})
	}
}
