# Voltgo BLE Protocol

The batteries speak **Modbus RTU framed over BLE GATT**. A standard Modbus
read-holding-registers frame is written to the write characteristic; the
response frame arrives as a notification on the notify characteristic.

Verified live against ZT-25.6V100Ah batteries (8S LiFePO4) from Linux/BlueZ.

> **History:** an earlier revision of this document described a custom
> `[VERSION][COMMAND][DATA]` protocol with zero-padded commands and claimed the
> batteries did not work with the Linux Bluetooth stack due to an MTU-exchange
> quirk. Both claims were artifacts of the same mistake: the Android HCI
> capture used to derive the protocol was a **btsnooz** log, which truncates
> ACL packets to 15 captured bytes — exactly hiding the register count and CRC
> at the end of each 8-byte command. Commands sent without a valid CRC are
> silently ignored by the BMS, which looked like a connection/handshake
> failure. There is no Linux incompatibility.

## BLE Service & Characteristics

| UUID | Handle | Description |
|------|--------|-------------|
| `00001006-0000-1000-8000-00805f9b34fb` | - | Primary service |
| `00001007-0000-1000-8000-00805f9b34fb` | 0x0011 | Notify characteristic (responses). Also readable: exposes a 200-byte buffer. |
| `00001008-0000-1000-8000-00805f9b34fb` | 0x0015 | Write characteristic (requests). Also readable: reads back the last written frame. |
| `00002902-0000-1000-8000-00805f9b34fb` | 0x0012 | CCCD for the notify characteristic |

The device advertises 16-bit service UUID `0xff00` in advertising data, and a
second GATT service `0xfa00` (purpose unknown, likely firmware OTA).

## Connection Sequence

1. Connect (no pairing/bonding — the link is unencrypted)
2. Discover service `1006` and characteristics `1007`/`1008`
3. Write `01 00` to the CCCD to enable notifications
4. Write Modbus request frames to `1008` (ATT Write Command / write-without-response)
5. Receive response frames as notifications on `1007`

No initialization or keep-alive command is required. The MTU exchange
direction does not matter; BlueZ's client-initiated exchange (517/160) works
fine. The peripheral requests connection parameters 50–75 ms interval, which
BlueZ accepts.

## Request Frame (Modbus RTU read holding registers)

```
[ADDR:1][FUNC:1][START:2 BE][COUNT:2 BE][CRC:2 LE]
```

| Field | Value |
|-------|-------|
| ADDR  | `0x01` (fixed slave address; other addresses get no reply) |
| FUNC  | `0x03` read holding registers |
| START | first register, big-endian |
| COUNT | number of registers, big-endian |
| CRC   | CRC-16/MODBUS (poly `0xA001`, init `0xFFFF`), low byte first |

The status poll used by the Voltgo app reads 41 registers from address 0:

```
01 03 00 00 00 29 84 14
```

**Frames with an invalid CRC are silently dropped** — the BMS sends no
exception response, no notification, nothing.

## Response Frame

```
[ADDR:1][FUNC:1][BYTECOUNT:1][DATA:N][CRC:2 LE]
```

Register values in DATA are **big-endian** (standard Modbus). The 41-register
status response is 87 bytes total and arrives as a single notification
(requires ATT MTU ≥ 90; BlueZ negotiates 160 with these devices).

Example (battery idle at ~100% SOC):

```
01 03 52 0a 7b 00 00 0d 16 0d 18 0d 19 0d 1c 0d 1f 0d 1c 0d 1b 0d 1b
00 00 ×8 00 1b 00 1b 00 1b 00 63 00 64 00 64 00 63 00 00 ×4 00 02
00 00 15 75 2a 00 1b 1b 00 00 00 00 00 08 03 e8 00 00 ×3 bd 78
```

## Register Map (holding registers 0–40)

Derived from live reads; fields marked *unverified* have only been observed
at rest/zero so far.

| Register | Field | Scaling | Notes |
|----------|-------|---------|-------|
| 0 | Pack voltage | ×0.01 V | verified (26.83 V observed) |
| 1 | Pack current | int16, ×0.1 A assumed | *unverified* — always 0 at idle; sign/scale need a load test |
| 2–17 | Cell voltages | ×1 mV | 16 slots; slots ≥ cell count read 0 |
| 18–20 | Temperature sensors | int16 °C | 3 sensors (27 °C observed) |
| 21 | SOC | % | 21/24 track together; 21 assumed SOC |
| 22 | SOH | % | 22/23 track together; 22 assumed SOH |
| 23–24 | Unknown | - | duplicate SOH/SOC values; mapping uncertain |
| 25–28 | Unknown (likely status/protection flags) | - | *unverified* — all zero on a healthy battery |
| 29 | Unknown | - | observed value 2 (possibly MOS state bitmap) |
| 31–33 | Unknown | - | observed `0x1575`, `0x2a00`, `0x1b1b` (0x1b1b = two bytes of 27 — possibly MOS/ambient temps) |
| 36 | Cell count | direct | verified (8) |
| 37 | Full capacity | ×0.1 Ah | verified (1000 = 100.0 Ah) |

Reads of undefined regions return zeros rather than Modbus exceptions.
Function `0x04` (read input registers) is answered as if it were `0x03`.
Function `0x01` (read coils) gets no reply.

## Device Info Block (holding registers 105–136)

ASCII strings, NUL-padded, register bytes in big-endian order:

```
"TC" \0\0 "-8S100-V1.0" \0... "Z01T202024-01-11" \0...
```

Observed fields: model/series ("TC", "-8S100-V1.0" = 8S 100Ah, version 1.0)
and a serial/manufacture-date block ("Z01T20" + "2024-01-11").

## Notes

- Device names follow the pattern `ZT-25.6V100Ah-XXXX`
- MAC OUI `A4:C1:37` (Telink Semiconductor)
- Manufacturer: ZETA (Android app package `com.zeta.ble.base.protocol`)
- The Android app polls the 41-register status block every 2–3 seconds
- The app also writes other frames observed only in truncated captures
  (first bytes `10 0d`, `10 02`, `10 03`, `10 04` — plausibly Modbus function
  `0x10` write-multiple-registers traffic, and `0x64`-family OTA commands on
  the secondary service). These need a full-length capture to re-derive;
  none are required for monitoring.

### Tested Devices

| Device Name | MAC Address |
|-------------|-------------|
| ZT-25.6V100Ah-1238 | a4:c1:37:43:a4:42 |
| ZT-25.6V100Ah-1221 | a4:c1:37:43:a4:33 |
| ZT-25.6V100Ah-1168 | a4:c1:37:23:a4:3f |
