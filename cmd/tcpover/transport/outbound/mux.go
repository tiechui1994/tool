package outbound

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/wss"
)

func NewMuxProxy(option WebSocketOption) (ctx.Proxy, error) {
	if option.Server == "" {
		return nil, fmt.Errorf("server must be set")
	}
	if !regexp.MustCompile(`^(ws|wss)://`).MatchString(option.Server) {
		return nil, fmt.Errorf("server must be startsWith wss:// or ws://")
	}

	manager, err := newClientWorkerManager(func() (*mux.ClientWorker, error) {
		// name: 直接连接, name is empty
		//       远程代理, name not empty
		// mode: ModeDirectMux | ModeForwardMux
		code := time.Now().Format("20060102150405__MuxAgent")
		conn, err := wss.WebSocketConnect(context.Background(), option.Server, &wss.ConnectParam{
			Name: option.Remote,
			Addr: "mux.cool:9527",
			Code: code,
			Mode: option.Mode,
			Role: "Agent",
		})
		if err != nil {
			return nil, err
		}

		return mux.NewClientWorker(conn), nil
	})

	if err != nil {
		return nil, err
	}

	return &MuxProxy{manager: manager}, nil
}

type MuxProxy struct {
	manager *clientWorkerManager
}

func (p *MuxProxy) Name() string {
	return "MuxTCPOver"
}

func (p *MuxProxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	upInput, upOutput := io.Pipe()
	downInput, downOutput := io.Pipe()
	destination := mux.Destination{
		Network: mux.TargetNetworkTCP,
		Address: metadata.RemoteAddress(),
	}
	err := p.manager.Dispatch(destination, NewPipeConn(downInput, upOutput, metadata))
	if err != nil {
		return nil, err
	}

	return NewPipeConn(upInput, downOutput, metadata), nil
}

func newClientWorkerManager(create func() (*mux.ClientWorker, error)) (*clientWorkerManager, error) {
	c := &clientWorkerManager{create: create}
	err := c.createClientWorker()
	return c, err
}

type clientWorkerManager struct {
	connCount    uint32
	workersCount uint32

	lock    sync.Mutex
	workers sync.Map
	create  func() (*mux.ClientWorker, error)
}

func (c *clientWorkerManager) Dispatch(destination mux.Destination, conn io.ReadWriteCloser) error {
	var dispatch bool
	var tryCount int
again:
	if tryCount > 1 {
		log.Printf("retry to manay, please try again later")
		return fmt.Errorf("retry to manay, please try again later")
	}
	c.workers.Range(func(key, value interface{}) bool {
		worker := value.(*mux.ClientWorker)
		if worker.Closed() {
			log.Printf("worker %v close", key)
			c.workers.Delete(key)
			return true
		}

		if worker.Dispatch(destination, conn) {
			dispatch = true
			return false
		}

		return true
	})

	if !dispatch {
		err := c.createClientWorker()
		if err != nil {
			log.Printf("createClientWorker: %v", err)
			return err
		}
		tryCount += 1
		goto again
	}

	atomic.AddUint32(&c.connCount, 1)
	if float64(atomic.LoadUint32(&c.connCount))/float64(atomic.LoadUint32(&c.workersCount)) > 7.5 {
		go c.createClientWorker()
	}

	return nil
}

func (c *clientWorkerManager) createClientWorker() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	worker, err := c.create()
	if err != nil {
		return err
	}

	c.workers.Store(time.Now(), worker)
	atomic.AddUint32(&c.workersCount, 1)
	return nil
}

func NewPipeConn(reader *io.PipeReader, writer *io.PipeWriter, meta *ctx.Metadata) net.Conn {
	return &pipeConn{
		reader: reader,
		writer: writer,
		local:  &addr{network: meta.NetWork, addr: meta.SourceAddress()},
		remote: &addr{network: meta.NetWork, addr: meta.RemoteAddress()},
	}
}

type addr struct {
	network string
	addr    string
}

func (a *addr) Network() string {
	return a.network
}

func (a *addr) String() string {
	return a.addr
}

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
