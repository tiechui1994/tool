package log

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type Severity int
type Facility int

const (
	kernel Facility = 0
	user   Facility = 1
	system Facility = 3
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

const (
	LOG_SYSLOG  = "/dev/log"
	LOG_SOCKET  = "/run/systemd/journal/socket"
)

var (
	journal = New(LOG_SYSLOG)
)

func init() {
	log.SetFlags(log.Ltime)
}

type Journal struct {
	_type   string
	addr    *net.UnixAddr
	conn    net.Conn
	connErr error
	once    sync.Once
}

func New(_type string) *Journal {
	j := Journal{_type: _type}
	j.connect()
	return &j
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

func syslogVar(w io.Writer, level Severity, msg string) {
	const space = " "
	fmt.Fprintf(w, "<%d>", uint(level)|uint(system)<<3) // PRI
	fmt.Fprintf(w, "%s", "tool:")                       // SYSLOG_IDENTIFIER
	fmt.Fprintf(w, space)
	fmt.Fprintln(w, msg+string([]byte{0})) // MSG
}

func socketVar(w io.Writer, name, value string) {
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

func consoleVar(w io.Writer, level int, msg string) {
	tags := map[int]string{
		ALERT:   "A",
		CRIT:    "C",
		ERR:     "E",
		WARNING: "W",
		NOTICE:  "N",
		INFO:    "I",
		DEBUG:   "D",
	}
	fmt.Fprintf(w, "%v", "tool")
	fmt.Fprintf(w, " ")
	fmt.Fprintf(w, "[%v]", tags[level])
	fmt.Fprintf(w, " ")
	fmt.Fprintf(w, msg) // MSG
}

func (j *Journal) writeMsg(level int, message string) error {
	c, err := j.connect()
	if err != nil {
		return err
	}

	conn := c.(*net.UnixConn)
	data := new(bytes.Buffer)
	if j._type == LOG_SOCKET {
		socketVar(data, "PRIORITY", strconv.Itoa(level))
		socketVar(data, "SYSLOG_IDENTIFIER", "tool")
		socketVar(data, "MESSAGE", message)
	} else if j._type == LOG_SYSLOG {
		syslogVar(data, Severity(level), message)
	} else {
		consoleVar(data, level, message)
		log.Println(data.String())
		return nil
	}

	_, err = conn.WriteToUnix(data.Bytes(), j.addr)
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

	_, err = writeMsgUnix(c.(*net.UnixConn), syscall.UnixRights(int(f.Fd())), j.addr)
	return err
}

func (j *Journal) connect() (net.Conn, error) {
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
		conn.SetWriteBuffer(8 * 1024 * 1024)
		j.conn = conn
		j.addr = &net.UnixAddr{Net: "unixgram", Name: j._type}
	})

	return j.conn, j.connErr
}

func Log(level int, format string, args ...interface{}) error {
	str := fmt.Sprintf(format, args...)
	return journal.writeMsg(level, str)
}
