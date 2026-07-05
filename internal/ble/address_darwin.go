package ble

import "tinygo.org/x/bluetooth"

// ParseAddress parses a BLE address string. On macOS, CoreBluetooth
// identifies devices by a per-machine UUID rather than a MAC address,
// so the string must be a UUID obtained from a previous scan.
func ParseAddress(s string) (bluetooth.Address, error) {
	uuid, err := bluetooth.ParseUUID(s)
	if err != nil {
		return bluetooth.Address{}, err
	}
	return bluetooth.Address{UUID: uuid}, nil
}
