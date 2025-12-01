package tcp

import (
	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"log"
	"valley-pkg/compressor"
	"valley-pkg/convert"
	"valley-pkg/crypter"
	"valley-pkg/parser"
)

const (
	// Version はフォーマットバージョンを表す
	Version = 1
	// HeaderLen はヘッダー長を表す
	HeaderLen = 16
	// FormatPos はBldの開始位置を表す
	FormatPos = 0
	// VersionPos はversionの開始位置を表す
	VersionPos = 3
	// KindPos はkindの開始位置を表す
	KindPos = 4
	// ParserPos はParserの開始位置を表す
	ParserPos = 5
	// CompressorPos はCompの開始位置を表す
	CompressorPos = 6
	// ExtensionPos はExtensionの開始位置を表す
	ExtensionPos = 7
	// LenPos はLenの開始位置を表す
	LenPos = 12
	// BodyPos はBodyの開始位置を表す
	BodyPos = HeaderLen
)

// ErrKind はメッセージ種別がおかしい場合のエラー
var ErrKind = errors.New("kind error")

// ErrHeaderShort はTCPのheaderデータ長が足りない場合のエラー
var ErrHeaderShort = errors.New("tcp header message is short")

// ErrBodyShort はTCPのbodyデータ長が足りない場合のエラー
var ErrBodyShort = errors.New("tcp body message is short")

// ErrFormat はデータの先頭を表す識別子が誤っている場合のエラー
var ErrFormat = errors.New("format error")

// ErrParser はパーサーの種類が間違っている場合のエラー
var ErrParser = errors.New("request parser is unsupported")

// ErrCompressor はコンプレッサーの種類が間違っている場合のエラー
var ErrCompressor = errors.New("request compressor is unsupported")

// ErrLen はデータのLenの値がおかしい場合のエラー
var ErrLen = errors.New("len is 0 or less")

// ErrHealthCheck はTCPのデータがない場合のエラー
var ErrHealthCheck = errors.New("health check")

// TcpMessage はTCP接続時にやり取りをするメッセージの構造体
type TcpMessage struct {
	Format         string         // 3バイト
	Version        int8           // 1バイト
	Kind           int8           // 1バイト
	ParserType     ParserType     // 1バイト
	CompressorType CompressorType // 1バイト
	Extension      [5]byte        // 5バイト
	Length         int32          // 4バイト
	Body           []byte
	Crypto         crypter.Crypter
}

// NewMessage は新規メッセージの作成
func NewMessage(format string, kind int8, parser ParserType, compressor CompressorType, crypt crypter.Crypter) *TcpMessage {
	message := &TcpMessage{Format: format, Version: Version, Kind: kind, ParserType: parser, CompressorType: compressor, Crypto: crypt}
	return message
}

// NewMessageFromByte はバイトから新規メッセージの作成
func NewMessageFromByte(format string, b []byte, crypt crypter.Crypter) (msg *TcpMessage, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = errors.Errorf("Recovered from: %w", rec)
		}
	}()

	// 全てのデータ長
	allLen := len(b)

	// ヘッダーデータが足りない
	if allLen < HeaderLen {
		return nil, ErrHeaderShort
	}

	bodyLength, err := convert.BytesToInt32(b[LenPos:BodyPos])
	if err != nil {
		return nil, err
	}

	if bodyLength < 0 {
		return nil, ErrLen
	}

	// データが足りない
	if allLen < int(HeaderLen+bodyLength) {
		// if allLen < int(HeaderLen+length) {
		return nil, ErrBodyShort
	}

	version, err := convert.BytesToInt8(b[VersionPos:KindPos])
	if err != nil {
		return nil, err
	}

	kind, err := convert.BytesToInt8(b[KindPos:ParserPos])
	if err != nil {
		return nil, err
	}

	parseType, err := convert.BytesToInt8(b[ParserPos:CompressorPos])
	if err != nil {
		return nil, err
	}

	compressType, err := convert.BytesToInt8(b[CompressorPos:ExtensionPos])
	if err != nil {
	}

	message := &TcpMessage{
		Format:         string(b[FormatPos:VersionPos]),
		Version:        version,
		Kind:           kind,
		ParserType:     ParserType(parseType),
		CompressorType: CompressorType(compressType),
		Crypto:         crypt,
		Length:         bodyLength,
	}

	if message.Format != format {
		log.Println(message.Format, format)
		return nil, errors.Errorf("beginning of data is not %s : %w", format, ErrFormat)
	}
	if !message.ParserType.IsAParserType() {
		return nil, ErrParser
	}
	if !message.CompressorType.IsACompressorType() {
		return nil, ErrCompressor
	}

	// 容量を指定しないと、slice元のデータを引き継ぐので注意
	// 第3引数を指定することで、容量を指定できる。
	message.Body = b[BodyPos : BodyPos+message.Length : BodyPos+message.Length]

	return message, nil
}

