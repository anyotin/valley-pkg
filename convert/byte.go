package convert

import (
	"encoding/binary"
	"github.com/cockroachdb/errors"
)

// ErrConvertToByte Byte配列への変換エラー
var ErrConvertToByte = errors.New("convert to byte error")

// ErrConvertFromByte Byte配列から変換エラー
var ErrConvertFromByte = errors.New("convert from byte error")

// BytesToInt8 byte列をint8へ変換
func BytesToInt8(b []byte) (int8, error) {
	if len(b) < 1 {
		return 0, ErrConvertToByte
	}
	return int8(b[0]), nil
}

// Int8ToByte int8をbyte配列へ変換
func Int8ToByte(i int8) []byte {
	return []byte{byte(i)}
}

// BytesToInt32 byte列をint32へ変換
func BytesToInt32(b []byte) (int32, error) {
	if len(b) < 4 {
		return 0, ErrConvertToByte
	}

	u := binary.BigEndian.Uint32(b)
	return int32(u), nil
}

// Int32ToByte int32をbyte配列へ変換
func Int32ToByte(i int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(i))
	return b
}

// BytesToString byte列をstringへ変換
func BytesToString(b []byte) (string, error) {
	return string(b), nil
}
