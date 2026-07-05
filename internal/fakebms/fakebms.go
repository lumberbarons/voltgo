// Package fakebms is an in-process emulator of a Voltgo battery BMS for
// testing. It implements the voltgo.Transport interface and speaks the same
// Modbus RTU frames as the real device, including its signature failure
// mode: requests with a bad CRC are silently dropped, which the caller
// observes as a timeout.
package fakebms

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/lumberbarons/voltgo/internal/ble"
	"github.com/lumberbarons/voltgo/internal/protocol"
)

// Device emulates a battery BMS behind a request/response transport.
// The zero value is not usable; construct with New.
type Device struct {
	mu        sync.Mutex
	regs      map[uint16]uint16
	connected bool

	dropNext    int             // silently drop this many requests (respond with timeout)
	corruptNext bool            // corrupt the CRC of the next response
	truncNext   bool            // truncate the next response mid-payload
	exception   byte            // if non-zero, answer the next request with this exception code
	dropStarts  map[uint16]bool // silently drop requests for these start registers
}

// New returns a Device preloaded with the register image of an idle
// ZT-25.6V100Ah battery (see LiveStatusRegisters and LiveDeviceInfoRegisters).
func New() *Device {
	d := &Device{
		regs:       make(map[uint16]uint16),
		connected:  true,
		dropStarts: make(map[uint16]bool),
	}
	d.LoadRegisters(0, LiveStatusRegisters)
	d.LoadRegisters(protocol.DeviceInfoStart, LiveDeviceInfoRegisters)
	return d
}

// LoadRegisters writes a block of register values starting at start.
func (d *Device) LoadRegisters(start uint16, values []uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, v := range values {
		d.regs[start+uint16(i)] = v
	}
}

// SetRegister sets a single register value.
func (d *Device) SetRegister(reg, value uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.regs[reg] = value
}

// DropNext makes the device silently ignore the next n requests, as the real
// BMS does for malformed frames. The caller sees ble.ErrTimeout.
func (d *Device) DropNext(n int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dropNext = n
}

// DropRequestsFor makes the device silently ignore any request whose start
// register is startReg (e.g. protocol.DeviceInfoStart to fail only the
// device-info read while status reads keep working).
func (d *Device) DropRequestsFor(startReg uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dropStarts[startReg] = true
}

// CorruptNextResponse flips a byte in the next response so its CRC is invalid.
func (d *Device) CorruptNextResponse() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.corruptNext = true
}

// TruncateNextResponse cuts the next response off mid-payload.
func (d *Device) TruncateNextResponse() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.truncNext = true
}

// RespondException makes the device answer the next request with a Modbus
// exception response carrying the given code.
func (d *Device) RespondException(code byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.exception = code
}

// Request implements voltgo.Transport. It decodes the request frame exactly
// as the device would and returns the matching response frame.
func (d *Device) Request(_ context.Context, frame []byte, _ time.Duration) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return nil, ble.ErrNotConnected
	}

	// The real BMS silently ignores malformed frames; the BLE layer then
	// times out waiting for a notification that never arrives.
	if len(frame) != 8 || !protocol.VerifyCRC(frame) {
		return nil, ble.ErrTimeout
	}
	if d.dropNext > 0 {
		d.dropNext--
		return nil, ble.ErrTimeout
	}

	slave := frame[0]
	function := frame[1]
	start := binary.BigEndian.Uint16(frame[2:4])
	count := binary.BigEndian.Uint16(frame[4:6])

	if slave != protocol.DefaultSlaveAddr {
		// Not addressed to this device: no response on the bus.
		return nil, ble.ErrTimeout
	}
	if d.dropStarts[start] {
		return nil, ble.ErrTimeout
	}
	if d.exception != 0 {
		code := d.exception
		d.exception = 0
		return protocol.NewExceptionResponse(slave, function, code), nil
	}
	if function != protocol.FuncReadHoldingRegisters {
		return protocol.NewExceptionResponse(slave, function, 0x01), nil
	}

	regs := make([]uint16, count)
	for i := range regs {
		regs[i] = d.regs[start+uint16(i)]
	}
	resp := protocol.NewReadResponse(slave, regs)

	if d.corruptNext {
		d.corruptNext = false
		resp[len(resp)/2] ^= 0xFF
	}
	if d.truncNext {
		d.truncNext = false
		resp = resp[:len(resp)/2]
	}
	return resp, nil
}

// Disconnect implements voltgo.Transport.
func (d *Device) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.connected = false
	return nil
}

// IsConnected implements voltgo.Transport.
func (d *Device) IsConnected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connected
}
