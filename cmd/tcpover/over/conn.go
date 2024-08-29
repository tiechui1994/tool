package over

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
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

func NewPipeConn(reader *io.PipeReader, writer *io.PipeWriter, meta *ctx.Metadata) net.Conn {
	return &pipeConn{
		reader: reader,
		writer: writer,
		local:  &addr{network: meta.NetWork, addr: meta.SourceAddress()},
		remote: &addr{network: meta.NetWork, addr: meta.RemoteAddress()},
	}
}

type addr struct {
	network string
	addr    string
}

func (a *addr) Network() string {
	return a.network
}

func (a *addr) String() string {
	return a.addr
}

type pipeConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	local  net.Addr
	remote net.Addr
}

func (p *pipeConn) Close() error {
	err := p.reader.Close()
	if err != nil {
		return err
	}

	err = p.writer.Close()
	return err
}

func (p *pipeConn) Read(b []byte) (n int, err error) {
	return p.reader.Read(b)
}

func (p *pipeConn) Write(b []byte) (n int, err error) {
	return p.writer.Write(b)
}

func (p *pipeConn) LocalAddr() net.Addr {
	return p.local
}

func (p *pipeConn) RemoteAddr() net.Addr {
	return p.remote
}

func (p *pipeConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *pipeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *pipeConn) SetWriteDeadline(t time.Time) error {
	return nil
}
