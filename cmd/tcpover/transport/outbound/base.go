package outbound

import (
	"context"
	"fmt"
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
)

type base struct {
	name      string
	proxyType ctx.ProxyType
}

func (p *base) Name() string {
	return p.name
}

func (p *base) Type() string {
	return p.proxyType
}

func (p *base) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	return nil, fmt.Errorf("not support")
}
