package bufio

import (
	"io"
	"net"
	"time"
)

func Relay(leftConn, rightConn net.Conn) {
	ch := make(chan error)

	go func() {
		_, err := io.Copy(leftConn, rightConn)
		_ = leftConn.SetReadDeadline(time.Now())
		ch <- err
	}()

	_, _ = io.Copy(rightConn, leftConn)
	_ = rightConn.SetReadDeadline(time.Now())
	<-ch
}

