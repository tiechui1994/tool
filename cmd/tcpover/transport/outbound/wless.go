package outbound

import (
	"context"
	"encoding/json"
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

const (
	DirectRecvOnly = "recv"
	DirectSendOnly = "send"
	DirectSendRecv = "sendrecv"
)

type WebSocketOption struct {
	Name   string   `proxy:"name"`
	Local  string   `proxy:"local"`
	Remote string   `proxy:"remote"`
	Mode   wss.Mode `proxy:"mode"`
	Server string   `proxy:"server"`
	Direct string   `proxy:"direct"`
	Mux    bool     `proxy:"mux"`
}

func NewWless(option WebSocketOption) (ctx.Proxy, error) {
	if option.Server == "" {
		return nil, fmt.Errorf("server must be set")
	}
	if !regexp.MustCompile(`^(ws|wss)://`).MatchString(option.Server) {
		return nil, fmt.Errorf("server must be startsWith wss:// or ws://")
	}

	var manager dispatcher
	var err error
	if option.Mux {
		manager, err = newMuxConnManager(option)
	} else {
		manager, err = newDirectConnManager(option)
	}
	if err != nil {
		return nil, err
	}

	if option.Direct == DirectRecvOnly || option.Direct == DirectSendRecv {
		responder := PassiveResponder{server: option.Server}
		responder.manage(option.Remote)
	}

	return &Wless{
		base: &base{
			name:      option.Name,
			proxyType: ctx.Wless,
		},
		manager: manager,
	}, nil
}

type dispatcher interface {
	DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)
}

type Wless struct {
	*base
	manager dispatcher
}

func (p *Wless) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	return p.manager.DialContext(ctx, metadata)
}

type directConnManager struct {
	createConn func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)
}

func newDirectConnManager(option WebSocketOption) (*directConnManager, error) {
	return &directConnManager{
		createConn: func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
			var mode wss.Mode
			if option.Mode.IsDirect() {
				mode = wss.ModeDirect
			} else if option.Mode.IsForward() {
				mode = wss.ModeForward
			} else {
				mode = wss.ModeDirect
			}
			// name: 直接连接, name is empty
			//       远程代理, name not empty
			// mode: ModeDirect | ModeForward
			code := time.Now().Format("20060102150405__Agent")
			conn, err := wss.WebSocketConnect(ctx, option.Server, &wss.ConnectParam{
				Name: option.Remote,
				Addr: metadata.RemoteAddress(),
				Code: code,
				Mode: mode,
				Role: wss.RoleAgent,
			})
			if err != nil {
				return nil, err
			}

			return conn, nil
		},
	}, nil
}

func (c *directConnManager) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	log.Println(metadata.SourceAddress(), "=>", metadata.RemoteAddress())
	return c.createConn(ctx, metadata)
}

type muxConnManager struct {
	connCount    uint32
	workersCount uint32

	lock       sync.Mutex
	workers    sync.Map
	createConn func() (*mux.ClientWorker, error)
}

func newMuxConnManager(option WebSocketOption) (*muxConnManager, error) {
	c := &muxConnManager{createConn: func() (*mux.ClientWorker, error) {
		var mode wss.Mode
		if option.Mode.IsDirect() {
			mode = wss.ModeDirectMux
		} else if option.Mode.IsForward() {
			mode = wss.ModeForwardMux
		} else {
			mode = wss.ModeDirectMux
		}
		// name: 直接连接, name is empty
		//       远程代理, name not empty
		// mode: ModeDirectMux | ModeForwardMux
		code := time.Now().Format("20060102150405__MuxAgent")
		conn, err := wss.WebSocketConnect(context.Background(), option.Server, &wss.ConnectParam{
			Name: option.Remote,
			Addr: "mux.cool:9527",
			Code: code,
			Mode: mode,
			Role: wss.RoleAgent,
		})
		if err != nil {
			return nil, err
		}

		return mux.NewClientWorker(conn), nil
	}}

	err := c.create()
	return c, err
}

func (c *muxConnManager) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	log.Println(metadata.SourceAddress(), "=>", metadata.RemoteAddress(), "mux")
	upInput, upOutput := io.Pipe()
	downInput, downOutput := io.Pipe()
	destination := mux.Destination{
		Network: mux.TargetNetworkTCP,
		Address: metadata.RemoteAddress(),
	}
	err := c.dispatch(destination, NewPipeConn(downInput, upOutput, metadata))
	if err != nil {
		return nil, err
	}

	return NewPipeConn(upInput, downOutput, metadata), nil
}

