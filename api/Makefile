# Makefile for CodeRunr API Go implementation

# Variables
BINARY_NAME=coderunr-api
MAIN_PATH=./cmd/server
BUILD_DIR=./build
DATA_DIR=/opt/coderunr

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-w -s"
BUILD_FLAGS=-trimpath $(LDFLAGS)

.PHONY: help build test clean run deps tidy dev docker

# Default target
help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

run: build ## Build and run the server
	@echo "Running $(BINARY_NAME)..."
	@mkdir -p $(DATA_DIR)/packages
	$(BUILD_DIR)/$(BINARY_NAME)

dev: ## Run in development mode with test data
	@echo "Running in development mode..."
	@mkdir -p /tmp/coderunr-dev/packages
	CODERUNR_DATA_DIRECTORY=/tmp/coderunr-dev \
	CODERUNR_LOG_LEVEL=debug \
	CODERUNR_BIND_ADDRESS=localhost:2000 \
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-dev $(MAIN_PATH) && \
	$(BUILD_DIR)/$(BINARY_NAME)-dev

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

fmt: ## Format code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

lint: ## Run golangci-lint (requires golangci-lint to be installed)
	@echo "Running linter..."
	golangci-lint run

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t coderunr-api:latest .

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	docker run -p 2000:2000 -v $(DATA_DIR):/opt/coderunr coderunr-api:latest

# Release targets
release-linux: ## Build for Linux
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

release-darwin: ## Build for macOS
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

release-windows: ## Build for Windows
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

release-all: release-linux release-darwin release-windows ## Build for all platforms

# Installation targets
install: build ## Install binary to system
	@echo "Installing $(BINARY_NAME)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

uninstall: ## Uninstall binary from system
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstalled $(BINARY_NAME)"

# Setup targets
setup-dev: ## Setup development environment
	@echo "Setting up development environment..."
	$(GOMOD) tidy
	@mkdir -p /tmp/coderunr-dev/packages
	@echo "Development environment ready!"

# Benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...
