package ctx

import (
	"context"
	"net"
)

type Proxy interface {
	Name() string
	DialContext(ctx context.Context, metadata *Metadata) (net.Conn, error)
}