func (c *muxConnManager) dispatch(destination mux.Destination, conn io.ReadWriteCloser) error {
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
		err := c.create()
		if err != nil {
			log.Printf("createClientWorker: %v", err)
			return err
		}
		tryCount += 1
		goto again
	}

	atomic.AddUint32(&c.connCount, 1)
	if float64(atomic.LoadUint32(&c.connCount))/float64(atomic.LoadUint32(&c.workersCount)) > 7.5 {
		go c.create()
	}

	return nil
}

func (c *muxConnManager) create() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	worker, err := c.createConn()
	if err != nil {
		return err
	}

	c.workers.Store(time.Now(), worker)
	atomic.AddUint32(&c.workersCount, 1)
	return nil
}

type ControlMessage struct {
	Command uint32
	Data    map[string]interface{}
}

const (
	CommandLink = 0x01
)

type PassiveResponder struct {
	server string
}

func (c *PassiveResponder) manage(name string) {
	times := 1
try:
	time.Sleep(time.Second * time.Duration(times))
	if times >= 64 {
		times = 1
	}

	conn, err := wss.RawWebSocketConnect(context.Background(), c.server, &wss.ConnectParam{
		Name: name,
		Role: wss.RoleManager,
	})
	if err != nil {
		log.Printf("Manage::DialContext: %v", err)
		times = times * 2
		goto try
	}

	var onceClose sync.Once
	closeFunc := func() {
		log.Printf("Manage Socket Close: %v", conn.Close())
		c.manage(name)
		log.Printf("Reconnect to server success")
	}

	go func() {
		defer onceClose.Do(closeFunc)

		for {
			var cmd ControlMessage
			_, p, err := conn.ReadMessage()
			if wss.IsClose(err) {
				return
			}
			if err != nil {
				log.Printf("ReadMessage: %+v", err)
				continue
			}
			err = json.Unmarshal(p, &cmd)
			if err != nil {
				log.Printf("Unmarshal: %v", err)
				continue
			}

			switch cmd.Command {
			case CommandLink:
				log.Printf("ControlMessage => cmd %v, data: %v", cmd.Command, cmd.Data)
				go func() {
					code := cmd.Data["Code"].(string)
					addr := cmd.Data["Addr"].(string)
					network := cmd.Data["Network"].(string)
					if v := cmd.Data["Mux"]; v.(bool) {
						err = c.connectLocalMux(code, network, addr)
					} else {
						err = c.connectLocal(code, network, addr)
					}
					if err != nil {
						log.Println("ConnectLocal:", err)
					}
				}()
			}
		}
	}()
}

func (c *PassiveResponder) connectLocalMux(code, network, addr string) error {
	conn, err := wss.WebSocketConnect(context.Background(), c.server, &wss.ConnectParam{
		Code: code,
		Role: wss.RoleAgent,
	})
	if err != nil {
		return err
	}

	remote := conn
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseRemote.Close()

	_, err = mux.NewServerWorker(context.Background(), mux.NewDispatcher(), remote)
	if err != nil {
		return err
	}

	return nil
}

func (c *PassiveResponder) connectLocal(code, network, addr string) error {
	var local io.ReadWriteCloser
	var err error
	local, err = net.Dial(network, addr)
	if err != nil {
		return err
	}

	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	conn, err := wss.WebSocketConnect(context.Background(), c.server, &wss.ConnectParam{
		Code: code,
		Role: wss.RoleAgent,
	})
	if err != nil {
		return err
	}

	remote := conn
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseLocal.Close()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		defer onceCloseRemote.Close()
		_, err = io.CopyBuffer(remote, local, make([]byte, wss.SocketBufferLength))
		log.Printf("ConnectLocal::error1: %v", err)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.CopyBuffer(local, remote, make([]byte, wss.SocketBufferLength))
		log.Printf("ConnectLocal::error2: %v", err)
	}()

	wg.Wait()
	return nil
}

type OnceCloser struct {
	io.Closer
	once sync.Once
}

func (c *OnceCloser) Close() (err error) {
	c.once.Do(func() {
		err = c.Closer.Close()
	})
	return err
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
