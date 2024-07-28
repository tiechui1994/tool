package tcpover

import (
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

type Server struct {
	manageConn sync.Map // addr <=> conn
	linkConn   sync.Map // code <=> []conn

	upgrade *websocket.Upgrader
	conn    int32 // number of active connections
}

func NewServer(token string) *Server {
	return &Server{
		upgrade: &websocket.Upgrader{},
	}
}

func (s *Server) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrade.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, `upgrade error`, http.StatusBadRequest)
		return
	}

	addr := r.URL.Query().Get("addr")
	code := r.URL.Query().Get("code")
	rule := r.URL.Query().Get("rule")

	// manage channel
	if rule == "manage" {
		s.manageConn.Store(addr, conn)
		for {
			select {}
		}
	}

	// link channel
	remote, err := net.Dial(`tcp`, addr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseRemote.Close()

	w.Header().Add(`Content-Length`, `0`)
	w.WriteHeader(http.StatusSwitchingProtocols)
	local, bio, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	log.Println("enter: number of connections:", atomic.AddInt32(&s.conn, +1))
	defer func() { log.Println("leave: number of connections:", atomic.AddInt32(&s.conn, -1)) }()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		// The returned bufio.Reader may contain unprocessed buffered data from the client.
		// Copy them to dst so we can use src directly.
		if n := bio.Reader.Buffered(); n > 0 {
			n64, err := io.CopyN(remote, bio, int64(n))
			if n64 != int64(n) || err != nil {
				log.Println("io.CopyN:", n64, err)
				return
			}
		}

		defer onceCloseRemote.Close()
		_, _ = io.Copy(remote, local)
	}()

	go func() {
		defer wg.Done()

		// flush any unwritten data.
		if err := bio.Writer.Flush(); err != nil {
			log.Println(`bio.Writer.Flush():`, err)
			return
		}

		defer onceCloseLocal.Close()
		_, _ = io.Copy(local, remote)
	}()

	wg.Wait()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if upgrade := r.Header.Get(`Upgrade`); upgrade != httpHeaderUpgrade {
		http.Error(w, `upgrade error`, http.StatusBadRequest)
		return
	}

	// the URL.Path doesn't matter.
	addr := r.URL.Query().Get("addr")
	remote, err := net.Dial(`tcp`, addr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	onceCloseRemote := &OnceCloser{Closer: remote}
	defer onceCloseRemote.Close()

	w.Header().Add(`Content-Length`, `0`)
	w.WriteHeader(http.StatusSwitchingProtocols)
	local, bio, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	onceCloseLocal := &OnceCloser{Closer: local}
	defer onceCloseLocal.Close()

	log.Println("enter: number of connections:", atomic.AddInt32(&s.conn, +1))
	defer func() { log.Println("leave: number of connections:", atomic.AddInt32(&s.conn, -1)) }()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		// The returned bufio.Reader may contain unprocessed buffered data from the client.
		// Copy them to dst so we can use src directly.
		if n := bio.Reader.Buffered(); n > 0 {
			n64, err := io.CopyN(remote, bio, int64(n))
			if n64 != int64(n) || err != nil {
				log.Println("io.CopyN:", n64, err)
				return
			}
		}

		defer onceCloseRemote.Close()
		_, _ = io.Copy(remote, local)
	}()

	go func() {
		defer wg.Done()

		// flush any unwritten data.
		if err := bio.Writer.Flush(); err != nil {
			log.Println(`bio.Writer.Flush():`, err)
			return
		}

		defer onceCloseLocal.Close()
		_, _ = io.Copy(local, remote)
	}()

	wg.Wait()
}
