package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalBLE_ValidPacketWithData(t *testing.T) {
	data := []byte{0x01, 0x03, 0xAA, 0xBB, 0xCC}

	packet, err := UnmarshalBLE(data)
	require.NoError(t, err)

	assert.Equal(t, byte(0x01), packet.Version)
	assert.Equal(t, byte(0x03), packet.Command)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, packet.Data)
}

func TestUnmarshalBLE_MinimumValidPacket(t *testing.T) {
	// Just version and command, no data
	data := []byte{0x01, 0x03}

	packet, err := UnmarshalBLE(data)
	require.NoError(t, err)

	assert.Equal(t, byte(0x01), packet.Version)
	assert.Equal(t, byte(0x03), packet.Command)
	assert.Nil(t, packet.Data)
}

func TestUnmarshalBLE_ShortPacket(t *testing.T) {
	data := []byte{0x01} // Only 1 byte

	_, err := UnmarshalBLE(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestUnmarshalBLE_EmptyPacket(t *testing.T) {
	data := []byte{}

	_, err := UnmarshalBLE(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestNewPacket(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00}
	packet := NewPacket(0x03, data)

	assert.Equal(t, byte(VersionByte), packet.Version)
	assert.Equal(t, byte(0x03), packet.Command)
	assert.Equal(t, data, packet.Data)
}

func TestNewPacket_NilData(t *testing.T) {
	packet := NewPacket(0x03, nil)

	assert.Equal(t, byte(VersionByte), packet.Version)
	assert.Equal(t, byte(0x03), packet.Command)
	assert.Nil(t, packet.Data)
}

func TestNewExtendedQueryCommand(t *testing.T) {
	cmd := NewExtendedQueryCommand(ExtSubCmd0D)

	assert.Equal(t, byte(ExtSubCmd0D), cmd.SubCommand)
	assert.Equal(t, []byte{0x00, 0x00, 0x00}, cmd.Data)
}

func TestExtendedCommand_MarshalBLE(t *testing.T) {
	cmd := &ExtendedCommand{
		SubCommand: ExtSubCmd0D,
		Data:       []byte{0x00, 0x00, 0x00},
	}

	result := cmd.MarshalBLE()

	expected := []byte{CmdExtended, ExtSubCmd0D, 0x00, 0x00, 0x00}
	assert.Equal(t, expected, result)
}

func TestExtendedCommand_MarshalBLE_EmptyData(t *testing.T) {
	cmd := &ExtendedCommand{
		SubCommand: 0x05,
		Data:       []byte{},
	}

	result := cmd.MarshalBLE()

	expected := []byte{CmdExtended, 0x05}
	assert.Equal(t, expected, result)
}

func TestExtendedCommand_MarshalBLE_LongerData(t *testing.T) {
	cmd := &ExtendedCommand{
		SubCommand: 0x05,
		Data:       []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	}

	result := cmd.MarshalBLE()

	expected := []byte{CmdExtended, 0x05, 0x01, 0x02, 0x03, 0x04, 0x05}
	assert.Equal(t, expected, result)
}

func TestConstants(t *testing.T) {
	// Verify protocol constants match expected values
	assert.Equal(t, 0x01, VersionByte)
	assert.Equal(t, 0x03, CmdType03)
	assert.Equal(t, 0x10, CmdExtended)
	assert.Equal(t, 0x0D, ExtSubCmd0D)
}
