package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lumberbarons/enerwatt"
)

func main() {
	ctx := context.Background()

	// Create a new client
	client, err := enerwatt.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Scanning for batteries...")

	// Scan for batteries for 10 seconds
	devices, err := client.Scan(ctx, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to scan: %v", err)
	}

	if len(devices) == 0 {
		log.Fatal("No batteries found")
	}

	fmt.Printf("Found %d device(s):\n", len(devices))
	for i, device := range devices {
		fmt.Printf("  %d. %s (%s) - RSSI: %d dBm\n", i+1, device.Name, device.Address, device.RSSI)
	}

	// Get raw scan results to connect
	fmt.Println("\nScanning again to get raw results...")
	rawResults, err := client.ScanRaw(ctx, 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to scan: %v", err)
	}

	if len(rawResults) == 0 {
		log.Fatal("No devices found")
	}

	// Example of connecting to the first device (commented out as it requires actual hardware)
	fmt.Println("\nExample connection code (uncomment to use with actual hardware):")
	fmt.Println("/*")
	fmt.Println("  // Connect to the first device")
	fmt.Println("  battery, err := client.ConnectByIndex(ctx, rawResults, 0)")
	fmt.Println("  if err != nil {")
	fmt.Println("    log.Fatalf(\"Failed to connect: %v\", err)")
	fmt.Println("  }")
	fmt.Println("  defer battery.Disconnect()")
	fmt.Println("")
	fmt.Println("  // Get battery status")
	fmt.Println("  status, err := battery.GetStatus(ctx)")
	fmt.Println("  if err != nil {")
	fmt.Println("    log.Fatalf(\"Failed to get status: %v\", err)")
	fmt.Println("  }")
	fmt.Println("")
	fmt.Println("  fmt.Printf(\"Voltage: %.2f V\\n\", status.Voltage)")
	fmt.Println("  fmt.Printf(\"Current: %.2f A\\n\", status.Current)")
	fmt.Println("  fmt.Printf(\"SOC: %d%%\\n\", status.SOC)")
	fmt.Println("  fmt.Printf(\"Temperature: %.1f°C\\n\", status.Temperature)")
	fmt.Println("*/")
}
