package rand

import "math/rand"

// RandomIntBetweenInclusive 特定範囲からランダム値を取得
func RandomIntBetweenInclusive(min int, max int, isMinInclusive bool, isMaxInclusive bool) int {
	if min > max {
		panic("min must be <= max")
	}

	// 両端は含む
	if isMinInclusive && isMaxInclusive {
		return rand.Intn(max-min+1) + min
	}

	// 最小は含む
	if isMinInclusive {
		if max-min < 1 {
			panic("need min < max for [min, max)")
		}
		return rand.Intn(max-min) + min
	}

	// 最大は含む
	if isMaxInclusive {
		if max-min < 1 {
			panic("need min < max for (min, max]")
		}
		return rand.Intn(max-min) + (min + 1)
	}

	// 両端は含まない
	if max-min < 2 {
		panic("need max-min >= 2 for (min, max)")
	}
	return rand.Intn(max-min-1) + (min + 1)
}
