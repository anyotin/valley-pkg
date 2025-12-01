package udp

import (
	"fmt"
	"testing"
)

func Test_Parser(t *testing.T) {
	fmt.Println("Parser_NONE is", Parser_NONE.String())

	anet := Parser(3)
	fmt.Println("anet is a Compressor?", anet.IsAParser())

	json := Parser(1)
	fmt.Println("json is a Compressor?", json.IsAParser())

	str, err := ParserString("PROTOBUF")
	if err != nil {
		fmt.Println(err)
		err = nil
	} else {
		fmt.Println(str)
	}

	str, err = ParserString("ANET")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(str)
	}
}
