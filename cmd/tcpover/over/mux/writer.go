package mux

import (
	"errors"
	"io"

	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
)

type Writer struct {
	dest     Destination
	writer   io.Writer
	id       uint16
	followup bool
	hasError bool
}

func NewWriter(id uint16, dest Destination, writer io.Writer) *Writer {
	return &Writer{
		id:       id,
		dest:     dest,
		writer:   writer,
		followup: false,
	}
}

func NewResponseWriter(id uint16, writer io.Writer) *Writer {
	return &Writer{
		id:       id,
		writer:   writer,
		followup: true,
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

func (w *Writer) writeMetaOnly() error {
	meta := w.getNextFrameMeta()
	b := buf.New()
	if err := meta.WriteTo(b); err != nil {
		return err
	}
	_, err := w.writer.Write(b.Bytes())
	return err
}

func writeMetaWithFrame(writer io.Writer, meta FrameMetadata, data []byte) error {
	frame := buf.New()
	if err := meta.WriteTo(frame); err != nil {
		return err
	}
	if _, err := WriteUint16(frame, uint16(len(data))); err != nil {
		return err
	}

	if len(data)+1 > 64*1024*1024 {
		return errors.New("value too large")
	}

	_, err := writer.Write(append(frame.Bytes(), data...))
	return err
}

func (w *Writer) writeData(data []byte) error {
	meta := w.getNextFrameMeta()
	meta.Option = OptionData

	return writeMetaWithFrame(w.writer, meta, data)
}

// WriteMultiBuffer implements buf.Writer.
func (w *Writer) WriteBuffer(b buf.Buffer) error {
	defer b.Release()

	if b.IsEmpty() {
		return w.writeMetaOnly()
	}

	if !b.IsEmpty() {
		if err := w.writeData(b.Bytes()); err != nil {
			return err
		}
	}

	return nil
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

	frame := buf.New()
	err := meta.WriteTo(frame)
	if err != nil {
		return err
	}

	w.writer.Write(frame.Bytes())
	return nil
}
