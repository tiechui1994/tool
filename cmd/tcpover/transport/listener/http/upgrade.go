package http

import (
	"net"
	"net/http"
	"strings"

	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/bufio"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
)

func isUpgradeRequest(req *http.Request) bool {
	for _, header := range req.Header["Connection"] {
		for _, elm := range strings.Split(header, ",") {
			if strings.EqualFold(strings.TrimSpace(elm), "Upgrade") {
				return true
			}
		}
	}

	return false
}

func handleUpgrade(conn net.Conn, request *http.Request) {
	defer conn.Close()
	removeProxyHeaders(request.Header)
	removeExtraHTTPHostPort(request)
	address := request.Host
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(address, "80")
	}

	dstAddr := socks5.ParseAddr(address)
	if dstAddr == nil {
		return
	}

	left, right := net.Pipe()
	go func() {
		remote, _ := net.Dial("tcp", dstAddr.String())
		bufio.Relay(right, remote)
	}()

	bufferedLeft := bufio.NewBufferedConn(left)
	defer bufferedLeft.Close()
	err := request.Write(bufferedLeft)
	if err != nil {
		return
	}

	resp, err := http.ReadResponse(bufferedLeft.Reader(), request)
	if err != nil {
		return
	}

	removeProxyHeaders(resp.Header)

	err = resp.Write(conn)
	if err != nil {
		return
	}
	if resp.StatusCode == http.StatusSwitchingProtocols {
		bufio.Relay(bufferedLeft, conn)
	}
}
