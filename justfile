# ABOUTME: Development task runner for the Ritual workflow engine
# ABOUTME: Provides common commands for building, testing, and running the application

# Build the ritual binary
build:
	@echo "Building ritual..."
	go build -o bin/ritual ./cmd/ritual

# Run tests with coverage
test:
	@echo "Running tests..."
	go test -v -cover ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code for issues
vet:
	@echo "Vetting code..."
	go vet ./...

# Run linter (if golangci-lint is available)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, skipping..."; \
	fi

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/

# Run with example workflow (once implemented)
run-example:
	@echo "Running example workflow..."
	@if [ -f examples/simple.yaml ]; then \
		go run ./cmd/ritual run examples/simple.yaml; \
	else \
		echo "No example workflow found yet"; \
	fi

# Run dry-run with example
dry-run-example:
	@echo "Dry-run example workflow..."
	@if [ -f examples/simple.yaml ]; then \
		go run ./cmd/ritual dry-run examples/simple.yaml; \
	else \
		echo "No example workflow found yet"; \
	fi

# Validate example workflow
validate-example:
	@echo "Validating example workflow..."
	@if [ -f examples/simple.yaml ]; then \
		go run ./cmd/ritual validate examples/simple.yaml; \
	else \
		echo "No example workflow found yet"; \
	fi

# Install the binary to GOPATH/bin
install:
	@echo "Installing ritual..."
	go install ./cmd/ritual

# Run all checks (format, vet, test)
check: fmt vet test

# Full CI pipeline
ci: tidy check lint build

# Show available commands
help:
	@echo "Available commands:"
	@echo "  build         - Build the ritual binary"
	@echo "  test          - Run tests with coverage"
	@echo "  test-race     - Run tests with race detection"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code for issues"
	@echo "  lint          - Run golangci-lint (if available)"
	@echo "  tidy          - Tidy dependencies"
	@echo "  clean         - Clean build artifacts"
	@echo "  run-example   - Run example workflow"
	@echo "  dry-run-example - Dry-run example workflow"
	@echo "  validate-example - Validate example workflow"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  check         - Run fmt, vet, and test"
	@echo "  ci            - Full CI pipeline"
	@echo "  help          - Show this help"