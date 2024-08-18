package mux

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/pkg/errors"
	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
)

type dispatcher struct {
}

func (d *dispatcher) Dispatch(ctx context.Context, dest Destination) (*Link, error) {
	var conn net.Conn
	var err error
	switch dest.Network {
	case TargetNetworkTCP:
		conn, err = net.Dial("tcp", dest.Address)
	case TargetNetworkUDP:
		conn, err = net.Dial("udp", dest.Address)
	default:
		return nil, fmt.Errorf("invalid network")
	}

	if err != nil {
		return nil, err
	}

	return &Link{Reader: conn, Writer: conn}, nil
}

func NewDispatcher() Dispatcher {
	return new(dispatcher)
}

type Dispatcher interface {
	Dispatch(ctx context.Context, dest Destination) (*Link, error)
}

type ServerWorker struct {
	local          *Link
	dispatcher     Dispatcher
	sessionManager *SessionManager
}

func NewServerWorker(ctx context.Context, d Dispatcher, link *Link) (*ServerWorker, error) {
	worker := &ServerWorker{
		dispatcher:     d,
		local:          link,
		sessionManager: NewSessionManager(),
	}
	go worker.run(ctx)
	return worker, nil
}

func handle(ctx context.Context, s *Session, output io.Writer) {
	writer := NewResponseWriter(s.ID, output, s.network)
	if _, err := buf.Copy(s.input, writer); err != nil {
		log.Printf("session %v ends. %v", s.ID, err)
		writer.hasError = true
		writer.err = err
	}

	writer.Close()
	s.Close()
}

func (w *ServerWorker) handleStatusKeepAlive(meta *FrameMetadata, reader io.Reader) (err error) {
	if meta.Option.Has(OptionData) {
		_, err = buf.CopyN(reader, buf.Discard, meta.DataLen)
	}
	return err
}

func (w *ServerWorker) handleStatusNew(ctx context.Context, meta *FrameMetadata, reader io.Reader) error {
	log.Printf("received request for: %v ", meta.Target)
	link, err := w.dispatcher.Dispatch(ctx, meta.Target)
	if err != nil {
		if meta.Option.Has(OptionData) {
			_, _ = buf.CopyN(reader, buf.Discard, meta.DataLen)
		}
		return fmt.Errorf("failed to dispatch request: %w", err)
	}
	s := &Session{
		input:   link.Reader,
		output:  link.Writer,
		parent:  w.sessionManager,
		ID:      meta.SessionID,
		network: meta.Target.Network,
	}

	w.sessionManager.Add(s)
	go handle(ctx, s, w.local.Writer)
	if !meta.Option.Has(OptionData) {
		return nil
	}

	if _, err := buf.CopyN(reader, s.output, meta.DataLen); err != nil {
		buf.Copy(reader, buf.Discard)
		Interrupt(s.input)
		return s.Close()
	}

	return nil
}

func (w *ServerWorker) handleStatusKeep(meta *FrameMetadata, reader io.Reader) (err error) {
	if !meta.Option.Has(OptionData) {
		return nil
	}

	s, found := w.sessionManager.Get(meta.SessionID)
	if !found {
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, w.local.Writer, TargetNetworkTCP)
		closingWriter.Close()
		_, err = buf.CopyN(reader, buf.Discard, meta.DataLen)
		return err
	}

	var written int64
	written, err = buf.CopyN(reader, s.output, meta.DataLen)
	if err != nil {
		log.Printf("ServerWorker::handleStatusKeep read: %v, write: %v, %v", buf.IsReadError(err), buf.IsWriteError(err), err)
	}
	if err != nil && buf.IsWriteError(err) {
		log.Printf("failed to write to downstream writer. closing session: %v, %v", s.ID, err)
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, w.local.Writer, TargetNetworkTCP)
		log.Printf("closingWriter close %v", closingWriter.Close())

		_, drainErr := buf.CopyN(reader, buf.Discard, meta.DataLen-written)
		Interrupt(s.input)
		s.Close()
		return drainErr
	}

	return err
}

func (w *ServerWorker) handleStatusEnd(meta *FrameMetadata, reader io.Reader) (err error) {
	if s, found := w.sessionManager.Get(meta.SessionID); found {
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

func (w *ServerWorker) handleFrame(ctx context.Context, reader io.Reader) error {
	var meta FrameMetadata
	err := meta.Unmarshal(reader)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	switch meta.SessionStatus {
	case SessionStatusKeepAlive:
		err = w.handleStatusKeepAlive(&meta, reader)
	case SessionStatusEnd:
		err = w.handleStatusEnd(&meta, reader)
	case SessionStatusNew:
		err = w.handleStatusNew(ctx, &meta, reader)
	case SessionStatusKeep:
		err = w.handleStatusKeep(&meta, reader)
	default:
		status := meta.SessionStatus
		return fmt.Errorf("unknown status: %v", status)
	}

	if err != nil {
		return fmt.Errorf("failed to process data: %w", err)
	}
	return nil
}

func (w *ServerWorker) run(ctx context.Context) {
	reader := w.local.Reader
	defer w.sessionManager.Close()

	for {
		select {
		case <-ctx.Done():
			log.Printf("ServerWorker::run done")
			return
		default:
			err := w.handleFrame(ctx, reader)
			if err != nil {
				if errors.Cause(err) != io.EOF {
					log.Printf("unexpected EOF: %v", err)
					Interrupt(reader)
				}
				return
			}
		}
	}
}
