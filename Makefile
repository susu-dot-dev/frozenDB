# frozenDB Makefile

.PHONY: ci deps tidy fmt lint test build build-cli build-examples clean clean-cli bump-version

# Build output directory
DIST_DIR := dist

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

## Build the frozendb CLI binary
build-cli:
	@echo "Building frozendb CLI..."
	@mkdir -p $(DIST_DIR)
	go build -o $(DIST_DIR)/frozendb ./cmd/frozendb

## Build example binaries
build-examples:
	@echo "Building examples..."
	@mkdir -p $(DIST_DIR)/examples
	go build -o $(DIST_DIR)/examples/getting_started ./examples/getting_started
	@echo "Copying sample database..."
	cp examples/getting_started/sample.fdb $(DIST_DIR)/examples/

## Run all checks and build
ci: deps tidy fmt lint test build build-cli build-examples
	@echo "CI pipeline completed successfully!"

## Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html
	go clean -cache

## Clean CLI binary
clean-cli:
	@echo "Cleaning frozendb CLI binary..."
	rm -f $(DIST_DIR)/frozendb

## Bump version and create release branch
bump-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is required"; \
		echo "Usage: make bump-version VERSION=v0.1.0"; \
		exit 1; \
	fi
	@echo "Bumping version to $(VERSION)..."
	@bash scripts/bump-version.sh $(VERSION)

## Help target
help:
	@echo "Available targets:"
	@echo "  ci            - Run all checks and build"
	@echo "  deps          - Install dependencies"
	@echo "  tidy          - Clean up dependencies"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  test-spec     - Run spec tests only"
	@echo "  test-unit     - Run unit tests only"
	@echo "  build         - Build the project"
	@echo "  build-cli     - Build the frozendb CLI binary (output: dist/frozendb)"
	@echo "  build-examples- Build example binaries (output: dist/examples/)"
	@echo "  clean         - Clean all build artifacts"
	@echo "  clean-cli     - Clean CLI binary only"
	@echo "  bump-version  - Bump version and create release branch (Usage: make bump-version VERSION=v0.1.0)"
	@echo "  help          - Show this help message"
