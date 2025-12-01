package compressor

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func makeData(size int) []byte {
	// sizeバイトのスライスを作成
	data := make([]byte, size)

	// 適当にパターンを入れる（ここでは 0,1,2,...,255 を繰り返し）
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}

	return data
}

type testDate struct {
	name    string
	input   []byte
	wantErr bool
}

// 圧縮率が100%以上になるボーダーの検証
func TestZstdCompressor_Compress_Up100(t *testing.T) {
	testDates := make([]testDate, 1024)

	for i := 0; i <= 1023; i++ {
		testDates[i] = testDate{
			input:   makeData(i + 1),
			wantErr: false,
		}
	}

	for i, tt := range testDates {
		z := &ZstdCompressor{}

		compressed, err := z.Compress(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("Compress() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

		// 圧縮率
		compressedRate := float64(len(compressed)) / float64(len(tt.input)) * 100

		compressed2, err := z.CompressWithDdzstd(compressed)
		if (err != nil) != tt.wantErr {
			t.Errorf("Compress() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

		// 圧縮率
		compressedRate2 := float64(len(compressed2)) / float64(len(tt.input)) * 100

		if i == 0 || i == len(testDates)-1 {
			fmt.Println("======================")
			t.Logf("元のサイズ: %d bytes", len(tt.input))

			fmt.Println("----- default zstd -----")
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed))
			t.Logf("圧縮率: %.2f%%", compressedRate)

			fmt.Println("----- zstdd -----")
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed2))
			t.Logf("圧縮率: %.2f%%", compressedRate2)
		} else if compressedRate == 100.00 {
			fmt.Println("======================")
			t.Logf("元のサイズ: %d bytes", len(tt.input))

			fmt.Println("----- default zstd -----")
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed))
			t.Logf("圧縮率: %.2f%%", compressedRate)
		} else if compressedRate2 == 100.00 {
			fmt.Println("======================")
			t.Logf("元のサイズ: %d bytes", len(tt.input))

			fmt.Println("----- zstdd -----")
			t.Logf("圧縮後のサイズ: %d bytes", len(compressed2))
			t.Logf("圧縮率: %.2f%%", compressedRate2)
		}
	}
}

// compress/zstd検証
func TestZstdCompressor_Compress(t *testing.T) {
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
			name:    "10MByte程度のデータの圧縮",
			input:   makeData(1024 * 1024),
			wantErr: false,
		},
		{
			name:    "1MByte程度のデータの圧縮",
			input:   makeData(1024 * 1024 * 1024),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startCompress := time.Now()

			z := &ZstdCompressor{}

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
	// zstd_test.go:144: 元のサイズ: 1048576 bytes
	// zstd_test.go:145: 圧縮後のサイズ: 382 bytes
	// zstd_test.go:146: 圧縮率: 0.04%
	// zstd_test.go:151: 圧縮時間: 1.511875ms (661.43 MB/s)
	// zstd_test.go:152: 解凍時間: 399.209µs (2504.95 MB/s)

	// 10MBの場合
	// zstd_test.go:149: 元のサイズ: 1073741824 bytes
	// zstd_test.go:150: 圧縮後のサイズ: 114959 bytes
	// zstd_test.go:151: 圧縮率: 0.01%
	// zstd_test.go:156: 圧縮時間: 292.731ms (3498.09 MB/s)
	// zstd_test.go:157: 解凍時間: 210.950125ms (4854.23 MB/s)
}

// DataDog/zstd検証
func TestZstdCompressor_DCompress(t *testing.T) {
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
			input:   makeData(1024 * 1024 * 1024),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startCompress := time.Now()

			z := &ZstdCompressor{}

			compressed, err := z.CompressWithDdzstd(tt.input)
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
			decompressed, err := z.DecompressWithDdzstd(compressed)
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
	// zstd_test.go:219: 元のサイズ: 1048576 bytes
	// zstd_test.go:220: 圧縮後のサイズ: 363 bytes
	// zstd_test.go:221: 圧縮率: 0.03%
	// zstd_test.go:226: 圧縮時間: 478.667µs (2089.14 MB/s)
	// zstd_test.go:227: 解凍時間: 1.246459ms (802.27 MB/s)

	// 10MBの場合
	// zstd_test.go:219: 元のサイズ: 1073741824 bytes
	// zstd_test.go:220: 圧縮後のサイズ: 98572 bytes
	// zstd_test.go:221: 圧縮率: 0.01%
	// zstd_test.go:226: 圧縮時間: 198.631584ms (5155.27 MB/s)
	// zstd_test.go:227: 解凍時間: 2.376351375s (430.91 MB/s)
}
