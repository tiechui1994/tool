package over

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
)

type Proxy struct {
	client *Client
}

func (p *Proxy) Name() string {
	return "TCPOver"
}

func (p *Proxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	var uid = fmt.Sprintf("%v:%v", metadata.Host, metadata.DstPort)
	conn, err := p.client.webSocketConnect(ctx, uid, "", RuleConnector)
	if err != nil {
		return nil, err
	}

	return NewSocketConn(conn), nil
}

type MuxProxy struct {
	client *mux.ClientWorker
}

func (p *MuxProxy) Name() string {
	return "muxTCPOver"
}

func (p *MuxProxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	upInput, upOutput := io.Pipe()
	downInput, downOutput := io.Pipe()
	
	ctx = context.WithValue(ctx, "destination", mux.Destination{
		Network: mux.TargetNetworkTCP,
		Address: fmt.Sprintf("%v:%v", metadata.DstIP, metadata.DstPort),
	})
	p.client.Dispatch(ctx, &mux.Link{
		Reader: downInput,
		Writer: upOutput,
	})
	
	return NewPipeConn(upInput, downOutput, metadata), nil
}
