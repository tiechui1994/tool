package outbound

import (
	"context"
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
)

type Direct struct {
	*base
}

func NewDirect() ctx.Proxy {
	return &Direct{
		base: &base{
			name:      "DIRECT",
			proxyType: ctx.Direct,
		},
	}
}

func (p *Direct) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", metadata.RemoteAddress())
}
