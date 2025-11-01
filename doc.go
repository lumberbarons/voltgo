// Package enerwatt provides a Go library for communicating with Enerwatt
// and compatible LiFePO4 batteries via Bluetooth Low Energy (BLE).
//
// This library implements the BLE protocol used by the Voltgo mobile app
// to monitor and control LiFePO4 battery management systems (BMS).
//
// Basic usage:
//
//	client, err := enerwatt.NewClient()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Scan for batteries
//	devices, err := client.Scan(ctx, 10*time.Second)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Connect to a device
//	battery, err := client.Connect(ctx, device)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer battery.Disconnect()
//
//	// Read battery status
//	status, err := battery.GetStatus(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// The library is organized into several packages:
//
//   - battery: Data structures for battery status, cell information, and device info
//   - ble: Low-level BLE connection handling and characteristic I/O
//   - protocol: Packet encoding/decoding and CRC16 checksum handling
//
// Protocol Details:
//
// The BLE protocol uses the following UUIDs:
//   - Service: 00001006-0000-1000-8000-00805f9b34fb
//   - Write: 00001008-0000-1000-8000-00805f9b34fb
//   - Notify: 00001007-0000-1000-8000-00805f9b34fb
//
// Packets use the format:
//
//	[VER][CMD][DATA_LEN_HIGH][DATA_LEN_LOW][DATA...][CRC16_HIGH][CRC16_LOW]
//
// Where VER is 0x01, CMD is the command byte, DATA_LEN is a 16-bit length,
// DATA is the payload, and CRC16 is a MODBUS checksum over all preceding bytes.
package enerwatt
