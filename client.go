package enerwatt

import (
	"context"
	"fmt"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/lumberbarons/enerwatt/battery"
	"github.com/lumberbarons/enerwatt/ble"
	"github.com/lumberbarons/enerwatt/protocol"
)

const (
	DefaultTimeout = 5 * time.Second
	DefaultScanDuration = 10 * time.Second
)

// Client is the main client for communicating with Enerwatt batteries
type Client struct {
	conn *ble.Connection
}

// NewClient creates a new Enerwatt client
func NewClient() (*Client, error) {
	conn, err := ble.NewConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create BLE connection: %w", err)
	}

	return &Client{
		conn: conn,
	}, nil
}

// Scan scans for nearby batteries and returns device info
func (c *Client) Scan(ctx context.Context, duration time.Duration) ([]battery.DeviceInfo, error) {
	results, err := c.conn.Scan(ctx, duration)
	if err != nil {
		return nil, err
	}

	devices := make([]battery.DeviceInfo, 0, len(results))
	for _, result := range results {
		devices = append(devices, battery.DeviceInfo{
			Name:    result.LocalName(),
			Address: result.Address.String(),
			RSSI:    result.RSSI,
		})
	}

	return devices, nil
}

// ScanRaw scans for nearby batteries and returns raw scan results
func (c *Client) ScanRaw(ctx context.Context, duration time.Duration) ([]bluetooth.ScanResult, error) {
	return c.conn.Scan(ctx, duration)
}

// Connect connects to a battery device by address
func (c *Client) Connect(ctx context.Context, address bluetooth.Address) (*Battery, error) {
	if err := c.conn.Connect(ctx, address); err != nil {
		return nil, err
	}

	return &Battery{
		conn: c.conn,
	}, nil
}

// ConnectByIndex connects to a battery device by scan result index
func (c *Client) ConnectByIndex(ctx context.Context, results []bluetooth.ScanResult, index int) (*Battery, error) {
	if index < 0 || index >= len(results) {
		return nil, fmt.Errorf("index out of range: %d", index)
	}

	return c.Connect(ctx, results[index].Address)
}

// Close closes the client and releases resources
func (c *Client) Close() error {
	return c.conn.Disconnect()
}

// Battery represents a connected battery
type Battery struct {
	conn *ble.Connection
}

// Disconnect disconnects from the battery
func (b *Battery) Disconnect() error {
	return b.conn.Disconnect()
}

// IsConnected returns whether the battery is connected
func (b *Battery) IsConnected() bool {
	return b.conn.IsConnected()
}

// SendCommand sends a raw command to the battery
func (b *Battery) SendCommand(ctx context.Context, cmd byte, data []byte) (*protocol.Packet, error) {
	return b.conn.SendCommand(ctx, cmd, data, DefaultTimeout)
}

// GetStatus retrieves the current battery status
// Note: This is a placeholder implementation. The actual command IDs and parsing
// logic need to be determined from further analysis of the Voltgo app
func (b *Battery) GetStatus(ctx context.Context) (*battery.Status, error) {
	// TODO: Implement actual status command and parsing
	// This requires identifying the specific command ID and response format
	// from the Voltgo app analysis

	resp, err := b.SendCommand(ctx, protocol.ResponseType03, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Placeholder parsing - needs actual implementation
	status := &battery.Status{
		UpdatedAt: time.Now(),
	}

	// Parse response data
	if len(resp.Data) > 0 {
		// TODO: Parse actual response format
		// This is where the response parsing logic from the Voltgo app
		// would be implemented
	}

	return status, nil
}

// GetCellVoltages retrieves individual cell voltages
func (b *Battery) GetCellVoltages(ctx context.Context) ([]battery.Cell, error) {
	// TODO: Implement cell voltage command and parsing
	resp, err := b.SendCommand(ctx, protocol.ResponseType04, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get cell voltages: %w", err)
	}

	// Placeholder parsing
	cells := make([]battery.Cell, 0)

	// Parse response data
	if len(resp.Data) > 0 {
		// TODO: Parse actual response format
	}

	return cells, nil
}

// GetInfo retrieves battery/BMS information
func (b *Battery) GetInfo(ctx context.Context) (*battery.Info, error) {
	// TODO: Implement info command and parsing
	resp, err := b.SendCommand(ctx, protocol.ResponseType02, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get info: %w", err)
	}

	info := &battery.Info{
		Chemistry: "LiFePO4",
	}

	// Parse response data
	if len(resp.Data) > 0 {
		// TODO: Parse actual response format
	}

	return info, nil
}

// WritePacket writes a raw packet to the battery
func (b *Battery) WritePacket(ctx context.Context, packet *protocol.Packet) error {
	return b.conn.WritePacket(ctx, packet)
}

// ReadResponse reads a response packet with timeout
func (b *Battery) ReadResponse(ctx context.Context, timeout time.Duration) (*protocol.Packet, error) {
	return b.conn.ReadResponse(ctx, timeout)
}
