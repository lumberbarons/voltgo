package voltgo

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tinygo.org/x/bluetooth"

	"github.com/lumberbarons/voltgo/internal/ble"
	"github.com/lumberbarons/voltgo/internal/fakebms"
	"github.com/lumberbarons/voltgo/internal/protocol"
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

func TestGetInfo_DeviceIdentityUnavailable(t *testing.T) {
	// If the device-info read times out, GetInfo still returns the fields
	// derived from the status block.
	b, dev := newTestBattery()
	dev.DropRequestsFor(protocol.DeviceInfoStart)

	info, err := b.GetInfo(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 25.6, info.NominalVoltage, 0.001)
	assert.Empty(t, info.DeviceStrings)
}

func TestGetDeviceIdentity(t *testing.T) {
	b, _ := newTestBattery()

	identity, err := b.GetDeviceIdentity(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"TC", "-8S100-V1.0", "Z01T202024-01-11"}, identity.Strings)
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

// fakeLink stands in for *ble.Connection. It embeds the BMS emulator so a
// Battery handed out by Connect talks to a real Modbus responder, and records
// the scan and connect traffic Client drives through it.
type fakeLink struct {
	*fakebms.Device

	scanResults   []bluetooth.ScanResult
	scanErr       error
	scanned       []time.Duration
	connectErr    error
	connectedTo   []bluetooth.Address
	disconnects   int
	disconnectErr error
}

func newFakeLink() *fakeLink {
	return &fakeLink{Device: fakebms.New()}
}

func (f *fakeLink) Scan(_ context.Context, duration time.Duration) ([]bluetooth.ScanResult, error) {
	f.scanned = append(f.scanned, duration)
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return f.scanResults, nil
}

func (f *fakeLink) Connect(_ context.Context, address bluetooth.Address) error {
	f.connectedTo = append(f.connectedTo, address)
	return f.connectErr
}

func (f *fakeLink) Disconnect() error {
	f.disconnects++
	if f.disconnectErr != nil {
		return f.disconnectErr
	}
	return f.Device.Disconnect()
}

// advertisement is the slice of bluetooth.AdvertisementPayload that Client
// reads: everything else is present only to satisfy the interface.
type advertisement struct {
	name string
}

func (a advertisement) LocalName() string                                     { return a.name }
func (a advertisement) HasServiceUUID(bluetooth.UUID) bool                    { return false }
func (a advertisement) ServiceUUIDs() []bluetooth.UUID                        { return nil }
func (a advertisement) Bytes() []byte                                         { return nil }
func (a advertisement) ManufacturerData() []bluetooth.ManufacturerDataElement { return nil }
func (a advertisement) ServiceData() []bluetooth.ServiceDataElement           { return nil }

// scanResult builds an advertisement for an address in whichever form this
// platform's ParseAddress accepts.
func scanResult(t *testing.T, address, name string, rssi int16) bluetooth.ScanResult {
	t.Helper()
	addr, err := ble.ParseAddress(address)
	require.NoError(t, err)
	return bluetooth.ScanResult{
		Address:              addr,
		RSSI:                 rssi,
		AdvertisementPayload: advertisement{name: name},
	}
}

// addressPair returns two addresses that parse on this platform: a MAC on
// Linux, a CoreBluetooth UUID on macOS.
func addressPair(t *testing.T) (string, string) {
	t.Helper()
	pairs := [][2]string{
		{"A4:C1:37:43:A4:42", "A4:C1:37:43:A4:33"},
		{"00000000-0000-1000-8000-00805f9b34f1", "00000000-0000-1000-8000-00805f9b34f2"},
	}
	for _, pair := range pairs {
		if _, err := ble.ParseAddress(pair[0]); err == nil {
			return pair[0], pair[1]
		}
	}
	t.Fatalf("no address form parses on %s", runtime.GOOS)
	return "", ""
}

