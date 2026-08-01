//go:build linux

package ble

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"tinygo.org/x/bluetooth"
)

const (
	bluezDest              = "org.bluez"
	dbusPropertiesIface    = "org.freedesktop.DBus.Properties"
	dbusGetManagedObjects  = "org.freedesktop.DBus.ObjectManager.GetManagedObjects"
	dbusPropertiesChangedM = "PropertiesChanged"
)

// matchDeviceProperties narrows the subscription to property changes on
// BlueZ devices, so the wait is not woken by unrelated bus traffic.
var matchDeviceProperties = []dbus.MatchOption{
	dbus.WithMatchInterface(dbusPropertiesIface),
	dbus.WithMatchMember(dbusPropertiesChangedM),
	dbus.WithMatchArg(dbusPropertiesChangedInterface, bluezDevice1Interface),
}

// waitForTeardown holds off the next dial until BlueZ reports the aborted
// link gone, rather than guessing at how long that takes.
//
// The abort is raised after BlueZ has connected, so the device is still up
// when it reaches the caller and its Connected property has yet to settle.
// Redialling into that window races the teardown. bleak waits for the same
// disconnect before it retries; this is the equivalent.
//
// Every failure to observe the teardown degrades to waiting out the ceiling —
// no D-Bus, no BlueZ, an unreadable object tree. That is the fixed delay this
// replaced, so a host where the watch cannot run is no worse off than before.
func waitForTeardown(ctx context.Context, address bluetooth.Address, ceiling time.Duration) error {
	bus, err := dbus.SystemBus()
	if err != nil {
		return sleepCtx(ctx, ceiling)
	}

	// The bus is shared with tinygo's own adapter, so the subscription is
	// added and removed around this wait and the connection is left open.
	signals := make(chan *dbus.Signal, 8)
	bus.Signal(signals)
	defer bus.RemoveSignal(signals)

	if err := bus.AddMatchSignal(matchDeviceProperties...); err != nil {
		return sleepCtx(ctx, ceiling)
	}
	//nolint:errcheck // Best effort teardown of a best effort subscription
	defer bus.RemoveMatchSignal(matchDeviceProperties...)

	// Read the device's state only after subscribing. A teardown that lands
	// between the two arrives on the channel; reading first would let it fall
	// into the gap and leave the wait sitting out the whole ceiling.
	path, connected, err := findDevice(bus, devicePathSuffix(address))
	if err != nil {
		return sleepCtx(ctx, ceiling)
	}
	if path == "" || !connected {
		// Already gone, or never known to BlueZ. Nothing to wait for.
		return nil
	}

	return awaitDisconnect(ctx, signals, path, ceiling)
}

// devicePathSuffix renders an address the way BlueZ names the device's object
// path: /org/bluez/hciN/dev_AA_BB_CC_DD_EE_FF. Only the trailing element is
// derived, so the wait does not need to know which adapter opened the link.
func devicePathSuffix(address bluetooth.Address) string {
	return "dev_" + strings.ReplaceAll(address.String(), ":", "_")
}

// findDevice locates the BlueZ object whose path ends in suffix and reports
// whether it is currently connected. An empty path means BlueZ is not
// tracking the device at all, which is as good as torn down.
func findDevice(bus *dbus.Conn, suffix string) (dbus.ObjectPath, bool, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := bus.Object(bluezDest, "/").Call(dbusGetManagedObjects, 0).Store(&objects); err != nil {
		return "", false, fmt.Errorf("list bluez objects: %w", err)
	}

	for path, interfaces := range objects {
		props, ok := interfaces[bluezDevice1Interface]
		if !ok || !strings.HasSuffix(string(path), suffix) {
			continue
		}
		connected, _ := props[bluezDevice1Connected].Value().(bool)
		return path, connected, nil
	}

	return "", false, nil
}
