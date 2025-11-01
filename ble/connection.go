package ble

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/lumberbarons/enerwatt/protocol"
)

var (
	ErrNotConnected     = errors.New("not connected to device")
	ErrNoDevice         = errors.New("no device found")
	ErrTimeout          = errors.New("operation timeout")
	ErrNoService        = errors.New("service not found")
	ErrNoCharacteristic = errors.New("characteristic not found")
)

// Connection represents a BLE connection to a battery
type Connection struct {
	adapter    *bluetooth.Adapter
	device     bluetooth.Device
	service    bluetooth.DeviceService
	writeChar  bluetooth.DeviceCharacteristic
	notifyChar bluetooth.DeviceCharacteristic
	connected  bool
	mu         sync.RWMutex
	notifyMu   sync.Mutex
	responses  chan []byte
	mtu        int
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
		mtu:       20, // Default BLE MTU
	}, nil
}

// Scan scans for nearby battery devices
func (c *Connection) Scan(ctx context.Context, duration time.Duration) ([]bluetooth.ScanResult, error) {
	var devices []bluetooth.ScanResult
	var mu sync.Mutex

	// Create context with timeout
	scanCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	err := c.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		mu.Lock()
		devices = append(devices, result)
		mu.Unlock()

		// Check if context is done
		select {
		case <-scanCtx.Done():
			adapter.StopScan()
		default:
		}
	})

	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	<-scanCtx.Done()
	c.adapter.StopScan()

	return devices, nil
}

// Connect connects to a BLE device by address
func (c *Connection) Connect(ctx context.Context, address bluetooth.Address) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return errors.New("already connected")
	}

	// Connect to device
	device, err := c.adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.device = device

	// Discover services
	services, err := device.DiscoverServices([]bluetooth.UUID{ServiceUUID})
	if err != nil {
		device.Disconnect()
		return fmt.Errorf("failed to discover services: %w", err)
	}

	if len(services) == 0 {
		device.Disconnect()
		return ErrNoService
	}

	c.service = services[0]

	// Discover characteristics
	chars, err := c.service.DiscoverCharacteristics([]bluetooth.UUID{
		WriteCharacteristicUUID,
		NotifyCharacteristicUUID,
	})
	if err != nil {
		device.Disconnect()
		return fmt.Errorf("failed to discover characteristics: %w", err)
	}

	for _, char := range chars {
		if char.UUID() == WriteCharacteristicUUID {
			c.writeChar = char
		} else if char.UUID() == NotifyCharacteristicUUID {
			c.notifyChar = char
		}
	}

	if c.writeChar.UUID().String() == "" || c.notifyChar.UUID().String() == "" {
		device.Disconnect()
		return ErrNoCharacteristic
	}

	// Enable notifications
	if err := c.notifyChar.EnableNotifications(func(buf []byte) {
		c.notifyMu.Lock()
		defer c.notifyMu.Unlock()

		// Copy data to prevent modification
		data := make([]byte, len(buf))
		copy(data, buf)

		select {
		case c.responses <- data:
		default:
			// Drop if channel is full
		}
	}); err != nil {
		device.Disconnect()
		return fmt.Errorf("failed to enable notifications: %w", err)
	}

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
	close(c.responses)
	c.responses = make(chan []byte, 10)

	return nil
}

// IsConnected returns whether the connection is active
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// WritePacket writes a packet to the device
func (c *Connection) WritePacket(ctx context.Context, packet *protocol.Packet) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return ErrNotConnected
	}

	data := packet.Marshal()

	// Write in chunks if data is larger than MTU
	chunkSize := c.mtu - 3 // Account for ATT overhead
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		if _, err := c.writeChar.WriteWithoutResponse(chunk); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}

		// Small delay between chunks
		if end < len(data) {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}

// ReadResponse reads a response with timeout
func (c *Connection) ReadResponse(ctx context.Context, timeout time.Duration) (*protocol.Packet, error) {
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
		return protocol.Unmarshal(data)
	case <-timer.C:
		return nil, ErrTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendCommand sends a command and waits for response
func (c *Connection) SendCommand(ctx context.Context, cmd byte, data []byte, timeout time.Duration) (*protocol.Packet, error) {
	packet := protocol.NewPacket(cmd, data)

	if err := c.WritePacket(ctx, packet); err != nil {
		return nil, err
	}

	return c.ReadResponse(ctx, timeout)
}
