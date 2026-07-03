# Voltgo

A Go library for communicating with Voltgo (and compatible) LiFePO4 batteries via Bluetooth Low Energy (BLE).

These batteries are sold under various brand names including **Enerwatt**, **TCED Worldwide**, and others, but all use the same BLE protocol compatible with the [Voltgo mobile app](https://voltgopower.com/products/voltgo-25-6v-100ah-lifepo-multipurpose-battery).

This library provides a simple interface to connect to and monitor LiFePO4 batteries that use a BMS with Bluetooth support, compatible with the Voltgo mobile app.

## Features

- BLE communication with Voltgo LiFePO4 battery BMS
- Read battery status (voltage, current, SOC, SOH, temperatures)
- Read individual cell voltages
- Read device info (model, capacity, manufacture date)
- Modbus RTU over BLE GATT protocol, verified against real hardware
- Cross-platform support (Linux, macOS, Windows)
- Built on TinyGo Bluetooth library
- Compatible with Enerwatt, TCED Worldwide, and other branded batteries using the Voltgo protocol

## Installation

```bash
go get github.com/lumberbarons/voltgo
```

## Requirements

- Go 1.24 or later
- Bluetooth Low Energy adapter
- Platform-specific Bluetooth support:
  - Linux: BlueZ
  - macOS: CoreBluetooth (built-in)
  - Windows: WinRT Bluetooth API

## Project Structure

```
voltgo/
├── battery/           # Battery data structures and types
│   └── types.go
├── ble/              # BLE connection handling
│   ├── connection.go
│   └── uuids.go
├── protocol/         # Modbus RTU framing and register parsing
│   ├── modbus.go
│   └── parser.go
├── examples/         # Example applications
│   ├── basic/
│   ├── monitor/
│   └── scan/
├── cmd/              # CLI tools
│   └── voltgo-cli/
├── client.go         # Main client interface
├── doc.go
├── go.mod
└── README.md
```

## Quick Start

### Scanning for Batteries

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/lumberbarons/voltgo"
)

func main() {
    ctx := context.Background()

    client, err := voltgo.NewClient()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Scan for 10 seconds (returns raw BLE scan results)
    results, err := client.ScanRaw(ctx, 10*time.Second)
    if err != nil {
        log.Fatal(err)
    }

    for i, result := range results {
        fmt.Printf("%d. %s (%s) - RSSI: %d dBm\n",
            i+1, result.LocalName(), result.Address.String(), result.RSSI)
    }
}
```

### Reading Battery Status

```go
// Connect to a battery device by address
battery, err := client.Connect(ctx, results[0].Address)
if err != nil {
    log.Fatal(err)
}
defer battery.Disconnect()

// Get current status
status, err := battery.GetStatus(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Voltage: %.2fV\n", status.Voltage)
fmt.Printf("Current: %.2fA\n", status.Current)
fmt.Printf("SOC: %d%%\n", status.SOC)
fmt.Printf("Temperature: %.1f°C\n", status.Temperature)

// Get individual cell voltages
cells, err := battery.GetCellVoltages(ctx)
if err != nil {
    log.Fatal(err)
}

for _, cell := range cells {
    fmt.Printf("Cell %d: %.3fV\n", cell.Index+1, cell.Voltage)
}
```

## Protocol Details

For protocol details, see [PROTOCOL.md](PROTOCOL.md).

## Examples

See the `examples/` directory for complete working examples:

- `examples/scan/` - Simple device scanner
- `examples/basic/` - Basic battery communication example
- `examples/monitor/` - Continuous battery monitoring

## API Reference

### Client

```go
// Create a new client
client, err := voltgo.NewClient()

// Scan for devices
devices, err := client.Scan(ctx, duration)

// Connect to a device
battery, err := client.Connect(ctx, device)

// Close the client
client.Close()
```

### Battery

```go
// Check connection status
isConnected := battery.IsConnected()

// Get battery status
status, err := battery.GetStatus(ctx)

// Get cell voltages
cells, err := battery.GetCellVoltages(ctx)

// Get battery info
info, err := battery.GetInfo(ctx)

// Read raw Modbus holding registers
regs, err := battery.ReadRegisters(ctx, startReg, count)

// Disconnect
battery.Disconnect()
```

## Development Status

The protocol (Modbus RTU over BLE GATT) has been reverse-engineered from HCI
traces and live probing, and is verified working against real ZT-25.6V100Ah
batteries on Linux/BlueZ.

Known gaps (see [PROTOCOL.md](PROTOCOL.md)):

- Current scaling/sign is assumed (int16, 0.1A) but has only been observed at 0A idle
- Status/protection flag registers are unmapped (all zero on a healthy battery)
- Write commands (charge/discharge switches, heating) are not yet implemented

## Contributing

Contributions welcome! If you have a compatible battery, please test and report results.

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT

## Acknowledgments

- Protocol analysis based on the Voltgo Android application
- Built with [TinyGo Bluetooth](https://github.com/tinygo-org/bluetooth) library
