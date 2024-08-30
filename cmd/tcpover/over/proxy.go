package over

import (
	"context"
	"fmt"
	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Proxy struct {
	manager *clientConnManager
}

func (p *Proxy) Name() string {
	return "TCPOver"
}

func (p *Proxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	return p.manager.Dispatch(ctx, metadata)
}

type clientConnManager struct {
	connCount uint32
	create    func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)
}

func NewClientConnManager(create func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)) (*clientConnManager, error) {
	c := &clientConnManager{create: create}
	return c, nil
}

func (c *clientConnManager) Dispatch(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	conn, err := c.create(ctx, metadata)
	if err != nil {
		return nil, err
	}
	atomic.AddUint32(&c.connCount, 1)

	return conn, nil
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
