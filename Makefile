.PHONY: help build test test-race test-coverage lint fmt vet clean install run bench watch stats list

# Default target
.DEFAULT_GOAL := help

# Build variables
BINARY_NAME := token-monitor
BUILD_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

## help: Display this help message
help:
	@echo "Token Monitor - Development Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/token-monitor
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## install: Install the binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) ./cmd/token-monitor
	@echo "Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

## run: Run the application
run: build
	@./$(BUILD_DIR)/$(BINARY_NAME)

## watch: Live monitoring of token usage
watch: build
	@./$(BUILD_DIR)/$(BINARY_NAME) watch

## stats: Show token usage statistics
stats: build
	@./$(BUILD_DIR)/$(BINARY_NAME) stats

## list: List all discovered sessions
list: build
	@./$(BUILD_DIR)/$(BINARY_NAME) list

## test: Run all tests
test:
	@echo "Running tests..."
	@go test -v ./...

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@go tool cover -func=coverage.out | tail -1

## bench: Run benchmark tests
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

## check: Run all checks (fmt, vet, lint, test-race)
check: fmt vet lint test-race
	@echo "All checks passed!"

## clean: Remove build artifacts and test coverage files
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@rm -f mem.prof cpu.prof block.prof mutex.prof
	@echo "Clean complete"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

## deps-update: Update dependencies
deps-update:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

## profile-cpu: Run CPU profiling
profile-cpu:
	@echo "Running CPU profiling..."
	@go test -cpuprofile=cpu.prof -bench=. ./...
	@echo "Profile saved to cpu.prof"
	@echo "View with: go tool pprof cpu.prof"

## profile-mem: Run memory profiling
profile-mem:
	@echo "Running memory profiling..."
	@go test -memprofile=mem.prof -bench=. ./...
	@echo "Profile saved to mem.prof"
	@echo "View with: go tool pprof mem.prof"

## ci: Run CI checks (used in CI/CD pipeline)
ci: deps check test-coverage
	@echo "CI checks complete"
