package tcpover

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	authHeaderType    = `HTTP2TCP`
	httpHeaderUpgrade = `http2tcp/1.0`
)

type Client struct {
	server    string
	userAgent string

	localConn sync.Map
}

func NewClient(server string) *Client {
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	return &Client{server: server}
}

// If non-empty, when connecting to the server, this User-Agent will be used
// instead of the default `Go-http-client/1.1`.
func (c *Client) SetUserAgent(userAgent string) {
	c.userAgent = userAgent
}

func (c *Client) Std(to string) {
	std := NewStdReadWriteCloser()
	code := time.Now().String()
	if err := c.ConnectServer(std, to, code); err != nil {
		log.Println(err)
	}
}

func (c *Client) Serve(listen string, to string) {
	lis, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatalln(err)
	}
	defer lis.Close()

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println(err)
			time.Sleep(time.Second * 5)
			continue
		}
		go func(conn io.ReadWriteCloser) {
			code := time.Now().String()
			if err := c.ConnectServer(conn, to, code); err != nil {
				log.Println(err)
			}
		}(conn)
	}
}

const (
	CommandLink  = 0x01
	CommandClose = 0x02
)

type ControlMessage struct {
	Command uint32
	Data    []byte
}

func (c *Client) Manage() error {
	header := http.Header{}
	header.Set("rule", "manage")
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), c.server, header)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		_ = resp.Write(buf)
		return fmt.Errorf("statusCode != 101:\n%s", buf.String())
	}

	go func() {
		for {
			var cmd ControlMessage
			err = conn.ReadJSON(&cmd)
			if err != nil {
				continue
			}

			switch cmd.Command {
			case CommandLink:
				var connectInfo struct{
					Code string
					Addr string
				}
				err = json.Unmarshal(cmd.Data, &connectInfo)
				if err != nil {
					continue
				}
				go func() {
					err = c.ConnectLocal(connectInfo.Addr,connectInfo.Code)
					if err != nil {
						log.Println("ConnectLocal:", err)
					}
				}()
			case CommandClose:
				var connectInfo struct{
					Code string
				}
				err = json.Unmarshal(cmd.Data, &connectInfo)
				if err != nil {
					continue
				}
				if val, ok := c.localConn.Load(connectInfo.Code); ok {
					val.(io.Closer).Close()
				}
			}
		}
	}()

	return nil
}

func (c *Client) ConnectServer(local io.ReadWriteCloser, addr, code string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	header := http.Header{}
	header.Set("addr", addr)
	header.Set("code", code)
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), c.server, header)
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
		_, _ = io.Copy(remote, local)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, _ = io.Copy(local, remote)
	}()

	wg.Wait()
	return nil
}

func (c *Client) ConnectLocal(addr, code string) error {
	local, err := net.Dial(`tcp`, addr)
	if err != nil {
		return err
	}
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	c.localConn.Store(code, onceCloseLocal)
	defer c.localConn.Delete(code)

	header := http.Header{}
	header.Set("addr", addr)
	header.Set("code", code)
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), c.server, header)
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
		_, _ = io.Copy(remote, local)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, _ = io.Copy(local, remote)
	}()

	wg.Wait()
	return nil
}

func (c *Client) proxy(local io.ReadWriteCloser, addr string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	u, err := url.Parse(c.server)
	if err != nil {
		return err
	}
	host := u.Hostname()
	port := u.Port()
	if port == `` {
		switch u.Scheme {
		case `http`:
			port = "80"
		case `https`:
			port = `443`
		default:
			return fmt.Errorf(`unknown scheme: %s`, u.Scheme)
		}
	}
	serverAddr := net.JoinHostPort(host, port)

	var remote net.Conn
	if u.Scheme == `http` {
		remote, err = net.Dial(`tcp`, serverAddr)
		if err != nil {
			return err
		}
	} else if u.Scheme == `https` {
		remote, err = tls.Dial(`tcp`, serverAddr, nil)
		if err != nil {
			return err
		}
	}
	if remote == nil {
		return fmt.Errorf("no server connection made")
	}

	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseRemote.Close()

	v := u.Query()
	v.Set(`addr`, addr)
	u.RawQuery = v.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add(`Connection`, `upgrade`)
	req.Header.Add(`Upgrade`, httpHeaderUpgrade)
	if c.userAgent != `` {
		req.Header.Add(`User-Agent`, c.userAgent)
	}

	if err := req.Write(remote); err != nil {
		return err
	}
	bior := bufio.NewReader(remote)
	resp, err := http.ReadResponse(bior, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		buf := bytes.NewBuffer(nil)
		resp.Write(buf)
		return fmt.Errorf("statusCode != 101:\n%s", buf.String())
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		defer onceCloseRemote.Close()
		_, _ = io.Copy(remote, local)
	}()

	go func() {
		defer wg.Done()

		if n := int64(bior.Buffered()); n > 0 {
			if nc, err := io.CopyN(local, bior, n); err != nil || nc != n {
				log.Println("io.CopyN:", nc, err)
				return
			}
		}

		defer onceCloseLocal.Close()
		_, _ = io.Copy(local, remote)
	}()

	wg.Wait()
	return nil
}
