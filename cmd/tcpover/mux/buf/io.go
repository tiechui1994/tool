package buf

import (
	"errors"
	"io"
)

type Reader interface {
	ReadBuffer(buf *Buffer) error
}

type Writer interface {
	WriteBuffer(buf *Buffer) error
}

type noOpWriter byte

func (noOpWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (noOpWriter) ReadFrom(reader io.Reader) (int64, error) {
	b := New()
	defer b.Release()

	totalBytes := int64(0)
	for {
		b.Clear()
		_, err := b.ReadFrom(reader)
		totalBytes += int64(b.Len())
		if err != nil {
			if errors.Is(err, io.EOF) {
				return totalBytes, nil
			}
			return totalBytes, err
		}
	}
}

func (noOpWriter) WriteBuffer(buf *Buffer) error {
	defer buf.Clear()
	return nil
}

var (
	Discard = noOpWriter(0)
)

type StdWriter struct {
	writer io.Writer
}

func (b *StdWriter) WriteBuffer(buf *Buffer) error {
	defer buf.Clear()
	_, err := b.writer.Write(buf.Bytes())
	return err
}

func NewStdWriter(writer io.Writer) *StdWriter {
	return &StdWriter{writer: writer}
}

type StdReader struct {
	reader io.Reader
}

func (b *StdReader) Read(p []byte) (n int, err error) {
	return b.reader.Read(p)
}

func (b *StdReader) ReadBuffer(buf *Buffer) error {
	n, err := buf.ReadFrom(b.reader)
	if n > 0 {
		return err
	}
	buf.Release()
	return err
}

func NewStdReader(reader io.Reader) *StdReader {
	return &StdReader{reader: reader}
}
