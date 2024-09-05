package over

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/outbound"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/wss"
)

type Client struct {
	server string

	localConn sync.Map
}

func NewClient(server string, proxy map[string][]string) *Client {
	if !strings.Contains(server, "://") {
		server = "wss://" + server
	}
	if proxy == nil {
		proxy = map[string][]string{}
	}

	return &Client{
		server: server,
	}
}

func (c *Client) Std(remoteName, remoteAddr string) error {
	var std io.ReadWriteCloser = NewStdReadWriteCloser()
	if Debug {
		std = NewEchoReadWriteCloser()
	}

	code := time.Now().Format("20060102150405__Std")
	if err := c.stdConnectServer(std, remoteName, remoteAddr, code); err != nil {
		log.Printf("Std::ConnectServer %v", err)
		return err
	}

	return nil
}

func (c *Client) ServeAgent(name, listenAddr string) error {
	c.manage(name)
	log.Printf("Agent start ....")

	// Remote: 直接连接, name is empty
	//       远程代理, name not empty
	// Mode: ModeDirect | ModeForward
	proxy, err := outbound.NewProxy(outbound.WebSocketOption{
		Server: c.server,
		Mode:   wss.ModeDirect,
		Remote: "",
	})
	if err != nil {
		return err
	}

	err = transport.RegisterListener("mixed", listenAddr)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	transport.RegisterProxy(proxy)
	<-done
	return nil
}

func (c *Client) ServeMuxAgent(name, listenAddr string) error {
	c.manage(name)
	log.Printf("MuxAgent start ....")

	// Remote: 直接连接, name is empty
	//       远程代理, name not empty
	// Mode: ModeDirectMux | ModeForwardMux
	proxy, err := outbound.NewMuxProxy(outbound.WebSocketOption{
		Remote: "",
		Mode:   wss.ModeDirectMux,
		Server: c.server,
	})
	if err != nil {
		return err
	}

	err = transport.RegisterListener("mixed", listenAddr)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	transport.RegisterProxy(proxy)
	<-done
	return nil
}

func (c *Client) manage(name string) {
	times := 1
try:
	time.Sleep(time.Second * time.Duration(times))
	if times >= 64 {
		times = 1
	}

	conn, err := wss.RawWebSocketConnect(context.Background(), c.server, &wss.ConnectParam{
		Name: name,
		Role: RuleManage,
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

func (c *Client) stdConnectServer(local io.ReadWriteCloser, remoteName, remoteAddr, code string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	var mode = wss.ModeForward
	if remoteName == "" || remoteName == remoteAddr {
		mode = wss.ModeDirect
	}

	conn, err := c.webSocketConnect(context.Background(), &wss.ConnectParam{
		Name: remoteName,
		Addr: remoteAddr,
		Code: code,
		Role: RuleConnector,
		Mode: mode,
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
		_, err = io.CopyBuffer(remote, local, make([]byte, SocketBufferLength))
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.CopyBuffer(local, remote, make([]byte, SocketBufferLength))
	}()

	wg.Wait()
	return nil
}

func (c *Client) connectLocalMux(code, network, addr string) error {
	conn, err := c.webSocketConnect(context.Background(), &wss.ConnectParam{
		Code: code,
		Role: RuleAgent,
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

func (c *Client) connectLocal(code, network, addr string) error {
	var local io.ReadWriteCloser
	var err error
	local, err = net.Dial(network, addr)
	if err != nil {
		return err
	}

	if Debug {
		local = NewEchoReadWriteCloser()
	}

	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	c.localConn.Store(code, onceCloseLocal)
	defer c.localConn.Delete(code)

	conn, err := c.webSocketConnect(context.Background(), &wss.ConnectParam{
		Code: code,
		Role: RuleConnector,
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
		_, err = io.CopyBuffer(remote, local, make([]byte, SocketBufferLength))
		log.Printf("ConnectLocal::error1: %v", err)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.CopyBuffer(local, remote, make([]byte, SocketBufferLength))
		log.Printf("ConnectLocal::error2: %v", err)
	}()

	wg.Wait()
	return nil
}

func (c *Client) webSocketConnect(ctx context.Context, param *wss.ConnectParam) (net.Conn, error) {
	return wss.WebSocketConnect(ctx, c.server, param)
}
