package ble

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDevicePath = dbus.ObjectPath("/org/bluez/hci0/dev_A4_C1_37_43_A4_33")

// propertiesChanged builds the signal BlueZ emits when a device property
// changes, in the shape godbus delivers it.
func propertiesChanged(path dbus.ObjectPath, iface string, changed map[string]dbus.Variant) *dbus.Signal {
	return &dbus.Signal{
		Path: path,
		Name: dbusSignalPropertiesChanged,
		Body: []any{iface, changed, []string{}},
	}
}

// send delivers sig to a wait that is expected to still be running. It fails
// the test rather than blocking forever when the wait has already returned.
func send(t *testing.T, signals chan *dbus.Signal, sig *dbus.Signal) {
	t.Helper()

	select {
	case signals <- sig:
	case <-time.After(time.Second):
		t.Fatal("wait was not listening for signals — it returned before the disconnect")
	}
}

func disconnected(path dbus.ObjectPath) *dbus.Signal {
	return propertiesChanged(path, bluezDevice1Interface, map[string]dbus.Variant{
		"Connected": dbus.MakeVariant(false),
	})
}

func TestDisconnectSignal_RecognisesTeardown(t *testing.T) {
	assert.True(t, disconnectSignal(disconnected(testDevicePath), testDevicePath))
}

func TestDisconnectSignal_IgnoresOtherDevices(t *testing.T) {
	other := dbus.ObjectPath("/org/bluez/hci0/dev_11_22_33_44_55_66")

	assert.False(t, disconnectSignal(disconnected(other), testDevicePath),
		"a second battery disconnecting must not release this wait")
}

func TestDisconnectSignal_IgnoresConnectAndUnrelatedProperties(t *testing.T) {
	connect := propertiesChanged(testDevicePath, bluezDevice1Interface, map[string]dbus.Variant{
		"Connected": dbus.MakeVariant(true),
	})
	rssi := propertiesChanged(testDevicePath, bluezDevice1Interface, map[string]dbus.Variant{
		"RSSI": dbus.MakeVariant(int16(-55)),
	})
	adapter := propertiesChanged(testDevicePath, "org.bluez.Adapter1", map[string]dbus.Variant{
		"Connected": dbus.MakeVariant(false),
	})

	assert.False(t, disconnectSignal(connect, testDevicePath))
	assert.False(t, disconnectSignal(rssi, testDevicePath))
	assert.False(t, disconnectSignal(adapter, testDevicePath))
}

func TestDisconnectSignal_IgnoresMalformedSignals(t *testing.T) {
	assert.False(t, disconnectSignal(nil, testDevicePath))
	assert.False(t, disconnectSignal(&dbus.Signal{Path: testDevicePath, Name: dbusSignalPropertiesChanged}, testDevicePath))
	assert.False(t, disconnectSignal(&dbus.Signal{
		Path: testDevicePath,
		Name: dbusSignalPropertiesChanged,
		Body: []any{bluezDevice1Interface, "not a property map"},
	}, testDevicePath))
}

// TestAwaitDisconnect_ReturnsWhenTeardownObserved is the heart of the change:
// the wait ends on the signal rather than on a clock, so the redial follows
// the teardown instead of a guess at how long it takes.
func TestAwaitDisconnect_ReturnsWhenTeardownObserved(t *testing.T) {
	signals := make(chan *dbus.Signal, 1)
	signals <- disconnected(testDevicePath)

	start := time.Now()
	err := awaitDisconnect(context.Background(), signals, testDevicePath, time.Hour)

	require.NoError(t, err)
	assert.Less(t, time.Since(start), time.Second,
		"the wait must end on the disconnect signal, not on the ceiling")
}

// TestAwaitDisconnect_WaitsUntilTheDeviceIsActuallyDown guards the opposite
// error: returning before the link is gone puts the next dial back into the
// race the fixed delay was there to avoid.
func TestAwaitDisconnect_WaitsUntilTheDeviceIsActuallyDown(t *testing.T) {
	signals := make(chan *dbus.Signal)
	released := make(chan error, 1)

	go func() {
		released <- awaitDisconnect(context.Background(), signals, testDevicePath, time.Hour)
	}()

	// Noise that does not mean the link is gone must not release the wait.
	send(t, signals, propertiesChanged(testDevicePath, bluezDevice1Interface, map[string]dbus.Variant{
		"RSSI": dbus.MakeVariant(int16(-55)),
	}))
	send(t, signals, disconnected(dbus.ObjectPath("/org/bluez/hci0/dev_11_22_33_44_55_66")))

	select {
	case <-released:
		t.Fatal("wait released before the device was reported disconnected")
	case <-time.After(50 * time.Millisecond):
	}

	send(t, signals, disconnected(testDevicePath))

	select {
	case err := <-released:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("wait did not end after the disconnect signal")
	}
}

// TestAwaitDisconnect_CeilingReleasesTheWait keeps a missed or absent signal
// from stalling the retry: the ceiling is a bound on the wait, not a failure,
// so the caller redials once it elapses.
func TestAwaitDisconnect_CeilingReleasesTheWait(t *testing.T) {
	signals := make(chan *dbus.Signal)

	start := time.Now()
	err := awaitDisconnect(context.Background(), signals, testDevicePath, 50*time.Millisecond)

	require.NoError(t, err, "a ceiling reached is not an error — the caller redials anyway")
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)
}

// TestAwaitDisconnect_ClosedSignalChannelStillHonoursTheCeiling covers the bus
// dropping the subscription: without a guard the receive would spin on a
// closed channel instead of waiting.
func TestAwaitDisconnect_ClosedSignalChannelStillHonoursTheCeiling(t *testing.T) {
	signals := make(chan *dbus.Signal)
	close(signals)

	start := time.Now()
	err := awaitDisconnect(context.Background(), signals, testDevicePath, 50*time.Millisecond)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond,
		"a closed channel must not turn the wait into a busy loop")
}

func TestAwaitDisconnect_CancelledContextEndsTheWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := awaitDisconnect(ctx, make(chan *dbus.Signal), testDevicePath, time.Hour)

	assert.ErrorIs(t, err, context.Canceled)
}
