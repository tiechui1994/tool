package mux

import (
	"fmt"
	"io"

	"github.com/tiechui1994/tool/cmd/tcpover/mux/buf"
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
func (r *PacketReader) ReadBuffer(b *buf.Buffer) error {
	if r.eof {
		return io.EOF
	}

	size, err := ReadUint16(r.reader)
	if err != nil {
		return err
	}
	if size > buf.Size {
		return fmt.Errorf("packet size too large: %v", size)
	}

	if _, err := b.ReadFullFrom(r.reader, int32(size)); err != nil {
		b.Release()
		return err
	}
	r.eof = true
	return nil
}

type ChunkStreamReader struct {
	reader io.Reader

	leftOverSize int32
	maxNumChunk  uint32
	numChunk     uint32
}

func NewChunkStreamReader(reader io.Reader, maxNumChunk uint32) *ChunkStreamReader {
	r := &ChunkStreamReader{
		maxNumChunk: maxNumChunk,
		reader:      reader,
	}

	return r
}

func (r *ChunkStreamReader) readSize() (uint16, error) {
	return ReadUint16(r.reader)
}

func (r *ChunkStreamReader) ReadBuffer(b *buf.Buffer) error {
	size := r.leftOverSize
	if size == 0 {
		r.numChunk++
		if r.maxNumChunk > 0 && r.numChunk > r.maxNumChunk {
			return io.EOF
		}
		nextSize, err := r.readSize()
		if err != nil {
			return err
		}
		if nextSize == 0 {
			return io.EOF
		}
		size = int32(nextSize)
	}
	r.leftOverSize = size

	b.Clear()
	buffer := b.Extend(r.leftOverSize)
	n, err := io.ReadFull(r.reader, buffer)
	if n > 0 {
		r.leftOverSize -= int32(n)
		return err
	}
	return err
}

func NewStreamReader(reader io.Reader) *ChunkStreamReader {
	return NewChunkStreamReader(reader, 1)
}
