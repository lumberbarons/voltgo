package ble

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/lumberbarons/voltgo/protocol"
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

// Connect connects to a BLE device by address
func (c *Connection) Connect(_ context.Context, address bluetooth.Address) error {
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

	// First discover ALL services to see what's available
	fmt.Printf("[DEBUG] Discovering all services...\n")
	allServices, err := device.DiscoverServices(nil)
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover all services: %w", err)
	}

	fmt.Printf("[DEBUG] Found %d services:\n", len(allServices))
	for i, svc := range allServices {
		fmt.Printf("[DEBUG]   %d: UUID=%s\n", i, svc.UUID().String())
	}

	// Discover our specific service
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
	fmt.Printf("[DEBUG] Using service: %s\n", c.service.UUID().String())

	// Discover ALL characteristics first to see what's available
	fmt.Printf("[DEBUG] Discovering all characteristics...\n")
	allChars, err := c.service.DiscoverCharacteristics(nil)
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover characteristics: %w", err)
	}

	fmt.Printf("[DEBUG] Found %d characteristics:\n", len(allChars))
	for i, char := range allChars {
		fmt.Printf("[DEBUG]   %d: UUID=%s\n", i, char.UUID().String())
	}

	// Discover specific characteristics
	chars, err := c.service.DiscoverCharacteristics([]bluetooth.UUID{
		WriteCharacteristicUUID,
		NotifyCharacteristicUUID,
	})
	if err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to discover characteristics: %w", err)
	}

	fmt.Printf("[DEBUG] Looking for Write UUID: %s\n", WriteCharacteristicUUID.String())
	fmt.Printf("[DEBUG] Looking for Notify UUID: %s\n", NotifyCharacteristicUUID.String())

	for _, char := range chars {
		fmt.Printf("[DEBUG] Found characteristic: %s\n", char.UUID().String())
		if char.UUID() == WriteCharacteristicUUID {
			c.writeChar = char
			fmt.Printf("[DEBUG] Assigned write characteristic\n")
		} else if char.UUID() == NotifyCharacteristicUUID {
			c.notifyChar = char
			fmt.Printf("[DEBUG] Assigned notify characteristic\n")
		}
	}

	if c.writeChar.UUID().String() == "" || c.notifyChar.UUID().String() == "" {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		fmt.Printf("[DEBUG] ERROR: Missing characteristics - write=%s, notify=%s\n",
			c.writeChar.UUID().String(), c.notifyChar.UUID().String())
		return ErrNoCharacteristic
	}

	// Enable notifications
	fmt.Printf("[DEBUG] Enabling notifications on characteristic %s...\n", c.notifyChar.UUID().String())
	if err := c.notifyChar.EnableNotifications(func(buf []byte) {
		c.notifyMu.Lock()
		defer c.notifyMu.Unlock()

		// Copy data to prevent modification
		data := make([]byte, len(buf))
		copy(data, buf)

		// Debug logging
		fmt.Printf("[DEBUG] Received notification: %d bytes: %x\n", len(data), data)

		select {
		case c.responses <- data:
			fmt.Printf("[DEBUG] Notification sent to channel\n")
		default:
			// Drop if channel is full
			fmt.Printf("[DEBUG] Warning: Notification dropped, channel full\n")
		}
	}); err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to enable notifications: %w", err)
	}
	fmt.Printf("[DEBUG] Notifications enabled successfully\n")

	// Give the device time to fully enable notifications
	fmt.Printf("[DEBUG] Waiting 500ms for device to be ready...\n")
	time.Sleep(500 * time.Millisecond)

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
func (c *Connection) WritePacket(_ context.Context, packet *protocol.Packet) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return ErrNotConnected
	}

	data := packet.Marshal()
	fmt.Printf("[DEBUG] Writing packet: cmd=0x%02x, len=%d, data=%x\n", packet.Command, len(data), data)

	// Write in chunks if data is larger than MTU
	chunkSize := c.mtu - 3 // Account for ATT overhead
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		fmt.Printf("[DEBUG] Writing chunk: offset=%d, len=%d, data=%x\n", i, len(chunk), chunk)
		if _, err := c.writeChar.WriteWithoutResponse(chunk); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
		fmt.Printf("[DEBUG] Write completed\n")

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

	fmt.Printf("[DEBUG] Waiting for response (timeout=%v)...\n", timeout)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case data := <-c.responses:
		fmt.Printf("[DEBUG] Received response from channel: %d bytes: %x\n", len(data), data)
		packet, err := protocol.Unmarshal(data)
		if err != nil {
			fmt.Printf("[DEBUG] Error unmarshaling packet: %v\n", err)
			return nil, err
		}
		fmt.Printf("[DEBUG] Unmarshaled packet: cmd=0x%02x, data_len=%d\n", packet.Command, len(packet.Data))
		return packet, nil
	case <-timer.C:
		fmt.Printf("[DEBUG] Response timeout after %v\n", timeout)
		return nil, ErrTimeout
	case <-ctx.Done():
		fmt.Printf("[DEBUG] Context cancelled: %v\n", ctx.Err())
		return nil, ctx.Err()
	}
}

// SendCommand sends a command and waits for response
func (c *Connection) SendCommand(ctx context.Context, cmd byte, data []byte, timeout time.Duration) (*protocol.Packet, error) {
	packet := protocol.NewPacket(cmd, data)

	if err := c.WritePacket(ctx, packet); err != nil {
		return nil, err
	}

	// Give the BMS a moment to process the command before waiting for response
	fmt.Printf("[DEBUG] Waiting 100ms for BMS to process command...\n")
	time.Sleep(100 * time.Millisecond)

	return c.ReadResponse(ctx, timeout)
}
