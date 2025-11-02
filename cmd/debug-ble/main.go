package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lumberbarons/voltgo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: debug-ble <MAC_ADDRESS>")
		fmt.Println("Example: debug-ble A4:C1:37:43:A4:33")
		os.Exit(1)
	}

	macAddr := os.Args[1]
	fmt.Printf("=== BLE Debug Tool ===\n")
	fmt.Printf("Target: %s\n\n", macAddr)

	client, err := voltgo.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Step 1: Scanning for device...")
	ctx := context.Background()
	results, err := client.ScanRaw(ctx, 10*time.Second)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	var deviceIndex = -1
	for i, result := range results {
		if result.Address.String() == macAddr {
			deviceIndex = i
			fmt.Printf("  ✓ Found device: %s\n", result.LocalName())
			break
		}
	}

	if deviceIndex == -1 {
		log.Fatalf("  ✗ Device not found")
	}

	fmt.Println("\nStep 2: Connecting...")
	battery, err := client.ConnectByIndex(ctx, results, deviceIndex)
	if err != nil {
		log.Fatalf("  ✗ Connection failed: %v", err)
	}
	defer battery.Disconnect()
	fmt.Println("  ✓ Connected")

	fmt.Println("\nStep 3: Waiting 2 seconds for spontaneous notifications...")
	time.Sleep(2 * time.Second)
	fmt.Println("  (No spontaneous notifications received)")

	fmt.Println("\nStep 4: Trying command 0x04...")
	ctx4, cancel4 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel4()
	_, err = battery.SendCommand(ctx4, 0x04, []byte{0x00, 0x00, 0x00, 0x00})
	if err != nil {
		fmt.Printf("  ✗ Command 0x04 failed: %v\n", err)
	} else {
		fmt.Println("  ✓ Command 0x04 succeeded!")
	}

	fmt.Println("\nStep 5: Trying command 0x03...")
	ctx3, cancel3 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel3()
	_, err = battery.SendCommand(ctx3, 0x03, []byte{0x00, 0x00, 0x00, 0x29})
	if err != nil {
		fmt.Printf("  ✗ Command 0x03 failed: %v\n", err)
	} else {
		fmt.Println("  ✓ Command 0x03 succeeded!")
	}

	fmt.Println("\nStep 6: Trying to read battery status via GetStatus()...")
	status, err := battery.GetStatus(ctx)
	if err != nil {
		fmt.Printf("  ✗ GetStatus failed: %v\n", err)
	} else {
		fmt.Println("  ✓ GetStatus succeeded!")
		fmt.Printf("\n=== Battery Status ===\n")
		fmt.Printf("Voltage:     %.2f V\n", status.Voltage)
		fmt.Printf("Current:     %.2f A\n", status.Current)
		fmt.Printf("SOC:         %d%%\n", status.SOC)
		fmt.Printf("Temperature: %.1f°C\n", status.Temperature)
		fmt.Printf("Cells:       %d\n", status.CellCount)
	}

	fmt.Println("\n=== Debug Complete ===")
	fmt.Println("If all steps failed, we need a btmon trace from the working Android app.")
	fmt.Println("Run: sudo btmon > android_working.log 2>&1 &")
	fmt.Println("Then use the Voltgo app to connect and read the battery.")
}
