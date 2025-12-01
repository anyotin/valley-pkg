package crypter

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
	"valley-pkg/rand"
)

func TestAes_pkcs7Pad(t *testing.T) {
	aesKey, _ := rand.GenerateRandomBytes(32)
	aesIv, _ := rand.GenerateRandomBytes(16)

	aes := Aes{aesKey: []byte(aesKey), aesIv: []byte(aesIv)}

	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "空のバイト列",
			input:    []byte{},
			expected: bytes.Repeat([]byte{16}, 16), // ブロックサイズ分のパディング
		},
		{
			name:     "1バイトの入力",
			input:    []byte{0xFF},
			expected: append([]byte{0xFF}, bytes.Repeat([]byte{15}, 15)...),
		},
		{
			name:     "ブロックサイズ-1の入力",
			input:    bytes.Repeat([]byte{0xAA}, 15),
			expected: append(bytes.Repeat([]byte{0xAA}, 15), byte(1)),
		},
		{
			name:     "ブロックサイズの入力",
			input:    bytes.Repeat([]byte{0xBB}, 16),
			expected: append(bytes.Repeat([]byte{0xBB}, 16), bytes.Repeat([]byte{16}, 16)...),
		},
		{
			name:     "ブロックサイズ+1の入力",
			input:    bytes.Repeat([]byte{0xCC}, 17),
			expected: append(bytes.Repeat([]byte{0xCC}, 17), bytes.Repeat([]byte{15}, 15)...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aes.pkcs7Pad(tt.input)

			// パディング後の長さチェック
			assert.Equal(t, 0, len(result)%16, "パディング後の長さはブロックサイズの倍数である必要があります")

			// 期待値との比較
			assert.Equal(t, tt.expected, result, "パディング結果が期待値と一致しません")

			// パディング値の検証
			paddingLength := int(result[len(result)-1])
			assert.True(t, paddingLength > 0 && paddingLength <= 16, "パディング長は1から16の範囲である必要があります")

			// パディング部分の値が全て同じであることを確認
			padding := result[len(result)-paddingLength:]
			for _, b := range padding {
				assert.Equal(t, byte(paddingLength), b, "パディング値が一致しません")
			}
		})
	}
}

func TestAes_pkcs7RemovePad(t *testing.T) {
	aesKey, _ := rand.GenerateRandomBytes(32)
	aesIv, _ := rand.GenerateRandomBytes(16)

	aes := Aes{aesKey: []byte(aesKey), aesIv: []byte(aesIv)}

	tests := []struct {
		name        string
		input       []byte
		expected    []byte
		expectError string
	}{
		{
			name:        "空の入力",
			input:       []byte{},
			expectError: "empty input",
		},
		{
			name:        "ブロックサイズでない入力",
			input:       bytes.Repeat([]byte{170}, 15),
			expectError: "input is not block-aligned",
		},
		{
			name:        "無効なパディング長（0）",
			input:       append(bytes.Repeat([]byte{170}, 15), byte(0)),
			expectError: "invalid padding length",
		},
		{
			name:        "無効なパディング長（17）",
			input:       append(bytes.Repeat([]byte{170}, 15), byte(17)),
			expectError: "invalid padding length",
		},
		{
			name:        "不一致のパディング値",
			input:       append(bytes.Repeat([]byte{170}, 14), []byte{170, 2}...),
			expectError: "invalid padding",
		},
		{
			// 正常系：1バイトのデータ + 15バイトのパディング
			name:     "有効なパディング（15バイト）",
			input:    append([]byte{170}, bytes.Repeat([]byte{15}, 15)...),
			expected: []byte{170},
		},
		{
			// 正常系：15バイトのデータ + 1バイトのパディング
			name:     "有効なパディング（1バイト）",
			input:    append(bytes.Repeat([]byte{170}, 15), byte(1)),
			expected: bytes.Repeat([]byte{170}, 15),
		},
		{
			// 正常系：16バイトのデータ + 16バイトのパディング
			name:     "有効なパディング（16バイト）",
			input:    append(bytes.Repeat([]byte{0xBB}, 16), bytes.Repeat([]byte{16}, 16)...),
			expected: bytes.Repeat([]byte{0xBB}, 16),
		},
		{
			// エラーケース：パディングが全データ
			name:        "パディングのみの入力",
			input:       bytes.Repeat([]byte{16}, 16),
			expectError: "padding less of len 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := aes.pkcs7RemovePad(tt.input)

			if tt.expectError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "パディング除去後のデータが期待値と一致しません")
		})
	}
}

