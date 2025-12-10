package voltgo

import (
	"context"
	"fmt"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/lumberbarons/voltgo/battery"
	"github.com/lumberbarons/voltgo/ble"
	"github.com/lumberbarons/voltgo/protocol"
)

const (
	DefaultTimeout      = 5 * time.Second
	DefaultScanDuration = 10 * time.Second
)

// Client is the main client for communicating with Voltgo batteries
type Client struct {
	conn *ble.Connection
}

// NewClient creates a new Voltgo client
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
	conn        *ble.Connection
	initialized bool
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

// SendInit sends the initialization command (0x10 0x0D)
// Per the Android app behavior, this should be sent immediately after connecting
// This command does not generate a response
func (b *Battery) SendInit(ctx context.Context) error {
	if err := b.conn.SendInit(ctx); err != nil {
		return fmt.Errorf("failed to send init: %w", err)
	}
	b.initialized = true
	return nil
}

// ensureInitialized sends the init command if not already sent
// Some devices may require this before responding to status queries
func (b *Battery) ensureInitialized(ctx context.Context) error {
	if b.initialized {
		return nil
	}
	// Send init command (0x10 0x0D) - some devices need this before responding
	fmt.Printf("[DEBUG] Sending init command 0x10 0x0D before first query...\n")
	if err := b.conn.SendInit(ctx); err != nil {
		return fmt.Errorf("failed to send init: %w", err)
	}
	// Small delay to let device process init
	time.Sleep(100 * time.Millisecond)
	b.initialized = true
	return nil
}

// GetStatus retrieves the current battery status
// Uses command 0x03 with 6-byte payload (total 8 bytes: 01 03 00 00 00 00 00 00)
func (b *Battery) GetStatus(ctx context.Context) (*battery.Status, error) {
	// Ensure init command has been sent
	if err := b.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// Use command 0x03 as seen in Android app traces
	// Android sends exactly 8 bytes: 01 03 00 00 00 00 00 00
	fmt.Printf("[DEBUG] Sending command 0x03...\n")
	cmdData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	resp, err := b.SendCommand(ctx, 0x03, cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Parse the BMS response
	bmsInfo, err := protocol.ParseBMSInfoResponse([][]byte{resp.Data})
	if err != nil {
		return nil, fmt.Errorf("failed to parse BMS response: %w", err)
	}

	// Convert to battery.Status format
	status := &battery.Status{
		Voltage:     float64(bmsInfo.Voltage),
		Current:     float64(bmsInfo.Current),
		SOC:         bmsInfo.SOC,
		SOH:         bmsInfo.SOH,
		Temperature: float64(averageTemp(bmsInfo.CellTemperatures)),
		CellCount:   bmsInfo.CellCount,
		Cells:       make([]battery.Cell, len(bmsInfo.CellVoltages)),
		UpdatedAt:   time.Now(),
	}

	// Convert cell voltages to battery.Cell format
	for i, voltage := range bmsInfo.CellVoltages {
		status.Cells[i] = battery.Cell{
			Index:   i,
			Voltage: float64(voltage),
		}
	}

	// Add capacity information if available
	// Note: The BMS response doesn't include capacity directly
	// This would need to be calculated or stored separately

	return status, nil
}

// averageTemp calculates the average of temperature readings
func averageTemp(temps []int8) float64 {
	if len(temps) == 0 {
		return 0
	}
	sum := 0
	for _, t := range temps {
		sum += int(t)
	}
	return float64(sum) / float64(len(temps))
}

// GetCellVoltages retrieves individual cell voltages
// This uses the same command as GetStatus (0x03) since cell voltages
// are included in the BMS info response
func (b *Battery) GetCellVoltages(ctx context.Context) ([]battery.Cell, error) {
	if err := b.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	cmdData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	resp, err := b.SendCommand(ctx, 0x03, cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to get cell voltages: %w", err)
	}

	// Parse the BMS response
	bmsInfo, err := protocol.ParseBMSInfoResponse([][]byte{resp.Data})
	if err != nil {
		return nil, fmt.Errorf("failed to parse BMS response: %w", err)
	}

	// Convert to battery.Cell format
	cells := make([]battery.Cell, len(bmsInfo.CellVoltages))
	for i, voltage := range bmsInfo.CellVoltages {
		cells[i] = battery.Cell{
			Index:   i,
			Voltage: float64(voltage),
		}
	}

	return cells, nil
}

// GetInfo retrieves battery/BMS information
// Note: This returns basic information derived from the BMS status
// The protocol doesn't have a separate "info" command
func (b *Battery) GetInfo(ctx context.Context) (*battery.Info, error) {
	if err := b.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// Get BMS status which contains cell count and other info
	cmdData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	resp, err := b.SendCommand(ctx, 0x03, cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to get info: %w", err)
	}

	bmsInfo, err := protocol.ParseBMSInfoResponse([][]byte{resp.Data})
	if err != nil {
		return nil, fmt.Errorf("failed to parse BMS response: %w", err)
	}

	// Calculate nominal voltage based on cell count (LiFePO4 = 3.2V per cell nominal)
	nominalVoltage := float64(bmsInfo.CellCount) * 3.2

	info := &battery.Info{
		Chemistry:      "LiFePO4",
		NominalVoltage: nominalVoltage,
		// Note: Model, Manufacturer, SerialNumber, HardwareVersion, SoftwareVersion
		// are not available in the BMS response. These would require separate commands
		// or manual configuration
	}

	return info, nil
}

// GetBMSInfo retrieves raw BMS information
// This is a low-level method that returns the parsed BMS data directly
func (b *Battery) GetBMSInfo(ctx context.Context) (*protocol.BMSInfo, error) {
	if err := b.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	cmdData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	resp, err := b.SendCommand(ctx, 0x03, cmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMS info: %w", err)
	}

	return protocol.ParseBMSInfoResponse([][]byte{resp.Data})
}

// GetProtectionStatus retrieves and decodes protection status flags
func (b *Battery) GetProtectionStatus(ctx context.Context) (*protocol.ProtectionStatus, error) {
	bmsInfo, err := b.GetBMSInfo(ctx)
	if err != nil {
		return nil, err
	}

	status := protocol.ParseProtectionFlags(bmsInfo.ProtectionFlags)
	return &status, nil
}
