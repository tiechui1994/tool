package over

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
)

type Client struct {
	server string
	dialer *websocket.Dialer

	localConn sync.Map
}

func NewClient(server string, proxy map[string][]string) *Client {
	if !strings.Contains(server, "://") {
		server = "ws://" + server
	}
	if proxy == nil {
		proxy = map[string][]string{}
	}

	return &Client{
		server: server,
		dialer: &websocket.Dialer{
			Proxy: http.ProxyFromEnvironment,
			NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				v := addr
				if val, ok := proxy[addr]; ok {
					v = val[rand.Intn(len(val))]
				}
				log.Printf("DialContext [%v]: %v", addr, v)
				return (&net.Dialer{}).DialContext(context.Background(), network, v)
			},
			HandshakeTimeout: 45 * time.Second,
			WriteBufferSize:  SocketBufferLength,
			ReadBufferSize:   SocketBufferLength,
		},
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

	manager, err := NewClientConnManager(func(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
		fmt.Println("addr", metadata.Host, metadata.NetWork, metadata.Type)
		// name: 直接连接, name is empty
		//       远程代理, name not empty
		// mode: ModeDirect | ModeForward
		addr := fmt.Sprintf("%v:%v", metadata.Host, metadata.DstPort)
		code := time.Now().Format("20060102150405__Agent")
		conn, err := c.webSocketConnect(ctx, &ConnectParam{
			name: "",
			addr: addr,
			code: code,
			rule: RuleAgent,
			mode: ModeDirect,
		})
		if err != nil {
			return nil, err
		}

		return NewSocketConn(conn), nil
	})
	if err != nil {
		return err
	}

	err = transport.RegisterListener("mixed", listenAddr)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	transport.RegisterProxy(&Proxy{manager: manager})
	<-done
	return nil
}

func (c *Client) ServeMuxAgent(name, listenAddr string) error {
	c.manage(name)
	log.Printf("MuxAgent start ....")

	// name: 直接连接, name is empty
	//       远程代理, name not empty
	// mode: ModeDirectMux | ModeForwardMux
 	manager, err := NewClientWorkerManager(func() (*mux.ClientWorker, error) {
		code := time.Now().Format("20060102150405__MuxAgent")
		conn, err := c.webSocketConnect(context.Background(), &ConnectParam{
			name: "",
			code: code,
			mode: ModeDirectMux,
			rule: RuleAgent,
		})
		if err != nil {
			return nil, err
		}

		connTemp := NewSocketConn(conn)
		return mux.NewClientWorker(&mux.Link{
			Reader: connTemp,
			Writer: connTemp,
		}), nil
	})
	if err != nil {
		return err
	}

	err = transport.RegisterListener("mixed", listenAddr)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	transport.RegisterProxy(&MuxProxy{manager: manager})
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

	conn, err := c.webSocketConnect(context.Background(), &ConnectParam{
		name: name,
		rule: RuleManage,
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
			if isClose(err) {
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

	var mode = ModeForward
	if remoteName == "" || remoteName == remoteAddr {
		mode = ModeDirect
	}

	conn, err := c.webSocketConnect(context.Background(), &ConnectParam{
		name: remoteName,
		addr: remoteAddr,
		code: code,
		rule: RuleConnector,
		mode: mode,
	})
	if err != nil {
		return err
	}

	remote := NewSocketReadWriteCloser(conn)
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
	conn, err := c.webSocketConnect(context.Background(), &ConnectParam{
		code: code,
		rule: RuleAgent,
	})
	if err != nil {
		return err
	}

	remote := NewSocketReadWriteCloser(conn)
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseRemote.Close()

	_, err = mux.NewServerWorker(context.Background(), mux.NewDispatcher(), &mux.Link{
		Reader: remote,
		Writer: remote,
	})
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

	conn, err := c.webSocketConnect(context.Background(), &ConnectParam{
		code: code,
		rule: RuleConnector,
	})
	if err != nil {
		return err
	}

	remote := NewSocketReadWriteCloser(conn)
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

type ConnectParam struct {
	name string
	addr string
	code string
	rule string
	mode string
}

func (c *Client) webSocketConnect(ctx context.Context, param *ConnectParam) (*websocket.Conn, error) {
	query := url.Values{}
	query.Set("name", param.name)
	query.Set("addr", param.addr)
	query.Set("code", param.code)
	query.Set("rule", param.rule)
	query.Set("mode", param.mode)
	u := c.server + "?" + query.Encode()
	conn, resp, err := c.dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		_ = resp.Write(buf)
		return nil, fmt.Errorf("statusCode != 101:\n%s", buf.String())
	}

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			err = conn.WriteControl(websocket.PingMessage, []byte(nil), time.Now().Add(time.Second))
			if isClose(err) {
				return
			}
			if err != nil {
				log.Printf("Ping: %v", err)
			}
		}
	}()

	return conn, err
}
