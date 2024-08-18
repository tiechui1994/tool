package mux

import (
	"encoding/binary"
	"io"
)

func ReadUint16(reader io.Reader) (uint16, error) {
	var b [2]byte
	if _, err := io.ReadFull(reader, b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b[:]), nil
}

// WriteUint16 writes an uint16 value into writer.
func WriteUint16(writer io.Writer, value uint16) (int, error) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], value)
	return writer.Write(b[:])
}

// WriteUint64 writes an uint64 value into writer.
func WriteUint64(writer io.Writer, value uint64) (int, error) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], value)
	return writer.Write(b[:])
}

func Interrupt(o interface{}) error {
	if v, ok := o.(io.Closer); ok {
		return v.Close()
	}

	return nil
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func Close(obj interface{}) error {
	if c, ok := obj.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
