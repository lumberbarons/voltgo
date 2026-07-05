//go:build !darwin

package ble

import (
	"strings"

	"tinygo.org/x/bluetooth"
)

// ParseAddress parses a BLE MAC address string such as "a4:c1:37:43:a4:42".
// bluetooth.ParseMAC only accepts uppercase hex digits, so normalize first.
func ParseAddress(s string) (bluetooth.Address, error) {
	mac, err := bluetooth.ParseMAC(strings.ToUpper(s))
	if err != nil {
		return bluetooth.Address{}, err
	}
	return bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: mac}}, nil
}
