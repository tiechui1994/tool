package ctx

import (
	"context"
	"net"
)

type Proxy interface {
	Name() string
	Type() ProxyType
	DialContext(ctx context.Context, metadata *Metadata) (net.Conn, error)
}

type ProxyType = string

const (
	Wless    ProxyType = "Wless"
	Direct   ProxyType = "Direct"
)
