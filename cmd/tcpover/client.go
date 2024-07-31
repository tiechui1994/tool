package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	LocalAgentTCP = "127.0.0.1:9988"
)

const (
	firstDataLength    = 20
	socketBufferLength = 16384

	RuleManage    = "manage"
	RuleAgent     = "Agent"
	RuleConnector = "Connector"
)

type Client struct {
	server string
	dialer *websocket.Dialer

	localConn sync.Map
}

func NewClient(server string) *Client {
	if !strings.Contains(server, "://") {
		server = "ws://" + server
	}

	m := map[string]string{
		"tcpover.pages.dev:443": "[2606:4700:310c::ac42:2d1f]:443",
	}

	return &Client{
		server: server,
		dialer: &websocket.Dialer{
			Proxy: http.ProxyFromEnvironment,
			NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				v := addr
				if val, ok := m[addr]; ok {
					v = val
				}
				fmt.Println("DialContext:", v)
				return (&net.Dialer{}).DialContext(context.Background(), network, v)
			},
			HandshakeTimeout: 45 * time.Second,
			WriteBufferSize:  socketBufferLength,
			ReadBufferSize:   socketBufferLength,
		},
	}
}

func (c *Client) Std(destUid string) error {
	var std io.ReadWriteCloser = NewStdReadWriteCloser()
	if debug {
		std = NewRandomStream()
	}

	code := time.Now().Format("20060102150405__Std")
	if err := c.ConnectServer(std, destUid, code); err != nil {
		log.Printf("Std::ConnectServer %v", err)
		return err
	}

	return nil
}

func (c *Client) Tcp(destUid string) error {
	conn, err := net.Dial("tcp", LocalAgentTCP)
	if err != nil {
		log.Printf("Tcp::Dial %v", err)
		return err
	}

	var first [firstDataLength]byte
	copy(first[:], destUid)
	n, err := conn.Write(first[:])
	if err != nil || n != firstDataLength {
		log.Printf("Tcp::Write First Data %v", err)
		return err
	}

	code := time.Now().Format("20060102150405__Tcp")
	if err := c.ConnectServer(conn, destUid, code); err != nil {
		log.Printf("Tcp::ConnectServer %v", err)
		return err
	}

	return nil
}

func (c *Client) Serve(uid string) error {
	lis, err := net.Listen("tcp", LocalAgentTCP)
	if err != nil {
		log.Printf("Serve::Listen %v", err)
		return err
	}
	defer lis.Close()

	c.Manage(uid)
	log.Printf("Connect Server Success")

	for {
		conn, err := lis.Accept()
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}

		go func(conn io.ReadWriteCloser) {
			defer conn.Close()
			var first [firstDataLength]byte
			n, err := conn.Read(first[:])
			if n != firstDataLength || err != nil {
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
			if err := c.ConnectServer(conn, destUid, code); err != nil {
				log.Printf("Serve::ConnectServer %v", err)
			}
		}(conn)
	}
}

const (
	CommandLink = 0x01
)

type ControlMessage struct {
	Command uint32
	Data    map[string]interface{}
}

var (
	webSocketCloseCode = []int{
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseProtocolError,
		websocket.CloseUnsupportedData,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
		websocket.CloseInvalidFramePayloadData,
		websocket.CloseInternalServerErr,
		websocket.CloseServiceRestart,
		websocket.CloseTryAgainLater,
	}
)

func isClose(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(*websocket.CloseError); ok {
		return websocket.IsCloseError(err, webSocketCloseCode...)
	}

	if v, ok := err.(syscall.Errno); ok {
		return v.Is(syscall.ECONNABORTED) || v.Is(syscall.ECONNRESET) ||
			v.Is(syscall.ETIMEDOUT) || v.Is(syscall.ECONNREFUSED) ||
			v.Is(syscall.ENETUNREACH) || v.Is(syscall.ENETRESET) ||
			v.Is(syscall.EPIPE)
	}

	if strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "broken pipe") {
		return true
	}

	return false
}

func (c *Client) Manage(uid string) {
	times := 1
try:
	time.Sleep(time.Second * time.Duration(times))
	if times >= 64 {
		times = 1
	}
	query := url.Values{}
	query.Set("rule", "manage")
	query.Set("uid", uid)
	conn, resp, err := c.dialer.DialContext(context.Background(), c.server+"?"+query.Encode(), nil)
	if err != nil {
		log.Printf("Manage::DialContext: %v", err)
		times = times * 2
		goto try
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		_ = resp.Write(buf)
		log.Printf("Manage::StatusCode not 101: %v", buf.String())
		times = times * 2
		goto try
	}

	go func() {
		defer func() {
			log.Printf("Manage Socket Close: %v", conn.Close())
			c.Manage(uid)
			log.Printf("Reconnect to server success")
		}()

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
					err = c.ConnectLocal(cmd.Data["Code"].(string))
					if err != nil {
						log.Println("ConnectLocal:", err)
					}
				}()
			}
		}
	}()
}

func (c *Client) ConnectServer(local io.ReadWriteCloser, destUid, code string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	query := url.Values{}
	query.Set("uid", destUid)
	query.Set("code", code)
	query.Set("rule", "Connector")
	u := c.server + "?" + query.Encode()
	log.Printf("ConnectServer: %v", u)
	conn, resp, err := c.dialer.DialContext(context.Background(), u, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		_ = resp.Write(buf)
		return fmt.Errorf("statusCode != 101:\n%s", buf.String())
	}

	remote := &SocketStream{conn: conn}
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseLocal.Close()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		defer onceCloseRemote.Close()
		_, err = io.CopyBuffer(remote, local, make([]byte, socketBufferLength))
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.CopyBuffer(local, remote, make([]byte, socketBufferLength))
	}()

	wg.Wait()
	return nil
}

func (c *Client) ConnectLocal(code string) error {
	var local io.ReadWriteCloser
	var err error
	local, err = net.Dial("tcp", "127.0.0.1:22")
	if err != nil {
		return err
	}

	if debug {
		local = NewEchoStream()
	}

	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	c.localConn.Store(code, onceCloseLocal)
	defer c.localConn.Delete(code)

	query := url.Values{}
	query.Set("uid", "anonymous")
	query.Set("code", code)
	query.Set("rule", "Agent")
	conn, resp, err := c.dialer.DialContext(context.Background(), c.server+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		_ = resp.Write(buf)
		return fmt.Errorf("statusCode != 101:\n%s", buf.String())
	}

	remote := &SocketStream{conn: conn}
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseLocal.Close()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		defer onceCloseRemote.Close()
		_, err = io.CopyBuffer(remote, local, make([]byte, socketBufferLength))
		log.Printf("ConnectLocal::error1: %v", err)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.CopyBuffer(local, remote, make([]byte, socketBufferLength))
		log.Printf("ConnectLocal::error2: %v", err)
	}()

	wg.Wait()
	return nil
}
