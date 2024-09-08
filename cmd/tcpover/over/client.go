package over

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/config"
	"github.com/tiechui1994/tool/cmd/tcpover/transport"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/wss"
)

var (
	Debug bool
)

type Client struct {
	server string
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

func (c *Client) Serve(config config.RawConfig) error {
	for _, v := range config.Proxies {
		proxy, err := transport.ParseProxy(v)
		if err != nil {
			return err
		}

		transport.RegisterProxy(proxy)
	}

	listenAddr := fmt.Sprintf("%v", config.Listen)
	err := transport.RegisterListener("mixed", listenAddr)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	<-done
	return nil
}

func (c *Client) stdConnectServer(local io.ReadWriteCloser, remoteName, remoteAddr, code string) error {
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	var mode = wss.ModeForward
	if remoteName == "" || remoteName == remoteAddr {
		mode = wss.ModeDirect
	}

	conn, err := wss.WebSocketConnect(context.Background(), c.server, &wss.ConnectParam{
		Name: remoteName,
		Addr: remoteAddr,
		Code: code,
		Role: wss.RoleConnector,
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
		_, err = io.CopyBuffer(remote, local, make([]byte, wss.SocketBufferLength))
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, err = io.CopyBuffer(local, remote, make([]byte, wss.SocketBufferLength))
	}()

	wg.Wait()
	return nil
}