func TestScan_DedupesAndMapsDevices(t *testing.T) {
	first, second := addressPair(t)
	link := newFakeLink()
	link.scanResults = []bluetooth.ScanResult{
		scanResult(t, first, "ZT-25.6V100Ah-1238", -73),
		scanResult(t, second, "ZT-25.6V100Ah-1221", -77),
		scanResult(t, first, "", -70), // nameless scan response, fresher RSSI
	}
	c := &Client{conn: link}

	devices, err := c.Scan(context.Background(), 3*time.Second)

	require.NoError(t, err)
	require.Len(t, devices, 2)
	// First-seen order preserved, freshest RSSI kept, name not erased.
	assert.Equal(t, "ZT-25.6V100Ah-1238", devices[0].Name)
	assert.Equal(t, int16(-70), devices[0].RSSI)
	assert.Equal(t, "ZT-25.6V100Ah-1221", devices[1].Name)
	assert.Equal(t, int16(-77), devices[1].RSSI)
	// Addresses come back in the string form Connect accepts.
	for _, device := range devices {
		_, err := ble.ParseAddress(device.Address)
		assert.NoError(t, err, "address %q should round-trip through ParseAddress", device.Address)
	}
	assert.Equal(t, []time.Duration{3 * time.Second}, link.scanned)
}

func TestScan_NoDevices(t *testing.T) {
	link := newFakeLink()
	c := &Client{conn: link}

	devices, err := c.Scan(context.Background(), time.Second)

	require.NoError(t, err)
	assert.Empty(t, devices)
}

func TestScan_PropagatesError(t *testing.T) {
	link := newFakeLink()
	link.scanErr = errors.New("adapter busy")
	c := &Client{conn: link}

	devices, err := c.Scan(context.Background(), time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "adapter busy")
	assert.Nil(t, devices)
}

func TestConnect_InvalidAddress(t *testing.T) {
	// Address parsing fails before any BLE work happens.
	link := newFakeLink()
	c := &Client{conn: link}

	b, err := c.Connect(context.Background(), "not-an-address")

	require.Error(t, err)
	assert.Nil(t, b)
	assert.Contains(t, err.Error(), "not-an-address")
	assert.Empty(t, link.connectedTo, "no connect should be attempted for an unparseable address")
}

func TestConnect_ReturnsBatteryOnTheLink(t *testing.T) {
	address, _ := addressPair(t)
	link := newFakeLink()
	c := &Client{conn: link}

	b, err := c.Connect(context.Background(), address)

	require.NoError(t, err)
	require.NotNil(t, b)
	// The Battery is wired to the same link, so it can read from it.
	status, err := b.GetStatus(context.Background())
	require.NoError(t, err)
	assert.Positive(t, status.Voltage)

	parsed, err := ble.ParseAddress(address)
	require.NoError(t, err)
	assert.Equal(t, []bluetooth.Address{parsed}, link.connectedTo)
}

func TestConnect_PropagatesLinkError(t *testing.T) {
	address, _ := addressPair(t)
	link := newFakeLink()
	link.connectErr = errors.New("le-connection-abort-by-local")
	c := &Client{conn: link}

	b, err := c.Connect(context.Background(), address)

	require.Error(t, err)
	assert.Nil(t, b)
	assert.Contains(t, err.Error(), "le-connection-abort-by-local")
}

func TestClose_DisconnectsLink(t *testing.T) {
	link := newFakeLink()
	c := &Client{conn: link}

	require.NoError(t, c.Close())

	assert.Equal(t, 1, link.disconnects)
	assert.False(t, link.IsConnected())
}

func TestClose_PropagatesDisconnectError(t *testing.T) {
	link := newFakeLink()
	link.disconnectErr = errors.New("adapter gone")
	c := &Client{conn: link}

	err := c.Close()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "adapter gone")
}

func TestReadRegisters_ErrorMentionsRegisterRange(t *testing.T) {
	b, dev := newTestBattery()
	dev.DropNext(1)

	_, err := b.ReadRegisters(context.Background(), 105, 32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "105-136")
}
