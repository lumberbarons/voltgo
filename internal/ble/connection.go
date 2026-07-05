package ble

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	ErrNotConnected     = errors.New("not connected to device")
	ErrNoDevice         = errors.New("no device found")
	ErrTimeout          = errors.New("operation timeout")
	ErrNoService        = errors.New("service not found")
	ErrNoCharacteristic = errors.New("characteristic not found")
	ErrFrameTooLarge    = errors.New("frame exceeds device buffer")
)

// maxFrameSize is the size of the device's write characteristic buffer.
const maxFrameSize = 200

// Connection represents a BLE connection to a battery
type Connection struct {
	adapter    *bluetooth.Adapter
	device     bluetooth.Device
	service    bluetooth.DeviceService
	writeChar  bluetooth.DeviceCharacteristic
	notifyChar bluetooth.DeviceCharacteristic
	connected  bool
	mu         sync.RWMutex
	responses  chan []byte
}

// NewConnection creates a new BLE connection handler
func NewConnection() (*Connection, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, fmt.Errorf("failed to enable BLE adapter: %w", err)
	}

	return &Connection{
		adapter:   adapter,
		responses: make(chan []byte, 10),
	}, nil
}

// Scan scans for nearby battery devices
func (c *Connection) Scan(ctx context.Context, duration time.Duration) ([]bluetooth.ScanResult, error) {
	var devices []bluetooth.ScanResult
	var mu sync.Mutex

	scanCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	err := c.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		mu.Lock()
		devices = append(devices, result)
		mu.Unlock()

		select {
		case <-scanCtx.Done():
			//nolint:errcheck // Best effort stop scan in callback
			adapter.StopScan()
		default:
		}
	})

	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	<-scanCtx.Done()
	//nolint:errcheck // Best effort stop scan
	c.adapter.StopScan()

	return devices, nil
}

// Connect connects to a BLE device by address and prepares the battery's
// write and notify characteristics.
func (c *Connection) Connect(_ context.Context, address bluetooth.Address) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return errors.New("already connected")
	}

	// Discard any notifications left over from a previous connection.
drain:
	for {
		select {
		case <-c.responses:
		default:
			break drain
		}
	}

	device, err := c.adapter.Connect(address, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(30 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.device = device

	services, err := device.DiscoverServices([]bluetooth.UUID{ServiceUUID})
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover services: %w", err)
	}
	if len(services) == 0 {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return ErrNoService
	}
	c.service = services[0]

	chars, err := c.service.DiscoverCharacteristics([]bluetooth.UUID{
		WriteCharacteristicUUID,
		NotifyCharacteristicUUID,
	})
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover characteristics: %w", err)
	}

	var haveWrite, haveNotify bool
	for _, char := range chars {
		switch char.UUID() {
		case WriteCharacteristicUUID:
			c.writeChar = char
			haveWrite = true
		case NotifyCharacteristicUUID:
			c.notifyChar = char
			haveNotify = true
		}
	}
	if !haveWrite || !haveNotify {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return ErrNoCharacteristic
	}

	if err := c.notifyChar.EnableNotifications(func(buf []byte) {
		data := make([]byte, len(buf))
		copy(data, buf)

		select {
		case c.responses <- data:
		default:
			// Drop if channel is full
		}
	}); err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to enable notifications: %w", err)
	}

	// Give the device a moment to settle after CCCD write
	time.Sleep(200 * time.Millisecond)

	c.connected = true
	return nil
}

// Disconnect disconnects from the device
func (c *Connection) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if err := c.device.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	c.connected = false
	// The responses channel is intentionally left open: the notification
	// callback may still fire after disconnect, and sending on a closed
	// channel would panic. Stale frames are drained on Connect and Request.
	return nil
}

// IsConnected returns whether the connection is active
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// WriteFrame writes a raw frame to the device's write characteristic.
func (c *Connection) WriteFrame(_ context.Context, frame []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return ErrNotConnected
	}
	if len(frame) > maxFrameSize {
		return fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, len(frame))
	}

	if _, err := c.writeChar.WriteWithoutResponse(frame); err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}
	return nil
}

// ReadFrame waits for the next notification frame, up to timeout.
func (c *Connection) ReadFrame(ctx context.Context, timeout time.Duration) ([]byte, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, ErrNotConnected
	}
	c.mu.RUnlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case data := <-c.responses:
		return data, nil
	case <-timer.C:
		return nil, ErrTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Request writes a frame and waits for the response notification. Any stale
// notifications queued before the write are discarded.
func (c *Connection) Request(ctx context.Context, frame []byte, timeout time.Duration) ([]byte, error) {
drain:
	for {
		select {
		case <-c.responses:
		default:
			break drain
		}
	}

	if err := c.WriteFrame(ctx, frame); err != nil {
		return nil, err
	}
	return c.ReadFrame(ctx, timeout)
}
