package outbound

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
)

func NewProxy(create func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)) (ctx.Proxy, error) {
	manager, err := newClientConnManager(create)
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
