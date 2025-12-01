package udp

import (
	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
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

// ErrShort はTCPのデータ長が足りない場合のエラー
var ErrShort = errors.New("tcp message is short")

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

// Message はTCP接続時にやり取りをするメッセージの構造体
type Message struct {
	Format     string     // 3バイト
	Version    int8       // 1バイト
	Kind       int8       // 1バイト
	Parser     Parser     // 1バイト
	Compressor Compressor // 1バイト
	Extension  [5]byte    // 5バイト
	Length     int32      // 4バイト
	Body       []byte
}

// NewMessage は新規メッセージの作成
func NewMessage(format string, kind int8, m proto.Message, parser Parser, compressor Compressor) (*Message, error) {
	message := &Message{Format: format, Version: Version, Kind: kind, Parser: parser, Compressor: compressor}
	if err := message.writeBody(m); err != nil {
		return nil, errors.Errorf("failed to write body: %w", err)
	}
	return message, nil
}

// NewMessageFromByte はバイトから新規メッセージの作成
func NewMessageFromByte(format string, b []byte) (*Message, error) {
	allLen := len(b)

	// データが足りない
	if allLen < HeaderLen {
		return nil, ErrShort
	}

	length := util.ByteToInt32(b[LenPos:BodyPos])

	if length < 0 {
		return nil, ErrLen
	}

	// データが足りない
	if allLen < int(HeaderLen+length) {
		return nil, ErrShort
	}

	message := &Message{
		Format:     string(b[FormatPos:VersionPos]),
		Version:    util.ByteToInt8(b[VersionPos:KindPos]),
		Kind:       util.ByteToInt8(b[KindPos:ParserPos]),
		Parser:     Parser(util.ByteToInt8(b[ParserPos:CompressorPos])),
		Compressor: Compressor(util.ByteToInt8(b[CompressorPos:ExtensionPos])),
		Length:     length,
	}

	if !message.Parser.IsAParser() {
		return nil, ErrParser
	}

	if !message.Compressor.IsACompressor() {
		return nil, ErrCompressor
	}

	// 容量を指定しないと、slice元のデータを引き継ぐので注意
	message.Body = b[BodyPos : BodyPos+message.Length : BodyPos+message.Length]

	// log.Printf("【Format:%v, Kind:%v, parser:%v, compressor:%v, Length:%v】", message.Format, message.Kind, message.Parser.String(), message.Compressor.String(), message.Length)
	// log.Printf("【message.Body:%v】", message.Body)

	// @todo どうする？ 破棄してしまう？
	if message.Format != format {
		return nil, ErrFormat
	}

	return message, nil
}

// ToByte は[]byteへの変換を実施
func (message *Message) ToByte() []byte {
	var b []byte
	b = append(b, []byte(message.Format)[0:3]...)
	b = append(b, util.Int8ToByte(message.Version)[0:1]...)
	b = append(b, util.Int8ToByte(message.Kind)[0:1]...)
	b = append(b, util.Int8ToByte(int8(message.Parser))[0:1]...)     // @todo あとで頑張る
	b = append(b, util.Int8ToByte(int8(message.Compressor))[0:1]...) // @todo あとで頑張る
	b = append(b, message.Extension[:]...)
	b = append(b, util.Int32ToByte(message.Length)[0:4]...)
	b = append(b, message.Body...)
	return b
}

// ToByteNl は[]byteへの変換と改行コードの付加を実施
func (message *Message) ToByteNl() []byte {
	return append(message.ToByte(), []byte("\n")...)
}

// ReadBody はバイナリデータを構造体に変換して読み取り
func (message *Message) ReadBody(m proto.Message) error {
	c, err := message.getCompressor()
	if err != nil {
		return errors.Errorf("failed to get compressor: %w", err)
	}
	uncomp, err := c.Uncompress(message.Body)
	if err != nil {
		return errors.Errorf("failed to uncompress: %w", err)
	}

	p, err := message.getParser()
	if err != nil {
		return errors.Errorf("failed to get parser: %w", err)
	}
	if err := p.Unmarshal(uncomp, m); err != nil {
		return errors.Errorf("failed to parse: %w", err)
	}
	return nil
}

// writeBody は構造体をバイナリデータに変換して書き込み
func (message *Message) writeBody(m proto.Message) error {
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
		if err != compressor.ErrIncompressible {
			return errors.Errorf("failed to compress: %w", err)
		}
		// サイズが小さいと圧縮できない可能性あり
		logrus.Infof("lz4 got, %s", err.Error())
		message.Compressor = Compressor_NONE
		comp = b
	}
	message.Body = comp
	message.Length = int32(len(message.Body))
	return nil
}

// getParser はパーサーを取得
func (message *Message) getParser() (parser.Parser, error) {
	switch message.Parser {
	case Parser_JSON:
		return &parser.JSONParser{}, nil
	case Parser_PROTOBUF:
		return &parser.PbParser{}, nil
	default:
		return nil, ErrParser
	}
}

// getCompressor はコンプレッサーを取得
func (message *Message) getCompressor() (compressor.Compressor, error) {
	switch message.Compressor {
	case Compressor_NONE:
		return &compressor.NoneCompressor{}, nil
	case Compressor_ZSTD:
		return &compressor.ZstdCompressor{}, nil
	default:
		return nil, ErrCompressor
	}
}
