package mux

import (
	"fmt"
	"io"

	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
)

type PacketReader struct {
	reader io.Reader
	eof    bool
}

// NewPacketReader creates a new PacketReader.
func NewPacketReader(reader io.Reader) *PacketReader {
	return &PacketReader{
		reader: reader,
		eof:    false,
	}
}

// ReadMultiBuffer implements buf.Reader.
func (r *PacketReader) ReadBuffer() (*buf.Buffer, error) {
	if r.eof {
		return nil, io.EOF
	}

	size, err := ReadUint16(r.reader)
	if err != nil {
		return nil, err
	}

	if size > buf.Size {
		return nil, fmt.Errorf("packet size too large: %v", size)
	}

	b := buf.New()
	if _, err := b.ReadFullFrom(r.reader, int32(size)); err != nil {
		b.Release()
		return nil, err
	}
	r.eof = true
	return b, nil
}
