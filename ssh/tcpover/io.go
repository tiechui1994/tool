package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	random "math/rand"
	"os"
	"sync"
	"time"
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
	if s.buf.Len() > 0 {
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

type EchoStream struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func NewEchoStream() *EchoStream {
	s := new(EchoStream)
	s.reader, s.writer = io.Pipe()
	return s
}

func (s *EchoStream) Close() error {
	err1 := s.reader.Close()
	err2 := s.writer.Close()

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func (s *EchoStream) Write(p []byte) (n int, err error) {
	return s.writer.Write(p)
}

func (s *EchoStream) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

type RandomStream struct {
	in  *os.File
	out *os.File
}

func NewRandomStream() *RandomStream {
	s := new(RandomStream)
	s.in, _ = os.Create("./in.txt")
	s.out, _ = os.Create("./out.txt")
	return s
}

func (s *RandomStream) Close() error {
	err1 := s.in.Close()
	err2 := s.out.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func (s *RandomStream) Write(p []byte) (n int, err error) {
	return s.out.Write(p)
}

func init() {
	random.Seed(time.Now().UnixNano())
}

func (s *RandomStream) Read(p []byte) (n int, err error) {
	time.Sleep(time.Duration(random.Int31n(100)) * time.Millisecond)

	data := make([]byte, socketBufferLength)
	_, _ = rand.Read(data)

	n = random.Intn(len(p))
	suffix := []byte(fmt.Sprintf("==>%v\n", n))
	for len(suffix) >= n {
		n = random.Intn(len(p))
		suffix = []byte(fmt.Sprintf("==>%v\n", n))
	}

	copy(p[:n-len(suffix)], data)
	copy(p[n-len(suffix):], suffix)

	log.Printf("%v ==> %v", len(p), n)
	_, _ = s.in.Write(p[:n])
	return n, err
}