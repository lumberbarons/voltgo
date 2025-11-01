# Voltgo BLE Protocol Commands

Complete command reference extracted from decompiled Voltgo Android app.

## Command Summary

| Command | Purpose | Request Payload | Response |
|---------|---------|-----------------|----------|
| 0x03 | Get BMS Info (alternate) | `[0x00, 0x00, 0x00, 0x29]` | BMS data packet |
| 0x04 | Get BMS Info (primary) | `[0x00, 0x00, 0x00, 0x29]` | BMS data packet |
| 0x02 | BT Name Max Length | TBD | Max name length |
| 0x05 | Unknown | TBD | TBD |
| 0x06 | Bluetooth Related | TBD | TBD |
| 0x09 | Set BT Name | Name bytes | Status |
| 0x0D | BT Name Request | None | BT name string |

## Primary Command: 0x03/0x04 - Get BMS Info

This is the main command to read all battery data including voltage, current, SOC, temperatures, and cell voltages.

### Request Format

**Packet Structure:**
```
[VER][CMD][LEN_H][LEN_L][DATA...][CRC16_H][CRC16_L]
```

**Example Request:**
```
01 03 00 04 00 00 00 29 XX XX
```

- `0x01` - Protocol version
- `0x03` - Command (can also use 0x04)
- `0x00 0x04` - Data length (4 bytes)
- `0x00 0x00 0x00 0x29` - Command data payload
- `XX XX` - CRC16/MODBUS checksum

### Response Format

**Multi-Packet Protocol:**
- Batteries with ≤16 cells: Single packet (Packet 0)
- Batteries with >16 cells: Multiple packets
  - Packet 0: Main data + cells 0-15
  - Packet 1: Cells 16-31
  - Packet N: Cells (N×16) to ((N+1)×16 - 1)

**Minimum packet 0 length:** 73 bytes (0x49)
**Recommended length:** 82 bytes (0x52) for all fields

### Packet 0 Structure (Main Data)

All multi-byte values are **LITTLE-ENDIAN**.

| Offset | Size | Field | Type | Scaling | Example |
|--------|------|-------|------|---------|---------|
| 0x00 | 2 | Total Voltage | Short (LE) | ÷ 100 → V | 5200 → 52.00V |
| 0x02 | 2 | Current | Short (LE) | ÷ 10 → A* | 250 → 25.0A |
| 0x04 | 2 | Cell 0 Voltage | Short (LE) | ÷ 1000 → V | 3250 → 3.250V |
| 0x06 | 2 | Cell 1 Voltage | Short (LE) | ÷ 1000 → V | 3251 → 3.251V |
| ... | ... | ... | ... | ... | ... |
| 0x22 | 2 | Cell 15 Voltage | Short (LE) | ÷ 1000 → V | 3250 → 3.250V |
| 0x25 | 1 | SOC (%) | Byte | Direct | 85 → 85% |
| 0x28 | 2 | SOH (%) | Short (LE) | Direct | 100 → 100% |
| 0x2E | 2 | Status Flags | Short (LE) | Bitmap | 0x0000 |
| 0x30 | 2 | Protection Status | Short (LE) | Bitmap | 0x0000 |
| 0x32 | 1 | Heating Status | Byte | 0x80=On | 0x80 → true |
| 0x36 | 2 | Warning Status | Short (LE) | Bitmap | 0x0000 |
| 0x42 | 1 | Cell Temp 0 (°C) | Byte (S) | Direct | 25 → 25°C |
| 0x43 | 1 | Cell Temp 1 (°C) | Byte (S) | Direct | 26 → 26°C |
| 0x44 | 1 | Cell Temp 2 (°C) | Byte (S) | Direct | 24 → 24°C |
| 0x45 | 1 | Cell Temp 3 (°C) | Byte (S) | Direct | 25 → 25°C |
| 0x48 | 2 | Cell Count | Short (LE) | Direct | 16 → 16 cells |
| 0x50 | 2 | Heating Switch | Short (LE) | Bit 4 | 0x0010 → true |

**Current Sign Handling:**
- Raw value ÷ 10 = preliminary value
- If preliminary > 3276.8: `final = preliminary - 6553.6` (negative, discharging)
- Otherwise: `final = preliminary` (positive, charging)
- Example: 65286 ÷ 10 = 6528.6, then 6528.6 - 6553.6 = **-25.0A**

### Continuation Packet Structure (Packet N, N > 0)

