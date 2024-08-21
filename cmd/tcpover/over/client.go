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

func (c *Client) Std(destUid string) error {
	var std io.ReadWriteCloser = NewStdReadWriteCloser()
	if Debug {
		std = NewEchoReadWriteCloser()
	}

	code := time.Now().Format("20060102150405__Std")
	if err := c.connectServer(std, destUid, code); err != nil {
		log.Printf("Std::ConnectServer %v", err)
		return err
	}

	return nil
}

func (c *Client) ServeAgent(destUid string) error {
	lis, err := net.Listen("tcp", LocalAgentTCP)
	if err != nil {
		log.Printf("Serve::Listen %v", err)
		return err
	}
	defer lis.Close()

	c.manage(destUid)
	log.Printf("Connect Server Success")

	for {
		conn, err := lis.Accept()
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}

		go func(conn io.ReadWriteCloser) {
			defer conn.Close()
			var first [FirstDataLength]byte
			n, err := conn.Read(first[:])
			if n != FirstDataLength || err != nil {
				log.Printf("Serve::Read Uid %v", err)
				return
			}

			var destUid string
			for i, v := range first {
				if v == 0 {
					destUid = string(first[:i])
					break
				}
			}
			if destUid == "" {
				log.Printf("Serve::destUid is empty")
				return
			}

			code := time.Now().Format("20060102150405__Serve")
			if err := c.connectServer(conn, destUid, code); err != nil {
				log.Printf("Serve::ConnectServer %v", err)
			}
		}(conn)
	}
}

func (c *Client) ServeProxy(localUid string) error {
	err := transport.RegisterListener("mixed", localUid)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	transport.RegisterProxy(&Proxy{c: c})
	<-done
	return nil
}

func (c *Client) manage(uid string) {
	times := 1
try:
	time.Sleep(time.Second * time.Duration(times))
	if times >= 64 {
		times = 1
	}

	conn, err := c.webSocketConnect(context.Background(), uid, "", RuleManage)
	if err != nil {
		log.Printf("Manage::DialContext: %v", err)
		times = times * 2
		goto try
	}

	var onceClose sync.Once
	closeFunc := func() {
		log.Printf("Manage Socket Close: %v", conn.Close())
		c.manage(uid)
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
				log.Printf("ReadMessage: %v", err)
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
					err = c.connectLocal(cmd.Data["Code"].(string))
					if err != nil {
						log.Println("ConnectLocal:", err)
					}
				}()
			}
		}
	}()
}

func (c *Client) connectServer(local io.ReadWriteCloser, destUid, code string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	conn, err := c.webSocketConnect(context.Background(), destUid, code, RuleConnector)
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

func (c *Client) connectLocal(code string) error {
	var local io.ReadWriteCloser
	var err error
	local, err = net.Dial("tcp", "127.0.0.1:22")
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

	conn, err := c.webSocketConnect(context.Background(), "anonymous", code, RuleAgent)
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

func (c *Client) webSocketConnect(ctx context.Context, uid, code, rule string) (*websocket.Conn, error) {
	query := url.Values{}
	query.Set("uid", uid)
	query.Set("code", code)
	query.Set("rule", rule)
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
		ticker := time.NewTicker(20 * time.Second)
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

type Proxy struct {
	c *Client
}

func (p *Proxy) Name() string {
	return "tcpover"
}

func (p *Proxy) DialContext(ctx context.Context, metadata *ctx.Metadata) (net.Conn, error) {
	var uid = fmt.Sprintf("%v:%v", metadata.Host, metadata.DstPort)
	conn, err := p.c.webSocketConnect(ctx, uid, "", RuleConnector)
	if err != nil {
		return nil, err
	}

	return NewSocketConn(conn), nil
}
