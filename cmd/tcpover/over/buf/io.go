package buf

type Reader interface {
	ReadBuffer(buf *Buffer) error
}

type Writer interface {
	WriteBuffer(buf *Buffer) error
}
