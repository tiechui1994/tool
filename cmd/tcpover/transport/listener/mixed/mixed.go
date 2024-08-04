package mixed

import (
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/bufio"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/cache"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/listener/http"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/listener/socks"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
)

type Listener struct {
	listener net.Listener
	addr     string
	cache    *cache.LruCache
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

	ml := &Listener{
		listener: l,
		addr:     addr,
		cache:    cache.New(cache.WithAge(30)),
	}
	go func() {
		for {
			c, err := ml.listener.Accept()
			if err != nil {
				if ml.closed {
					break
				}
				continue
			}
			go handleConn(c, in, ml.cache)
		}
	}()

	return ml, nil
}

func handleConn(conn net.Conn, in chan<- ctx.ConnContext, cache *cache.LruCache) {
	conn.(*net.TCPConn).SetKeepAlive(true)

	bufConn := bufio.NewBufferedConn(conn)
	head, err := bufConn.Peek(1)
	if err != nil {
		return
	}

	switch head[0] {
	case socks5.Version:
		socks.HandleSocks5(bufConn, in)
	default:
		http.HandleConn(bufConn, in, cache)
	}
}
