package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	"io"
	"os"
	"sync"
)

type OnceCloser struct {
	io.Closer
	once sync.Once
}

func (c *OnceCloser) Close() (err error) {
	c.once.Do(func() {
		err = c.Closer.Close()
	})
	return err
}

type StdReadWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func NewStdReadWriteCloser() *StdReadWriteCloser {
	return &StdReadWriteCloser{
		ReadCloser:  os.Stdin,
		WriteCloser: os.Stdout,
	}
}

func (c *StdReadWriteCloser) Close() error {
	err1 := c.ReadCloser.Close()
	err2 := c.WriteCloser.Close()

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

type SocketStream struct {
	buf  bytes.Buffer
	conn *websocket.Conn
}

func (s *SocketStream) Close() error {
	return s.conn.Close()
}

func (s *SocketStream) Write(p []byte) (n int, err error) {
	err = s.conn.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}

func (s *SocketStream) Read(p []byte) (n int, err error) {
	if s.buf.Len() > 0  {
		return s.buf.Read(p)
	}

	_, data, err := s.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	if len(data) >= len(p) {
		n = copy(p, data)
		s.buf.Write(data[n:])
		return n, nil
	} else {
		n = copy(p, data)
		return n, nil
	}
}
