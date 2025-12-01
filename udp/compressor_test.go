package udp

import (
	"fmt"
	"testing"
)

func Test_Compressor(t *testing.T) {
	fmt.Println("Compressor_NONE is", Compressor_NONE.String())

	anet := Compressor(3)
	fmt.Println("anet is a Compressor?", anet.IsACompressor())

	zstd := Compressor(1)
	fmt.Println("zstd is a Compressor?", zstd.IsACompressor())

	str, err := CompressorString("NONE")
	if err != nil {
		fmt.Println(err)
		err = nil
	} else {
		fmt.Println(str)
	}

	str, err = CompressorString("ANET")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(str)
	}
}
