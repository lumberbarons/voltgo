package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lumberbarons/voltgo/ble"
)

func main() {
	fmt.Println("Testing BLE notifications...")

	conn, err := ble.NewConnection()
	if err != nil {
		log.Fatalf("Failed to create connection: %v", err)
	}

	// Scan for device
	fmt.Println("Scanning...")
	ctx := context.Background()
	results, err := conn.Scan(ctx, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to scan: %v", err)
	}

	// Find our device
	var deviceAddr string
	for _, result := range results {
		name := result.LocalName()
		if len(name) > 0 && (name[:2] == "ZT" || name[:6] == "Voltgo") {
			deviceAddr = result.Address.String()
			fmt.Printf("Found device: %s at %s\n", name, deviceAddr)
			break
		}
	}

	if deviceAddr == "" {
		log.Fatal("No Voltgo/ZT device found")
	}

	// Connect
	fmt.Println("Connecting...")
	for _, result := range results {
		if result.Address.String() == deviceAddr {
			if err := conn.Connect(ctx, result.Address); err != nil {
				log.Fatalf("Failed to connect: %v", err)
			}
			break
		}
	}

	fmt.Println("Connected! Waiting for notifications...")
	fmt.Println("Press Ctrl+C to exit")

	// Just wait and see if we receive any spontaneous notifications
	time.Sleep(30 * time.Second)

	fmt.Println("No notifications received in 30 seconds")
}
