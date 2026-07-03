.PHONY: build test clean examples fmt vet scan cli cli-arm64

# Build all examples
build: examples

# Build example binaries
examples:
	@echo "Building examples..."
	@mkdir -p bin
	go build -o bin/scan ./examples/scan
	go build -o bin/basic ./examples/basic
	go build -o bin/monitor ./examples/monitor
	@echo "Done. Binaries in bin/"

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run the scan example
scan: examples
	./bin/scan

# Run the monitor example
monitor: examples
	./bin/monitor

# Build CLI tool
cli:
	@echo "Building voltgo-cli..."
	@mkdir -p bin
	go build -o bin/voltgo-cli ./cmd/voltgo-cli
	@echo "Done. Binary at bin/voltgo-cli"

# Build CLI tool for ARM64 Linux
cli-arm64:
	@echo "Building voltgo-cli for arm64 Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=arm64 go build -o bin/voltgo-cli-linux-arm64 ./cmd/voltgo-cli
	@echo "Done. Binary at bin/voltgo-cli-linux-arm64"

# Download dependencies
deps:
	go mod download
	go mod tidy

# Check for common issues
check: fmt vet
	@echo "Code checks passed"

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Build all examples"
	@echo "  examples   - Build example binaries"
	@echo "  cli        - Build voltgo-cli tool"
	@echo "  cli-arm64  - Build voltgo-cli for arm64 Linux"
	@echo "  test       - Run tests"
	@echo "  clean      - Remove build artifacts"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  scan       - Run the scan example"
	@echo "  monitor    - Run the monitor example"
	@echo "  deps       - Download dependencies"
	@echo "  check      - Run fmt and vet"
	@echo "  help       - Show this help message"
