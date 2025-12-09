# Voltgo BLE Protocol

Protocol documentation based on HCI trace analysis from Android Voltgo app.

## BLE Service & Characteristics

| UUID | Handle | Description |
|------|--------|-------------|
| `00001006-0000-1000-8000-00805f9b34fb` | - | Service UUID |
| `00001007-0000-1000-8000-00805f9b34fb` | 0x0011 | Notify Characteristic (receive responses) |
| `00001008-0000-1000-8000-00805f9b34fb` | 0x0015 | Write Characteristic (send commands) |
| `00002902-0000-1000-8000-00805f9b34fb` | 0x0012 | Client Characteristic Configuration Descriptor |

## Connection Sequence

1. Connect to device
2. Discover service `00001006-...`
3. Discover characteristics `00001007-...` (notify) and `00001008-...` (write)
4. Enable notifications: Write `01 00` to CCCD handle 0x0012
5. Send commands to write characteristic (handle 0x0015)
6. Receive responses via notifications (handle 0x0011)

## Command Format

BLE commands use a simple format with NO length field and NO CRC:

```
[VERSION:1][COMMAND:1][DATA:4]
```

All commands are 6 bytes total.

| Byte | Description |
|------|-------------|
| 0 | Version (always `0x01`) |
| 1 | Command byte |
| 2-5 | Data (4 bytes, zero-padded) |

## Commands

### Command 0x03 - Get Battery Status

Primary command for reading battery data.

```
Write:    01 03 00 00 00 00
Response: 01 03 52 00 00 00 ... (90 bytes)
```

Returns voltage, current, SOC, cell voltages, temperatures, and protection status.

### Command 0x10 - Extended Commands

Extended commands use a sub-command byte as the first data byte:

| Sub-Cmd | Bytes | Response | Purpose |
|---------|-------|----------|---------|
| 0x02 | `01 10 02 00 00 00` | `10 02 01` (1 byte) | Query config |
| 0x03 | `01 10 03 00 00 00` | `10 03 02` (1 byte) | Query config |
| 0x04 | `01 10 04 00 00 00` | `10 04 ...` (12 bytes) | Query config |
| 0x06 | `01 10 06 01 00 00 00` | Unknown | Set config (7 bytes) |
| 0x0A | `01 10 0a 01 00 00 00` | Unknown | Set config (7 bytes) |
| 0x0D | `01 10 0d 00 00 00` | No response | Keep-alive/init |

Command 0x10 0x0D is sent frequently (every few seconds) and does not generate a response.

## Response Format

Responses arrive via BLE notifications with format:

```
[VERSION:1][COMMAND:1][DATA:N]
```

### Command 0x03 Response Structure

90-byte response. All multi-byte values are **little-endian**.

| Offset | Size | Field | Scaling |
|--------|------|-------|---------|
| 0x00 | 4 | Header (`52 00 00 00`) | - |
| 0x04 | 4 | Total Voltage | raw / 100 = V |
| 0x08 | 4 | Current | raw / 10 = A (signed) |
| 0x0C | 2 | Cell 0 Voltage | raw / 1000 = V |
| 0x0E | 2 | Cell 1 Voltage | raw / 1000 = V |
| ... | 2 | Cell N Voltage | raw / 1000 = V |
| 0x25 | 1 | SOC (%) | direct |
| 0x28 | 2 | SOH (%) | direct |
| 0x2E | 2 | Status Flags | bitmap |
| 0x30 | 2 | Protection Status | bitmap |
| 0x32 | 1 | Heating Status | 0x80 = on |
| 0x36 | 2 | Warning Status | bitmap |
| 0x42 | 1 | Cell Temp 0 (C) | signed |
| 0x43 | 1 | Cell Temp 1 (C) | signed |
| 0x44 | 1 | Cell Temp 2 (C) | signed |
| 0x45 | 1 | Cell Temp 3 (C) | signed |
| 0x48 | 2 | Cell Count | direct |

### Current Sign Handling

Raw current value uses unsigned 16-bit with overflow for negative:
- If `raw / 10 > 3276.8`: `current = (raw / 10) - 6553.6` (discharging)
- Otherwise: `current = raw / 10` (charging)

## App Behavior

Based on trace analysis, the Android app:

1. Sends command 0x10 0x0D immediately after connecting (initialization)
2. Queries configuration with 0x10 0x02, 0x10 0x03, 0x10 0x04 once at startup
3. Polls battery status with 0x03 every 2-3 seconds
4. Sends 0x10 0x0D periodically as keep-alive

## ATT Protocol Reference

| Opcode | Name | Direction |
|--------|------|-----------|
| 0x12 | Write Request | Client -> Server |
| 0x13 | Write Response | Server -> Client |
| 0x52 | Write Command | Client -> Server (no response) |
| 0x1b | Handle Value Notification | Server -> Client |

Commands use ATT Write Command (0x52) which does not require acknowledgment.

## Notes

- Tested with ZT-25.6V100Ah batteries (Voltgo/Enerwatt compatible)
