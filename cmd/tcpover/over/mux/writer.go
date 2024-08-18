package mux

import (
	"errors"
	"io"
	"log"

	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
)

type Writer struct {
	dest     Destination
	writer   io.Writer
	id       uint16
	followup bool
	hasError bool
	err      error
	network  TargetNetwork
}

func NewWriter(id uint16, dest Destination, writer io.Writer, network TargetNetwork) *Writer {
	return &Writer{
		id:       id,
		dest:     dest,
		writer:   writer,
		followup: false,
		network:  network,
	}
}

func NewResponseWriter(id uint16, writer io.Writer, network TargetNetwork) *Writer {
	return &Writer{
		id:       id,
		writer:   writer,
		followup: true,
		network:  network,
	}
}

func (w *Writer) getNextFrameMeta() FrameMetadata {
	meta := FrameMetadata{
		SessionID: w.id,
		Target:    w.dest,
	}

	if w.followup {
		meta.SessionStatus = SessionStatusKeep
	} else {
		w.followup = true
		meta.SessionStatus = SessionStatusNew
	}

	return meta
}

func (w *Writer) writeMetaOnly() (n int, err error) {
	meta := w.getNextFrameMeta()
	frame := buf.New()
	if err := meta.WriteTo(frame); err != nil {
		return 0, err
	}
	n, err = w.writer.Write(frame.Bytes())
	return n, err
}

func (w *Writer) writeMetaWithFrame(data []byte) (n int, err error) {
	meta := w.getNextFrameMeta()
	meta.Option = OptionData

	frame := buf.New()
	if err := meta.WriteTo(frame); err != nil {
		return 0, err
	}
	if n, err = WriteUint16(frame, uint16(len(data))); err != nil {
		return n, err
	}

	if len(data)+1 > 64*1024*1024 {
		return 0, errors.New("value too large")
	}

	n, err = w.writer.Write(frame.Bytes())
	if err != nil {
		return n, err
	}

	n, err = w.writer.Write(data)
	return n, err
}

// WriteMultiBuffer implements buf.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return w.writeMetaOnly()
	}

	return w.writeMetaWithFrame(p)
}

// Close implements common.Closable.
func (w *Writer) Close() error {
	meta := FrameMetadata{
		SessionID:     w.id,
		SessionStatus: SessionStatusEnd,
	}
	if w.hasError {
		meta.Option = OptionError
	}

	stack := callers()
	log.Printf("close: [%v], %v", w.id, w.err)
	log.Printf("close stack: %+v", stack)
	frame := buf.New()
	Must(meta.WriteTo(frame))
	_, err := w.writer.Write(frame.Bytes())
	return err
}
