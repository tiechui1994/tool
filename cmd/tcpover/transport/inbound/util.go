package inbound

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
)

func parseSocksAddr(target socks5.Addr) *ctx.Metadata {
	metadata := &ctx.Metadata{}

	switch target[0] {
	case socks5.AtypDomainName:
		metadata.Host = strings.TrimRight(string(target[2:2+target[1]]), ".")
		metadata.DstPort = uint16((int(target[2+target[1]]) << 8) | int(target[2+target[1]+1]))
	case socks5.AtypIPv4:
		metadata.DstIP = net.IP(target[1 : 1+net.IPv4len])
		metadata.DstPort = uint16((int(target[1+net.IPv4len]) << 8) | int(target[1+net.IPv4len+1]))
	case socks5.AtypIPv6:
		metadata.DstIP = net.IP(target[1 : 1+net.IPv6len])
		metadata.DstPort = uint16((int(target[1+net.IPv6len]) << 8) | int(target[1+net.IPv6len+1]))
	}

	return metadata
}

func parseHTTPAddr(request *http.Request) *ctx.Metadata {
	host := request.URL.Hostname()
	port, _ := strconv.ParseUint(EmptyOr(request.URL.Port(), "80"), 10, 16)
	host = strings.TrimRight(host, ".")

	metadata := &ctx.Metadata{
		NetWork: "tcp",
		Host:    host,
		DstIP:   nil,
		DstPort: uint16(port),
	}

	if ip := net.ParseIP(host); ip != nil {
		metadata.DstIP = ip
	}

	return metadata
}

func parseAddr(addr net.Addr) (net.IP, int, error) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP, a.Port, nil
	case *net.UDPAddr:
		return a.IP, a.Port, nil
	default:
		return nil, 0, fmt.Errorf("unknown address type %s", addr.String())
	}
}

func EmptyOr(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

