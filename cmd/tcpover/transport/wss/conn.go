package wss

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type socketConn struct {
	*websocket.Conn
	lock   sync.Mutex
	reader io.Reader
}

func NewSocketConn(conn *websocket.Conn) net.Conn {
	return &socketConn{
		Conn: conn,
	}
}

func (s *socketConn) Close() error {
	return s.Conn.Close()
}

func (s *socketConn) SetDeadline(t time.Time) error {
	err := s.SetReadDeadline(t)
	if err != nil {
		return err
	}

	err = s.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return nil
}

func (s *socketConn) Write(p []byte) (n int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	err = s.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}

func (s *socketConn) Read(b []byte) (int, error) {
	for {
		reader, err := s.getReader()
		if err != nil {
			return 0, err
		}

		nBytes, err := reader.Read(b)
		if errors.Is(err, io.EOF) {
			s.reader = nil
			continue
		}
		return nBytes, err
	}
}

func (s *socketConn) getReader() (io.Reader, error) {
	if s.reader != nil {
		return s.reader, nil
	}

	_, reader, err := s.NextReader()
	if err != nil {
		return nil, err
	}
	s.reader = reader
	return reader, nil
}

var (
	webSocketCloseCode = []int{
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseProtocolError,
		websocket.CloseUnsupportedData,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
		websocket.CloseInvalidFramePayloadData,
		websocket.CloseInternalServerErr,
		websocket.CloseServiceRestart,
		websocket.CloseTryAgainLater,
	}
)

func isSyscallError(v syscall.Errno) bool {
	return v.Is(syscall.ECONNABORTED) || v.Is(syscall.ECONNRESET) ||
		v.Is(syscall.ETIMEDOUT) || v.Is(syscall.ECONNREFUSED) ||
		v.Is(syscall.ENETUNREACH) || v.Is(syscall.ENETRESET) ||
		v.Is(syscall.EPIPE)
}

func IsClose(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(*websocket.CloseError); ok {
		return websocket.IsCloseError(err, webSocketCloseCode...)
	}

	if v, ok := err.(*net.OpError); ok {
		if vv, ok := v.Err.(syscall.Errno); ok {
			result := isSyscallError(vv)
			if result {
				fmt.Println("net.OpError", err)
			}
			return result
		}
	}

	if v, ok := err.(syscall.Errno); ok {
		result := isSyscallError(v)
		if result {
			fmt.Println("syscall.Errno", err)
		}
		return result
	}

	if strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}

	if errors.Is(err, websocket.ErrCloseSent) {
		return true
	}

	return false
}
