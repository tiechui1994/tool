package buf

import (
	"fmt"
	"io"
)

func copyBuffer(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = writeError{ew}
				break
			}
			if nr != nw {
				err = fmt.Errorf("short buffer")
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = readError{er}
			}
			break
		}
	}
	return written, err
}

// Copy dumps all payload from reader to writer or stops when an error occurs. It returns nil when EOF.
func Copy(reader io.Reader, writer io.Writer) (written int64, err error) {
	buf := New()
	written, err = copyBuffer(writer, reader, buf.v)
	if err != nil && err != io.EOF {
		return written, err
	}
	return written, nil
}

func CopyN(reader io.Reader, writer io.Writer, n int64) (written int64, err error) {
	buf := New()
	written, err = copyBuffer(writer, io.LimitReader(reader, n), buf.v)
	if err != nil && err != io.EOF {
		return written, err
	}
	return written, nil
}

type readError struct {
	error
}

func (e readError) Error() string {
	return e.error.Error()
}

// IsReadError returns true if the error in Copy() comes from reading.
func IsReadError(err error) bool {
	_, ok := err.(readError)
	return ok
}

type writeError struct {
	error
}

func (e writeError) Error() string {
	return e.error.Error()
}

// IsWriteError returns true if the error in Copy() comes from writing.
func IsWriteError(err error) bool {
	_, ok := err.(writeError)
	return ok
}
