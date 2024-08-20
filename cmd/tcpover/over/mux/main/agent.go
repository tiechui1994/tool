package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/over/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
)

type pipeConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	local  net.Addr
	remote net.Addr
}

func (p *pipeConn) Close() error {
	err := p.reader.Close()
	if err != nil {
		return err
	}

	err = p.writer.Close()
	return err
}

func (p *pipeConn) Read(b []byte) (n int, err error) {
	return p.reader.Read(b)
}

func (p *pipeConn) Write(b []byte) (n int, err error) {
	return p.writer.Write(b)
}

func (p *pipeConn) LocalAddr() net.Addr {
	return p.local
}

func (p *pipeConn) RemoteAddr() net.Addr {
	return p.remote
}

func (p *pipeConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *pipeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *pipeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type Proxy struct {
	client *mux.ClientWorker
}

func (p *Proxy) Name() string {
	return "tcpover"
}

func (p *Proxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	upInput, upOutput := io.Pipe()
	downInput, downOutput := io.Pipe()

	local, _ := net.ResolveTCPAddr(metadata.NetWork, fmt.Sprintf("%v:%v", metadata.SrcIP, metadata.SrcPort))
	remote, _ := net.ResolveTCPAddr(metadata.NetWork, fmt.Sprintf("%v:%v", metadata.DstIP, metadata.DstPort))
	conn := &pipeConn{
		reader: upInput,
		writer: downOutput,
		local:  local,
		remote: remote,
	}

	fmt.Println("destination: ", fmt.Sprintf("%v:%v", metadata.DstIP, metadata.DstPort),)
	ctx = context.WithValue(ctx, "destination", mux.Destination{
		Network: mux.TargetNetworkTCP,
		Address: fmt.Sprintf("%v:%v", metadata.DstIP, metadata.DstPort),
	})
	p.client.Dispatch(ctx, &mux.Link{
		Reader: downInput,
		Writer: upOutput,
	})
	return conn, nil
}

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	link := &mux.Link{Reader: conn, Writer: conn}
	client := mux.NewClientWorker(link)
	proxy := &Proxy{
		client: client,
	}
	err = transport.RegisterListener("mixed", "127.0.0.1:1080")
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}

	done := make(chan struct{})
	transport.RegisterProxy(proxy)
	<-done
}
