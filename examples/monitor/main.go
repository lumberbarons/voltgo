package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/lumberbarons/voltgo"
)

func main() {
	scanDuration := flag.Duration("scan", 10*time.Second, "Scan duration")
	deviceIndex := flag.Int("device", 0, "Device index to connect to (0-based)")
	continuous := flag.Bool("continuous", false, "Continuously monitor battery")
	interval := flag.Duration("interval", 5*time.Second, "Monitoring interval (if continuous)")
	flag.Parse()

	ctx := context.Background()

	// Create client
	client, err := voltgo.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Scan for devices
	fmt.Printf("Scanning for batteries (%v)...\n", *scanDuration)
	devices, err := client.Scan(ctx, *scanDuration)
	if err != nil {
		client.Close()
		//nolint:gocritic // Cleanup done before exit
		log.Fatalf("Failed to scan: %v", err)
	}

	if len(devices) == 0 {
		log.Fatal("No batteries found")
	}

	fmt.Printf("\nFound %d device(s):\n", len(devices))
	for i, device := range devices {
		fmt.Printf("  %d. %s (%s) - RSSI: %d dBm\n",
			i, device.Name, device.Address, device.RSSI)
	}

	// Connect to selected device
	if *deviceIndex < 0 || *deviceIndex >= len(devices) {
		log.Fatalf("Invalid device index: %d (must be 0-%d)", *deviceIndex, len(devices)-1)
	}

	fmt.Printf("\nConnecting to device %d...\n", *deviceIndex)
	battery, err := client.Connect(ctx, devices[*deviceIndex].Address)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		//nolint:errcheck // Best effort cleanup
		battery.Disconnect()
	}()

	fmt.Println("Connected!")

	// Monitor battery
	if *continuous {
		fmt.Printf("\nMonitoring battery every %v (press Ctrl+C to stop)...\n\n", *interval)
		for {
			displayBatteryStatus(ctx, battery)
			time.Sleep(*interval)
		}
	} else {
		displayBatteryStatus(ctx, battery)
	}
}

func displayBatteryStatus(ctx context.Context, battery *voltgo.Battery) {
	// Get battery status
	status, err := battery.GetStatus(ctx)
	if err != nil {
		log.Printf("Failed to get status: %v", err)
		return
	}

	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("Battery Status - %s\n", status.UpdatedAt.Format("15:04:05"))
	fmt.Printf("═══════════════════════════════════════════════════════════\n\n")

	fmt.Printf("General:\n")
	fmt.Printf("  Voltage:     %.2f V\n", status.Voltage)
	fmt.Printf("  Current:     %.2f A ", status.Current)
	switch {
	case status.Current > 0:
		fmt.Printf("(Charging)\n")
	case status.Current < 0:
		fmt.Printf("(Discharging)\n")
	default:
		fmt.Printf("(Idle)\n")
	}
	fmt.Printf("  Power:       %.2f W\n", status.Voltage*status.Current)
	fmt.Printf("  SOC:         %d%%\n", status.SOC)
	fmt.Printf("  SOH:         %d%%\n", status.SOH)
	fmt.Printf("  Temperature: %.1f°C\n", status.Temperature)
	fmt.Printf("  Cell Count:  %d\n\n", status.CellCount)

	fmt.Printf("Cell Voltages:\n")
	if len(status.Cells) > 0 {
		// Find min and max cell voltages
		minVoltage := status.Cells[0].Voltage
		maxVoltage := status.Cells[0].Voltage
		minIndex := 0
		maxIndex := 0

		for i, cell := range status.Cells {
			if cell.Voltage < minVoltage {
				minVoltage = cell.Voltage
				minIndex = i
			}
			if cell.Voltage > maxVoltage {
				maxVoltage = cell.Voltage
				maxIndex = i
			}
		}

		delta := maxVoltage - minVoltage

		// Display cells in rows of 4
		for i := 0; i < len(status.Cells); i++ {
			if i%4 == 0 && i > 0 {
				fmt.Println()
			}
			fmt.Printf("  Cell %2d: %.3fV ", status.Cells[i].Index+1, status.Cells[i].Voltage)
		}
		fmt.Printf("\n\n")

		fmt.Printf("  Min: %.3fV (Cell %d)\n", minVoltage, minIndex+1)
		fmt.Printf("  Max: %.3fV (Cell %d)\n", maxVoltage, maxIndex+1)
		fmt.Printf("  Delta: %.3fV\n\n", delta)
	}

	if len(status.Temperatures) > 0 {
		fmt.Printf("Temperatures:\n")
		for i, temp := range status.Temperatures {
			fmt.Printf("  Sensor %d: %d°C\n", i+1, temp)
		}
		fmt.Println()
	}

	fmt.Printf("═══════════════════════════════════════════════════════════\n\n")
}
