package outbound

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/wss"
)

type WebSocketOption struct {
	Remote string   `proxy:"remote"`
	Mode   wss.Mode `proxy:"mode"`
	Server string   `proxy:"Server"`
}

func NewProxy(option WebSocketOption) (ctx.Proxy, error) {
	if option.Server == "" {
		return nil, fmt.Errorf("server must be set")
	}
	if !regexp.MustCompile(`^(ws|wss)://`).MatchString(option.Server) {
		return nil, fmt.Errorf("server must be startsWith wss:// or ws://")
	}

	manager, err := newClientConnManager(func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
		// name: 直接连接, name is empty
		//       远程代理, name not empty
		// mode: ModeDirect | ModeForward
		code := time.Now().Format("20060102150405__Agent")
		conn, err := wss.WebSocketConnect(ctx, option.Server, &wss.ConnectParam{
			Name: option.Remote,
			Addr: metadata.RemoteAddress(),
			Code: code,
			Role: "Agent",
			Mode: option.Mode,
		})
		if err != nil {
			return nil, err
		}

		return conn, nil
	})
	if err != nil {
		return nil, err
	}

	return &Proxy{manager: manager}, nil
}

type Proxy struct {
	manager *clientConnManager
}

func (p *Proxy) Name() string {
	return "TCPOver"
}

func (p *Proxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	return p.manager.Dispatch(ctx, metadata)
}

type clientConnManager struct {
	connCount uint32
	create    func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)
}

func newClientConnManager(create func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)) (*clientConnManager, error) {
	c := &clientConnManager{create: create}
	return c, nil
}

func (c *clientConnManager) Dispatch(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	conn, err := c.create(ctx, metadata)
	if err != nil {
		return nil, err
	}
	atomic.AddUint32(&c.connCount, 1)

	return conn, nil
}
