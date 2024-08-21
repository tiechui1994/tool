package over

import (
	"context"
	"fmt"
	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
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
	manager *clientWorkerManager
}

func (p *MuxProxy) Name() string {
	return "MuxTCPOver"
}

func (p *MuxProxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	upInput, upOutput := io.Pipe()
	downInput, downOutput := io.Pipe()

	ctx = context.WithValue(ctx, "destination", mux.Destination{
		Network: mux.TargetNetworkTCP,
		Address: fmt.Sprintf("%v:%v", metadata.DstIP, metadata.DstPort),
	})
	p.manager.Dispatch(ctx, &mux.Link{
		Reader: downInput,
		Writer: upOutput,
	})

	return NewPipeConn(upInput, downOutput, metadata), nil
}

func NewClientWorkerManager(create func() (*mux.ClientWorker, error)) (*clientWorkerManager, error) {
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

func (c *clientWorkerManager) Dispatch(ctx context.Context, link *mux.Link) {
	c.workers.Range(func(key, value interface{}) bool {
		worker := value.(*mux.ClientWorker)
		if worker.IsClosing() {
			c.workers.Delete(key)
		}
		return worker.Dispatch(ctx, link)
	})

	atomic.AddUint32(&c.connCount, 1)
	if float64(atomic.LoadUint32(&c.connCount))/float64(atomic.LoadUint32(&c.workersCount)) > 7.5 {
		go c.createClientWorker()
	}
}

func (c *clientWorkerManager) createClientWorker() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if float64(atomic.LoadUint32(&c.connCount))/float64(atomic.LoadUint32(&c.workersCount)) < 7.5 {
		return nil
	}

	worker, err := c.create()
	if err != nil {
		return err
	}

	c.workers.Store(time.Now(), worker)
	atomic.AddUint32(&c.workersCount, 1)
	return nil
}
