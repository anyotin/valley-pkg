package crypter

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

type Crypter interface {
	EnCrypt(plainText []byte) ([]byte, error)
	DeCrypt(cipherText []byte) ([]byte, error)
}

type Aes struct {
	aesKey []byte
	aesIv  []byte
}

// NewAes コンストラクタ
func NewAes(aesKey string, aesIv string) (Crypter, error) {
	if aesKey == "" || aesIv == "" {
		return nil, errors.New("key and IV must not be empty")
	}

	key := []byte(aesKey)
	iv := []byte(aesIv)

	// AESキーの長さを検証（16, 24, 32バイトのいずれか）
	validKeyLengths := map[int]bool{16: true, 24: true, 32: true}
	if !validKeyLengths[len(key)] {
		return nil, fmt.Errorf("invalid key length: %d bytes; must be 16, 24, or 32 bytes", len(key))
	}

	// IVの長さを検証（16バイト）
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("invalid IV length: %d bytes; must be %d bytes", len(iv), aes.BlockSize)
	}

	return &Aes{
		aesKey: key,
		aesIv:  iv,
	}, nil
}

// pkcs7Pad 暗号化のパディング追加
func (ae *Aes) pkcs7Pad(cipherText []byte) []byte {
	// 入力データの長さをブロックサイズで割った余り
	remain := len(cipherText) % aes.BlockSize
	length := aes.BlockSize - remain

	// パディング用のバイト列を作成
	// 3バイトのパディングが必要な場合、`[3,3,3]`
	trailing := bytes.Repeat([]byte{byte(length)}, length)
	return append(cipherText, trailing...)
}

// pkcs7UnPad 複合時ののパディング除去
func (ae *Aes) pkcs7RemovePad(src []byte) ([]byte, error) {
	length := len(src)

	paddingLen := int(src[length-1])
	if paddingLen == 0 || paddingLen > aes.BlockSize {
		return nil, errors.New("invalid padding length")
	}

	// 追加されたパディングは全て同じか検証
	for i := length - paddingLen; i < length; i++ {
		if src[i] != byte(paddingLen) {
			return nil, errors.New("invalid padding")
		}
	}

	end := length - paddingLen
	if end < 1 {
		return nil, errors.New("padding less of len 1")
	}

	return src[:end], nil
}

// EnCrypt 暗号化
func (ae *Aes) EnCrypt(plainText []byte) ([]byte, error) {
	if len(plainText) < 1 {
		return nil, errors.New("encrypt val is empty.")
	}

	pkPlainText := ae.pkcs7Pad(plainText)

	// AES暗号化のための暗号化オブジェクトを作成
	block, err := aes.NewCipher(ae.aesKey)
	if err != nil {
		fmt.Printf("Error: NewCipher(%d bytes) = %s", len(ae.aesKey), err)
		return nil, err
	}

	cipherText := make([]byte, len(pkPlainText))

	// CBC（Cipher Block Chaining）モードで暗号化
	cbc := cipher.NewCBCEncrypter(block, ae.aesIv)
	cbc.CryptBlocks(cipherText, pkPlainText)
	return cipherText, nil
}

// DeCrypt 複合化
func (ae *Aes) DeCrypt(cipherText []byte) ([]byte, error) {
	if len(cipherText) < 1 {
		return nil, errors.New("decrypt val is empty.")
	}

	// ブロックサイズチェック
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("input is not block-aligned")
	}

	block, err := aes.NewCipher(ae.aesKey)
	if err != nil {
		fmt.Printf("Error: aes new cipher err:%v", err)
		return nil, err
	}

	plainText := make([]byte, len(cipherText))

	//CBC（Cipher Block Chaining）モードで複合化
	cbc := cipher.NewCBCDecrypter(block, ae.aesIv)
	cbc.CryptBlocks(plainText, cipherText)
	return ae.pkcs7RemovePad(plainText)
}
