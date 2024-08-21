package mux

import (
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/tiechui1994/tool/cmd/tcpover/mux/buf"
)

type Writer struct {
	dest     Destination
	writer   io.Writer
	id       uint16
	followup bool
	hasError bool
	err      error
	network  TargetNetwork
	tag      string
}

func NewWriter(id uint16, dest Destination, writer io.Writer, network TargetNetwork) *Writer {
	return &Writer{
		id:       id,
		dest:     dest,
		writer:   writer,
		followup: false,
		network:  network,
		tag:      "===>",
	}
}

func NewResponseWriter(id uint16, writer io.Writer, network TargetNetwork) *Writer {
	return &Writer{
		id:       id,
		writer:   writer,
		followup: true,
		network:  network,
		tag:      "<===",
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

func (w *Writer) writeMetaOnly() (err error) {
	meta := w.getNextFrameMeta()
	frame := buf.New()
	if err := meta.WriteTo(frame); err != nil {
		return err
	}
	_, err = w.writer.Write(frame.Bytes())
	return err
}

func (w *Writer) writeMetaWithFrame(data *buf.Buffer) (err error) {
	meta := w.getNextFrameMeta()
	meta.Option = OptionData

	frame := buf.New()
	defer frame.Release()
	if err := meta.WriteTo(frame); err != nil {
		return err
	}

	if _, err = WriteUint16(frame, uint16(data.Len())); err != nil {
		return err
	}
	if data.Len()+1 > 64*1024*1024 {
		return errors.New("value too large")
	}

	_, err = w.writer.Write(frame.Bytes())
	if err != nil {
		return err
	}

	_, err = w.writer.Write(data.Bytes())
	return err
}

// WriteMultiBuffer implements buf.Writer.
func (w *Writer) WriteBuffer(buf *buf.Buffer) (err error) {
	defer func() {
		if err != nil {
			fmt.Println("WriteBuffer", err)
		}
	}()
	defer buf.Clear()
	if buf.IsEmpty() {
		return w.writeMetaOnly()
	}

	return w.writeMetaWithFrame(buf)
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
