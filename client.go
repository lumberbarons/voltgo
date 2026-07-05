package voltgo

import (
	"context"
	"fmt"
	"time"

	"github.com/lumberbarons/voltgo/battery"
	"github.com/lumberbarons/voltgo/internal/ble"
	"github.com/lumberbarons/voltgo/internal/protocol"
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

// Connect connects to a battery device by the address string reported in
// battery.DeviceInfo: a MAC address ("a4:c1:37:43:a4:42"), or on macOS the
// CoreBluetooth UUID assigned during scanning.
func (c *Client) Connect(ctx context.Context, address string) (*Battery, error) {
	addr, err := ble.ParseAddress(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}

	if err := c.conn.Connect(ctx, addr); err != nil {
		return nil, err
	}

	return &Battery{
		conn: c.conn,
	}, nil
}

// Close closes the client and releases resources
func (c *Client) Close() error {
	return c.conn.Disconnect()
}

// transport is the subset of ble.Connection that Battery uses. It exists so
// battery queries can be unit-tested against canned response frames.
type transport interface {
	Request(ctx context.Context, frame []byte, timeout time.Duration) ([]byte, error)
	Disconnect() error
	IsConnected() bool
}

// Battery represents a connected battery
type Battery struct {
	conn transport
}

// Disconnect disconnects from the battery
func (b *Battery) Disconnect() error {
	return b.conn.Disconnect()
}

// IsConnected returns whether the battery is connected
func (b *Battery) IsConnected() bool {
	return b.conn.IsConnected()
}

// ReadRegisters reads count holding registers starting at startReg.
// This is the low-level primitive underlying all queries; the battery
// silently ignores malformed requests, which surfaces here as a timeout.
func (b *Battery) ReadRegisters(ctx context.Context, startReg, count uint16) ([]uint16, error) {
	frame := protocol.NewReadRequest(protocol.DefaultSlaveAddr, startReg, count)

	resp, err := b.conn.Request(ctx, frame, DefaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to read registers %d-%d: %w", startReg, startReg+count-1, err)
	}

	regs, err := protocol.ParseReadResponse(resp, protocol.DefaultSlaveAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid response for registers %d-%d: %w", startReg, startReg+count-1, err)
	}
	return regs, nil
}

// GetBMSInfo reads and parses the status register block. This is the
// low-level variant of GetStatus and includes the raw registers.
func (b *Battery) GetBMSInfo(ctx context.Context) (*battery.BMSInfo, error) {
	regs, err := b.ReadRegisters(ctx, 0, protocol.StatusRegisterCount)
	if err != nil {
		return nil, err
	}
	return protocol.ParseBMSInfo(regs)
}

// GetStatus retrieves the current battery status
func (b *Battery) GetStatus(ctx context.Context) (*battery.Status, error) {
	bmsInfo, err := b.GetBMSInfo(ctx)
	if err != nil {
		return nil, err
	}

	status := &battery.Status{
		Voltage:      bmsInfo.Voltage,
		Current:      bmsInfo.Current,
		SOC:          bmsInfo.SOC,
		SOH:          bmsInfo.SOH,
		Temperature:  averageTemp(bmsInfo.Temperatures),
		Temperatures: bmsInfo.Temperatures,
		CellCount:    bmsInfo.CellCount,
		Cells:        make([]battery.Cell, len(bmsInfo.CellVoltages)),
		UpdatedAt:    time.Now(),
	}

	for i, voltage := range bmsInfo.CellVoltages {
		status.Cells[i] = battery.Cell{
			Index:   i,
			Voltage: voltage,
		}
	}

	return status, nil
}

// GetCellVoltages retrieves individual cell voltages
func (b *Battery) GetCellVoltages(ctx context.Context) ([]battery.Cell, error) {
	status, err := b.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return status.Cells, nil
}

// GetInfo retrieves battery identity information: chemistry and nominal
// voltage derived from the cell count, plus the device's ASCII identity
// strings (model, hardware version, manufacture date).
func (b *Battery) GetInfo(ctx context.Context) (*battery.Info, error) {
	bmsInfo, err := b.GetBMSInfo(ctx)
	if err != nil {
		return nil, err
	}

	info := &battery.Info{
		Chemistry: "LiFePO4",
		// LiFePO4 nominal is 3.2V per cell
		NominalVoltage: float64(bmsInfo.CellCount) * 3.2,
		CapacityAh:     bmsInfo.FullCapacityAh,
	}

	if identity, err := b.GetDeviceIdentity(ctx); err == nil {
		info.DeviceStrings = identity.Strings
	}

	return info, nil
}

// GetDeviceIdentity reads the ASCII device-info register block (model,
// hardware version, manufacture date).
func (b *Battery) GetDeviceIdentity(ctx context.Context) (*battery.DeviceIdentity, error) {
	regs, err := b.ReadRegisters(ctx, protocol.DeviceInfoStart, protocol.DeviceInfoCount)
	if err != nil {
		return nil, err
	}
	return protocol.ParseDeviceInfo(regs), nil
}

// averageTemp calculates the average of temperature readings
func averageTemp(temps []int) float64 {
	if len(temps) == 0 {
		return 0
	}
	sum := 0
	for _, t := range temps {
		sum += t
	}
	return float64(sum) / float64(len(temps))
}
