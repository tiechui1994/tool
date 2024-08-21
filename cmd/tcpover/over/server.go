package over

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
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
		_, _ = io.CopyBuffer(remote, local, make([]byte, SocketBufferLength))
	}()

	go func() {
		defer wg.Done()

		defer onceCloseLocal.Close()
		_, _ = io.CopyBuffer(local, remote, make([]byte, SocketBufferLength))
	}()

	wg.Wait()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	code := r.URL.Query().Get("code")
	rule := r.URL.Query().Get("rule")

	regex := regexp.MustCompile(`^([a-zA-Z0-9.]+):(\d+)$`)
	if rule == RuleConnector && regex.MatchString(uid) {
		conn, err := net.Dial("tcp", uid)
		if err != nil {
			log.Printf("tcp connect [%v] : %v", uid, err)
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
		return
	}

	if rule == RuleConnector {
		manage, ok := s.manageConn.Load(uid)
		if !ok {
			log.Printf("agent [%v] not running", uid)
			http.Error(w, fmt.Sprintf("Agent [%v] not connect", uid), http.StatusBadRequest)
			return
		}
		_ = manage.(*websocket.Conn).WriteJSON(ControlMessage{
			Command: CommandLink,
			Data:    map[string]interface{}{"Code": code},
		})
	}

	conn, err := s.upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		http.Error(w, fmt.Sprintf("upgrade error: %v", err), http.StatusBadRequest)
		return
	}
	log.Printf("enter connections:%v, code:%v, uid:%v, rule:%v", atomic.AddInt32(&s.conn, +1), code, uid, rule)
	defer func() {
		log.Printf("leave connections:%v  code:%v, uid:%v, rule:%v", atomic.AddInt32(&s.conn, -1), code, uid, rule)
	}()

	// manage channel
	if rule == RuleManage {
		s.manageConn.Store(uid, conn)
		defer s.manageConn.Delete(uid)

		done := make(chan struct{})
		conn.SetPingHandler(func(message string) error {
			err := conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(time.Second))
			if err == websocket.ErrCloseSent {
				return nil
			} else if e, ok := err.(net.Error); ok && e.Temporary() {
				return nil
			} else if err == nil {
				return nil
			}

			if isClose(err) {
				close(done)
				log.Printf("closing ..... : %v", conn.Close())
				return err
			}

			log.Printf("pong error: %v", err)
			return err
		})
		<-done
	}

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
