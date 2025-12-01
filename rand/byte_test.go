package rand

import (
	"github.com/stretchr/testify/assert"
	"log"
	"math"
	"testing"
)

func TestGenerateRandomBytes(t *testing.T) {
	result, err := GenerateRandomBytes(16)
	assert.NoError(t, err)

	assert.Len(t, result, 16)
	log.Println(result)
}

// TestDuplicateProbability 重複回数テスト
func TestDuplicateProbability(t *testing.T) {
	iterations := 1000000 // テスト回数
	length := 16          // 生成する文字列の長さ
	duplicateCount := 0   // 重複回数
	generated := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		str, err := GenerateRandomBytes(length)
		assert.NoError(t, err)

		if generated[str] {
			duplicateCount++
		} else {
			generated[str] = true
		}
	}

	duplicateRate := float64(duplicateCount) / float64(iterations) * 100

	t.Logf("テスト回数: %d", iterations)
	t.Logf("文字列の長さ: %d", length)
	t.Logf("重複回数: %d", duplicateCount)
	t.Logf("重複率: %.2f%%", duplicateRate)
	t.Logf("使用可能な文字種: %d", len(Letters))
	t.Logf("理論上の組み合わせ総数: %.0f", math.Pow(float64(len(Letters)), float64(length)))
}
