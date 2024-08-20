package mux

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/tiechui1994/tool/cmd/tcpover/over/buf"
)

type SessionStatus byte

const (
	SessionStatusNew       SessionStatus = 0x01
	SessionStatusKeep      SessionStatus = 0x02
	SessionStatusEnd       SessionStatus = 0x03
	SessionStatusKeepAlive SessionStatus = 0x04
)

func (v SessionStatus) String() string {
	switch v {
	case SessionStatusNew:
		return "SessionStatusNew"
	case SessionStatusKeep:
		return "SessionStatusKeep"
	case SessionStatusEnd:
		return "SessionStatusEnd"
	case SessionStatusKeepAlive:
		return "SessionStatusKeepAlive"
	default:
		return "Unknown"
	}
}

const (
	OptionData  Byte = 0x01
	OptionError Byte = 0x02
)

type TargetNetwork byte

const (
	TargetNetworkUnknown TargetNetwork = 0x00
	TargetNetworkTCP     TargetNetwork = 0x01
	TargetNetworkUDP     TargetNetwork = 0x02
)

/*
Frame format
2 bytes - length
2 bytes - session id
1 bytes - status
1 bytes - option

1 byte - network
N bytes - address

*/

type Byte byte

func (b Byte) Has(bb Byte) bool {
	return (b & bb) != 0
}

type Destination struct {
	Network TargetNetwork
	Address string
}

type FrameMetadata struct {
	SessionID     uint16
	SessionStatus SessionStatus
	Option        Byte

	Target Destination
}

func (f FrameMetadata) WriteTo(b *buf.Buffer) error {
	lenBytes := b.Extend(2)

	len0 := b.Len()
	sessionBytes := b.Extend(2)
	binary.BigEndian.PutUint16(sessionBytes, f.SessionID)

	b.WriteByte(byte(f.SessionStatus))
	b.WriteByte(byte(f.Option))

	if f.SessionStatus == SessionStatusNew {
		b.WriteByte(byte(f.Target.Network))
		b.WriteString(f.Target.Address)
	}

	len1 := b.Len()
	binary.BigEndian.PutUint16(lenBytes, uint16(len1-len0))
	return nil
}

// Unmarshal reads FrameMetadata from the given reader.
func (f *FrameMetadata) Unmarshal(reader io.Reader) error {
	metaLen, err := ReadUint16(reader)
	if err != nil {
		return err
	}
	if metaLen > 512 {
		return fmt.Errorf("invalid metalen %v", metaLen)
	}

	b := buf.New()
	defer b.Release()

	if _, err := b.ReadFullFrom(reader, int32(metaLen)); err != nil {
		return err
	}

	return f.UnmarshalFromBuffer(b)
}

// UnmarshalFromBuffer reads a FrameMetadata from the given buffer.
// Visible for testing only.
func (f *FrameMetadata) UnmarshalFromBuffer(b *buf.Buffer) error {
	if b.Len() < 4 {
		return fmt.Errorf("insufficient buffer: %v", b.Len())
	}

	f.SessionID = binary.BigEndian.Uint16(b.BytesTo(2))
	f.SessionStatus = SessionStatus(b.Byte(2))
	f.Option = Byte(b.Byte(3))
	f.Target.Network = TargetNetworkUnknown

	if f.SessionStatus == SessionStatusNew {
		if b.Len() < 8 {
			return fmt.Errorf("insufficient buffer: %v", b.Len())
		}
		network := TargetNetwork(b.Byte(4))
		b.Advance(5)
		addr := b.Bytes()

		switch network {
		case TargetNetworkTCP:
			f.Target = Destination{Address: string(addr), Network: TargetNetworkTCP}
		case TargetNetworkUDP:
			f.Target = Destination{Address: string(addr), Network: TargetNetworkUDP}
		default:
			return fmt.Errorf("unknown network type: %v", network)
		}
	}
	return nil
}
