package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Encode/decode symmetry properties. Example-based tests pin the wire format
// to live captures; these pin the encoder and parser to each other.

func TestRoundTrip_ReadResponse(t *testing.T) {
	// 125 registers is the Modbus RTU maximum for one read.
	for _, count := range []int{1, 2, 41, 125} {
		regs := make([]uint16, count)
		for i := range regs {
			regs[i] = uint16(i * 257) // varied values incl. high bytes
		}

		frame := NewReadResponse(DefaultSlaveAddr, regs)
		require.True(t, VerifyCRC(frame), "count=%d", count)

		parsed, err := ParseReadResponse(frame, DefaultSlaveAddr)
		require.NoError(t, err, "count=%d", count)
		assert.Equal(t, regs, parsed, "count=%d", count)
	}
}

func TestRoundTrip_ExceptionResponse(t *testing.T) {
	frame := NewExceptionResponse(DefaultSlaveAddr, FuncReadHoldingRegisters, 0x02)
	require.True(t, VerifyCRC(frame))

	_, err := ParseReadResponse(frame, DefaultSlaveAddr)
	var mbErr *ModbusError
	require.ErrorAs(t, err, &mbErr)
	assert.Equal(t, byte(FuncReadHoldingRegisters), mbErr.Function)
	assert.Equal(t, byte(0x02), mbErr.Code)
}

func TestAppendCRC_AlwaysVerifies(t *testing.T) {
	inputs := [][]byte{
		{0x00},
		{0x01, 0x03},
		{0xFF, 0xFF, 0xFF, 0xFF},
		make([]byte, 200),
	}
	for _, in := range inputs {
		assert.True(t, VerifyCRC(AppendCRC(append([]byte(nil), in...))))
	}
}

func TestNewReadRequest_Structure(t *testing.T) {
	frame := NewReadRequest(DefaultSlaveAddr, 105, 32)
	require.Len(t, frame, 8)
	assert.True(t, VerifyCRC(frame))
	assert.Equal(t, byte(DefaultSlaveAddr), frame[0])
	assert.Equal(t, byte(FuncReadHoldingRegisters), frame[1])
	assert.Equal(t, []byte{0x00, 105, 0x00, 32}, frame[2:6])
}
