package over

import (
	"errors"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
)

const (
	SocketBufferLength = 16384

	RuleManage    = "manage"
	RuleAgent     = "Agent"
	RuleConnector = "Connector"

	ModeDirect     = "direct"
	ModeForward    = "forward"
	ModeDirectMux  = "directMux"
	ModeForwardMux = "forwardMux"

	CommandLink = 0x01
)

var (
	Debug bool
)

type ControlMessage struct {
	Command uint32
	Data    map[string]interface{}
}

var (
	webSocketCloseCode = []int{
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseProtocolError,
		websocket.CloseUnsupportedData,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
		websocket.CloseInvalidFramePayloadData,
		websocket.CloseInternalServerErr,
		websocket.CloseServiceRestart,
		websocket.CloseTryAgainLater,
	}
)

func isClose(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(*websocket.CloseError); ok {
		return websocket.IsCloseError(err, webSocketCloseCode...)
	}

	if v, ok := err.(syscall.Errno); ok {
		return v.Is(syscall.ECONNABORTED) || v.Is(syscall.ECONNRESET) ||
			v.Is(syscall.ETIMEDOUT) || v.Is(syscall.ECONNREFUSED) ||
			v.Is(syscall.ENETUNREACH) || v.Is(syscall.ENETRESET) ||
			v.Is(syscall.EPIPE)
	}

	if strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "broken pipe") {
		return true
	}

	if errors.Is(err, websocket.ErrCloseSent) {
		return true
	}

	return false
}
