package protocol

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Frames in these tests were captured from a live ZT-25.6V100Ah battery.

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func TestCRC16_KnownVectors(t *testing.T) {
	// CRC of the status request body, as recovered from the Android app's
	// residual frame in the device write buffer
	assert.Equal(t, uint16(0x1484), CRC16(mustHex(t, "010300000029")))
	// CRC of a single-register read, verified against a live response
	assert.Equal(t, uint16(0x0A84), CRC16(mustHex(t, "010300000001")))
}

func TestNewReadRequest_MatchesAppFrame(t *testing.T) {
	// The exact status poll frame the Voltgo Android app sends
	frame := NewReadRequest(DefaultSlaveAddr, 0, 41)
	assert.Equal(t, mustHex(t, "0103000000298414"), frame)
}

func TestVerifyCRC(t *testing.T) {
	frame := mustHex(t, "0103020a7bfec7") // live single-register response
	assert.True(t, VerifyCRC(frame))

	corrupted := append([]byte(nil), frame...)
	corrupted[3] ^= 0xFF
	assert.False(t, VerifyCRC(corrupted))

	assert.False(t, VerifyCRC([]byte{0x01}))
}

func TestParseReadResponse_SingleRegister(t *testing.T) {
	// Live response: read 1 register at 0 -> pack voltage 26.83V
	frame := mustHex(t, "0103020a7bfec7")

	regs, err := ParseReadResponse(frame, DefaultSlaveAddr)
	require.NoError(t, err)
	require.Len(t, regs, 1)
	assert.Equal(t, uint16(2683), regs[0])
}

func TestParseReadResponse_FullStatusBlock(t *testing.T) {
	// Live 87-byte status response (41 registers)
	frame := mustHex(t, "0103520a7b00000d160d180d190d1c0d1f0d1c0d1b0d1b"+
		"00000000000000000000000000000000001b001b001b00630064006400630000"+
		"0000000000000002000015752a001b1b00000000000803e8000000000000bd78")

	regs, err := ParseReadResponse(frame, DefaultSlaveAddr)
	require.NoError(t, err)
	require.Len(t, regs, 41)
	assert.Equal(t, uint16(2683), regs[RegVoltage])
	assert.Equal(t, uint16(3350), regs[RegCellBase])
	assert.Equal(t, uint16(8), regs[RegCellCount])
	assert.Equal(t, uint16(1000), regs[RegFullCapacity])
}

func TestParseReadResponse_Errors(t *testing.T) {
	valid := mustHex(t, "0103020a7bfec7")

	t.Run("short frame", func(t *testing.T) {
		_, err := ParseReadResponse([]byte{0x01, 0x03}, DefaultSlaveAddr)
		assert.ErrorIs(t, err, ErrShortFrame)
	})

	t.Run("bad crc", func(t *testing.T) {
		bad := append([]byte(nil), valid...)
		bad[3] ^= 0xFF
		_, err := ParseReadResponse(bad, DefaultSlaveAddr)
		assert.ErrorIs(t, err, ErrCRCMismatch)
	})

	t.Run("wrong slave", func(t *testing.T) {
		_, err := ParseReadResponse(valid, 0x02)
		assert.ErrorIs(t, err, ErrWrongSlave)
	})

	t.Run("wrong function", func(t *testing.T) {
		frame := AppendCRC(mustHex(t, "0106020a7b"))
		_, err := ParseReadResponse(frame, DefaultSlaveAddr)
		assert.ErrorIs(t, err, ErrWrongFunction)
	})

	t.Run("exception response", func(t *testing.T) {
		frame := AppendCRC(mustHex(t, "018302"))
		_, err := ParseReadResponse(frame, DefaultSlaveAddr)
		var mbErr *ModbusError
		require.ErrorAs(t, err, &mbErr)
		assert.Equal(t, byte(0x03), mbErr.Function)
		assert.Equal(t, byte(0x02), mbErr.Code)
	})

	t.Run("inconsistent byte count", func(t *testing.T) {
		frame := AppendCRC(mustHex(t, "0103040a7b")) // claims 4 bytes, has 2
		_, err := ParseReadResponse(frame, DefaultSlaveAddr)
		assert.Error(t, err)
	})
}
