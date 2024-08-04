package socks

import (
	"io"
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/auth"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/bufio"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/inbound"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
)

type Listener struct {
	listener net.Listener
	addr     string
	closed   bool
}

// RawAddress implements C.Listener
func (l *Listener) RawAddress() string {
	return l.addr
}

// Address implements C.Listener
func (l *Listener) Address() string {
	return l.listener.Addr().String()
}

// Close implements C.Listener
func (l *Listener) Close() error {
	l.closed = true
	return l.listener.Close()
}

func New(addr string, in chan<- ctx.ConnContext) (ctx.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	sl := &Listener{
		listener: l,
		addr:     addr,
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if sl.closed {
					break
				}
				continue
			}
			go handleSocks(c, in)
		}
	}()

	return sl, nil
}

func handleSocks(conn net.Conn, in chan<- ctx.ConnContext) {
	conn.(*net.TCPConn).SetKeepAlive(true)
	bufConn := bufio.NewBufferedConn(conn)
	head, err := bufConn.Peek(1)
	if err != nil {
		conn.Close()
		return
	}

	switch head[0] {
	case socks5.Version:
		HandleSocks5(bufConn, in)
	default:
		conn.Close()
	}
}

func HandleSocks5(conn net.Conn, in chan<- ctx.ConnContext) {
	target, command, err := socks5.ServerHandshake(conn, auth.NewAuthenticator(nil))
	if err != nil {
		conn.Close()
		return
	}
	if command == socks5.CmdUDPAssociate {
		defer conn.Close()
		io.Copy(io.Discard, conn)
		return
	}
	in <- inbound.NewSocket(target, conn, ctx.SOCKS5)
}
