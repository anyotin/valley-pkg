package compressor

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestLz4Compressor_Compress_Up100(t *testing.T) {
	testDates := make([]testDate, 1024)

	for i := 0; i <= 1023; i++ {
		testDates[i] = testDate{
			input:   makeData(i + 1),
			wantErr: false,
		}
	}

	for i, tt := range testDates {
		z := &Lz4Compressor{}

		compressed, err := z.Compress(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("Compress() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

		// 圧縮率
		compressedRate := float64(len(compressed)) / float64(len(tt.input)) * 100

		if i == 0 || i == len(testDates)-1 {
			fmt.Println("======================")
			t.Logf("元のサイズ: %d bytes", len(tt.input))
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed))
			t.Logf("圧縮率: %.2f%%", compressedRate)
		} else if compressedRate == 100.00 {
			fmt.Println("======================")
			t.Logf("元のサイズ: %d bytes", len(tt.input))
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed))
			t.Logf("圧縮率: %.2f%%", compressedRate)
		}
	}
}

func TestLz4Compressor_Compress(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "軽いデータの圧縮",
			input:   []byte("Hello, World!"),
			wantErr: false,
		},
		{
			name:    "1KByte程度のデータの圧縮",
			input:   makeData(1024),
			wantErr: false,
		},
		{
			name:    "1MByte程度のデータの圧縮",
			input:   makeData(1024 * 1024),
			wantErr: false,
		},
		{
			name:    "10MByte程度のデータの圧縮",
			input:   makeData(1024 * 1024 * 1024),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startCompress := time.Now()

			z := &Lz4Compressor{}

			compressed, err := z.Compress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			elapsedCompress := time.Since(startCompress)

			// 圧縮されたデータが入力より小さいことを確認
			if len(compressed) >= len(tt.input) {
				t.Logf("圧縮前: %d bytes, 圧縮後: %d bytes", len(tt.input), len(compressed))
			}

			// 圧縮→解凍で元のデータと一致することを確認
			startDecompress := time.Now()
			decompressed, err := z.Decompress(compressed)
			if err != nil {
				t.Errorf("Uncompress() error = %v", err)
				return
			}
			elapsedDecompress := time.Since(startDecompress)

			if !bytes.Equal(tt.input, decompressed) {
				t.Error("圧縮→解凍後のデータが元のデータと一致しません")
			}

			t.Logf("元のサイズ: %d bytes", len(tt.input))
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed))
			t.Logf("圧縮率: %.2f%%", float64(len(compressed))/float64(len(tt.input))*100)

			inputSizeMB := float64(len(tt.input)) / (1024 * 1024)
			compressThroughput := inputSizeMB / elapsedCompress.Seconds()
			decompressThroughput := inputSizeMB / elapsedDecompress.Seconds()
			t.Logf("圧縮時間: %s (%.2f MB/s)", elapsedCompress, compressThroughput)
			t.Logf("解凍時間: %s (%.2f MB/s)", elapsedDecompress, decompressThroughput)
		})
	}

	// 1MBの場合
	// lz4_test.go:100: 元のサイズ: 1048576 bytes
	// lz4_test.go:101: 圧縮後のサイズ: 4410 bytes
	// lz4_test.go:102: 圧縮率: 0.42%
	// lz4_test.go:107: 圧縮時間: 2.793666ms (357.95 MB/s)
	// lz4_test.go:108: 解凍時間: 2.55425ms (391.50 MB/s)

	// 10MBの場合
	// lz4_test.go:105: 元のサイズ: 1073741824 bytes
	// lz4_test.go:106: 圧縮後のサイズ: 4283407 bytes
	// lz4_test.go:107: 圧縮率: 0.40%
	// lz4_test.go:112: 圧縮時間: 334.988667ms (3056.82 MB/s)
	// lz4_test.go:113: 解凍時間: 2.054247417s (498.48 MB/s)
}
