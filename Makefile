.PHONY: build test clean examples fmt vet lint scan cli cli-arm64 cover cover-html cover-check

# Coverage is measured over the library only. The examples, the CLI, and the
# fake BMS harness are demos and test infrastructure, so counting them would
# make the number track how much example code exists rather than how well the
# library is tested.
COVER_EXCLUDE = -e '/examples/' -e '/cmd/' -e '/internal/fakebms/'
COVER_PROFILE = coverage.out
COVER_LIB_PROFILE = coverage-lib.out
# Set below current coverage to leave headroom. Note that the total is
# platform-dependent: internal/ble has _darwin/_linux/_other files, so a macOS
# run and CI's Linux run cover different code and report different totals
# (74.5% vs 73.5% at the time of writing). CI's Linux number is the one that
# gates and the lower of the two, so set this from a Linux run and leave enough
# margin for the gap.
COVER_MIN ?= 71

# Extra flags for the coverage test run; CI sets this to -v.
GOTEST_FLAGS ?=

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

# Run tests with coverage. Tests every package so the examples and CLI still
# get compiled, then filters the profile down to the library for reporting.
cover:
	go test $(GOTEST_FLAGS) -race -covermode=atomic -coverprofile=$(COVER_PROFILE) ./...
	@grep -v $(COVER_EXCLUDE) $(COVER_PROFILE) > $(COVER_LIB_PROFILE)
	@go tool cover -func=$(COVER_LIB_PROFILE) | tail -1

# Open a browsable coverage report
cover-html: cover
	go tool cover -html=$(COVER_LIB_PROFILE) -o coverage.html
	@echo "Report written to coverage.html"

# Fail if library coverage drops below COVER_MIN (what CI enforces)
cover-check: cover
	@total=$$(go tool cover -func=$(COVER_LIB_PROFILE) | awk '/^total:/ {print $$3}' | tr -d '%'); \
	if awk "BEGIN { exit !($$total < $(COVER_MIN)) }"; then \
		echo "FAIL: library coverage $$total% is below the $(COVER_MIN)% minimum"; \
		exit 1; \
	fi; \
	echo "OK: library coverage $$total% meets the $(COVER_MIN)% minimum"

# Clean build artifacts
clean:
	rm -rf bin/ $(COVER_PROFILE) $(COVER_LIB_PROFILE) coverage.html
	go clean

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run golangci-lint (what CI enforces)
lint:
	golangci-lint run

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
check: fmt vet lint
	@echo "Code checks passed"

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Build all examples"
	@echo "  examples   - Build example binaries"
	@echo "  cli        - Build voltgo-cli tool"
	@echo "  cli-arm64  - Build voltgo-cli for arm64 Linux"
	@echo "  test       - Run tests"
	@echo "  cover      - Run tests with coverage and print the total"
	@echo "  cover-html - Write a browsable coverage report"
	@echo "  cover-check- Fail if coverage is below COVER_MIN (what CI enforces)"
	@echo "  clean      - Remove build artifacts"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  lint       - Run golangci-lint (what CI enforces)"
	@echo "  scan       - Run the scan example"
	@echo "  monitor    - Run the monitor example"
	@echo "  deps       - Download dependencies"
	@echo "  check      - Run fmt, vet, and lint"
	@echo "  help       - Show this help message"
