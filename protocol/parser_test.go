package protocol

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestPacket creates a valid BMS packet with specified values.
// Returns a packet of at least minPacket0Length (74 bytes).
func buildTestPacket(opts testPacketOpts) []byte {
	size := minPacket0Length
	if opts.fullLength {
		size = fullPacket0Length
	}
	packet := make([]byte, size)

	// Header at 0x00 (4 bytes)
	copy(packet[offsetHeader:], []byte{0x52, 0x00, 0x00, 0x00})

	// Voltage at 0x04 (uint32 LE, value * 100)
	binary.LittleEndian.PutUint32(packet[offsetVoltage:], uint32(opts.voltage*100))

	// Current at 0x08 (uint32 LE, value * 10, handle negative)
	var rawCurrent uint32
	if opts.current >= 0 {
		rawCurrent = uint32(opts.current * 10)
	} else {
		// Negative current: add 6553.6 * 10 = 65536
		rawCurrent = uint32((opts.current + 6553.6) * 10)
	}
	binary.LittleEndian.PutUint32(packet[offsetCurrent:], rawCurrent)

	// Cell voltages at 0x0C (up to 16 cells, uint16 LE each, value * 1000)
	for i, v := range opts.cellVoltages {
		if i >= 16 {
			break
		}
		offset := offsetCellVoltages + (i * 2)
		binary.LittleEndian.PutUint16(packet[offset:], uint16(v*1000))
	}

	// SOC at 0x25 (1 byte)
	packet[offsetSOC] = byte(opts.soc)

	// SOH at 0x28 (uint16 LE)
	binary.LittleEndian.PutUint16(packet[offsetSOH:], uint16(opts.soh))

	// Status flags at 0x2E (uint16 LE)
	binary.LittleEndian.PutUint16(packet[offsetStatusFlags:], opts.statusFlags)

	// Protection flags at 0x30 (uint16 LE)
	binary.LittleEndian.PutUint16(packet[offsetProtectFlags:], opts.protectionFlags)

	// Heating status at 0x32 (1 byte, 0x80 = active)
	if opts.heatingActive {
		packet[offsetHeatingStatus] = heatingActiveValue
	}

	// Warning flags at 0x36 (uint16 LE)
	binary.LittleEndian.PutUint16(packet[offsetWarningFlags:], opts.warningFlags)

	// Cell temperatures at 0x42 (4 signed bytes)
	for i, t := range opts.cellTemps {
		if i >= 4 {
			break
		}
		packet[offsetCellTemps+i] = byte(t)
	}

	// Cell count at 0x48 (uint16 LE)
	binary.LittleEndian.PutUint16(packet[offsetCellCount:], uint16(opts.cellCount))

	// Heating switch at 0x50 (uint16 LE, only in full length packets)
	if opts.fullLength {
		var heatingSwitch uint16
		if opts.heatingSwitchOn {
			heatingSwitch = heatingSwitchBit
		}
		binary.LittleEndian.PutUint16(packet[offsetHeatingSwitch:], heatingSwitch)
	}

	return packet
}

type testPacketOpts struct {
	voltage         float32
	current         float32
	cellVoltages    []float32
	cellCount       int
	soc             int
	soh             int
	statusFlags     uint16
	protectionFlags uint16
	warningFlags    uint16
	heatingActive   bool
	heatingSwitchOn bool
	cellTemps       []int8
	fullLength      bool
}

