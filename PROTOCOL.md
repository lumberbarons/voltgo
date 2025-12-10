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
2. **Wait for MTU Exchange** (see critical note below)
3. Discover service `00001006-...`
4. Discover characteristics `00001007-...` (notify) and `00001008-...` (write)
5. Enable notifications: Write `01 00` to CCCD handle 0x0012
6. Send commands to write characteristic (handle 0x0015)
7. Receive responses via notifications (handle 0x0011)

## MTU Exchange (CRITICAL)

**The battery initiates the ATT MTU exchange, not the central device.**

This is unusual behavior - typically the central (phone/computer) initiates MTU negotiation. These batteries expect to send the MTU Request themselves.

### Working Flow (Android)

```
Battery  ──► MTU Request (Client RX MTU: 160)
Phone    ◄── MTU Response (Server RX MTU: 517)
         ... communication works ...
```

In the Android HCI trace, the **battery** sends `ATT: Exchange MTU Request` with MTU 160, and the phone responds.

### Broken Flow (Linux/BlueZ)

```
BlueZ    ──► MTU Request (Client RX MTU: 517)   ← BlueZ initiates first!
Battery  ◄── MTU Response (Server RX MTU: 160)
Battery  ──► MTU Request (Client RX MTU: 160)   ← Battery also tries to initiate
         ... no response, battery ignores commands ...
```

BlueZ automatically initiates MTU exchange immediately after connection. The battery also tries to initiate its own MTU exchange, but since BlueZ already did one, the battery's request may not be handled properly. The battery then fails to respond to any ATT commands.

### Workaround (Unverified)

To communicate with these batteries from Linux, you may need to:

1. Stop `bluetoothd` to prevent automatic MTU negotiation
2. Use raw HCI sockets to handle the connection
3. Wait for the battery's MTU Request and respond to it
4. Only then send commands

This has not been fully verified to work. The battery firmware appears to have a bug where it doesn't properly handle the central initiating MTU exchange.

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

| Sub-Cmd | Response Format | Observed Value |
|---------|-----------------|----------------|
| 0x02 | `10 02` + 1 byte data | `10 02 01` |
| 0x03 | `10 03` + 1 byte data | `10 03 02` |
| 0x04 | `10 04` + length byte (0x0c) + 12 bytes data | `10 04 0c ...` |

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

## Command 0x64 - Firmware Update (OTA)

Firmware update commands use a separate command byte `0x64` with sub-commands for the OTA process.
Discovered via reverse engineering the Android app (`com.zeta.ble.base.protocol`).

### Command Format

```
[COMMAND:1][SUBCMD:1][ADDRESSCODE:1][DATA:N]
```

| Byte | Description |
|------|-------------|
| 0 | Command byte (`0x64`) |
| 1 | Sub-command byte |
| 2 | Address code (always `0x01`) |
| 3+ | Data (variable length) |

### Sub-Commands

| SubCmd | Name | Request Data | Response Data |
|--------|------|--------------|---------------|
| 0x01 | FirmwareSync | (none) | [result:1][errorCode:1] |
| 0x02 | FirmwareStartWrite | [firmwareType:1][dataLength:4LE] | [result:1][errorCode:1] |
| 0x03 | FirmwareWrite | [packageNo:2LE][dataLength:2LE][data:N] | [packageNo:2LE][result:1][errorCode:1] |
| 0x04 | FirmwareFinishWrite | [crc32:4LE] | [result:1][errorCode:1] |
| 0x05 | FirmwareFinish | (none) | [result:1][errorCode:1] |

### Firmware Update Flow

```
┌─────────┐                              ┌─────────────┐
│  App    │                              │   Battery   │
└────┬────┘                              └──────┬──────┘
     │                                          │
     │─────── 0x64 0x01: Sync ─────────────────►
     │◄────── 0x64 0x01: Sync OK ──────────────│
     │                                          │
     │─────── 0x64 0x02: Start(type, len) ─────►
     │◄────── 0x64 0x02: Start OK ─────────────│
     │                                          │
     │─────── 0x64 0x03: Write(0, data) ───────►
     │◄────── 0x64 0x03: Write OK (pkg 0) ─────│
     │                                          │
     │─────── 0x64 0x03: Write(1, data) ───────►
     │◄────── 0x64 0x03: Write OK (pkg 1) ─────│
     │        ... repeat for all packets ...    │
     │                                          │
     │─────── 0x64 0x04: FinishWrite(crc32) ───►
     │◄────── 0x64 0x04: FinishWrite OK ───────│
     │                                          │
     │─────── 0x64 0x05: Finish ───────────────►
     │◄────── 0x64 0x05: Finish OK ────────────│
     │                                          │
```

### Result/Error Codes

| Result | Meaning |
|--------|---------|
| 0x00 | Success |
| Other | Error (see errorCode for details) |

## Secondary BLE Service (Firmware OTA)

The device exposes additional handles used for firmware OTA transfers:

| Handle | Type | Description |
|--------|------|-------------|
| 0x0027 | Notify | Heartbeat / firmware update notifications |
| 0x002a | Write | Firmware data transfer endpoint |

### Handle 0x0027 Heartbeat Format

The device sends periodic heartbeat notifications on handle 0x0027:

```
[TYPE:1][ZERO:1][SEQUENCE:1]  →  22 00 51, 22 00 52, 22 00 53...
```

| Byte | Description |
|------|-------------|
| 0 | Type byte (`0x22`) |
| 1 | Zero padding (`0x00`) |
| 2 | Sequence counter (incrementing: 0x51, 0x52, 0x53... wraps) |

**Observed behavior:**
- Heartbeat sent approximately every 25 seconds
- Sequence counter increments with each heartbeat
- During firmware updates: burst 256-byte notifications
- Uses Command 0x64 protocol documented above

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
- Manufacturer: ZETA (based on app package `com.zeta.ble.base.protocol`)
- All multi-byte values are little-endian
- Standard CRC32 used for firmware verification

### Tested Devices

| Device Name | MAC Address |
|-------------|-------------|
| ZT-25.6V100Ah-1168 | a4:c1:37:23:a4:3f |
| ZT-25.6V100Ah-1221 | a4:c1:37:43:a4:33 |
