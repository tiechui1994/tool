package transport

import (
	"context"
	"fmt"
	"hash/crc32"
	"net"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/bufio"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/listener/http"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/listener/mixed"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/listener/socks"
	"github.com/tiechui1994/tool/log"
)

func preHandleMetadata(metadata *ctx.Metadata) error {
	if ip := net.ParseIP(metadata.Host); ip != nil {
		metadata.DstIP = ip
		metadata.Host = ""
	}
	return nil
}

func resolveMetadata(metadata *ctx.Metadata) (ctx.Proxy, error) {
	hash := crc32.ChecksumIEEE([]byte(metadata.Host))
	return proxies[int(hash)%len(proxies)], nil
}

func handleTCPConn(connCtx ctx.ConnContext) {
	defer connCtx.Conn().Close()

	metadata := connCtx.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	if err := preHandleMetadata(metadata); err != nil {
		log.Debugln("[Metadata PreHandle] error: %s", err)
		return
	}

	proxy, err := resolveMetadata(metadata)
	if err != nil {
		log.Warnln("[Metadata] parse failed: %s", err.Error())
		return
	}

	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	remote, err := proxy.DialContext(c, metadata)
	if err != nil {
		log.Warnln("[TCP] dial %s --> %s error: %s", metadata.SourceAddress(), metadata.RemoteAddress(), err.Error())
		return
	}

	bufio.Relay(remote, connCtx.Conn())
}

var (
	proxies []ctx.Proxy
	in      chan ctx.ConnContext
)

func init() {
	in = make(chan ctx.ConnContext, 100)
	go func() {
		for ctxConn := range in {
			go handleTCPConn(ctxConn)
		}
	}()
}

func RegisterListener(_type, addr string) error {
	var err error
	switch _type {
	case "socks":
		_, err = socks.New(addr, in)
	case "http":
		_, err = http.New(addr, in)
	case "mixed":
		_, err = mixed.New(addr, in)
	default:
		err = fmt.Errorf("invalid type")
	}
	return err
}

func RegisterProxy(proxy ctx.Proxy) {
	proxies = append(proxies, proxy)
}
