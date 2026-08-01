package ble

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	bluezDevice1Interface       = "org.bluez.Device1"
	bluezDevice1Connected       = "Connected"
	dbusSignalPropertiesChanged = "org.freedesktop.DBus.Properties.PropertiesChanged"

	// dbusPropertiesChangedInterface and dbusPropertiesChangedChanges index
	// the PropertiesChanged body: the interface whose properties changed,
	// then the changed properties themselves. A third element lists the
	// invalidated property names and is not needed here.
	dbusPropertiesChangedInterface = 0
	dbusPropertiesChangedChanges   = 1
)

// disconnectSignal reports whether sig is BlueZ announcing that the link to
// path is gone.
//
// Every property change on every device reaches the same subscription, so
// this filters on three things at once: the signal has to concern this
// device, the Device1 interface, and the Connected property specifically.
// Anything else — RSSI updates, a second battery disconnecting, the same
// device connecting — leaves the wait running.
func disconnectSignal(sig *dbus.Signal, path dbus.ObjectPath) bool {
	if sig == nil || sig.Name != dbusSignalPropertiesChanged || sig.Path != path {
		return false
	}
	if len(sig.Body) <= dbusPropertiesChangedChanges {
		return false
	}

	iface, ok := sig.Body[dbusPropertiesChangedInterface].(string)
	if !ok || iface != bluezDevice1Interface {
		return false
	}

	changes, ok := sig.Body[dbusPropertiesChangedChanges].(map[string]dbus.Variant)
	if !ok {
		return false
	}

	connected, ok := changes[bluezDevice1Connected].Value().(bool)
	return ok && !connected
}

// awaitDisconnect blocks until a signal reports the link to path down, the
// ceiling elapses, or ctx is cancelled.
//
// Reaching the ceiling is deliberately not an error. The wait exists to stop
// the next dial racing a teardown that is still in progress, and a teardown
// that was never announced — a missed signal, a device BlueZ has already
// forgotten — is not a reason to fail the connect. The caller redials either
// way; the ceiling only bounds how long it holds off. Cancellation is the one
// outcome that must stop the retry, so it alone comes back as an error.
func awaitDisconnect(ctx context.Context, signals <-chan *dbus.Signal, path dbus.ObjectPath, ceiling time.Duration) error {
	timer := time.NewTimer(ceiling)
	defer timer.Stop()

	for {
		select {
		case sig, ok := <-signals:
			if !ok {
				// The subscription is gone, so no disconnect can arrive.
				// Nil the channel so this receive blocks rather than
				// spinning on a closed one, and let the ceiling run out.
				signals = nil
				continue
			}
			if disconnectSignal(sig, path) {
				return nil
			}
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
