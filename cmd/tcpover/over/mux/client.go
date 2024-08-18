package mux

import (
	"context"
	"io"
	"log"

	"github.com/pkg/errors"
	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
)

type Link struct {
	Reader io.Reader
	Writer io.Writer
}

type ClientWorker struct {
	sessionManager *SessionManager
	remote         *Link
}

func NewClientWorker(remote *Link) *ClientWorker {
	m := &ClientWorker{
		remote:         remote,
		sessionManager: NewSessionManager(),
	}
	go m.fetchRemoteOutput()

	return m
}

// N => 1 转发
func (m *ClientWorker) Dispatch(ctx context.Context, proxyLink *Link) bool {
	//if m.IsFull() || m.Closed() {
	//	return false
	//}

	session := m.sessionManager.Allocate()
	if session == nil {
		return false
	}
	session.input = proxyLink.Reader
	session.output = proxyLink.Writer
	go m.fetchLocalInput(ctx, session, m.remote.Writer)
	return true
}

// N => 1 转发
func (m *ClientWorker) fetchLocalInput(ctx context.Context, s *Session, output io.Writer) {
	dest := ctx.Value("destination").(Destination)
	s.network = dest.Network
	writer := NewWriter(s.ID, dest, output, dest.Network)
	defer s.Close()
	defer writer.Close()

	if err := writeFirstPayload(s.input, writer); err != nil {
		writer.hasError = true
		writer.err = err
		Interrupt(s.input)
		return
	}

	if _, err := buf.Copy(s.input, writer); err != nil {
		writer.hasError = true
		writer.err = err
		Interrupt(s.input)
		return
	}
}

func writeFirstPayload(reader io.Reader, writer *Writer) error {
	_, err := writer.Write([]byte{})
	return err
}

// 1 => N 转发, 读取远程
func (m *ClientWorker) fetchRemoteOutput() {
	reader := m.remote.Reader
	var meta FrameMetadata
	for {
		err := meta.Unmarshal(reader)
		if err != nil {
			if errors.Cause(err) != io.EOF {
				log.Printf("failed to read metadata: %v", err)
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

func (m *ClientWorker) handleStatueKeepAlive(meta *FrameMetadata, reader io.Reader) (err error) {
	if meta.Option.Has(OptionData) {
		_, err = buf.CopyN(reader, buf.Discard, meta.DataLen)
	}
	return err
}

func (m *ClientWorker) handleStatusNew(meta *FrameMetadata, reader io.Reader) (err error) {
	if meta.Option.Has(OptionData) {
		_, err = buf.CopyN(reader, buf.Discard, meta.DataLen)
	}
	return err
}

func (m *ClientWorker) handleStatusKeep(meta *FrameMetadata, reader io.Reader) (err error) {
	if !meta.Option.Has(OptionData) {
		return nil
	}

	s, found := m.sessionManager.Get(meta.SessionID)
	if !found {
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, m.remote.Writer, meta.Target.Network)
		closingWriter.Close()

		_, err = buf.CopyN(reader, buf.Discard, meta.DataLen)
		return err
	}

	var written int64
	written, err = buf.CopyN(reader, s.output, meta.DataLen)
	if err != nil {
		log.Printf("ClientWorker::handleStatusKeep read: %v, write: %v, %v", buf.IsReadError(err), buf.IsWriteError(err), err)
	}

	if err != nil && buf.IsWriteError(err) {
		log.Printf("failed to write to downstream. closing session %v", s.ID)
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, m.remote.Writer, meta.Target.Network)
		closingWriter.Close()

		_, drainErr := buf.CopyN(reader, buf.Discard, meta.DataLen-written)
		Interrupt(s.input)
		s.Close()
		return drainErr
	}

	return err
}

func (m *ClientWorker) handleStatusEnd(meta *FrameMetadata, reader io.Reader) (err error) {
	log.Printf("session [%v] end [%v]", meta.SessionID, meta.Target)
	if s, found := m.sessionManager.Get(meta.SessionID); found {
		if meta.Option.Has(OptionError) {
			Interrupt(s.input)
			Interrupt(s.output)
		}
		s.Close()
	}
	if meta.Option.Has(OptionData) {
		_, err = buf.CopyN(reader, buf.Discard, meta.DataLen)
	}
	return err
}
