package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/sigurn/crc16"
)

// Protocol constants based on Voltgo app analysis
const (
	VersionByte = 0x01

	// Command types
	CmdMultiFrame = 0x64 // 100 - Multi-frame packets
	CmdType03     = 0x03 // Battery status command
	CmdType04     = 0x04
	CmdExtended   = 0x10 // Extended commands (no version prefix)

	// Extended sub-commands
	ExtSubCmd02 = 0x02 // Query config
	ExtSubCmd03 = 0x03 // Query config
	ExtSubCmd04 = 0x04 // Query config (12 bytes data response)
	ExtSubCmd06 = 0x06 // Set config
	ExtSubCmd0A = 0x0A // Set config
	ExtSubCmd0D = 0x0D // Keep-alive/init (no response)

	MinPacketSize = 4 // VER + CMD + CRC16
)

// CRC16 table for MODBUS
var crc16Table = crc16.MakeTable(crc16.CRC16_MODBUS)

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

// Marshal encodes the packet into bytes with CRC16 checksum
// Format: [VER][CMD][DATA_LEN_HIGH][DATA_LEN_LOW][DATA...][CRC16_HIGH][CRC16_LOW]
func (p *Packet) Marshal() []byte {
	dataLen := len(p.Data)
	packetSize := 4 + dataLen + 2 // VER + CMD + LEN(2) + DATA + CRC(2)
	buf := make([]byte, packetSize)

	buf[0] = p.Version
	buf[1] = p.Command
	binary.BigEndian.PutUint16(buf[2:4], uint16(dataLen))
	copy(buf[4:], p.Data)

	// Calculate CRC16 over all bytes except the last 2 CRC bytes
	checksum := crc16.Checksum(buf[:packetSize-2], crc16Table)
	binary.BigEndian.PutUint16(buf[packetSize-2:], checksum)

	return buf
}

// Unmarshal decodes bytes into a packet and verifies CRC16
// This is for non-BLE protocols that include length and CRC fields
func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MinPacketSize {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	// Verify CRC16
	dataLen := len(data)
	expectedCRC := binary.BigEndian.Uint16(data[dataLen-2:])
	actualCRC := crc16.Checksum(data[:dataLen-2], crc16Table)

	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("CRC mismatch: expected 0x%04X, got 0x%04X", expectedCRC, actualCRC)
	}

	packet := &Packet{
		Version: data[0],
		Command: data[1],
	}

	// Check if there's length field (for packets with DATA_LEN field)
	if len(data) > MinPacketSize {
		payloadLen := binary.BigEndian.Uint16(data[2:4])
		if len(data) < int(4+payloadLen+2) {
			return nil, fmt.Errorf("packet data length mismatch")
		}
		packet.Data = make([]byte, payloadLen)
		copy(packet.Data, data[4:4+payloadLen])
	}

	return packet, nil
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

// NewExtendedCommand creates a new extended command
func NewExtendedCommand(subCmd byte, data []byte) *ExtendedCommand {
	return &ExtendedCommand{
		SubCommand: subCmd,
		Data:       data,
	}
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

// NewExtendedSetCommand creates a set extended command (6 bytes)
// Format: 10 XX VV 00 00 00
func NewExtendedSetCommand(subCmd, value byte) *ExtendedCommand {
	return &ExtendedCommand{
		SubCommand: subCmd,
		Data:       []byte{value, 0x00, 0x00, 0x00},
	}
}

// MultiFramePacket represents a multi-frame packet (CMD=0x64)
// Format: [0x01][0x64][LEN_H][LEN_L][FRAME_ID_H][FRAME_ID_L][DATA...][CRC16]
type MultiFramePacket struct {
	FrameID uint16
	Data    []byte
}

// NewMultiFramePacket creates a new multi-frame packet
func NewMultiFramePacket(frameID uint16, data []byte) *MultiFramePacket {
	return &MultiFramePacket{
		FrameID: frameID,
		Data:    data,
	}
}

// Marshal encodes the multi-frame packet
func (m *MultiFramePacket) Marshal() []byte {
	payload := make([]byte, 2+len(m.Data))
	binary.BigEndian.PutUint16(payload[0:2], m.FrameID)
	copy(payload[2:], m.Data)

	packet := NewPacket(CmdMultiFrame, payload)
	return packet.Marshal()
}

// UnmarshalMultiFrame decodes a multi-frame packet
func UnmarshalMultiFrame(packet *Packet) (*MultiFramePacket, error) {
	if packet.Command != CmdMultiFrame {
		return nil, fmt.Errorf("not a multi-frame packet: command=0x%02X", packet.Command)
	}

	if len(packet.Data) < 2 {
		return nil, fmt.Errorf("multi-frame packet too short")
	}

	return &MultiFramePacket{
		FrameID: binary.BigEndian.Uint16(packet.Data[0:2]),
		Data:    packet.Data[2:],
	}, nil
}
