package udp

import (
	"net"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"
)

// DefaultParser はDefaultのParser
var DefaultParser = Parser_JSON

// DefaultCompressor はDefaultのCompressor
var DefaultCompressor = Compressor_NONE

// DialUDP はnet.DialUDPのラッパー
func DialUDP(address string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, errors.Errorf("Dial UDP error: %w", err)
	}
	return net.DialUDP("udp", nil, udpAddr)
}

// ListenUDP はnet.ListenUDPのラッパー
func ListenUDP(address string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, errors.Errorf("Resolve UDPAddr error: %w", err)
	}
	return net.ListenUDP("udp", udpAddr)
}

// Conn はconnのインターフェース
type Conn interface {
	MessageHandler
	ConfigSetter
}

// MessageHandler はMessageのHandlerインターフェース
type MessageHandler interface {
	WriteMessage(kind int8, m proto.Message) error
	WriteMessageTo(kind int8, m proto.Message, addr net.Addr) error
	ReadMessage() (*Message, error)
	ReadMessageFrom() (*Message, net.Addr, error)
}

// ConfigSetter は設定のセット用のインターフェース
type ConfigSetter interface {
	SetParser(parser Parser)
	SetCompressor(compressor Compressor)
}

// conn はUDP通信の管理用の構造体
// parserとcompressorは最初のメッセージを送信する側が決める
type conn struct {
	conn       *net.UDPConn
	format     string
	parser     Parser
	compressor Compressor
}

// NewConn ははConnの初期化を行う
func NewConn(udpConn *net.UDPConn, format string) Conn {
	return &conn{conn: udpConn, format: format, parser: DefaultParser, compressor: DefaultCompressor}
}

// SetParser はParserを設定する
func (conn *conn) SetParser(parser Parser) {
	conn.parser = parser
}

// SetCompressor はCompressorを設定する
func (conn *conn) SetCompressor(compressor Compressor) {
	conn.compressor = compressor
}

// ReadMessage はコネクションからメッセージの読み取りを行う
func (conn *conn) ReadMessage() (*Message, error) {
	b := make([]byte, 1024)
	n, err := (*(conn.conn)).Read(b)
	if err != nil {
		return nil, errors.Errorf("udp read error: %w", err)
	}
	message, err := NewMessageFromByte(conn.format, b[:n])
	if err != nil {
		return nil, errors.Errorf("failed to read udp message: %w", err)
	}

	return message, nil
}

// ReadMessageFrom は指定のAddrからメッセージの読み取りを行う
func (conn *conn) ReadMessageFrom() (*Message, net.Addr, error) {
	b := make([]byte, 1024)
	n, sender, err := (*(conn.conn)).ReadFrom(b)
	if err != nil {
		return nil, nil, errors.Errorf("udp read error: %w", err)
	}
	message, err := NewMessageFromByte(conn.format, b[:n])
	if err != nil {
		return nil, nil, errors.Errorf("failed to read udp message: %w", err)
	}
	return message, sender, nil
}

// WriteMessage はコネクションに対してメッセージを書き込む
func (conn *conn) WriteMessage(kind int8, m proto.Message) error {
	message, err := NewMessage(conn.format, kind, m, conn.parser, conn.compressor)
	if err != nil {
		return errors.Errorf("failed to create udp message: %w", err)
	}
	return conn.write(message)
}

// WriteMessageTo は指定のAddrにメッセージを書き込む
func (conn *conn) WriteMessageTo(kind int8, m proto.Message, addr net.Addr) error {
	message, err := NewMessage(conn.format, kind, m, conn.parser, conn.compressor)
	if err != nil {
		return errors.Errorf("failed to create udp message: %w", err)
	}
	return conn.writeTo(message, addr)
}

// Write はコネクションにメッセージを書き込む
func (conn *conn) write(message *Message) error {
	if _, err := (*(conn.conn)).Write(message.ToByte()); err != nil {
		return errors.Errorf("failed to write: %w", err)
	}
	return nil
}

// WriteTo は指定のAddrにメッセージを書き込む
func (conn *conn) writeTo(message *Message, addr net.Addr) error {
	if _, err := (*(conn.conn)).WriteTo(message.ToByte(), addr); err != nil {
		return errors.Errorf("failed to write: %w", err)
	}
	return nil
}
