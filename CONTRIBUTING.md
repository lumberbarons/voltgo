# Contributing to Voltgo

Thank you for your interest in contributing to the Voltgo Go library! This document provides guidelines and information for contributors.

Note: These batteries are sold under various brand names including **Voltgo**, **Enerwatt**, **TCED Worldwide**, and others, but all use the same BLE protocol compatible with the Voltgo mobile app.

## Project Status

The protocol (Modbus RTU over BLE GATT) is implemented and verified against real ZT-25.6V100Ah batteries. The main areas needing development are:

1. **Register Map Completion** - Verifying the current register's sign/scaling under load and mapping the protection flag registers (see PROTOCOL.md)
2. **Write Commands** - Charge/discharge switches and heating control are not yet implemented
3. **Hardware Coverage** - Testing with other battery models and on macOS/Windows
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

CI enforces [golangci-lint](https://golangci-lint.run/) using the repo's `.golangci.yml` config. Run it locally before pushing:

```bash
golangci-lint run
```

`make vet` runs `go vet` only, which is a subset of what CI checks.

### Pre-commit Hooks (optional)

The repo includes a pre-commit config that runs golangci-lint on each commit. To enable it, install [pre-commit](https://pre-commit.com/) and run:

```bash
pre-commit install
```

## Project Structure

```
voltgo/
├── battery/          # Battery data structures
├── ble/             # BLE connection handling
├── protocol/        # Modbus RTU framing and register parsing
├── cmd/             # CLI tools (voltgo-cli)
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

- **Frame captures from other battery models** — the easiest high-value
  contribution. Drop a captured response frame into
  `protocol/testdata/corpus/<model>/` (see the README there) and the parsers
  are validated against your hardware forever, no CI hardware needed
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
- Fuzz targets live in `protocol/fuzz_test.go`; run longer campaigns with
  `go test -fuzz=FuzzParseReadResponse ./protocol`

### Component Tests

- Client-level tests run against `internal/fakebms`, an in-process BMS
  emulator that speaks real Modbus frames and can inject faults (silent
  drops, corrupt CRCs, truncation, exception responses) — extend it rather
  than hand-rolling response bytes in tests
- Captured frames from real batteries live in `protocol/testdata/corpus/`
  and are replay-tested against the parsers

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
### Status Poll (function 0x03, read holding registers)

**Request:**
```
01 03 00 00 00 29 84 14
```
(ADDR=0x01, FUNC=0x03, START=0, COUNT=41, CRC-16/MODBUS low byte first)

**Response:**
```
01 03 52 [82 bytes of register data] [CRC:2]
```

**Data Format:**
- Register 0: Pack voltage (×0.01 V)
- Register 1: Pack current (int16, ×0.1 A)
- Register 21: SOC percentage (0-100)
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
