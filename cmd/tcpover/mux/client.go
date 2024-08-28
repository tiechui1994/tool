package mux

import (
	"errors"
	"io"
	"log"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/mux/buf"
)

type ClientStrategy struct {
	MaxConcurrency uint32
	MaxConnection  uint32
}

type ClientWorker struct {
	sessionManager *SessionManager
	remote         io.ReadWriteCloser
	limit          ClientStrategy
	done           chan struct{}
}

func NewClientWorker(remote io.ReadWriteCloser) *ClientWorker {
	m := &ClientWorker{
		remote:         remote,
		sessionManager: NewSessionManager(),
		limit: ClientStrategy{
			MaxConnection:  16,
			MaxConcurrency: 16,
		},
		done: make(chan struct{}),
	}
	go m.pullRemoteOutput()
	go m.monitor()

	return m
}

func (m *ClientWorker) IsClosing() bool {
	sm := m.sessionManager
	if m.limit.MaxConnection > 0 && sm.Count() >= int(m.limit.MaxConnection) {
		return true
	}
	return false
}

func (m *ClientWorker) IsFull() bool {
	if m.IsClosing() || m.Closed() {
		return true
	}

	sm := m.sessionManager
	if m.limit.MaxConcurrency > 0 && sm.Size() >= int(m.limit.MaxConcurrency) {
		return true
	}
	return false
}

func (m *ClientWorker) Closed() bool {
	select {
	case <-m.done:
		return true
	default:
		return false
	}
}

// forward conn by local
func (m *ClientWorker) Dispatch(destination Destination, conn io.ReadWriteCloser) bool {
	if m.IsFull() {
		return false
	}

	session := m.sessionManager.Allocate()
	if session == nil {
		return false
	}

	session.conn = conn
	go m.pushLocalInput(destination, session, m.remote)
	return true
}

// write data to remote
func (m *ClientWorker) pushLocalInput(destination Destination, s *Session, output io.Writer) {
	s.network = destination.Network
	writer := NewWriter(s.ID, destination, output, destination.Network)
	defer writer.Close()
	defer s.Close()

	if err := writeFirstPayload(buf.NewStdReader(s.conn), writer); err != nil {
		writer.hasError = true
		writer.err = err
		return
	}

	if err := buf.Copy(buf.NewStdReader(s.conn), writer); err != nil {
		writer.hasError = true
		writer.err = err
		return
	}
}

func writeFirstPayload(reader io.Reader, writer *Writer) error {
	return writer.WriteBuffer(&buf.Buffer{})
}

func (m *ClientWorker) monitor() {
	timer := time.NewTicker(time.Minute * 30)
	defer timer.Stop()

	for {
		select {
		case <-m.done:
			m.sessionManager.Close()
			m.remote.Close()
			return
		case <-timer.C:
			size := m.sessionManager.Size()
			if size == 0 && m.sessionManager.CloseIfNoSession() {
				close(m.done)
			}
		}
	}
}

// read data from remote
func (m *ClientWorker) pullRemoteOutput() {
	defer func() {
		close(m.done)
	}()

	reader := buf.NewStdReader(m.remote)
	var meta FrameMetadata
	for {
		err := meta.Unmarshal(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("failed to read remote: %v", err)
			}
			break
		}

		switch meta.SessionStatus {
		case SessionStatusKeepAlive:
			err = m.handleStatueKeepAlive(&meta, reader)
		case SessionStatusEnd:
			err = m.handleStatusEnd(&meta, reader)
		case SessionStatusNew:
			err = m.handleStatusNew(&meta, reader)
		case SessionStatusKeep:
			err = m.handleStatusKeep(&meta, reader)
		default:
			status := meta.SessionStatus
			log.Printf("unknown status: %v", status)
			return
		}

		if err != nil {
			log.Printf("failed to process data")
			return
		}
	}
}

func (m *ClientWorker) handleStatueKeepAlive(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	if meta.Option.Has(OptionData) {
		err = buf.Copy(NewStreamReader(reader), buf.Discard)
	}
	return err
}

func (m *ClientWorker) handleStatusNew(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	if meta.Option.Has(OptionData) {
		err = buf.Copy(NewStreamReader(reader), buf.Discard)
	}
	return err
}

func (m *ClientWorker) handleStatusKeep(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	if !meta.Option.Has(OptionData) {
		return nil
	}

	s, found := m.sessionManager.Get(meta.SessionID)
	if !found {
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, m.remote, meta.Target.Network)
		closingWriter.Close()

		return buf.Copy(NewStreamReader(reader), buf.Discard)
	}

	err = buf.Copy(s.NewOnceReader(reader), buf.NewStdWriter(s.conn))
	if err != nil {
		log.Printf("ClientWorker::handleStatusKeep read: %v, write: %v, %v", buf.IsReadError(err), buf.IsWriteError(err), err)
	}
	if err != nil && buf.IsWriteError(err) {
		log.Printf("failed to write to downstream. closing session %v", s.ID)
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, m.remote, meta.Target.Network)
		closingWriter.Close()

		s.Close()
		return err
	}

	return err
}

func (m *ClientWorker) handleStatusEnd(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	log.Printf("ClientWorker::session [%v] end.", meta.SessionID)
	if s, found := m.sessionManager.Get(meta.SessionID); found {
		s.Close()
	}
	if meta.Option.Has(OptionData) {
		err = buf.Copy(NewStreamReader(reader), buf.Discard)
	}
	return err
}
