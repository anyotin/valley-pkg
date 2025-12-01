package arithmetic

// Gcd 最大公約数を求める
func Gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 { // 念のため負数対策
		return -a
	}
	return a
}

// Lcm 最小公倍数を求める
func Lcm(p, q int) int {
	return p / Gcd(p, q) * q
}

// ModExp 冪乗のMod(繰り返し二乗法)
func ModExp(base, exp, mod int) int {
	result := 1 % mod
	base = base % mod

	for exp > 0 {
		// ビット演算 1桁目を確認
		if exp&1 == 1 {
			result = (result * base) % mod
		}

		base = (base * base) % mod

		// 右へ1bitずらす。1101 -> 110
		exp >>= 1
	}
	return result
}
