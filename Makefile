# frozenDB Makefile

.PHONY: ci deps tidy fmt lint test build clean

# Default target
all: ci

## Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download

## Clean up dependencies
tidy:
	@echo "Tidying go.mod..."
	go mod tidy

## Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## Run linter
lint:
	golangci-lint run

## Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## Run spec tests only
test-spec:
	@echo "Running spec tests..."
	go test -v ./... -run "^Test_S_"

## Run unit tests only
test-unit:
	@echo "Running unit tests..."
	go test -v ./... -run "^Test[^S]"

## Build the project
build:
	@echo "Building project..."
	go build ./...

## Run all checks and build
ci: deps tidy fmt lint test build
	@echo "CI pipeline completed successfully!"

## Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -f coverage.out coverage.html
	go clean -cache

## Help target
help:
	@echo "Available targets:"
	@echo "  ci           - Run all checks and build"
	@echo "  deps         - Install dependencies"
	@echo "  tidy         - Clean up dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  test-spec    - Run spec tests only"
	@echo "  test-unit    - Run unit tests only"
	@echo "  build        - Build the project"
	@echo "  clean        - Clean build artifacts"
	@echo "  help         - Show this help message"
