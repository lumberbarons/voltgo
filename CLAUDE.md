# Voltgo

Go library for monitoring Voltgo (and compatible) LiFePO4 batteries over BLE. The batteries speak Modbus RTU framed over BLE GATT.

## Navigation

| File | What | When to read |
|------|------|--------------|
| `client.go` | Public client and battery API | Adding or changing exported methods |
| `battery/` | Battery data structures returned to callers | Changing status, cell, or info types |
| `ble/` | BLE connection handling and GATT UUIDs | Debugging connection or notification issues |
| `protocol/` | Modbus RTU framing and register parsing | Changing frame encoding or the register map |
| `cmd/voltgo-cli/` | CLI tool (scan, read) | Adding or changing CLI commands |
| `examples/` | Example applications (scan, basic, monitor) | Writing usage documentation or new examples |
| `doc.go` | Package godoc with usage overview | Updating the public API surface |
| `README.md` | User-facing overview, quick start, API reference | Changing features or the public API |
| `PROTOCOL.md` | Reverse-engineered protocol and register map | Working on protocol or parser changes |
| `CONTRIBUTING.md` | Contributor workflow and open work areas | Preparing a PR |
| `Makefile` | Build, test, and lint targets | Adding build tooling |
| `go.mod` | Module definition and dependencies | Adding dependencies |
| `LICENSE` | MIT license | Checking licensing terms |

## Commands

- Test: `make test`
- Format + vet: `make check`
- Lint (what CI enforces): `golangci-lint run`
- Build examples: `make examples`
- Build CLI: `make cli` (or `make cli-arm64` for Linux ARM64 targets)

## Gotchas

- Hardware tests need a real battery; unit tests (`make test`) run without one.
- The BMS silently drops frames with a bad CRC — no exception response. A timeout usually means a malformed request, not a dead connection.
