package mux

import (
	"io"
	"sync"

	"github.com/tiechui1994/tool/cmd/tcpover/mux/buf"
)

type SessionManager struct {
	sync.RWMutex
	sessions map[uint16]*Session
	count    uint16
	closed   bool
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		count:    0,
		sessions: make(map[uint16]*Session, 16),
	}
}

func (m *SessionManager) Closed() bool {
	m.RLock()
	defer m.RUnlock()

	return m.closed
}

func (m *SessionManager) Size() int {
	m.RLock()
	defer m.RUnlock()

	return len(m.sessions)
}

func (m *SessionManager) Count() int {
	m.RLock()
	defer m.RUnlock()

	return int(m.count)
}

func (m *SessionManager) Allocate() *Session {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return nil
	}

	m.count++
	s := &Session{
		ID:     m.count,
		parent: m,
	}
	m.sessions[s.ID] = s
	return s
}

func (m *SessionManager) Add(s *Session) {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return
	}

	m.count++
	m.sessions[s.ID] = s
}

func (m *SessionManager) Remove(id uint16) {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return
	}

	delete(m.sessions, id)

	if len(m.sessions) == 0 {
		m.sessions = make(map[uint16]*Session, 16)
	}
}

func (m *SessionManager) Get(id uint16) (*Session, bool) {
	m.RLock()
	defer m.RUnlock()

	if m.closed {
		return nil, false
	}

	s, found := m.sessions[id]
	return s, found
}

func (m *SessionManager) CloseIfNoSession() bool {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return true
	}

	if len(m.sessions) != 0 {
		return false
	}

	m.closed = true
	return true
}

func (m *SessionManager) Close() error {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	for _, s := range m.sessions {
		Close(s.input)
		Close(s.output)
	}

	m.sessions = nil
	return nil
}

type Session struct {
	input   io.Reader
	output  io.Writer
	ID      uint16
	parent  *SessionManager
	network TargetNetwork
}

func (s *Session) Close() error {
	Close(s.output)
	Close(s.input)
	s.parent.Remove(s.ID)
	return nil
}

// NewReader creates a buf.Reader based on the transfer type of this Session.
func (s *Session) NewOnceReader(reader io.Reader) buf.Reader {
	if s.network == TargetNetworkTCP {
		return NewStreamReader(reader)
	}
	return NewPacketReader(reader)
}