// ToByte は[]byteへの変換を実施
func (message *TcpMessage) ToByte() []byte {
	var b []byte
	b = append(b, []byte(message.Format)[0:3]...)
	b = append(b, convert.Int8ToByte(message.Version)...)
	b = append(b, convert.Int8ToByte(message.Kind)...)
	b = append(b, convert.Int8ToByte(int8(message.ParserType))...)
	b = append(b, convert.Int8ToByte(int8(message.CompressorType))...)
	b = append(b, message.Extension[:]...)
	b = append(b, convert.Int32ToByte(message.Length)...)
	b = append(b, message.Body...)
	return b
}

// ToByteNl は[]byteへの変換と改行コードの付加を実施
func (message *TcpMessage) ToByteNl() []byte {
	return append(message.ToByte(), []byte("\n")...)
}

// UnpackReadBody 読み取り後のデータ装飾の解放
func (message *TcpMessage) UnpackReadBody(m proto.Message) error {
	decrypt, err := message.Crypto.DeCrypt(message.Body)
	if err != nil {

		return errors.Errorf("failed to decrypto: %w", err)
	}

	c, err := message.getCompressor()
	if err != nil {
		return errors.Errorf("failed to get compressor: %w", err)
	}

	deComp, err := c.Decompress(decrypt)
	if err != nil {
		return errors.Errorf("failed to uncompress: %w", err)
	}

	p, err := message.getParser()
	if err != nil {
		return errors.Errorf("failed to get parser: %w", err)
	}
	if err := p.Unmarshal(deComp, m); err != nil {
		return errors.Errorf("failed to parse: %w", err)
	}
	return nil
}

// PackWriteBody 書き込む前のデータの装飾
func (message *TcpMessage) PackWriteBody(m proto.Message) error {
	p, err := message.getParser()
	if err != nil {
		return errors.Errorf("failed to get parser: %w", err)
	}
	b, err := p.Marshal(m)
	if err != nil {
		return errors.Errorf("failed to parse: %w", err)
	}

	c, err := message.getCompressor()
	if err != nil {
		return errors.Errorf("failed to get compressor: %w", err)
	}
	comp, err := c.Compress(b)
	if err != nil {
		if !errors.Is(err, compressor.ErrNotShrunk) {
			return errors.Errorf("failed to compress: %w", err)
		}

		// サイズが小さいと圧縮できない可能性あり
		logrus.Infof("lz4 got, %s", err.Error())
		message.CompressorType = None
		comp = b
	}

	encrypt, err := message.Crypto.EnCrypt(comp)
	message.Body = encrypt
	message.Length = int32(len(message.Body))
	return nil
}

// getParser はパーサーを取得
func (message *TcpMessage) getParser() (parser.Parser, error) {
	switch message.ParserType {
	case JSON:
		return &parser.JSONParser{}, nil
	case ParserPos:
		return &parser.PbParser{}, nil
	default:
		return nil, ErrParser
	}
}

// getCompressor はコンプレッサーを取得
func (message *TcpMessage) getCompressor() (compressor.Compresser, error) {
	switch message.CompressorType {
	case None:
		return &compressor.NoneCompressor{}, nil
	case ZSTD:
		return &compressor.ZstdCompressor{}, nil
	default:
		return nil, ErrCompressor
	}
}
