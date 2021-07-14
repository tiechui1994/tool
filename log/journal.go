package log

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	socket  = "/run/systemd/journal/stdout"
	addr    = &net.UnixAddr{Name: socket, Net: "unixgram"}
	journal = &Journal{}
)

type Journal struct {
	conn    *net.UnixConn
	connErr error
	once    sync.Once
}

func openUnlinkTempFile() (*os.File, error) {
	f, err := ioutil.TempFile("/dev/shm/", "tmp-journald")
	if err != nil {
		return nil, err
	}

	err = syscall.Unlink(f.Name())
	if err != nil {
		return nil, err
	}

	return f, nil
}

func appendVariable(w io.Writer, name, value string) {
	if strings.ContainsRune(value, '\n') {
		/* When the value contains a newline, we write:
		 * - the variable name, followed by a newline
		 * - the size (in 64bit little endian format)
		 * - the data, followed by a newline
		 */
		fmt.Fprintln(w, name)
		binary.Write(w, binary.LittleEndian, uint64(len(value)))
		fmt.Fprintln(w, value)
	} else {
		/* just write the variable and value all on one line */
		fmt.Fprintf(w, "%s=%s\n", name, value)
	}
}

func (j *Journal) writeMsg(message string) error {
	c, err := j.journalConn1()
	if err != nil {
		return err
	}

	data := new(bytes.Buffer)
	appendVariable(data, "PRIORITY", strconv.Itoa(int(5)))
	appendVariable(data, "MESSAGE", message)

	_, err = io.Copy(j.conn, data)
	if err == nil {
		return nil
	}

	//var errno syscall.Errno
	//switch e := err.(type) {
	//case *net.OpError:
	//	switch e := e.Err.(type) {
	//	case *os.SyscallError:
	//		errno = e.Err.(syscall.Errno)
	//	}
	//}
	//
	//if errno != syscall.EMSGSIZE && errno != syscall.ENOBUFS {
	//	return err
	//}

	f, err := openUnlinkTempFile()
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data.Bytes())
	if err != nil {
		return err
	}

	_, err = writeMsgUnix(c, syscall.UnixRights(int(f.Fd())), addr)
	return err
}

func (j *Journal) journalConn() (*net.UnixConn, error) {
	j.once.Do(func() {
		fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
		if err != nil {
			j.connErr = err
			return
		}

		f := os.NewFile(uintptr(fd), "UNIX Socket")
		defer f.Close()

		fc, err := net.FileConn(f)
		if err != nil {
			j.connErr = err
			return
		}

		uc, ok := fc.(*net.UnixConn)
		if !ok {
			fc.Close()
			j.connErr = errors.New("not a UNIX Conn")
			return
		}

		uc.SetReadBuffer(8 * 1024 * 1024)
		j.conn = uc
	})

	return j.conn, j.connErr
}

func (j *Journal) journalConn1() (*net.UnixConn, error) {
	j.once.Do(func() {
		if _, err := os.Stat(socket); err == nil {
			conn, err := net.Dial("unix", socket)
			if err != nil {
				j.connErr = err
				return
			}

			j.conn = conn.(*net.UnixConn)
			return
		}

		bind, err := net.ResolveUnixAddr("unixgram", "")
		if err != nil {
			j.connErr = err
			return
		}

		conn, err := net.ListenUnixgram("unixgram", bind)
		if err != nil {
			j.connErr = err
			return
		}

		j.conn = conn
	})

	return j.conn, j.connErr
}

func Log(format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)
	err := journal.writeMsg(str)
	fmt.Println(err)
}
