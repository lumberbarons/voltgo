package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lumberbarons/voltgo"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "voltgo-cli",
		Usage: "CLI tool for Voltgo battery management",
		Commands: []*cli.Command{
			{
				Name:  "scan",
				Usage: "Scan for nearby Voltgo batteries",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:    "duration",
						Aliases: []string{"d"},
						Value:   10 * time.Second,
						Usage:   "Scan duration",
					},
				},
				Action: scanCommand,
			},
			{
				Name:      "read",
				Usage:     "Read battery status by MAC address",
				ArgsUsage: "<MAC_ADDRESS>",
				Action:    readCommand,
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func scanCommand(ctx context.Context, cmd *cli.Command) error {
	duration := cmd.Duration("duration")

	fmt.Printf("Scanning for batteries (duration: %s)...\n", duration)

	client, err := voltgo.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	devices, err := client.Scan(ctx, duration)
	if err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("No batteries found")
		return nil
	}

	fmt.Printf("\nFound %d device(s):\n\n", len(devices))
	for i, device := range devices {
		fmt.Printf("%d. Name:    %s\n", i+1, device.Name)
		fmt.Printf("   Address: %s\n", device.Address)
		fmt.Printf("   RSSI:    %d dBm\n", device.RSSI)
		fmt.Println()
	}

	return nil
}

func readCommand(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() < 1 {
		return fmt.Errorf("MAC address required")
	}

	macAddr := cmd.Args().Get(0)
	fmt.Printf("Connecting to battery at %s...\n", macAddr)

	client, err := voltgo.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// First scan to find the device
	fmt.Println("Scanning for device...")
	results, err := client.ScanRaw(ctx, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	// Find the device by MAC address
	var deviceIndex = -1
	for i, result := range results {
		if result.Address.String() == macAddr {
			deviceIndex = i
			break
		}
	}

	if deviceIndex == -1 {
		return fmt.Errorf("device with address %s not found", macAddr)
	}

	// Connect to the device
	fmt.Println("Connecting...")
	battery, err := client.ConnectByIndex(ctx, results, deviceIndex)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() {
		//nolint:errcheck // Best effort cleanup
		battery.Disconnect()
	}()

	// Get battery status
	fmt.Println("Reading battery status...")
	status, err := battery.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Display battery information
	fmt.Println("=== Battery Status ===")
	fmt.Printf("Voltage:     %.2f V\n", status.Voltage)
	fmt.Printf("Current:     %.2f A\n", status.Current)
	fmt.Printf("SOC:         %d%%\n", status.SOC)
	fmt.Printf("SOH:         %d%%\n", status.SOH)
	fmt.Printf("Cell Count:  %d\n", status.CellCount)
	for i, temp := range status.Temperatures {
		fmt.Printf("Temp %d:      %d°C\n", i+1, temp)
	}
	fmt.Println()

	// Display cell voltages
	fmt.Println("=== Cell Voltages ===")
	for _, cell := range status.Cells {
		fmt.Printf("Cell %2d: %.3f V\n", cell.Index+1, cell.Voltage)
	}
	fmt.Println()

	// Display device info
	info, err := battery.GetInfo(ctx)
	if err != nil {
		fmt.Printf("Warning: Failed to get device info: %v\n", err)
		return nil
	}

	fmt.Println("=== Device Info ===")
	fmt.Printf("Chemistry:       %s\n", info.Chemistry)
	fmt.Printf("Nominal Voltage: %.1f V\n", info.NominalVoltage)
	fmt.Printf("Capacity:        %.1f Ah\n", info.CapacityAh)
	for _, s := range info.DeviceStrings {
		fmt.Printf("Device String:   %s\n", s)
	}
	fmt.Println()

	return nil
}
