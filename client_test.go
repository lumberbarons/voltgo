package voltgo

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lumberbarons/voltgo/internal/protocol"
)

// statusRegisters is a status block captured from a ZT-25.6V100Ah battery
// (8S LiFePO4, idle at ~100% SOC).
var statusRegisters = []uint16{
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

// deviceInfoRegisters is a live read of registers 105-136:
// "TC", "-8S100-V1.0", serial/date block.
var deviceInfoRegisters = []uint16{
	0x5443, 0x0000, 0x2d38, 0x5331, 0x3030, 0x2d56, 0x312e, 0x3000,
	0x0000, 0x0000, 0x0000, 0x0000, 0x5a30, 0x3154, 0x3230, 0x3230,
	0x3234, 0x2d30, 0x312d, 0x3131, 0x0000, 0x0000, 0x0000, 0x0000,
	0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
}

var errNoResponse = errors.New("no canned response for request")

// fakeTransport answers read-holding-registers requests with canned register
// blocks keyed by the request's start register.
type fakeTransport struct {
	registers map[uint16][]uint16
	requests  []uint16 // start registers of received requests, in order
}

func (f *fakeTransport) Request(_ context.Context, frame []byte, _ time.Duration) ([]byte, error) {
	start := binary.BigEndian.Uint16(frame[2:4])
	f.requests = append(f.requests, start)

	regs, ok := f.registers[start]
	if !ok {
		return nil, errNoResponse
	}

	resp := []byte{protocol.DefaultSlaveAddr, protocol.FuncReadHoldingRegisters, byte(len(regs) * 2)}
	for _, r := range regs {
		resp = append(resp, byte(r>>8), byte(r))
	}
	return protocol.AppendCRC(resp), nil
}

func (f *fakeTransport) Disconnect() error { return nil }
func (f *fakeTransport) IsConnected() bool { return true }

func newFakeBattery(registers map[uint16][]uint16) (*Battery, *fakeTransport) {
	ft := &fakeTransport{registers: registers}
	return &Battery{conn: ft}, ft
}

func TestGetStatus(t *testing.T) {
	b, _ := newFakeBattery(map[uint16][]uint16{0: statusRegisters})

	status, err := b.GetStatus(context.Background())
	require.NoError(t, err)

	assert.InDelta(t, 26.83, status.Voltage, 0.001)
	assert.InDelta(t, 0.0, status.Current, 0.001)
	assert.Equal(t, 99, status.SOC)
	assert.Equal(t, 100, status.SOH)
	assert.Equal(t, 8, status.CellCount)
	require.Len(t, status.Cells, 8)
	assert.Equal(t, 0, status.Cells[0].Index)
	assert.InDelta(t, 3.350, status.Cells[0].Voltage, 0.0001)
	assert.InDelta(t, 3.355, status.Cells[7].Voltage, 0.0001)
	assert.Equal(t, []int{27, 27, 27}, status.Temperatures)
	assert.InDelta(t, 27.0, status.Temperature, 0.001)
	assert.False(t, status.UpdatedAt.IsZero())
}

func TestGetCellVoltages(t *testing.T) {
	b, _ := newFakeBattery(map[uint16][]uint16{0: statusRegisters})

	cells, err := b.GetCellVoltages(context.Background())
	require.NoError(t, err)
	require.Len(t, cells, 8)
	assert.InDelta(t, 3.359, cells[4].Voltage, 0.0001)
}

func TestGetInfo(t *testing.T) {
	b, _ := newFakeBattery(map[uint16][]uint16{
		0:                        statusRegisters,
		protocol.DeviceInfoStart: deviceInfoRegisters,
	})

	info, err := b.GetInfo(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "LiFePO4", info.Chemistry)
	assert.InDelta(t, 25.6, info.NominalVoltage, 0.001)
	assert.InDelta(t, 100.0, info.CapacityAh, 0.001)
	assert.Equal(t, []string{"TC", "-8S100-V1.0", "Z01T202024-01-11"}, info.DeviceStrings)
}

func TestGetInfo_DeviceIdentityUnavailable(t *testing.T) {
	// A battery that doesn't answer the device-info read should still yield
	// the info derived from the status block.
	b, _ := newFakeBattery(map[uint16][]uint16{0: statusRegisters})

	info, err := b.GetInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "LiFePO4", info.Chemistry)
	assert.Empty(t, info.DeviceStrings)
}

func TestGetDeviceIdentity(t *testing.T) {
	b, ft := newFakeBattery(map[uint16][]uint16{
		protocol.DeviceInfoStart: deviceInfoRegisters,
	})

	identity, err := b.GetDeviceIdentity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"TC", "-8S100-V1.0", "Z01T202024-01-11"}, identity.Strings)
	assert.Equal(t, []uint16{protocol.DeviceInfoStart}, ft.requests)
}

func TestReadRegisters_TransportError(t *testing.T) {
	b, _ := newFakeBattery(nil)

	_, err := b.ReadRegisters(context.Background(), 0, protocol.StatusRegisterCount)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoResponse)
	assert.Contains(t, err.Error(), "registers 0-40")
}

func TestAverageTemp_Empty(t *testing.T) {
	assert.Zero(t, averageTemp(nil))
}
