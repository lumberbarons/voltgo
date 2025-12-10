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

	// Connect to device with extended timeout
	// Use longer connection parameters for slower devices
	connTimeout := bluetooth.NewDuration(30 * time.Second)
	fmt.Printf("[DEBUG] Connection timeout set to: %d units (should be ~30s)\n", connTimeout)
	device, err := c.adapter.Connect(address, bluetooth.ConnectionParams{
		ConnectionTimeout: connTimeout,
	})
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
	fmt.Printf("[DEBUG] Notification callback will write to channel with capacity %d\n", cap(c.responses))

	notifyCount := 0
	if err := c.notifyChar.EnableNotifications(func(buf []byte) {
		notifyCount++
		fmt.Printf("[DEBUG] >>> NOTIFICATION RECEIVED #%d: %d bytes <<<\n", notifyCount, len(buf))

		c.notifyMu.Lock()
		defer c.notifyMu.Unlock()

		// Copy data to prevent modification
		data := make([]byte, len(buf))
		copy(data, buf)

		// Debug logging
		fmt.Printf("[DEBUG] Notification data: %x\n", data)

		select {
		case c.responses <- data:
			fmt.Printf("[DEBUG] Notification #%d sent to channel successfully\n", notifyCount)
		default:
			// Drop if channel is full
			fmt.Printf("[DEBUG] ERROR: Notification #%d dropped, channel full!\n", notifyCount)
		}
	}); err != nil {
		//nolint:errcheck // Best effort cleanup on error
		device.Disconnect()
		return fmt.Errorf("failed to enable notifications: %w", err)
	}
	fmt.Printf("[DEBUG] Notifications enabled successfully\n")

	// Give device time to set up notification handler
	time.Sleep(200 * time.Millisecond)
	fmt.Printf("[DEBUG] Post-notification setup delay complete\n")

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
// For BLE, we write raw command bytes without packet framing
func (c *Connection) WritePacket(_ context.Context, packet *protocol.Packet) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return ErrNotConnected
	}

	// Build simple command: [VERSION][COMMAND][DATA...]
	// No length field or CRC for BLE ATT writes
	data := make([]byte, 2+len(packet.Data))
	data[0] = packet.Version
	data[1] = packet.Command
	copy(data[2:], packet.Data)

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
		// Use BLE-specific unmarshaling (no CRC or length field)
		packet, err := protocol.UnmarshalBLE(data)
		if err != nil {
			fmt.Printf("[DEBUG] Error unmarshaling BLE packet: %v\n", err)
			return nil, err
		}
		fmt.Printf("[DEBUG] Unmarshaled BLE packet: ver=0x%02x, cmd=0x%02x, data_len=%d\n", packet.Version, packet.Command, len(packet.Data))
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

	// No delay - immediately wait for response
	// Device typically responds in ~200ms per Android trace
	return c.ReadResponse(ctx, timeout)
}

// WriteRaw writes raw bytes to the device (for extended commands without version prefix)
func (c *Connection) WriteRaw(_ context.Context, data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return ErrNotConnected
	}

	fmt.Printf("[DEBUG] Writing raw data: len=%d, data=%x\n", len(data), data)

	if _, err := c.writeChar.WriteWithoutResponse(data); err != nil {
		return fmt.Errorf("failed to write raw: %w", err)
	}
	fmt.Printf("[DEBUG] Raw write completed\n")

	return nil
}

// SendExtendedCommand sends an extended command (0x10) without version prefix
// These commands don't have the version byte prefix
func (c *Connection) SendExtendedCommand(ctx context.Context, extCmd *protocol.ExtendedCommand) error {
	data := extCmd.MarshalBLE()
	return c.WriteRaw(ctx, data)
}

// SendInit sends the initialization command (0x10 0x0D)
// This should be called after connecting, per the Android app behavior
func (c *Connection) SendInit(ctx context.Context) error {
	fmt.Printf("[DEBUG] Sending init command 0x10 0x0D...\n")
	initCmd := protocol.NewExtendedQueryCommand(protocol.ExtSubCmd0D)
	return c.SendExtendedCommand(ctx, initCmd)
}
