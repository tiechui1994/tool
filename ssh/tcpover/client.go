package main

import (
	"bytes"
	"context"
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

var (
	LocalAgentTCP = "127.0.0.1:9988"
)

const (
	firstDataLength = 20
)

type Client struct {
	server string

	localConn sync.Map
}

func NewClient(server string) *Client {
	if !strings.Contains(server, "://") {
		server = "ws://" + server
	}
	return &Client{server: server}
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

	err = c.Manage(uid)
	if err != nil {
		log.Printf("Serve::Manage %v", err)
		return err
	}

	for {
		conn, err := lis.Accept()
		if err != nil {
			time.Sleep(time.Second * 5)
			continue
		}

		go func(conn io.ReadWriteCloser) {
			var first [firstDataLength]byte
			n, err := conn.Read(first[:])
			if n != firstDataLength || err != nil {
				_ = conn.Close()
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
				_ = conn.Close()
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

func (c *Client) Manage(uid string) error {
	query := url.Values{}
	query.Set("rule", "manage")
	query.Set("uid", uid)
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), c.server+"?"+query.Encode(), nil)
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
			//err = conn.ReadJSON(&cmd)
			//if err != nil {
			//	continue
			//}

			_, p, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
					conn.Close()
					return
				}
				log.Printf("ReadMessage: %v", err)
				continue
			}
			log.Printf(`[%v]`, string(p))
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

	return nil
}

func (c *Client) ConnectServer(local io.ReadWriteCloser, destUid, code string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	query := url.Values{}
	query.Set("uid", destUid)
	query.Set("code", code)
	u := c.server + "?" + query.Encode()
	log.Printf("ConnectServer: %v", u)
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), u, nil)
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
		_, err = io.Copy(remote, local)
		log.Printf("error1: %v", err)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.Copy(local, remote)
		log.Printf("error2: %v", err)
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
	conn, resp, err := websocket.DefaultDialer.DialContext(context.Background(), c.server+"?"+query.Encode(), nil)
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
		_, err = io.Copy(remote, local)
		log.Printf("ConnectLocal::error1: %v", err)
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.Copy(local, remote)
		log.Printf("ConnectLocal::error2: %v", err)
	}()

	wg.Wait()
	return nil
}
