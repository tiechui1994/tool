package over

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tiechui1994/tool/cmd/tcpover/mux"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/wss"
)

type PairGroup struct {
	done chan struct{}
	conn []*websocket.Conn
}

type Server struct {
	manageConn sync.Map // addr <=> conn

	groupMux  sync.RWMutex
	groupConn map[string]*PairGroup // code <=> []conn

	upgrade *websocket.Upgrader
	conn    int32 // number of active connections
}

func NewServer() *Server {
	return &Server{
		upgrade:   &websocket.Upgrader{},
		groupConn: map[string]*PairGroup{},
	}
}

func (s *Server) copy(local, remote io.ReadWriteCloser, deferCallback func()) {
	onceCloseLocal := &OnceCloser{Closer: local}
	onceCloseRemote := &OnceCloser{Closer: remote}

	defer func() {
		_ = onceCloseRemote.Close()
		_ = onceCloseLocal.Close()
		if deferCallback != nil {
			deferCallback()
		}
	}()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		defer onceCloseRemote.Close()
		_, _ = io.CopyBuffer(remote, local, make([]byte, wss.SocketBufferLength))
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, _ = io.CopyBuffer(local, remote, make([]byte, wss.SocketBufferLength))
	}()

	wg.Wait()
}

func (s *Server) directConnect(addr string, r *http.Request, w http.ResponseWriter) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("tcp connect [%v] : %v", addr, err)
		http.Error(w, fmt.Sprintf("tcp connect failed: %v", err), http.StatusInternalServerError)
		return
	}

	socket, err := s.upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		http.Error(w, fmt.Sprintf("upgrade error: %v", err), http.StatusBadRequest)
		return
	}

	local := conn
	remote := NewSocketReadWriteCloser(socket)
	s.copy(local, remote, nil)
}

func (s *Server) muxConnect(r *http.Request, w http.ResponseWriter) {
	socket, err := s.upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		http.Error(w, fmt.Sprintf("upgrade error: %v", err), http.StatusBadRequest)
		return
	}

	remote := NewSocketReadWriteCloser(socket)
	_, err = mux.NewServerWorker(context.Background(), mux.NewDispatcher(), remote)
	if err != nil {
		log.Printf("new mux serverWorker error: %v", err)
		http.Error(w, fmt.Sprintf("Mux ServerWorker error: %v", err), http.StatusInternalServerError)
		return
	}
}

func (s *Server) manageConnect(name string, conn *websocket.Conn) {
	s.manageConn.Store(name, conn)
	defer s.manageConn.Delete(name)

	conn.SetPingHandler(func(message string) error {
		err := conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(time.Second))
		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		} else if err == nil {
			return nil
		}

		if wss.IsClose(err) {
			log.Printf("closing ..... : %v", conn.Close())
			return err
		}
		log.Printf("pong error: %v", err)
		return err
	})
	for {
		_, _, err := conn.ReadMessage()
		if wss.IsClose(err) {
			log.Printf("closing ..... : %v", conn.Close())
			return
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "websocket" {
		var u *url.URL
		if regexp.MustCompile(`^/https?://`).MatchString(r.RequestURI) {
			u, _ = url.Parse(r.RequestURI[1:])
		} else {
			u, _ = url.Parse("https://api.quinn.eu.org")
		}
		log.Printf("url: %v", u)
		s.ProxyHandler(u, w, r)
		return
	}

	name := r.URL.Query().Get("name")
	addr := r.URL.Query().Get("addr")
	code := r.URL.Query().Get("code")
	role := r.URL.Query().Get("rule")
	mode := wss.Mode(r.URL.Query().Get("mode"))

	log.Printf("enter connections:%v, code:%v, name:%v, role:%v", atomic.AddInt32(&s.conn, +1), code, name, role)
	defer func() {
		log.Printf("leave connections:%v  code:%v, name:%v, role:%v", atomic.AddInt32(&s.conn, -1), code, name, role)
	}()

	regex := regexp.MustCompile(`^([a-zA-Z0-9.]+):(\d+)$`)

	// 情况1: 直接连接
	if (role == wss.RoleAgent || role == wss.RoleConnector) && mode.IsDirect() && regex.MatchString(addr) {
		if mode.IsMux() {
			s.muxConnect(r, w)
		} else {
			s.directConnect(addr, r, w)
		}
		return
	}

	// 情况2: 主动连接方, 需要通过被动方
	if (role == wss.RoleAgent || role == wss.RoleConnector) && name != "" {
		manage, ok := s.manageConn.Load(name)
		if !ok {
			log.Printf("agent [%v] not running", name)
			http.Error(w, fmt.Sprintf("Agent [%v] not connect", name), http.StatusBadRequest)
			return
		}

		data := map[string]interface{}{
			"Code":    code,
			"Addr":    addr,
			"Network": "tcp",
			"Mux":     mode.IsMux(),
		}
		_ = manage.(*websocket.Conn).WriteJSON(ControlMessage{
			Command: CommandLink,
			Data:    data,
		})
	}

	conn, err := s.upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		http.Error(w, fmt.Sprintf("upgrade error: %v", err), http.StatusBadRequest)
		return
	}

	// 情况3: 管理员通道
	if role == wss.RoleManager {
		s.manageConnect(name, conn)
		return
	}

	// 情況4: 正常配对连接
	s.groupMux.Lock()
	if pair, ok := s.groupConn[code]; ok {
		pair.conn = append(pair.conn, conn)
		s.groupMux.Unlock()

		local := NewSocketReadWriteCloser(pair.conn[0])
		remote := NewSocketReadWriteCloser(pair.conn[1])

		s.copy(local, remote, func() {
			close(pair.done)
			s.groupMux.Lock()
			delete(s.groupConn, code)
			s.groupMux.Unlock()
		})
	} else {
		pair := &PairGroup{
			done: make(chan struct{}),
			conn: []*websocket.Conn{conn},
		}
		s.groupConn[code] = pair
		s.groupMux.Unlock()
		<-pair.done
	}
}

func (s *Server) ProxyHandler(target *url.URL, w http.ResponseWriter, r *http.Request) {
	(&httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target
			req.Host = target.Host
			req.RequestURI = target.RequestURI()
			req.Header.Set("Host", target.Host)
		},
	}).ServeHTTP(w, r)
}

type ControlMessage struct {
	Command uint32
	Data    map[string]interface{}
}

const (
	CommandLink = 0x01
)
