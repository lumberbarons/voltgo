package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/lumberbarons/enerwatt"
)

func main() {
	duration := flag.Duration("duration", 10*time.Second, "Scan duration")
	flag.Parse()

	ctx := context.Background()

	// Create a new client
	client, err := enerwatt.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Printf("Scanning for batteries for %v...\n", *duration)

	// Scan for batteries
	devices, err := client.Scan(ctx, *duration)
	if err != nil {
		log.Fatalf("Failed to scan: %v", err)
	}

	if len(devices) == 0 {
		fmt.Println("No devices found")
		return
	}

	fmt.Printf("\nFound %d device(s):\n\n", len(devices))
	for i, device := range devices {
		fmt.Printf("%d. %s\n", i+1, device.Name)
		fmt.Printf("   Address: %s\n", device.Address)
		fmt.Printf("   RSSI: %d dBm\n", device.RSSI)
		fmt.Println()
	}
}
