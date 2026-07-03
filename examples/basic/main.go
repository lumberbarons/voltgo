package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lumberbarons/voltgo"
)

func main() {
	ctx := context.Background()

	// Create a new client
	client, err := voltgo.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Scanning for batteries...")

	// Scan for batteries for 10 seconds
	devices, err := client.Scan(ctx, 10*time.Second)
	if err != nil {
		client.Close()
		//nolint:gocritic // Cleanup done before exit
		log.Fatalf("Failed to scan: %v", err)
	}

	if len(devices) == 0 {
		log.Fatal("No batteries found")
	}

	fmt.Printf("Found %d device(s):\n", len(devices))
	for i, device := range devices {
		fmt.Printf("  %d. %s (%s) - RSSI: %d dBm\n", i+1, device.Name, device.Address, device.RSSI)
	}

	fmt.Println("\nTo actually connect and read data, uncomment the code below:")
	fmt.Println("(or use the 'monitor' example for a full featured monitoring tool)")
	fmt.Println()
	fmt.Println("Example code to connect and read battery data:")
	fmt.Println("```go")
	fmt.Println("// Get raw scan results")
	fmt.Println("rawResults, _ := client.ScanRaw(ctx, 10*time.Second)")
	fmt.Println()
	fmt.Println("// Connect to first device")
	fmt.Println("battery, err := client.ConnectByIndex(ctx, rawResults, 0)")
	fmt.Println("if err != nil {")
	fmt.Println("    log.Fatal(err)")
	fmt.Println("}")
	fmt.Println("defer battery.Disconnect()")
	fmt.Println()
	fmt.Println("// Get battery status")
	fmt.Println("status, err := battery.GetStatus(ctx)")
	fmt.Println("if err != nil {")
	fmt.Println("    log.Fatal(err)")
	fmt.Println("}")
	fmt.Println()
	fmt.Println("// Display data")
	fmt.Printf("fmt.Printf(\"Voltage: %%.2f V\\\\n\", status.Voltage)\n")
	fmt.Printf("fmt.Printf(\"Current: %%.2f A\\\\n\", status.Current)\n")
	fmt.Printf("fmt.Printf(\"SOC: %%d%%%%%%%%\\\\n\", status.SOC)\n")
	fmt.Printf("fmt.Printf(\"Temperature: %%.1f°C\\\\n\", status.Temperature)\n")
	fmt.Println()
	fmt.Println("// Get cell voltages")
	fmt.Println("for i, cell := range status.Cells {")
	fmt.Printf("    fmt.Printf(\"Cell %%d: %%.3f V\\\\n\", i+1, cell.Voltage)\n")
	fmt.Println("}")
	fmt.Println()
	fmt.Println("// Get device info (model, capacity, firmware strings)")
	fmt.Println("info, _ := battery.GetInfo(ctx)")
	fmt.Printf("fmt.Printf(\"Capacity: %%.1f Ah\\\\n\", info.CapacityAh)\n")
	fmt.Println("```")
	fmt.Println()
	fmt.Println("See examples/monitor/ for a complete monitoring application.")
}
