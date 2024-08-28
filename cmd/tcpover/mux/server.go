package mux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/mux/buf"
)

type pipeReadWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func (p *pipeReadWriteCloser) Close() error {
	e1 := p.ReadCloser.Close()
	e2 := p.WriteCloser.Close()
	if e1 != nil {
		return e1
	}
	if e2 != nil {
		return e2
	}

	return nil
}

type echoDispatcher struct {
}

func (d *echoDispatcher) Dispatch(ctx context.Context, dest Destination) (io.ReadWriteCloser, error) {
	reader, writer := io.Pipe()
	return &pipeReadWriteCloser{reader, writer}, nil
}

type dispatcher struct {
}

func (d *dispatcher) Dispatch(ctx context.Context, dest Destination) (io.ReadWriteCloser, error) {
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

	return conn, nil
}

func NewDispatcher(echo ...bool) Dispatcher {
	if len(echo) > 0 {
		return new(echoDispatcher)
	}
	return new(dispatcher)
}

type Dispatcher interface {
	Dispatch(ctx context.Context, dest Destination) (io.ReadWriteCloser, error)
}

type ServerWorker struct {
	local          io.ReadWriteCloser
	dispatcher     Dispatcher
	sessionManager *SessionManager
}

func NewServerWorker(ctx context.Context, d Dispatcher, conn io.ReadWriteCloser) (*ServerWorker, error) {
	worker := &ServerWorker{
		dispatcher:     d,
		local:          conn,
		sessionManager: NewSessionManager(),
	}
	go worker.run(ctx)
	return worker, nil
}

func (w *ServerWorker) ActiveConnections() uint32 {
	return uint32(w.sessionManager.Size())
}

func (w *ServerWorker) Closed() bool {
	return w.sessionManager.Closed()
}

func (w *ServerWorker) handleStatusNew(ctx context.Context, meta *FrameMetadata, reader *buf.StdReader) error {
	log.Printf("worker: %p, received request for: %v.", w, meta.Target)
	link, err := w.dispatcher.Dispatch(ctx, meta.Target)
	if err != nil {
		return fmt.Errorf("failed to dispatch request: %w", err)
	}

	s := &Session{
		conn:    link,
		parent:  w.sessionManager,
		ID:      meta.SessionID,
		network: meta.Target.Network,
	}
	w.sessionManager.Add(s)

	go func() {
		writer := NewResponseWriter(s.ID, w.local, s.network)
		if err := buf.Copy(buf.NewStdReader(s.conn), writer); err != nil {
			writer.hasError = true
			writer.err = err
		}

		log.Printf("ServerWorker::session %v ends. %v", s.ID, writer.err)
		writer.Close()
		s.Close()
	}()

	if !meta.Option.Has(OptionData) {
		return nil
	}
	if err := buf.Copy(s.NewOnceReader(reader), buf.NewStdWriter(s.conn)); err != nil {
		buf.Copy(reader, buf.Discard)
		return s.Close()
	}

	return nil
}

func (w *ServerWorker) handleStatusKeepAlive(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	if meta.Option.Has(OptionData) {
		err = buf.Copy(NewStreamReader(reader), buf.Discard)
	}
	return err
}

func (w *ServerWorker) handleStatusKeep(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	if !meta.Option.Has(OptionData) {
		return nil
	}

	s, found := w.sessionManager.Get(meta.SessionID)
	if !found {
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, w.local, TargetNetworkTCP)
		closingWriter.Close()
		return buf.Copy(NewStreamReader(reader), buf.Discard)
	}

	err = buf.Copy(s.NewOnceReader(reader), buf.NewStdWriter(s.conn))
	if err != nil {
		log.Printf("ServerWorker::handleStatusKeep read: %v, write: %v, %v", buf.IsReadError(err), buf.IsWriteError(err), err)
	}
	if err != nil && buf.IsWriteError(err) {
		log.Printf("failed to write to downstream writer. closing session: %v, %v", s.ID, err)
		// Notify remote peer to close this session.
		closingWriter := NewResponseWriter(meta.SessionID, w.local, TargetNetworkTCP)
		log.Printf("closingWriter close %v", closingWriter.Close())

		s.Close()
		return err
	}

	return err
}

func (w *ServerWorker) handleStatusEnd(meta *FrameMetadata, reader *buf.StdReader) (err error) {
	if s, found := w.sessionManager.Get(meta.SessionID); found {
		s.Close()
	}
	if meta.Option.Has(OptionData) {
		err = buf.Copy(NewStreamReader(reader), buf.Discard)
	}
	return err
}

func (w *ServerWorker) handleFrame(ctx context.Context, reader *buf.StdReader) error {
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
	reader := buf.NewStdReader(w.local)
	defer w.sessionManager.Close()
	defer w.local.Close()

	for {
		select {
		case <-ctx.Done():
			log.Printf("ServerWorker::run done")
			return
		default:
			err := w.handleFrame(ctx, reader)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.Printf("unexpected EOF: %v", err)
				}
				return
			}
		}
	}
}
