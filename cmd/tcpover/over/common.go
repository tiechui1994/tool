package over

const (
	SocketBufferLength = 16384

	RuleManage    = "manage"
	RuleAgent     = "Agent"
	RuleConnector = "Connector"

	CommandLink = 0x01
)

var (
	Debug bool
)

type ControlMessage struct {
	Command uint32
	Data    map[string]interface{}
}
