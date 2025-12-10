package protocol

import (
	"fmt"
)

// Protocol constants based on Voltgo app analysis
const (
	VersionByte = 0x01

	// Command types
	CmdType03   = 0x03 // Battery status command
	CmdExtended = 0x10 // Extended commands (no version prefix)

	// Extended sub-commands
	ExtSubCmd0D = 0x0D // Keep-alive/init (no response)
)

// Packet represents a protocol packet
type Packet struct {
	Version byte
	Command byte
	Data    []byte
}

// NewPacket creates a new packet with the given command and data
func NewPacket(command byte, data []byte) *Packet {
	return &Packet{
		Version: VersionByte,
		Command: command,
		Data:    data,
	}
}

// UnmarshalBLE decodes BLE notification data into a packet
// BLE responses use format: [VERSION:1][COMMAND:1][DATA:N] with no length field or CRC
func UnmarshalBLE(data []byte) (*Packet, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("BLE packet too short: %d bytes", len(data))
	}

	packet := &Packet{
		Version: data[0],
		Command: data[1],
	}

	// Everything after version and command is data
	if len(data) > 2 {
		packet.Data = make([]byte, len(data)-2)
		copy(packet.Data, data[2:])
	}

	return packet, nil
}

// ExtendedCommand represents an extended command (0x10) without version prefix
// Format: [0x10][SUBCMD][DATA:3-4]
type ExtendedCommand struct {
	SubCommand byte
	Data       []byte
}

// MarshalBLE encodes the extended command for BLE transmission
// Extended commands do NOT use the version prefix
func (e *ExtendedCommand) MarshalBLE() []byte {
	buf := make([]byte, 2+len(e.Data))
	buf[0] = CmdExtended // 0x10
	buf[1] = e.SubCommand
	copy(buf[2:], e.Data)
	return buf
}

// NewExtendedQueryCommand creates a query extended command (5 bytes)
// Format: 10 XX 00 00 00
func NewExtendedQueryCommand(subCmd byte) *ExtendedCommand {
	return &ExtendedCommand{
		SubCommand: subCmd,
		Data:       []byte{0x00, 0x00, 0x00},
	}
}
