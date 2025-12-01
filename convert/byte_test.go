package convert

import "testing"

func TestBytesToInt8(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    int8
		wantErr bool
	}{
		{
			name:    "正常値: 0x00",
			input:   []byte{0x00},
			want:    0,
			wantErr: false,
		},
		{
			name:    "正常値: 0x7F (int8最大値)",
			input:   []byte{0x7F},
			want:    127,
			wantErr: false,
		},
		{
			name:    "正常値: 0xFF (-1)",
			input:   []byte{0xFF},
			want:    -1,
			wantErr: false,
		},
		{
			name:    "異常値: バイト数不足",
			input:   []byte{},
			want:    0,
			wantErr: true,
		},
		{
			name:    "正常値: 0x80 (int8最小値)",
			input:   []byte{0x80},
			want:    -128,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BytesToInt8(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("BytesToInt8() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BytesToInt8() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInt8ToByte(t *testing.T) {
	tests := []struct {
		name  string
		input int8
		want  []byte
	}{
		{
			name:  "0",
			input: 0,
			want:  []byte{0x00},
		},
		{
			name:  "127 (int8最大値)",
			input: 127,
			want:  []byte{0x7F},
		},
		{
			name:  "-1",
			input: -1,
			want:  []byte{0xFF},
		},
		{
			name:  "-128 (int8最小値)",
			input: -128,
			want:  []byte{0x80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Int8ToByte(tt.input)
			if len(got) != 1 {
				t.Errorf("Int8ToByte() のバイト長が不正: %d", len(got))
			}
			if got[0] != tt.want[0] {
				t.Errorf("Int8ToByte() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBytesToInt32(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    int32
		wantErr bool
	}{
		{
			name:    "正常値: 0x00000001",
			input:   []byte{0x00, 0x00, 0x00, 0x01},
			want:    1,
			wantErr: false,
		},
		{
			name:    "正常値: 0x7FFFFFFF (int32最大値)",
			input:   []byte{0x7F, 0xFF, 0xFF, 0xFF},
			want:    2147483647,
			wantErr: false,
		},
		{
			name:    "正常値: 0xFFFFFFFF (int32最小値 -1)",
			input:   []byte{0xFF, 0xFF, 0xFF, 0xFF},
			want:    -1,
			wantErr: false,
		},
		{
			name:    "異常値: バイト数不足",
			input:   []byte{0x01, 0x02, 0x03},
			want:    0,
			wantErr: true,
		},
		{
			name:    "正常値: 0x80000000 (int32最小値)",
			input:   []byte{0x80, 0x00, 0x00, 0x00},
			want:    -2147483648,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BytesToInt32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("BytesToInt32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BytesToInt32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInt32ToByte(t *testing.T) {
	tests := []struct {
		name  string
		input int32
		want  []byte
	}{
		{
			name:  "0",
			input: 0,
			want:  []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name:  "1",
			input: 1,
			want:  []byte{0x00, 0x00, 0x00, 0x01},
		},
		{
			name:  "int32最大値",
			input: 2147483647,
			want:  []byte{0x7F, 0xFF, 0xFF, 0xFF},
		},
		{
			name:  "int32最小値",
			input: -2147483648,
			want:  []byte{0x80, 0x00, 0x00, 0x00},
		},
		{
			name:  "-1",
			input: -1,
			want:  []byte{0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Int32ToByte(tt.input)
			if len(got) != 4 {
				t.Errorf("Int32ToByte() のバイト長が不正: %d", len(got))
			}
			for i := 0; i < 4; i++ {
				if got[i] != tt.want[i] {
					t.Errorf("Int32ToByte() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
