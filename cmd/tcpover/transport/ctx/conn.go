package ctx

import (
	"fmt"
	"net"
)

type ConnContext interface {
	Metadata() *Metadata
	Conn() net.Conn
}

const (
	HTTP Type = iota
	HTTPCONNECT
	SOCKS5
)

type Type int

func (t Type) String() string {
	switch t {
	case HTTP:
		return "HTTP"
	case HTTPCONNECT:
		return "HTTP Connect"
	case SOCKS5:
		return "Socks5"
	default:
		return "Unknown"
	}
}

type Metadata struct {
	NetWork string `json:"network"`
	Type    Type   `json:"type"`
	SrcIP   net.IP `json:"sourceIP"`
	DstIP   net.IP `json:"destinationIP"`
	SrcPort uint16 `json:"sourcePort"`
	DstPort uint16 `json:"destinationPort"`
	Host    string `json:"host"`
	Origin  string `json:"origin"`
}

func (m *Metadata) RemoteAddress() string {
	return net.JoinHostPort(m.String(), fmt.Sprintf("%v", m.DstPort))
}

func (m *Metadata) SourceAddress() string {
	return net.JoinHostPort(m.SrcIP.String(), fmt.Sprintf("%v", m.SrcPort))
}

func (m *Metadata) String() string {
	if m.Host != "" {
		return m.Host
	} else if m.DstIP != nil {
		return m.DstIP.String()
	} else {
		return "<nil>"
	}
}

func (m *Metadata) Valid() bool {
	return m.Host != "" || m.DstIP != nil
}

type connContext struct {
	metadata *Metadata
	conn     net.Conn
}

func (c *connContext) Metadata() *Metadata {
	return c.metadata
}

func (c *connContext) Conn() net.Conn {
	return c.conn
}

func NewConnContext(conn net.Conn, metadata *Metadata) ConnContext {
	return &connContext{
		metadata: metadata,
		conn:     conn,
	}
}
