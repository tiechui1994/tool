package inbound

import (
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
)

func NewHTTP(target socks5.Addr, source net.Addr, originTarget net.Addr, conn net.Conn) ctx.ConnContext {
	metadata := parseSocksAddr(target)
	metadata.NetWork = "tcp"
	metadata.Type = ctx.HTTP
	if ip, port, err := parseAddr(source); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = uint16(port)
	}
	if originTarget != nil {
		metadata.Origin = originTarget.String()
	}
	return ctx.NewConnContext(conn, metadata)
}
