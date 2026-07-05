// Package voltgo provides a Go library for communicating with Voltgo
// and compatible LiFePO4 batteries via Bluetooth Low Energy (BLE).
//
// These batteries are sold under various brand names including Enerwatt,
// TCED Worldwide, and others, but use the same BLE protocol compatible
// with the Voltgo mobile app.
//
// This library implements the BLE protocol used by the Voltgo mobile app
// to monitor and control LiFePO4 battery management systems (BMS).
//
// Basic usage:
//
//	client, err := voltgo.NewClient()
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
//	// Connect to a device by its address string
//	battery, err := client.Connect(ctx, devices[0].Address)
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
// Data types returned to callers live in the battery package. BLE transport
// and Modbus framing are implementation details under internal/.
//
// Protocol Details:
//
// The BLE protocol uses the following UUIDs:
//   - Service: 00001006-0000-1000-8000-00805f9b34fb
//   - Write: 00001008-0000-1000-8000-00805f9b34fb
//   - Notify: 00001007-0000-1000-8000-00805f9b34fb
//
// The batteries speak Modbus RTU framed over GATT: a standard Modbus
// read-holding-registers request (slave address 0x01, function 0x03,
// CRC-16/MODBUS) is written to the write characteristic, and the response
// frame arrives as a notification on the notify characteristic. Frames with
// an invalid CRC are silently ignored by the BMS. See PROTOCOL.md for the
// register map.
package voltgo
