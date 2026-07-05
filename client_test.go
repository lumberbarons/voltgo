package voltgo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lumberbarons/voltgo/ble"
	"github.com/lumberbarons/voltgo/internal/fakebms"
	"github.com/lumberbarons/voltgo/protocol"
)

// These are component tests: they exercise every public Battery method
// end-to-end against an in-process BMS emulator that speaks real Modbus
// frames, including the device's silent-drop-on-bad-frame behavior.

func newTestBattery() (*Battery, *fakebms.Device) {
	dev := fakebms.New()
	return NewBattery(dev), dev
}

func TestGetStatus_MapsAllFields(t *testing.T) {
	b, _ := newTestBattery()

	status, err := b.GetStatus(context.Background())
	require.NoError(t, err)

	assert.InDelta(t, 26.83, status.Voltage, 0.001)
	assert.InDelta(t, 0.0, status.Current, 0.001)
	assert.Equal(t, 99, status.SOC)
	assert.Equal(t, 100, status.SOH)
	assert.Equal(t, 8, status.CellCount)
	assert.InDelta(t, 27.0, status.Temperature, 0.001)
	assert.Equal(t, []int{27, 27, 27}, status.Temperatures)
	assert.WithinDuration(t, time.Now(), status.UpdatedAt, 5*time.Second)

	require.Len(t, status.Cells, 8)
	for i, cell := range status.Cells {
		assert.Equal(t, i, cell.Index)
	}
	assert.InDelta(t, 3.350, status.Cells[0].Voltage, 0.0001)
	assert.InDelta(t, 3.355, status.Cells[7].Voltage, 0.0001)
}

func TestGetStatus_DischargeCurrent(t *testing.T) {
	b, dev := newTestBattery()
	dev.SetRegister(protocol.RegCurrent, 0xFFCE) // int16 -50 -> -5.0A

	status, err := b.GetStatus(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, -5.0, status.Current, 0.001)
}

func TestGetCellVoltages(t *testing.T) {
	b, _ := newTestBattery()

	cells, err := b.GetCellVoltages(context.Background())
	require.NoError(t, err)
	require.Len(t, cells, 8)
	assert.InDelta(t, 3.359, cells[4].Voltage, 0.0001)
}

func TestGetInfo(t *testing.T) {
	b, _ := newTestBattery()

	info, err := b.GetInfo(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "LiFePO4", info.Chemistry)
	assert.InDelta(t, 25.6, info.NominalVoltage, 0.001) // 8 cells x 3.2V
	assert.InDelta(t, 100.0, info.CapacityAh, 0.001)
	assert.Equal(t, []string{"TC", "-8S100-V1.0", "Z01T202024-01-11"}, info.DeviceStrings)
}

func TestGetInfo_DeviceInfoUnavailable(t *testing.T) {
	// If the device-info read times out, GetInfo still returns the fields
	// derived from the status block.
	b, dev := newTestBattery()
	dev.DropRequestsFor(protocol.DeviceInfoStart)

	info, err := b.GetInfo(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 25.6, info.NominalVoltage, 0.001)
	assert.Empty(t, info.DeviceStrings)
}

func TestGetDeviceInfo(t *testing.T) {
	b, _ := newTestBattery()

	info, err := b.GetDeviceInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"TC", "-8S100-V1.0", "Z01T202024-01-11"}, info.Strings)
}

func TestGetBMSInfo_RawRegisters(t *testing.T) {
	b, _ := newTestBattery()

	info, err := b.GetBMSInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, fakebms.LiveStatusRegisters, info.RawRegisters)
}

func TestReadRegisters_Single(t *testing.T) {
	b, _ := newTestBattery()

	regs, err := b.ReadRegisters(context.Background(), 0, 1)
	require.NoError(t, err)
	assert.Equal(t, []uint16{2683}, regs)
}

func TestReadRegisters_SilentDropSurfacesAsTimeout(t *testing.T) {
	// The BMS drops malformed frames without an exception response; the
	// caller must see the BLE timeout, not a parse error.
	b, dev := newTestBattery()
	dev.DropNext(1)

	_, err := b.ReadRegisters(context.Background(), 0, 1)
	assert.ErrorIs(t, err, ble.ErrTimeout)
}

func TestReadRegisters_CorruptedResponse(t *testing.T) {
	b, dev := newTestBattery()
	dev.CorruptNextResponse()

	_, err := b.ReadRegisters(context.Background(), 0, 1)
	assert.ErrorIs(t, err, protocol.ErrCRCMismatch)
}

func TestReadRegisters_TruncatedResponse(t *testing.T) {
	b, dev := newTestBattery()
	dev.TruncateNextResponse()

	_, err := b.ReadRegisters(context.Background(), 0, protocol.StatusRegisterCount)
	require.Error(t, err)
}

func TestReadRegisters_ExceptionResponse(t *testing.T) {
	b, dev := newTestBattery()
	dev.RespondException(0x02) // illegal data address

	_, err := b.ReadRegisters(context.Background(), 0, 1)
	var mbErr *protocol.ModbusError
	require.ErrorAs(t, err, &mbErr)
	assert.Equal(t, byte(0x02), mbErr.Code)
}

func TestGetStatus_AfterDisconnect(t *testing.T) {
	b, _ := newTestBattery()
	require.NoError(t, b.Disconnect())
	assert.False(t, b.IsConnected())

	_, err := b.GetStatus(context.Background())
	assert.ErrorIs(t, err, ble.ErrNotConnected)
}

func TestAverageTemp(t *testing.T) {
	assert.Equal(t, 0.0, averageTemp(nil))
	assert.Equal(t, 27.0, averageTemp([]int{27}))
	assert.InDelta(t, 26.666, averageTemp([]int{27, 27, 26}), 0.001)
	assert.Equal(t, -5.0, averageTemp([]int{-10, 0}))
}

func TestConnectByIndex_OutOfRange(t *testing.T) {
	c := &Client{}

	_, err := c.ConnectByIndex(context.Background(), nil, 0)
	assert.Error(t, err)

	_, err = c.ConnectByIndex(context.Background(), nil, -1)
	assert.Error(t, err)
}

func TestReadRegisters_ErrorMentionsRegisterRange(t *testing.T) {
	b, dev := newTestBattery()
	dev.DropNext(1)

	_, err := b.ReadRegisters(context.Background(), 105, 32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "105-136")
}
