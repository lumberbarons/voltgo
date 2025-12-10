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

BLE commands have different formats depending on the command type:

### Command 0x03 (Battery Status)

```
[VERSION:1][COMMAND:1][DATA:6]
```

| Byte | Description |
|------|-------------|
| 0 | Version (always `0x01`) |
| 1 | Command byte (`0x03`) |
| 2-7 | Data (6 bytes, zero-padded) |

**Total: 8 bytes**

### Extended Commands (0x10)

Extended commands do NOT use the version prefix:

```
[COMMAND:1][SUBCMD:1][DATA:3-4]
```

| Byte | Description |
|------|-------------|
| 0 | Command byte (`0x10`) |
| 1 | Sub-command byte |
| 2-4/5 | Data (3-4 bytes, zero-padded) |

**Total: 5 bytes (queries) or 6 bytes (set commands)**

## Commands

### Command 0x03 - Get Battery Status

Primary command for reading battery data.

```
Write:    01 03 00 00 00 00 00 00  (8 bytes)
Response: 01 03 52 00 00 00 ...    (87 bytes ATT payload)
```

Returns voltage, current, SOC, cell voltages, temperatures, and protection status.

### Command 0x10 - Extended Commands

Extended commands do NOT use the version prefix. Format: `[0x10][SUBCMD][DATA]`

| Sub-Cmd | Write Bytes | Response | Purpose |
|---------|-------------|----------|---------|
| 0x02 | `10 02 00 00 00` (5 bytes) | `10 02 01` | Query config |
| 0x03 | `10 03 00 00 00` (5 bytes) | `10 03 02` | Query config |
| 0x04 | `10 04 00 00 00` (5 bytes) | `10 04 0c ...` (15 bytes) | Query config (12 bytes data) |
| 0x06 | `10 06 01 00 00 00` (6 bytes) | Unknown | Set config |
| 0x0A | `10 0a 01 00 00 00` (6 bytes) | Unknown | Set config |
| 0x0D | `10 0d 00 00 00` (5 bytes) | No response | Keep-alive/init |

Command 0x10 0x0D is sent frequently (every few seconds) and does not generate a response.

## Response Format

Responses arrive via BLE notifications. Format varies by command type:

### Command 0x03 Response

Includes version prefix:
```
[VERSION:1][COMMAND:1][DATA:N]  →  01 03 52 00 00 00 ...
```

**87 bytes** ATT payload. All multi-byte values are **little-endian**.

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

### Extended Command (0x10) Responses

Extended command responses do NOT include the version prefix:
```
[COMMAND:1][SUBCMD:1][DATA:N]  →  10 02 01 ...
```

| Sub-Cmd | Response Format |
|---------|-----------------|
| 0x02 | `10 02` + 1 byte data |
| 0x03 | `10 03` + 1 byte data |
| 0x04 | `10 04` + length byte (0x0c) + 12 bytes data |

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

## Secondary BLE Service (Undocumented)

The device exposes additional handles that appear to be a secondary service:

| Handle | Type | Description |
|--------|------|-------------|
| 0x0027 | Notify | Receives periodic notifications (~25 sec interval) |
| 0x002a | Write | Data transfer endpoint |

**Observed behavior:**
- Handle 0x0027 sends 15-byte heartbeat notifications periodically
- Burst data transfers observed (256-byte notifications) - possibly OTA/firmware update
- Purpose is unknown/undocumented

## Communication Flow

### Connection & Initialization

```
┌─────────┐                              ┌─────────────┐
│  App    │                              │   Battery   │
└────┬────┘                              └──────┬──────┘
     │                                          │
     │─────── BLE Connect ──────────────────────►
     │                                          │
     │◄────── Connection Established ───────────│
     │                                          │
     │─────── Write 0x0012: 01 00 ──────────────►  (Enable notifications)
     │                                          │
     │◄────── Write Response ───────────────────│
     │                                          │
     │─────── Cmd 0x03: 01 03 00... ────────────►  (Get status)
     │        [Handle 0x0015]                   │
     │                                          │
     │◄────── Notify: 01 03 52 00... ───────────│  (Status response)
     │        [Handle 0x0011]                   │
     │                                          │
     │─────── Cmd 0x10 0x0D: 10 0d 00... ───────►  (Keep-alive/init)
     │                                          │
     │        (no response)                     │
     │                                          │
```

### Polling Loop

```
┌─────────┐                              ┌─────────────┐
│  App    │                              │   Battery   │
└────┬────┘                              └──────┬──────┘
     │                                          │
     ├─────── Cmd 0x03: 01 03 00... ────────────►  (Get status)
     │                                          │
     │◄────── Notify: 01 03 52 00... ───────────│  (Status response)
     │                                          │
     │        ... 2-3 sec delay ...             │
     │                                          │
     ├─────── Cmd 0x10 0x0D: 10 0d 00... ───────►  (Keep-alive)
     │                                          │
     │        (no response)                     │
     │                                          │
     │        ... repeat ...                    │
     │                                          │
```

## Notes

- Tested with ZT-25.6V100Ah batteries (Voltgo/Enerwatt compatible)
- Device names follow pattern: `ZT-25.6V100Ah-XXXX`
