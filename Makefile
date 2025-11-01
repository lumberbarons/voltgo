.PHONY: build test clean examples fmt vet scan

# Build all examples
build: examples

# Build example binaries
examples:
	@echo "Building examples..."
	@mkdir -p bin
	go build -o bin/scan ./examples/scan
	go build -o bin/basic ./examples/basic
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
	@echo "  build     - Build all examples"
	@echo "  examples  - Build example binaries"
	@echo "  test      - Run tests"
	@echo "  clean     - Remove build artifacts"
	@echo "  fmt       - Format code"
	@echo "  vet       - Run go vet"
	@echo "  scan      - Run the scan example"
	@echo "  deps      - Download dependencies"
	@echo "  check     - Run fmt and vet"
	@echo "  help      - Show this help message"
