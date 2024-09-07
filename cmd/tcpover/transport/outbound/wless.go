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
}

func NewWless(option WebSocketOption) (ctx.Proxy, error) {
	if option.Server == "" {
		return nil, fmt.Errorf("server must be set")
	}
	if !regexp.MustCompile(`^(ws|wss)://`).MatchString(option.Server) {
		return nil, fmt.Errorf("server must be startsWith wss:// or ws://")
	}

	manager, err := newClientConnManager(func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
		// name: 直接连接, name is empty
		//       远程代理, name not empty
		// mode: ModeDirect | ModeForward
		code := time.Now().Format("20060102150405__Agent")
		fmt.Println(metadata.RemoteAddress())
		conn, err := wss.WebSocketConnect(ctx, option.Server, &wss.ConnectParam{
			Name: option.Remote,
			Addr: metadata.RemoteAddress(),
			Code: code,
			Role: "Agent",
			Mode: option.Mode,
		})
		if err != nil {
			return nil, err
		}

		return conn, nil
	})
	if err != nil {
		return nil, err
	}

	if option.Direct == DirectRecvOnly || option.Direct == DirectSendRecv {
		responder := PassiveResponder{server: option.Server}
		responder.manage(option.Remote)
	}

	return &Wless{
		base: &base{
			name:  option.Name,
			proxyType: ctx.Wless,
		},
		manager: manager,
	}, nil
}

type Wless struct {
	*base
	manager   *clientConnManager
}

func (p *Wless) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	return p.manager.Dispatch(ctx, metadata)
}

type clientConnManager struct {
	connCount uint32
	create    func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)
}

func newClientConnManager(create func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error)) (*clientConnManager, error) {
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

const (
	CommandLink = 0x01
)

type ControlMessage struct {
	Command uint32
	Data    map[string]interface{}
}

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
		Role: "manager",
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
		Role: "Agent",
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
		Role: "Agent",
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
