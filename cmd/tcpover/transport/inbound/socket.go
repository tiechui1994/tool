package inbound

import (
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
	"net"
)

func NewSocket(target socks5.Addr, conn net.Conn, source ctx.Type) ctx.ConnContext {
	metadata := parseSocksAddr(target)
	metadata.NetWork = "tcp"
	metadata.Type = source
	if ip, port, err := parseAddr(conn.RemoteAddr()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = uint16(port)
	}
	//if addrPort, err := netip.ParseAddrPort(conn.LocalAddr().String()); err == nil {
	//	metadata.OriginDst = addrPort
	//}
	return ctx.NewConnContext(conn, metadata)
}
