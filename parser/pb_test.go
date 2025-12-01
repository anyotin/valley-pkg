package parser

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"testing"
	"valley-pkg/parser/pb_go"
)

func TestPbParser_Marshal(t *testing.T) {
	tests := []struct {
		name    string
		input   proto.Message
		want    []byte
		wantErr bool
	}{
		{
			name: "正常系: Protocol Bufferメッセージの変換",
			input: &pb_go.CommonRequestParam{
				PlayerId:       "player123",
				PlatformUserId: "platform456",
			},
			want:    nil, // Protocol Bufferでシリアライズされた結果は動的に検証
			wantErr: false,
		},
		{
			name:    "異常系: nilの入力",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
	}

	parser := &PbParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Marshal(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, ErrTypeAssert, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)

			if tt.input != nil {
				assert.NotNil(t, got)
			}

			// Protocol Bufferメッセージの場合、元のメッセージに戻せることを確認
			if protoMsg, ok := tt.input.(*pb_go.CommonRequestParam); ok {
				unmarshaled := &pb_go.CommonRequestParam{}
				err = parser.Unmarshal(got, unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, protoMsg.PlayerId, unmarshaled.PlayerId)
				assert.Equal(t, protoMsg.PlatformUserId, unmarshaled.PlatformUserId)
			}
		})
	}
}