func TestParseBMSInfoResponse_ValidPacket(t *testing.T) {
	opts := testPacketOpts{
		voltage:      51.20,
		current:      5.5,
		cellVoltages: []float32{3.200, 3.210, 3.205, 3.198, 3.201, 3.199, 3.203, 3.207, 3.200, 3.210, 3.205, 3.198, 3.201, 3.199, 3.203, 3.207},
		cellCount:    16,
		soc:          85,
		soh:          100,
		cellTemps:    []int8{25, 26, 24, 25},
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	assert.InDelta(t, 51.20, float64(info.Voltage), 0.01)
	assert.InDelta(t, 5.5, float64(info.Current), 0.1)
	assert.Equal(t, 16, info.CellCount)
	assert.Len(t, info.CellVoltages, 16)
	assert.InDelta(t, 3.200, float64(info.CellVoltages[0]), 0.001)
	assert.InDelta(t, 3.210, float64(info.CellVoltages[1]), 0.001)
	assert.Equal(t, 85, info.SOC)
	assert.Equal(t, 100, info.SOH)
	assert.Equal(t, []int8{25, 26, 24, 25}, info.CellTemperatures)
}

func TestParseBMSInfoResponse_DischargingCurrent(t *testing.T) {
	// Test negative current (discharging)
	// The formula: if currentFloat > 3276.8, subtract 6553.6
	opts := testPacketOpts{
		voltage:      51.20,
		current:      -10.5, // discharging at 10.5A
		cellVoltages: make([]float32, 16),
		cellCount:    16,
		cellTemps:    []int8{25, 25, 25, 25},
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	assert.InDelta(t, -10.5, float64(info.Current), 0.1)
}

func TestParseBMSInfoResponse_ChargingCurrent(t *testing.T) {
	opts := testPacketOpts{
		voltage:      51.20,
		current:      25.0, // charging at 25A
		cellVoltages: make([]float32, 16),
		cellCount:    16,
		cellTemps:    []int8{25, 25, 25, 25},
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	assert.InDelta(t, 25.0, float64(info.Current), 0.1)
}

func TestParseBMSInfoResponse_HeatingStatus(t *testing.T) {
	opts := testPacketOpts{
		voltage:       51.20,
		cellVoltages:  make([]float32, 16),
		cellCount:     16,
		cellTemps:     []int8{-5, -5, -5, -5}, // Cold temps
		heatingActive: true,
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	assert.True(t, info.HeatingActive)
	assert.Equal(t, []int8{-5, -5, -5, -5}, info.CellTemperatures)
}

func TestParseBMSInfoResponse_HeatingSwitchOn(t *testing.T) {
	opts := testPacketOpts{
		voltage:         51.20,
		cellVoltages:    make([]float32, 16),
		cellCount:       16,
		cellTemps:       []int8{25, 25, 25, 25},
		heatingSwitchOn: true,
		fullLength:      true, // Need full length for heating switch
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	assert.True(t, info.HeatingSwitchOn)
}

func TestParseBMSInfoResponse_EmptyPackets(t *testing.T) {
	_, err := ParseBMSInfoResponse([][]byte{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no packets provided")
}

func TestParseBMSInfoResponse_ShortPacket(t *testing.T) {
	shortPacket := make([]byte, minPacket0Length-1) // One byte too short
	_, err := ParseBMSInfoResponse([][]byte{shortPacket})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestParseBMSInfoResponse_CellCountExceedsMax(t *testing.T) {
	opts := testPacketOpts{
		voltage:      51.20,
		cellVoltages: make([]float32, 16),
		cellCount:    501, // Exceeds maxCellCount (500)
		cellTemps:    []int8{25, 25, 25, 25},
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	// Should default to 16 when exceeds max
	assert.Equal(t, defaultCellCount, info.CellCount)
}

func TestParseBMSInfoResponse_ProtectionFlags(t *testing.T) {
	opts := testPacketOpts{
		voltage:         51.20,
		cellVoltages:    make([]float32, 16),
		cellCount:       16,
		cellTemps:       []int8{25, 25, 25, 25},
		protectionFlags: 0x0005, // OverVoltage + OverCurrent
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2
	}
	packet := buildTestPacket(opts)

	info, err := ParseBMSInfoResponse([][]byte{packet})
	require.NoError(t, err)

	assert.Equal(t, uint16(0x0005), info.ProtectionFlags)
}

func TestParseBMSInfoResponse_MultiPacket(t *testing.T) {
	// Test a 32-cell battery (requires 2 packets)
	opts := testPacketOpts{
		voltage:      102.4, // 32 cells * 3.2V
		cellVoltages: make([]float32, 16),
		cellCount:    32,
		soc:          90,
		cellTemps:    []int8{25, 26, 24, 25},
	}
	for i := range opts.cellVoltages {
		opts.cellVoltages[i] = 3.2 + float32(i)*0.001 // Slight variation
	}
	packet0 := buildTestPacket(opts)

	// Second packet contains cells 16-31
	packet1 := make([]byte, 32) // 16 cells * 2 bytes each
	for i := 0; i < 16; i++ {
		voltage := uint16((3.2 + float32(16+i)*0.001) * 1000)
		binary.LittleEndian.PutUint16(packet1[i*2:], voltage)
	}

	info, err := ParseBMSInfoResponse([][]byte{packet0, packet1})
	require.NoError(t, err)

	assert.Equal(t, 32, info.CellCount)
	assert.Len(t, info.CellVoltages, 32)
	// Check first cell from packet 0
	assert.InDelta(t, 3.200, float64(info.CellVoltages[0]), 0.001)
	// Check first cell from packet 1 (cell 16)
	assert.InDelta(t, 3.216, float64(info.CellVoltages[16]), 0.001)
}

func TestParseProtectionFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    uint16
		expected ProtectionStatus
	}{
		{
			name:     "no flags",
			flags:    0x0000,
			expected: ProtectionStatus{},
		},
		{
			name:  "all flags",
			flags: 0x00FF,
			expected: ProtectionStatus{
				OverVoltage:          true,
				UnderVoltage:         true,
				OverCurrent:          true,
				OverTemperature:      true,
				UnderTemperature:     true,
				ShortCircuit:         true,
				DischargeOverCurrent: true,
				ChargeOverCurrent:    true,
			},
		},
		{
			name:  "over voltage only",
			flags: 0x0001,
			expected: ProtectionStatus{
				OverVoltage: true,
			},
		},
		{
			name:  "under voltage only",
			flags: 0x0002,
			expected: ProtectionStatus{
				UnderVoltage: true,
			},
		},
		{
			name:  "over current only",
			flags: 0x0004,
			expected: ProtectionStatus{
				OverCurrent: true,
			},
		},
		{
			name:  "over temperature only",
			flags: 0x0008,
			expected: ProtectionStatus{
				OverTemperature: true,
			},
		},
		{
			name:  "under temperature only",
			flags: 0x0010,
			expected: ProtectionStatus{
				UnderTemperature: true,
			},
		},
		{
			name:  "short circuit only",
			flags: 0x0020,
			expected: ProtectionStatus{
				ShortCircuit: true,
			},
		},
		{
			name:  "discharge over current only",
			flags: 0x0040,
			expected: ProtectionStatus{
				DischargeOverCurrent: true,
			},
		},
		{
			name:  "charge over current only",
			flags: 0x0080,
			expected: ProtectionStatus{
				ChargeOverCurrent: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProtectionFlags(tt.flags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{-1, 0, -1},
		{0, -1, -1},
	}

	for _, tt := range tests {
		result := minInt(tt.a, tt.b)
		assert.Equal(t, tt.expected, result)
	}
}
