package buf

import (
	"io"
)

func copyBuffer(dst Writer, src Reader) (err error) {
	buf := New()
	for {
		er := src.ReadBuffer(buf)
		if !buf.IsEmpty() {
			ew := dst.WriteBuffer(buf)
			if ew != nil {
				err = writeError{ew}
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
	return err
}

// Copy dumps all payload from reader to writer or stops when an error occurs. It returns nil when EOF.
func Copy(reader Reader, writer Writer) (err error) {
	err = copyBuffer(writer, reader)
	if err != nil && err != io.EOF {
		return err
	}
	return nil
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
