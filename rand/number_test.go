package rand

import (
	"testing"
)

func TestRandomIntBetweenInclusive(t *testing.T) {
	type args struct {
		min, max       int
		isMinInclusive bool
		isMaxInclusive bool
	}
	tests := []struct {
		name      string
		args      args
		wantMin   int
		wantMax   int
		wantPanic bool
	}{
		{
			name:      "異常: 同値で最小値を含む",
			args:      args{min: 3, max: 3, isMinInclusive: true, isMaxInclusive: false},
			wantPanic: true,
		},
		{
			name:      "異常: 同値で最大値を含む",
			args:      args{min: 3, max: 3, isMinInclusive: false, isMaxInclusive: true},
			wantPanic: true,
		},
		{
			name:      "異常: 最小値が最大値より大きい",
			args:      args{min: 5, max: 3, isMinInclusive: true, isMaxInclusive: true},
			wantPanic: true,
		},
		{
			name:      "異常: 範囲の選択肢が足りない",
			args:      args{min: 2, max: 3, isMinInclusive: false, isMaxInclusive: false},
			wantPanic: true,
		},
		{
			name:    "正常: 両端を含む",
			args:    args{min: 2, max: 5, isMinInclusive: true, isMaxInclusive: true},
			wantMin: 2,
			wantMax: 5,
		},
		{
			name:    "正常: 最小値を含む",
			args:    args{min: 2, max: 5, isMinInclusive: true, isMaxInclusive: false},
			wantMin: 2,
			wantMax: 4,
		},
		{
			name:    "正常: 最大値を含む",
			args:    args{min: 2, max: 5, isMinInclusive: false, isMaxInclusive: true},
			wantMin: 3,
			wantMax: 5,
		},
		{
			name:    "正常: 両端を含まない",
			args:    args{min: 2, max: 6, isMinInclusive: false, isMaxInclusive: false},
			wantMin: 3,
			wantMax: 5,
		},
		{
			name:    "正常: 同値で両端を含む",
			args:    args{min: 3, max: 3, isMinInclusive: true, isMaxInclusive: true},
			wantMin: 3,
			wantMax: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Errorf("expected panic but did not")
				}
				if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			if tt.wantPanic {
				RandomIntBetweenInclusive(tt.args.min, tt.args.max, tt.args.isMinInclusive, tt.args.isMaxInclusive)
				return
			}

			values := make(map[int]bool)
			for i := 0; i < 100; i++ {
				got := RandomIntBetweenInclusive(tt.args.min, tt.args.max, tt.args.isMinInclusive, tt.args.isMaxInclusive)
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("got value out of range: %d (expected between %d and %d)", got, tt.wantMin, tt.wantMax)
				}
				values[got] = true
			}
			// 範囲の全てにアクセスできるか（min==max以外）
			if tt.wantMin != tt.wantMax && len(values) != (tt.wantMax-tt.wantMin+1) {
				t.Errorf("not all values in range returned: got %v", values)
			}
		})
	}
}
