package rand

import (
	"crypto/rand"
	"fmt"
)

// Letters URL-safe な英数字
const Letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateRandomString 指定された数のランダムな文字列を生成します
func GenerateRandomString(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be a positive integer: %d", length)
	}

	// crypto/randを使用して乱数を生成
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %v", err)
	}

	for i := 0; i < length; i++ {
		bytes[i] = Letters[int(bytes[i])%len(Letters)]
	}

	return string(bytes), nil
}
