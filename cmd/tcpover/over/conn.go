package over

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

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

func (c *socketConn) Close() error {
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
