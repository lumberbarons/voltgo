# Contributing to Voltgo

Thank you for your interest in contributing to the Voltgo Go library! This document provides guidelines and information for contributors.

Note: These batteries are sold under various brand names including **Voltgo**, **Enerwatt**, **TCED Worldwide**, and others, but all use the same BLE protocol compatible with the Voltgo mobile app.

## Project Status

This is currently a framework implementation with the core protocol structure in place. The main areas needing development are:

1. **Command Identification** - Determining the exact command bytes for specific BMS operations
2. **Response Parsing** - Implementing parsers for battery data responses
3. **Testing** - Testing with actual hardware
4. **Documentation** - Expanding documentation based on real-world usage

## How to Contribute

### Prerequisites

- Go 1.24 or later
- Git
- A compatible LiFePO4 battery with BLE BMS (for hardware testing)
- BLE adapter on your development machine

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/yourusername/voltgo.git
   cd voltgo
   ```
3. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. Make your changes
5. Test your changes:
   ```bash
   make check
   go test ./...
   ```
6. Commit your changes:
   ```bash
   git commit -am "Add feature: description"
   ```
7. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
8. Open a Pull Request

## Development Setup

### Install Dependencies

```bash
make deps
```

### Build Examples

```bash
make examples
```

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Lint Code

```bash
make vet
```

## Project Structure

```
voltgo/
├── battery/          # Battery data structures
├── ble/             # BLE connection handling
├── protocol/        # Modbus RTU framing and register parsing
├── examples/        # Example applications
├── client.go        # Main client interface
└── PROTOCOL.md      # Protocol documentation
```

## Areas Needing Help

### 1. Register Map Completion

The protocol (Modbus RTU over BLE) is working, but parts of the register map
are unmapped or unverified (see PROTOCOL.md). If you have hardware, you can
help by:

- Reading the status block under load (charging/discharging) to verify the
  current register's sign and scaling
- Triggering protection states (carefully!) to map the flag registers
- Documenting differences between battery models

**Tools:**
- `voltgo-cli read <MAC>` plus `Battery.ReadRegisters()` for raw probing
- Wireshark with BLE support / `btmon` on Linux
- Android HCI snoop log — use a **full** btsnoop capture, not the bugreport
  btsnooz log, which truncates packet payloads

### 2. Command Implementation

The Voltgo app also writes configuration/control frames (charge/discharge
switches, heating) that are not yet re-derived from a full-length capture:

- Modbus function `0x10` (write multiple registers) traffic
- `0x64`-family firmware OTA commands on the secondary service

### 3. Testing

- Unit tests for protocol encoding/decoding
- Integration tests with mock data
- Hardware tests with real batteries
- Cross-platform testing (Linux, macOS, Windows)

### 4. Documentation

- Add godoc comments to all exported types/functions
- Update README with real-world examples
- Document battery compatibility
- Create troubleshooting guide

## Code Style

- Follow standard Go formatting (`gofmt`)
- Write meaningful commit messages
- Add comments for complex logic
- Keep functions focused and small
- Use descriptive variable names

## Testing Guidelines

### Unit Tests

- Test all protocol encoding/decoding
- Test CRC16 calculations
- Test packet validation
- Mock BLE connections for client tests

### Integration Tests

- Use recorded BLE traffic for replay testing
- Test error handling
- Test timeout scenarios

### Hardware Tests

- Test with multiple battery models
- Test all implemented commands
- Document hardware-specific behavior

## Submitting Issues

When submitting issues, please include:

- **Bug Reports:**
  - Go version
  - OS and version
  - Battery model/BMS type
  - Steps to reproduce
  - Expected vs actual behavior
  - Relevant logs or packet captures

- **Feature Requests:**
  - Clear description of the feature
  - Use case / motivation
  - Proposed API (if applicable)

- **Hardware Compatibility Reports:**
  - Battery brand and model
  - BMS version
  - What works / doesn't work
  - Any special considerations

## Protocol Analysis

If you're working on protocol analysis, see `PROTOCOL.md` for current knowledge. When documenting new findings:

1. Document the command byte value
2. Document the request format (if applicable)
3. Document the response format
4. Provide sample packet captures (hex dump)
5. Note any variations between battery models

Example:

```markdown
### Get Battery Status (CMD: 0x03)

**Request:**
```
01 03 00 00 XX XX
```
(No payload data)

**Response:**
```
01 03 00 0E [14 bytes of data] XX XX
```

**Data Format:**
- Bytes 0-1: Total voltage (big-endian, in centivolts)
- Bytes 2-3: Current (big-endian, signed, in centiamperes)
- Bytes 4: SOC percentage (0-100)
- ...
```

## Code of Conduct

- Be respectful and constructive
- Welcome newcomers
- Focus on what is best for the project
- Show empathy towards others

## Questions?

- Open an issue for questions
- Check existing issues and documentation first
- Be specific about what you're trying to achieve

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

Thank you for contributing to Voltgo!
