package mux

import (
	"errors"
	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
	"io"
)

type Writer struct {
	dest         Destination
	writer       io.Writer
	id           uint16
	followup     bool
	hasError     bool
}

func NewWriter(id uint16, dest Destination, writer io.Writer) *Writer {
	return &Writer{
		id:           id,
		dest:         dest,
		writer:       writer,
		followup:     false,
	}
}

func NewResponseWriter(id uint16, writer io.Writer, transferType protocol.TransferType) *Writer {
	return &Writer{
		id:           id,
		writer:       writer,
		followup:     true,
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

func writeMetaWithFrame(writer io.Writer, meta FrameMetadata, data buf.Buffer) error {
	frame := buf.New()
	if err := meta.WriteTo(frame); err != nil {
		return err
	}
	if _, err := WriteUint16(frame, uint16(data.Len())); err != nil {
		return err
	}

	if data.Len()+1 > 64*1024*1024 {
		return errors.New("value too large")
	}

	//sliceSize := len(data) + 1
	//mb2 := make(buf.MultiBuffer, 0, sliceSize)
	//mb2 = append(mb2, frame)
	//mb2 = append(mb2, data...)
	//return writer.WriteMultiBuffer(mb2)
}

func (w *Writer) writeData(mb buf.MultiBuffer) error {
	meta := w.getNextFrameMeta()
	meta.Option = OptionData

	return writeMetaWithFrame(w.writer, meta, mb)
}

// WriteMultiBuffer implements buf.Writer.
func (w *Writer) WriteMultiBuffer(mb buf.MultiBuffer) error {
	defer buf.ReleaseMulti(mb)

	if mb.IsEmpty() {
		return w.writeMetaOnly()
	}

	for !mb.IsEmpty() {
		var chunk buf.MultiBuffer
		if w.transferType == protocol.TransferTypeStream {
			mb, chunk = buf.SplitSize(mb, 8*1024)
		} else {
			mb2, b := buf.SplitFirst(mb)
			mb = mb2
			chunk = buf.MultiBuffer{b}
		}
		if err := w.writeData(chunk); err != nil {
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

	w.writer.WriteMultiBuffer(buf.MultiBuffer{frame})
	return nil
}
