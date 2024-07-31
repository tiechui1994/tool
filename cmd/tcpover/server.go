package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	code := r.URL.Query().Get("code")
	rule := r.URL.Query().Get("rule")

	if rule == RuleConnector {
		manage, ok := s.manageConn.Load(uid)
		if !ok {
			log.Printf("agent [%v] not running", uid)
			http.Error(w, fmt.Sprintf("Agent [%v] not connect", uid), http.StatusBadRequest)
			return
		}
		if ok {
			_ = manage.(*websocket.Conn).WriteJSON(ControlMessage{
				Command: CommandLink,
				Data:    map[string]interface{}{"Code": code},
			})
		}
	}

	conn, err := s.upgrade.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, `upgrade error`, http.StatusBadRequest)
		return
	}
	log.Printf("enter: number of connections:%v, code:%v, uid:%v, rule:%v", atomic.AddInt32(&s.conn, +1), code, uid, rule)
	defer func() { log.Println("leave: number of connections:", atomic.AddInt32(&s.conn, -1)) }()

	// manage channel
	if rule == RuleManage {
		s.manageConn.Store(uid, conn)
		defer s.manageConn.Delete(uid)
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				err = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
				if isClose(err) {
					log.Println("closing.....", conn.Close())
					return
				}
				if err != nil {
					log.Println("ping error", err)
					continue
				}
			}
		}
	}

	s.groupMux.Lock()
	if v, ok := s.groupConn[code]; ok {
		v.conn = append(v.conn, conn)
		s.groupMux.Unlock()

		local := &SocketStream{conn: v.conn[0]}
		remote := &SocketStream{conn: v.conn[1]}

		onceCloseLocal := &OnceCloser{Closer: local}
		onceCloseRemote := &OnceCloser{Closer: remote}

		defer func() {
			close(v.done)
			s.groupMux.Lock()
			delete(s.groupConn, code)
			s.groupMux.Unlock()

			_ = onceCloseRemote.Close()
			_ = onceCloseLocal.Close()
		}()

		wg := &sync.WaitGroup{}
		wg.Add(2)

		go func() {
			defer wg.Done()

			defer onceCloseRemote.Close()
			_, _ = io.CopyBuffer(remote, local, make([]byte, socketBufferLength))
		}()

		go func() {
			defer wg.Done()

			defer onceCloseLocal.Close()
			_, _ = io.CopyBuffer(local, remote, make([]byte, socketBufferLength))
		}()

		wg.Wait()
	} else {
		pg := &PairGroup{
			done: make(chan struct{}),
			conn: []*websocket.Conn{conn},
		}
		s.groupConn[code] = pg
		s.groupMux.Unlock()

		<-pg.done
	}
}
