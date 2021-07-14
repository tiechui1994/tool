package log

import (
	"bytes"
	"encoding/binary"
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

const (
	EMERG = iota
	ALERT
	CRIT
	ERR
	WARNING
	NOTICE
	INFO
	DEBUG
)

var (
	socket  = "/run/systemd/journal/socket"
	syslog  = "/run/systemd/journal/syslog"
	addr    = &net.UnixAddr{Name: socket, Net: "unixgram"}
	journal = &Journal{}
)

type Journal struct {
	conn    *net.UnixConn
	connErr error
	once    sync.Once
}

func openUnlinkTempFile() (*os.File, error) {
	f, err := ioutil.TempFile("/dev/shm/", "journal.XXXXX")
	if err != nil {
		return nil, err
	}

	err = syscall.Unlink(f.Name())
	if err != nil {
		return nil, err
	}

	return f, nil
}

func appendVar(w io.Writer, name, value string) {
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

func (j *Journal) writeMsg(level int, message string) error {
	c, err := j.journalConn()
	if err != nil {
		return err
	}

	data := new(bytes.Buffer)
	appendVar(data, "PRIORITY", strconv.Itoa(level))
	appendVar(data, "UNIT", "tool")
	appendVar(data, "SYSLOG_IDENTIFIER", "systemd")
	appendVar(data, "MESSAGE", message)

	_, err = io.Copy(j.conn, data)
	if err == nil {
		return nil
	}

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

		conn.SetReadBuffer(8 * 1024 * 1024)
		j.conn = conn
	})

	return j.conn, j.connErr
}

func Log(level int, format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)
	journal.writeMsg(level, str)
}
