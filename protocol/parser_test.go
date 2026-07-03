package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveStatusRegisters is a status block captured from a ZT-25.6V100Ah
// battery (8S LiFePO4, idle at ~100% SOC).
var liveStatusRegisters = []uint16{
	2683,                                           // 0: voltage (26.83V)
	0,                                              // 1: current
	3350, 3352, 3353, 3356, 3359, 3356, 3355, 3355, // 2-9: cells 1-8
	0, 0, 0, 0, 0, 0, 0, 0, // 10-17: unused cell slots
	27, 27, 27, // 18-20: temperatures
	99, 100, 100, 99, // 21-24: SOC, SOH, +2 unmapped
	0, 0, 0, 0, // 25-28
	2, 0, // 29-30
	0x1575, 0x2a00, 0x1b1b, // 31-33
	0, 0, // 34-35
	8,       // 36: cell count
	1000,    // 37: full capacity (100.0Ah)
	0, 0, 0, // 38-40
}

func TestParseBMSInfo_LiveData(t *testing.T) {
	info, err := ParseBMSInfo(liveStatusRegisters)
	require.NoError(t, err)

	assert.InDelta(t, 26.83, info.Voltage, 0.001)
	assert.InDelta(t, 0.0, info.Current, 0.001)
	assert.Equal(t, 8, info.CellCount)
	require.Len(t, info.CellVoltages, 8)
	assert.InDelta(t, 3.350, info.CellVoltages[0], 0.0001)
	assert.InDelta(t, 3.355, info.CellVoltages[7], 0.0001)
	assert.Equal(t, []int{27, 27, 27}, info.Temperatures)
	assert.Equal(t, 99, info.SOC)
	assert.Equal(t, 100, info.SOH)
	assert.InDelta(t, 100.0, info.FullCapacityAh, 0.001)
	assert.Equal(t, liveStatusRegisters, info.RawRegisters)
}

func TestParseBMSInfo_NegativeCurrent(t *testing.T) {
	regs := append([]uint16(nil), liveStatusRegisters...)
	regs[RegCurrent] = 0xFFCE // int16 -50 -> -5.0A discharge

	info, err := ParseBMSInfo(regs)
	require.NoError(t, err)
	assert.InDelta(t, -5.0, info.Current, 0.001)
}

func TestParseBMSInfo_NegativeTemperature(t *testing.T) {
	regs := append([]uint16(nil), liveStatusRegisters...)
	regs[RegTempBase] = 0xFFF6 // int16 -10°C

	info, err := ParseBMSInfo(regs)
	require.NoError(t, err)
	assert.Equal(t, -10, info.Temperatures[0])
}

func TestParseBMSInfo_TooShort(t *testing.T) {
	_, err := ParseBMSInfo(liveStatusRegisters[:10])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestParseBMSInfo_ImplausibleCellCount(t *testing.T) {
	regs := append([]uint16(nil), liveStatusRegisters...)
	regs[RegCellCount] = 0

	_, err := ParseBMSInfo(regs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cell count")

	regs[RegCellCount] = 500
	_, err = ParseBMSInfo(regs)
	assert.Error(t, err)
}

func TestParseDeviceInfo_LiveData(t *testing.T) {
	// Live read of registers 105-136: "TC", "-8S100-V1.0", serial/date block
	regs := []uint16{
		0x5443, 0x0000, 0x2d38, 0x5331, 0x3030, 0x2d56, 0x312e, 0x3000,
		0x0000, 0x0000, 0x0000, 0x0000, 0x5a30, 0x3154, 0x3230, 0x3230,
		0x3234, 0x2d30, 0x312d, 0x3131, 0x0000, 0x0000, 0x0000, 0x0000,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	}

	info := ParseDeviceInfo(regs)
	assert.Equal(t, []string{"TC", "-8S100-V1.0", "Z01T202024-01-11"}, info.Strings)
}

func TestParseDeviceInfo_Empty(t *testing.T) {
	info := ParseDeviceInfo(make([]uint16, 32))
	assert.Empty(t, info.Strings)
}
