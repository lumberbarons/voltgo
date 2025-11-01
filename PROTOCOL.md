# Voltgo BLE Protocol Documentation

This document describes the BLE protocol used by the Voltgo app to communicate with LiFePO4 battery management systems, based on analysis of the decompiled Android application.

## BLE Service and Characteristics

### UUIDs

- **Service UUID**: `00001006-0000-1000-8000-00805f9b34fb`
- **Write Characteristic**: `00001008-0000-1000-8000-00805f9b34fb`
- **Notify Characteristic**: `00001007-0000-1000-8000-00805f9b34fb`
- **CCC Descriptor**: `00002902-0000-1000-8000-00805f9b34fb`

### Connection Flow

1. Connect to BLE device
2. Discover services (filter by Service UUID)
3. Discover characteristics (Write and Notify)
4. Enable notifications on Notify characteristic
5. Write commands to Write characteristic
6. Receive responses via Notify characteristic

## Packet Format

### Standard Packet Structure

```
+------+------+----------+----------+--------+----------+----------+
| VER  | CMD  | LEN_HIGH | LEN_LOW  |  DATA  | CRC_HIGH | CRC_LOW  |
+------+------+----------+----------+--------+----------+----------+
  1B     1B       1B         1B       N bytes     1B        1B
```

- **VER** (1 byte): Protocol version, always `0x01`
- **CMD** (1 byte): Command identifier
- **LEN_HIGH, LEN_LOW** (2 bytes): Data length in big-endian format
- **DATA** (N bytes): Payload data (length specified by LEN field)
- **CRC_HIGH, CRC_LOW** (2 bytes): CRC16/MODBUS checksum in big-endian format

### CRC16 Calculation

- **Algorithm**: CRC-16/MODBUS
- **Polynomial**: 0x8005
- **Initial value**: 0xFFFF
- **Input reflection**: Yes
- **Output reflection**: Yes
- **XOR output**: 0x0000
- **Calculated over**: All bytes except the final 2 CRC bytes
- **Byte order**: Big-endian

### Multi-Frame Packets

For longer transmissions, the protocol uses multi-frame packets with `CMD=0x64` (100):

```
+------+------+----------+----------+-------------+-------------+--------+----------+----------+
| 0x01 | 0x64 | LEN_HIGH | LEN_LOW  | FRAME_ID_HI | FRAME_ID_LO |  DATA  | CRC_HIGH | CRC_LOW  |
+------+------+----------+----------+-------------+-------------+--------+----------+----------+
  1B     1B       1B         1B           1B            1B        N bytes     1B        1B
```

- **FRAME_ID** (2 bytes): Frame identifier in big-endian format
- Used when data exceeds single packet capacity

## Command Protocol

### Command Type Mappings

Based on response parser analysis, the following command/response type mappings exist:

| Response CMD | Command Type | Notes |
|--------------|--------------|-------|
| `0x02` | Type 3 | - |
| `0x03` | Type 11 (0x0B) | - |
| `0x04` | Type 5 | - |
| `0x05` | Type 13 (0x0D) | - |
| `0x06` | Type 6 | - |
| `0x07` | Type 7 | - |
| `0x09` | Type 9 | - |
| `0x0A` | Type 10 | - |
| `0x0B` | Type 4 | - |
| `0x0C` | Type 12 | - |
| `0x0D` | Type 5 | - |

**Note**: The actual command-to-function mappings need to be determined through testing or further app analysis.

## Response Format

### Generic Response Structure

Responses follow the same packet structure as commands. The response data format varies by command:

#### Single Byte Status Response
```
[STATUS_BYTE]
```
- Single byte value (0-255)
- Meaning depends on command

#### MAC Address / String Response
```
[STATUS][STRING_DATA (10 bytes)]
```
- First byte: Status/count
- Following bytes: 10-byte string (MAC address or identifier)

#### List Response
```
[STATUS][COUNT][ITEM1 (10 bytes)][ITEM2 (10 bytes)]...[ITEMn (10 bytes)]
```
- First byte: Status code
- Second byte: Number of items
- Each item: 10 bytes
- Items packed sequentially

#### Binary Data Response
```
[DATA_BYTES...]
```
- Raw binary data
- Format depends on specific command

## MTU Handling

The protocol respects BLE MTU (Maximum Transmission Unit) constraints:

- **Default MTU**: 20 bytes
- **Negotiated MTU**: App attempts to negotiate larger MTU
- **Effective payload**: MTU - 3 bytes (ATT overhead)
- **Chunking**: Large packets split into MTU-sized chunks
- **Inter-chunk delay**: ~10ms between chunk writes

## Implementation Notes

### Threading Model
- BLE operations on main/UI thread (Android requirement)
- Response parsing on background handler thread
- Callbacks delivered via Android Handler messages

### Error Handling
- CRC16 verification on all received packets
- Malformed packet exceptions
- Connection state tracking
- Retry logic for failed operations

### Timing Considerations
- Small delays between chunk writes
- Response timeout handling
- Scan duration limits

## TODO: Unknown Protocol Elements

The following aspects need further investigation:

1. **Specific Command IDs**: What command byte values trigger specific BMS operations?
   - Get battery status (voltage, current, SOC, temperature)
   - Get cell voltages
   - Get protection status
   - Get battery info/model
   - Set parameters (if supported)

2. **Response Data Formats**: Exact byte layouts for:
   - Battery status response
   - Cell voltage response
   - Protection flags
   - Capacity information
   - Cycle count

3. **Data Encoding**:
   - Voltage scale factors (mV? cV?)
   - Current scale factors and sign convention
   - Temperature encoding (Celsius? Kelvin? Scale factor?)
   - Percentage encoding

4. **Multi-Frame Protocol**:
   - When are multi-frame packets used?
   - Frame ID sequencing rules
   - Reassembly logic

5. **Command Parameters**:
   - Which commands require payload data?
   - Parameter encoding formats

## Protocol Analysis Complete

**UPDATE:** The complete protocol has been reverse-engineered from the decompiled Voltgo app source code. **Bluetooth sniffing is NOT required** - all command bytes, response formats, scaling factors, and parsing logic have been extracted directly from the app's smali code.

See `COMMANDS.md` for complete command reference including:
- Exact command bytes (0x03/0x04 for BMS info)
- Complete response packet structure with byte offsets
- All scaling factors (voltage ÷ 100, current ÷ 10, cell voltage ÷ 1000)
- Multi-packet assembly for >16 cell batteries
- Status flag bitmaps

## Testing Approach

To validate the protocol implementation:

1. **Hardware Testing**: Connect to actual battery and verify parsed values match Voltgo app
2. **Edge Cases**: Test with different cell counts, negative current, protection events
3. **Multi-Packet**: Test with batteries having >16 cells
4. **Validate Checksums**: Verify CRC16 implementation matches
5. **Cross-Platform**: Test on Linux, macOS, Windows

## References

- Voltgo Android App Package: `com.voltgopower.monitor`
- Key Classes (obfuscated):
  - `j3/a.smali` - Packet parser
  - `j3/b.smali` - Response router
  - `k3/a.smali`, `l3/a.smali` - Response handlers
  - `Li4/b.smali` - CRC16 and protocol utilities