| Offset | Size | Field | Type | Scaling |
|--------|------|-------|------|---------|
| 0x00 | 2 | Cell (N×16 + 0) Voltage | Short (LE) | ÷ 1000 → V |
| 0x02 | 2 | Cell (N×16 + 1) Voltage | Short (LE) | ÷ 1000 → V |
| ... | ... | ... | ... | ... |
| 0x1E | 2 | Cell (N×16 + 15) Voltage | Short (LE) | ÷ 1000 → V |

- Each continuation packet contains up to 16 cell voltages (32 bytes)
- Last packet may contain fewer cells
- Cell index calculation: `startCell = packetNumber × 16`

## Data Type Reference

### Scaling Factors

| Field | Raw Type | Scaling | Output Unit |
|-------|----------|---------|-------------|
| Total Voltage | uint16 | ÷ 100 | Volts |
| Current | uint16 | ÷ 10 (+ sign logic) | Amperes |
| Cell Voltage | uint16 | ÷ 1000 | Volts |
| SOC | uint8 | Direct | Percentage (0-100) |
| SOH | uint16 | Direct | Percentage (0-100) |
| Temperature | int8 | Direct | Celsius |

### Status Flags (Offset 0x2E)

16-bit bitmap (specific bit meanings TBD - requires testing)

### Protection Status Flags (Offset 0x30)

16-bit bitmap (specific bit meanings TBD - requires testing)

Likely includes:
- Over-voltage protection
- Under-voltage protection
- Over-current protection
- Over-temperature protection
- Short circuit protection

### Warning Status Flags (Offset 0x36)

16-bit bitmap (specific bit meanings TBD - requires testing)

### Heating Status (Offset 0x32)

- `0x80` - Heating is active
- Any other value - Heating is off

### Heating Switch (Offset 0x50)

- Check bit 4 (0x10) of the 16-bit value
- Bit set (value & 0x10 != 0) - Switch enabled
- Bit clear - Switch disabled

## Firmware Update Protocol

Commands 0x64 with subcmd 0x01-0x05 are used for firmware updates. These are not typically needed for normal battery monitoring.

| SubCmd | Purpose |
|--------|---------|
| 0x01 | Firmware Sync Request/Response |
| 0x02 | Firmware Write Request/Response |
| 0x03 | Firmware Data Write/Status |
| 0x04 | Firmware Finish Write Request/Response |
| 0x05 | Firmware Finish Request/Response |

## Implementation Notes

### Byte Order
All multi-byte values use **little-endian** byte order. This has been confirmed by analyzing the `W0` function in the Voltgo app which reads shorts with the little-endian flag set.

### Packet Validation
- Minimum packet 0 length: 73 bytes (0x49)
- Cell count must be 0-500 (defaults to 16 if invalid)
- All packets must pass CRC16/MODBUS checksum validation

### Multi-Packet Assembly
For batteries with >16 cells:
1. Parse packet 0 completely for main data and first 16 cells
2. For each continuation packet N (N ≥ 1):
   - Calculate starting cell index: `startCell = N × 16`
   - Read up to 16 cell voltages
   - Stop when `startCell + cellsRead >= totalCellCount`

### Error Handling
- Invalid cell count (>500): Default to 16 cells
- Packet too short: Return error
- CRC mismatch: Return error
- Missing continuation packets: May result in incomplete cell voltage array

## Testing Recommendations

1. **Single Packet (≤16 cells):**
   - Verify all fields parse correctly
   - Check voltage/current scaling
   - Validate temperature readings

2. **Multi-Packet (>16 cells):**
   - Test with 17, 32, 33 cell batteries
   - Verify correct cell indexing
   - Check last packet with partial cells

3. **Edge Cases:**
   - Negative current (discharging)
   - High SOC/SOH values
   - Protection flags active
   - Heating enabled
   - Temperature extremes

4. **Hardware Variations:**
   - Different battery brands using same BMS
   - Various cell configurations (4S, 8S, 16S, etc.)
   - Firmware versions

## Source Files

Analysis based on decompiled Voltgo app:
- `/voltgo/smali/d3/a.smali` - Main BMS response parser
- `/voltgo/smali/n3/a.smali` - BMSData structure
- `/voltgo/smali/n3/b.smali` - BMS info request builder
- `/voltgo/smali/n3/c.smali` - BMS info response wrapper
- `/voltgo/smali/i4/b.smali` - W0 function (short reader)
- `/voltgo/smali/k3/a.smali` - Command-to-parser router
