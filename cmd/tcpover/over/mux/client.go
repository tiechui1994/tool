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
	go m.pullRemoteOutput()

	return m
}

// N => 1 转发
func (m *ClientWorker) Dispatch(ctx context.Context, conn *Link) bool {
	//if m.IsFull() || m.Closed() {
	//	return false
	//}

	session := m.sessionManager.Allocate()
	if session == nil {
		return false
	}
	session.input = conn.Reader
	session.output = conn.Writer
	go m.pushLocalInput(ctx, session, m.remote.Writer)
	return true
}

// N => 1 转发
func (m *ClientWorker) pushLocalInput(ctx context.Context, s *Session, output io.Writer) {
	dest := ctx.Value("destination").(Destination)
	s.network = dest.Network
	writer := NewWriter(s.ID, dest, output, dest.Network)
	defer s.Close()
	defer writer.Close()

	if err := writeFirstPayload(buf.NewStdReader(s.input), writer); err != nil {
		writer.hasError = true
		writer.err = err
		Interrupt(s.input)
		return
	}

	if err := buf.Copy(buf.NewStdReader(s.input), writer); err != nil {
		writer.hasError = true
		writer.err = err
		Interrupt(s.input)
		return
	}
}

func writeFirstPayload(reader io.Reader, writer *Writer) error {
	return writer.WriteBuffer(&buf.Buffer{})
}

// 1 => N 转发, 读取远程
func (m *ClientWorker) pullRemoteOutput() {
	reader := buf.NewStdReader(m.remote.Reader)
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
		closingWriter := NewResponseWriter(meta.SessionID, m.remote.Writer, meta.Target.Network)
		closingWriter.Close()

		return buf.Copy(NewStreamReader(reader), buf.Discard)
	}

	err = buf.Copy(s.NewReader(reader), buf.NewStdWriter(s.output))
	if err != nil {
		log.Printf("ClientWorker::handleStatusKeep read: %v, write: %v, %v", buf.IsReadError(err), buf.IsWriteError(err), err)
	}
	if err != nil && buf.IsWriteError(err) {
		log.Printf("failed to write to downstream. closing session %v", s.ID)
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, m.remote.Writer, meta.Target.Network)
		closingWriter.Close()

		Interrupt(s.input)
		s.Close()
		return err
	}

	return err
}

func (m *ClientWorker) handleStatusEnd(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	log.Printf("ClientWorker::session [%v] end.", meta.SessionID)
	if s, found := m.sessionManager.Get(meta.SessionID); found {
		if meta.Option.Has(OptionError) {
			Interrupt(s.input)
			Interrupt(s.output)
		}
		s.Close()
	}
	if meta.Option.Has(OptionData) {
		err = buf.Copy(NewStreamReader(reader), buf.Discard)
	}
	return err
}
