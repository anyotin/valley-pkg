package tcp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
	crypter "valley-pkg/crypter/mock"
)

var mockCrypter = &crypter.MockCrypter{}

func TestNewMessageFromByte(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		data    []byte
		wantErr bool
		errType error
	}{
		{
			name:    "正常系: 有効なメッセージデータ",
			format:  "TST",
			data:    createValidMessageData(),
			wantErr: false,
		},
		{
			name:    "正常系: ボディ長さが0のメッセージ",
			format:  "TST",
			data:    createZeroLengthBodyData(),
			wantErr: false,
		},
		{
			name:    "正常系: 最小有効メッセージ（ヘッダーのみ）",
			format:  "TST",
			data:    createMinimalValidData(),
			wantErr: false,
		},
		{
			name:    "異常系: データ長が不足（ヘッダー未満）",
			format:  "TST",
			data:    make([]byte, 10), // HeaderLen(16)未満
			wantErr: true,
			errType: ErrHeaderShort,
		},
		{
			name:    "異常系: 負の長さ",
			format:  "TST",
			data:    createInvalidLengthData(),
			wantErr: true,
			errType: ErrLen,
		},
		{
			name:    "異常系: フォーマット不一致",
			format:  "TST",
			data:    createWrongFormatData(),
			wantErr: true,
			errType: ErrFormat,
		},
		{
			name:    "異常系: ボディ長さが不足",
			format:  "TST",
			data:    createInsufficientBodyData(),
			wantErr: true,
			errType: ErrBodyShort,
		},
		{
			name:    "異常系: 未対応パーサータイプ",
			format:  "TST",
			data:    createUnsupportedParserData(),
			wantErr: true,
			errType: ErrParser,
		},
		{
			name:    "異常系: 未対応コンプレッサータイプ",
			format:  "TST",
			data:    createUnsupportedCompressorData(),
			wantErr: true,
			errType: ErrCompressor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewMessageFromByte(tt.format, tt.data, mockCrypter)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, msg)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, msg)
				assert.Equal(t, tt.format, msg.Format)
				assert.Equal(t, Version, int(msg.Version))
				assert.NotNil(t, msg.Crypto)
			}
		})
	}
}

func TestTcpMessage_ToByte(t *testing.T) {
	message := &TcpMessage{
		Format:         "TST",
		Version:        1,
		Kind:           1,
		ParserType:     JSON,
		CompressorType: None,
		Extension:      [5]byte{0, 0, 0, 0, 0},
		Length:         10,
		Body:           []byte("test data!"),
		Crypto:         mockCrypter,
	}

	result := message.ToByte()

	assert.NotNil(t, result)
	assert.True(t, len(result) >= HeaderLen)

	// フォーマット部分の確認
	assert.Equal(t, "TST", string(result[0:3]))

	// バージョン部分の確認
	assert.Equal(t, byte(1), result[3])

	// Kind部分の確認
	assert.Equal(t, byte(1), result[4])
}

func TestTcpMessage_ToByteNl(t *testing.T) {
	message := &TcpMessage{
		Format:         "TST",
		Version:        1,
		Kind:           1,
		ParserType:     JSON,
		CompressorType: None,
		Extension:      [5]byte{0, 0, 0, 0, 0},
		Length:         4,
		Body:           []byte("test"),
		Crypto:         mockCrypter,
	}

	result := message.ToByteNl()

	assert.NotNil(t, result)
	assert.True(t, bytes.HasSuffix(result, []byte("\n")))
}

// 正常系用のヘルパー関数

// createValidMessageData は有効なメッセージデータを作成
func createValidMessageData() []byte {
	data := make([]byte, HeaderLen+8)
	copy(data[0:3], "TST") // Format
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 0            // Parser (JSON)
	data[6] = 0            // Compressor (None)
	// Extension (5 bytes) はゼロのまま
	// Length = 8
	data[15] = 8
	copy(data[16:24], "testBody") // 8バイトのボディ
	return data
}

// createZeroLengthBodyData はボディ長さ0のデータを作成
func createZeroLengthBodyData() []byte {
	data := make([]byte, HeaderLen)
	copy(data[0:3], "TST") // Format
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 0            // Parser (JSON)
	data[6] = 0            // Compressor (None)
	// Extension (5 bytes) はゼロのまま
	// Length = 0 (デフォルト)
	return data
}

// createMinimalValidData は最小有効データを作成
func createMinimalValidData() []byte {
	data := make([]byte, HeaderLen+1)
	copy(data[0:3], "TST") // Format
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 0            // Parser (JSON)
	data[6] = 0            // Compressor (None)
	// Extension (5 bytes) はゼロのまま
	// Length = 1
	data[15] = 1
	data[16] = 'A' // 1バイトのボディ
	return data
}

// 既存の異常系ヘルパー関数に追加

// createInsufficientBodyData はボディ長さが不足するデータを作成
func createInsufficientBodyData() []byte {
	data := make([]byte, HeaderLen+2)
	copy(data[0:3], "TST") // Format
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 0            // Parser (JSON)
	data[6] = 0            // Compressor (None)
	// Length = 10 だが実際のボディは2バイトのみ
	data[15] = 10
	data[16] = 'A'
	data[17] = 'B'
	return data
}

// createUnsupportedParserData は未対応パーサータイプのデータを作成
func createUnsupportedParserData() []byte {
	data := make([]byte, HeaderLen+4)
	copy(data[0:3], "TST") // Format
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 99           // 未対応Parser
	data[6] = 0            // Compressor (None)
	// Length = 4
	data[15] = 4
	copy(data[16:20], "test")
	return data
}

// createUnsupportedCompressorData は未対応コンプレッサータイプのデータを作成
func createUnsupportedCompressorData() []byte {
	data := make([]byte, HeaderLen+4)
	copy(data[0:3], "TST") // Format
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 0            // Parser (JSON)
	data[6] = 99           // 未対応Compressor
	// Length = 4
	data[15] = 4
	copy(data[16:20], "test")
	return data
}

// ヘルパー関数: 無効な長さを持つテストデータを作成
func createInvalidLengthData() []byte {
	data := make([]byte, HeaderLen+4)
	copy(data[0:3], "TST")
	data[3] = 1 // Version
	data[4] = 1 // Kind
	data[5] = 0 // Parser
	data[6] = 0 // Compressor
	// Length部分に負の値を設定（-1）
	data[12] = 0xFF
	data[13] = 0xFF
	data[14] = 0xFF
	data[15] = 0xFF
	return data
}

// ヘルパー関数: 間違ったフォーマットのテストデータを作成
func createWrongFormatData() []byte {
	data := make([]byte, HeaderLen+4)
	copy(data[0:3], "WRG") // 間違ったフォーマット
	data[3] = 1            // Version
	data[4] = 1            // Kind
	data[5] = 0            // Parser（JSON）
	data[6] = 0            // Compressor（None）
	data[15] = 4           // Length = 4
	copy(data[16:20], "body")

	return data
}
