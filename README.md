# Enerwatt

A Go library for communicating with Enerwatt (and compatible) LiFePO4 batteries via Bluetooth Low Energy (BLE).

This library provides a simple interface to connect to and monitor LiFePO4 batteries that use a BMS with Bluetooth support, compatible with the Voltgo mobile app.

## Features

- BLE communication with Enerwatt LiFePO4 battery BMS
- Read battery status (voltage, current, SOC, temperature)
- Read individual cell voltages
- Protocol implementation based on Voltgo app
- Cross-platform support (Linux, macOS, Windows)
- Built on TinyGo Bluetooth library

## Installation

```bash
go get github.com/lumberbarons/enerwatt
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
enerwatt/
├── battery/           # Battery data structures and types
│   └── types.go
├── ble/              # BLE connection handling
│   ├── connection.go
│   └── uuids.go
├── protocol/         # Protocol packet handling
│   ├── commands.go
│   └── packet.go
├── examples/         # Example applications
│   ├── basic/
│   └── scan/
├── client.go         # Main client interface
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

    "github.com/lumberbarons/enerwatt"
)

func main() {
    ctx := context.Background()

    client, err := enerwatt.NewClient()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Scan for 10 seconds
    devices, err := client.Scan(ctx, 10*time.Second)
    if err != nil {
        log.Fatal(err)
    }

    for i, device := range devices {
        fmt.Printf("%d. %s (%s) - RSSI: %d dBm\n",
            i+1, device.Name, device.Address, device.RSSI)
    }
}
```

### Reading Battery Status

```go
// Connect to a battery device
battery, err := client.Connect(ctx, device)
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

The library implements the BLE protocol used by the Voltgo app, reverse-engineered from the Android application.

### BLE UUIDs

- **Service UUID**: `00001006-0000-1000-8000-00805f9b34fb`
- **Write Characteristic**: `00001008-0000-1000-8000-00805f9b34fb`
- **Notify Characteristic**: `00001007-0000-1000-8000-00805f9b34fb`

### Packet Format

```
[VER][CMD][DATA_LEN_HIGH][DATA_LEN_LOW][DATA...][CRC16_HIGH][CRC16_LOW]
```

- **VER**: Protocol version (0x01)
- **CMD**: Command byte
- **DATA_LEN**: 16-bit data length (big-endian)
- **DATA**: Variable length payload
- **CRC16**: CRC16/MODBUS checksum (big-endian)

### Multi-Frame Packets

For longer transmissions (CMD=0x64):

```
[0x01][0x64][LEN_H][LEN_L][FRAME_ID_H][FRAME_ID_L][DATA...][CRC16]
```

## Examples

See the `examples/` directory for complete working examples:

- `examples/scan/` - Simple device scanner
- `examples/basic/` - Basic battery communication example

## API Reference

### Client

```go
// Create a new client
client, err := enerwatt.NewClient()

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

// Send raw command
response, err := battery.SendCommand(ctx, cmdByte, data)

// Disconnect
battery.Disconnect()
```

## Development Status

**Fully Implemented!** The protocol has been completely reverse-engineered from the Voltgo Android app:

- [x] Command IDs identified (0x03 for BMS info)
- [x] Response parsing implemented with full byte-level decoding
- [x] Complete data field mappings (voltage, current, SOC, SOH, cells, temps)
- [x] Protection status parsing with flag decoding
- [x] Multi-packet support for >16 cell batteries

### Implemented Features

- ✅ Read battery voltage, current, SOC, SOH
- ✅ Read individual cell voltages (up to 500 cells)
- ✅ Read cell temperatures (4 sensors)
- ✅ Parse protection status flags
- ✅ Parse status and warning flags
- ✅ Heating system status
- ✅ Little-endian multi-byte value handling
- ✅ CRC16/MODBUS checksum validation

### Testing Needed

The implementation is complete but **requires hardware testing** to verify:
- Actual battery communication
- Multi-packet assembly for large battery packs
- Protection flag bit meanings
- Edge cases and error handling

Contributions welcome! If you have a compatible battery, please test and report results.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT

## Acknowledgments

- Protocol analysis based on the Voltgo Android application
- Built with [TinyGo Bluetooth](https://github.com/tinygo-org/bluetooth) library
- CRC16 implementation by [sigurn/crc16](https://github.com/sigurn/crc16)