func TestAes_EnCrypt(t *testing.T) {
	aesKey, _ := rand.GenerateRandomBytes(32)
	aseIv, _ := rand.GenerateRandomBytes(16)

	aes, err := NewAes(aesKey, aseIv)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		input       []byte
		expectError string
	}{
		{
			name:        "空の入力",
			input:       []byte{},
			expectError: "encrypt val is empty",
		},
		{
			name:  "1バイトの入力",
			input: []byte{0xFF},
		},
		{
			name:  "15バイトの入力",
			input: bytes.Repeat([]byte{0xAA}, 15),
		},
		{
			name:  "16バイトの入力（ブロックサイズ）",
			input: bytes.Repeat([]byte{0xBB}, 16),
		},
		{
			name:  "17バイトの入力（ブロックサイズ+1）",
			input: bytes.Repeat([]byte{0xCC}, 17),
		},
		{
			name:  "32バイトの入力（2ブロック）",
			input: bytes.Repeat([]byte{0xDD}, 32),
		},
		{
			name:  "ASCII文字列の入力",
			input: []byte("Hello, World!"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := aes.EnCrypt(tt.input)

			if tt.expectError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			// 暗号化されたデータの検証
			// 1. ブロックサイズの倍数であることを確認
			assert.Equal(t, 0, len(result)%16, "暗号化後のデータはブロックサイズの倍数である必要があります")

			// 2. 元のデータと異なることを確認
			assert.NotEqual(t, tt.input, result, "暗号化後のデータは元のデータと異なる必要があります")

			// 3. 元のデータより長いか同じ長さであることを確認
			assert.GreaterOrEqual(t, len(result), len(tt.input), "暗号化後のデータは元のデータ以上の長さである必要があります")

			// 4. 同じ入力で2回暗号化した場合、同じ結果になることを確認
			result2, err := aes.EnCrypt(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, result, result2, "同じ入力での暗号化結果は一致する必要があります")
		})
	}
}

func TestAes_DeCrypt(t *testing.T) {
	aesKey, _ := rand.GenerateRandomBytes(32)
	aseIv, _ := rand.GenerateRandomBytes(16)

	aes, err := NewAes(aesKey, aseIv)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		input       []byte
		expectError string
	}{
		{
			name:        "空の入力",
			input:       []byte{},
			expectError: "decrypt val is empty",
		},
		{
			name:        "ブロックサイズ未満の入力",
			input:       bytes.Repeat([]byte{0xAA}, 15),
			expectError: "input is not block-aligned",
		},
		{
			name:  "16バイトの入力（1ブロック）",
			input: bytes.Repeat([]byte{0xBB}, 16),
		},
		{
			name:  "32バイトの入力（2ブロック）",
			input: bytes.Repeat([]byte{0xCC}, 32),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := aes.DeCrypt(tt.input)

			if tt.expectError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			assert.Error(t, err) // 不正な暗号文を復号しようとした場合はエラーになるはず
		})
	}

	// 暗号化→復号の一連のテスト
	encryptDecryptTests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:  "1バイトのデータ",
			input: []byte{0xFF},
		},
		{
			name:  "15バイトのデータ",
			input: bytes.Repeat([]byte{0xAA}, 15),
		},
		{
			name:  "16バイトのデータ",
			input: bytes.Repeat([]byte{0xBB}, 16),
		},
		{
			name:  "17バイトのデータ",
			input: bytes.Repeat([]byte{0xCC}, 17),
		},
		{
			name:  "ASCII文字列",
			input: []byte("Hello, World!"),
		},
		{
			name:  "日本語UTF-8文字列",
			input: []byte("こんにちは世界"),
		},
	}

	for _, tt := range encryptDecryptTests {
		t.Run(tt.name+"_暗号化→復号", func(t *testing.T) {
			// 暗号化
			encrypted, err := aes.EnCrypt(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, encrypted)

			// 復号
			decrypted, err := aes.DeCrypt(encrypted)
			assert.NoError(t, err)
			assert.NotNil(t, decrypted)

			// 元のデータと一致することを確認
			assert.Equal(t, tt.input, decrypted, "復号後のデータが元のデータと一致しません")
		})
	}

	t.Run("異なるIVでの復号失敗", func(t *testing.T) {
		// オリジナルのデータを暗号化
		original := []byte("Test Message")
		encrypted, err := aes.EnCrypt(original)
		assert.NoError(t, err)

		// 異なるIVで新しいAesインスタンスを作成
		differentIv, _ := rand.GenerateRandomBytes(16)
		aes2, err := NewAes(aesKey, differentIv)
		assert.NoError(t, err)

		// 異なるIVで復号を試みる
		decrypted, err := aes2.DeCrypt(encrypted)
		if err == nil {
			// 復号に成功しても、元のデータとは異なるはず
			assert.NotEqual(t, original, decrypted)
		}
	})

	t.Run("異なる鍵での復号失敗", func(t *testing.T) {
		// オリジナルのデータを暗号化
		original := []byte("Test Message")
		encrypted, err := aes.EnCrypt(original)
		assert.NoError(t, err)

		// 異なる鍵で新しいAesインスタンスを作成
		differentKey, _ := rand.GenerateRandomBytes(32)
		aes2, err := NewAes(differentKey, aseIv)
		assert.NoError(t, err)

		// 異なる鍵で復号を試みる
		decrypted, err := aes2.DeCrypt(encrypted)
		if err == nil {
			// 復号に成功しても、元のデータとは異なるはず
			assert.NotEqual(t, original, decrypted)
		}
	})
}
