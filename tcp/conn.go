package tcp

import (
	"bufio"
	"log"
	"net"
	"strings"
	"syscall"
	"time"
	"valley-pkg/crypter"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"
)

// ErrEof はEofの場合のエラー
var ErrEof = errors.New("EOF")

var ErrEofShort = errors.New("EOF_SHORT")

var ErrEpipe = errors.New("EPIPE")

// ErrEconnreset はECONNRESETの場合のエラー
var ErrEconnreset = errors.New("ECONNRESET")

// ErrClosedConnection use of closed network connection
var ErrClosedConnection = errors.New("CLOSED_CONNECTION")

// DefaultParser はDefaultのParser
var DefaultParser = JSON

// DefaultCompressor はDefaultのCompressor
var DefaultCompressor = None

// DialTCP はnet.DialTCPのラッパー
func DialTCP(address string) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, errors.Errorf("Dial TCP error: %w", err)
	}
	return net.DialTCP("tcp", nil, tcpAddr)
}

// ListenTCP はnet.ListenTCPのラッパー
func ListenTCP(address string) (*net.TCPListener, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, errors.Errorf("Resolve TCPAddr error: %w", err)
	}
	return net.ListenTCP("tcp", tcpAddr)
}

// Conn はconnのインターフェース
type Conn interface {
	MessageHandler
	ConfigSetter
	RemoteAddr() net.Addr
}

// MessageHandler はMessageのHandlerインターフェース
type MessageHandler interface {
	WriteMessage(kind int8, m proto.Message) error
	ReadMessage() (*TcpMessage, error)
}

// ConfigSetter は設定のセット用のインターフェース
type ConfigSetter interface {
	SetParser(parser ParserType)
	SetCompressor(compressor CompressorType)
	SetDeadLine(seconds int)
	SetCrypter(crypter crypter.Crypter)
}

// messageConn はTcpコネクション管理用の構造体
// Scannerは一度だけ初期化する想定
// parserとcompressorは最初のメッセージを送信する側が決める
type messageConn struct {
	conn       *net.TCPConn
	scanner    *bufio.Scanner
	format     string
	parser     ParserType
	compressor CompressorType
	crypter    crypter.Crypter
}

// NewConn はConnの初期化を行う
func NewConn(tcpConn *net.TCPConn, format string) Conn {
	scanner := bufio.NewScanner(tcpConn)

	// 1byte毎にデータを分割してスキャンする設定
	scanner.Split(bufio.ScanBytes)
	return &messageConn{conn: tcpConn, scanner: scanner, format: format, parser: DefaultParser, compressor: DefaultCompressor}
}

// RemoteAddr はRemoteAddr
func (mc *messageConn) RemoteAddr() net.Addr {
	return mc.conn.RemoteAddr()
}

// SetParser はParserを設定する
func (mc *messageConn) SetParser(p ParserType) {
	mc.parser = p
}

// SetCompressor はCompressorを設定する
func (mc *messageConn) SetCompressor(c CompressorType) {
	mc.compressor = c
}

// SetCrypter はcrypterを設定する
func (mc *messageConn) SetCrypter(c crypter.Crypter) {
	mc.crypter = c
}

func (mc *messageConn) SetDeadLine(seconds int) {
	mc.conn.SetDeadline(time.Now().Add(time.Duration(seconds) * time.Second))
}

// WriteMessage はコネクションに対してメッセージを書き込む
func (mc *messageConn) WriteMessage(kind int8, m proto.Message) error {
	message := NewMessage(mc.format, kind, mc.parser, mc.compressor, mc.crypter)
	err := message.PackWriteBody(m)
	if err != nil {
		return errors.Errorf("failed to create message: %w", err)
	}
	return mc.write(message)
}

// ReadMessage はコネクションからメッセージの読み取りを行う
func (mc *messageConn) ReadMessage() (*TcpMessage, error) {
	var rem []byte
	var message *TcpMessage
	var err error

	for {
		if ok := mc.scanner.Scan(); !ok {
			err = mc.scanner.Err()
			if err == nil {
				if len(rem) > 0 {
					return nil, ErrEofShort
				}
				return nil, ErrEof
			}
			if errors.Is(err, syscall.ECONNRESET) {
				return nil, ErrEconnreset
			}

			// use of closed network connection / net.ErrClosed など
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "closed") {
				return nil, ErrClosedConnection
			}

			return nil, errors.Errorf("tcp scan error: %w", mc.scanner.Err())
		}

		b := mc.scanner.Bytes()

		// 途中までしか読み込めていないものは、結合してから再度メッセージ化する
		rem = append(rem, b...)

		if len(rem) == 0 {
			return nil, ErrHealthCheck
		}

		message, err = NewMessageFromByte(mc.format, rem, mc.crypter)
		if err == nil {
			break
		}
		switch true {
		case errors.Is(err, ErrLen), errors.Is(err, ErrFormat):
			// logrus.Infof("%v", err)
			return nil, err
		case errors.Is(err, ErrHeaderShort):
		case errors.Is(err, ErrBodyShort):
		default:
			return nil, err
		}
	}
	return message, nil
}

// write はコネクションに対して、メッセージを書き込む
func (mc *messageConn) write(tcpMessage *TcpMessage) error {
	b := tcpMessage.ToByte()

	for len(b) > 0 {
		n, err := mc.conn.Write(b)
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return ErrEpipe
			}
			if errors.Is(err, syscall.ECONNRESET) {
				return ErrEconnreset
			}

			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "closed") {
				return ErrClosedConnection
			}

			return errors.Errorf("tcp write error: %w", err)
		}

		if n == 0 {
			log.Println("tcp write error: n == 0")
			return nil
		}

		b = b[n:]
	}

	return nil
}
