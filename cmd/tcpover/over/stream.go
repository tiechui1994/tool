package over

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	random "math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

type socketReadWriteCloser struct {
	buf  bytes.Buffer
	conn *websocket.Conn
}

func NewSocketReadWriteCloser(socket *websocket.Conn) io.ReadWriteCloser {
	return &socketReadWriteCloser{conn: socket}
}

func (s *socketReadWriteCloser) Close() error {
	return s.conn.Close()
}

func (s *socketReadWriteCloser) Write(p []byte) (n int, err error) {
	err = s.conn.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}

func (s *socketReadWriteCloser) Read(p []byte) (n int, err error) {
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

type echoReadWriteCloser struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func NewEchoReadWriteCloser() io.ReadWriteCloser {
	s := new(echoReadWriteCloser)
	s.reader, s.writer = io.Pipe()
	return s
}

func (s *echoReadWriteCloser) Close() error {
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

func (s *echoReadWriteCloser) Write(p []byte) (n int, err error) {
	return s.writer.Write(p)
}

func (s *echoReadWriteCloser) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

type randomReadWriteCloser struct {
	in  *os.File
	out *os.File
}

func init() {
	random.Seed(time.Now().UnixNano())
}

func NewRandomReadWriteCloser() io.ReadWriteCloser {
	s := new(randomReadWriteCloser)
	s.in, _ = os.Create("./in.txt")
	s.out, _ = os.Create("./out.txt")
	return s
}

func (s *randomReadWriteCloser) Close() error {
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

func (s *randomReadWriteCloser) Write(p []byte) (n int, err error) {
	return s.out.Write(p)
}

func (s *randomReadWriteCloser) Read(p []byte) (n int, err error) {
	time.Sleep(time.Duration(random.Int31n(100)) * time.Millisecond)

	data := make([]byte, SocketBufferLength)
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

type socketConn struct {
	io.ReadWriteCloser
	*websocket.Conn
}

func NewSocketConn(conn *websocket.Conn) net.Conn {
	return &socketConn{
		ReadWriteCloser: NewSocketReadWriteCloser(conn),
		Conn:            conn,
	}
}

func (c *socketConn) Close() error  {
	return c.Conn.Close()
}

func (c *socketConn) SetDeadline(t time.Time) error {
	err := c.Conn.SetReadDeadline(t)
	if err != nil {
		return err
	}

	err = c.Conn.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return nil
}
