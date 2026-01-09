# AgentManager Makefile

.PHONY: all build build-cli build-helper clean test test-verbose test-pkg test-unit test-coverage test-coverage-summary test-short test-integration benchmark lint install fmt vet deps

# Detect OS
ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
    EXE_EXT := .exe
    RM_CMD := cmd /c if exist bin rmdir /s /q bin
    RM_DIST_CMD := cmd /c if exist dist rmdir /s /q dist
    MKDIR_CMD := cmd /c if not exist coverage mkdir coverage
    DATE_CMD := $(shell powershell -Command "[DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')")
    NULL_REDIRECT := 2>NUL
    PATH_SEP := \\
    # Race detection requires CGO which is not available on Windows by default
    RACE_FLAG :=
else
    DETECTED_OS := $(shell uname -s)
    EXE_EXT :=
    RM_CMD := rm -rf bin/
    RM_DIST_CMD := rm -rf dist/
    MKDIR_CMD := mkdir -p coverage
    DATE_CMD := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
    NULL_REDIRECT := 2>/dev/null
    PATH_SEP := /
    RACE_FLAG := -race
endif

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty $(NULL_REDIRECT) || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD $(NULL_REDIRECT) || echo "none")
DATE ?= $(DATE_CMD)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go settings
GOBIN ?= $(shell go env GOPATH)$(PATH_SEP)bin

# Default target
all: build

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build both binaries
build: build-cli build-helper

# Build CLI binary
build-cli:
	@echo "Building agentmgr..."
	go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr$(EXE_EXT) ./cmd/agentmgr

# Build helper binary
build-helper:
	@echo "Building agentmgr-helper..."
	go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr-helper$(EXE_EXT) ./cmd/agentmgr-helper

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	GOOS=darwin GOARCH=amd64 go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr-darwin-amd64 ./cmd/agentmgr
	GOOS=darwin GOARCH=arm64 go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr-darwin-arm64 ./cmd/agentmgr
	GOOS=linux GOARCH=amd64 go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr-linux-amd64 ./cmd/agentmgr
	GOOS=linux GOARCH=arm64 go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr-linux-arm64 ./cmd/agentmgr
	GOOS=windows GOARCH=amd64 go build -buildvcs=false $(LDFLAGS) -o bin/agentmgr-windows-amd64.exe ./cmd/agentmgr

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(RM_CMD)
	$(RM_DIST_CMD)

# Run tests
test:
	go test $(RACE_FLAG) -cover ./...

# Run tests with verbose output
test-verbose:
	go test $(RACE_FLAG) -cover -v ./...

# Run tests for specific package (usage: make test-pkg PKG=agent)
test-pkg:
	go test $(RACE_FLAG) -cover -v ./pkg/$(PKG)/...

# Run unit tests only (pkg packages)
test-unit:
	go test $(RACE_FLAG) -cover ./pkg/...

# Run tests with coverage report
test-coverage:
	$(MKDIR_CMD)
	go test $(RACE_FLAG) -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

# Run tests with coverage summary
test-coverage-summary:
	go test $(RACE_FLAG) -cover ./...

# Run short tests (skip slow tests)
test-short:
	go test $(RACE_FLAG) -short ./...

# Run integration tests
test-integration:
	go test $(RACE_FLAG) -v -tags=integration ./...

# Benchmark tests
benchmark:
	go test -bench=. -benchmem ./...

# Run linter
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Install to GOBIN
install: build
	@echo "Installing to $(GOBIN)..."
ifeq ($(OS),Windows_NT)
	copy bin\agentmgr.exe "$(GOBIN)\agentmgr.exe"
	copy bin\agentmgr-helper.exe "$(GOBIN)\agentmgr-helper.exe"
else
	install -m 755 bin/agentmgr $(GOBIN)/
	install -m 755 bin/agentmgr-helper $(GOBIN)/
endif

# Install to /usr/local/bin (Unix only, requires sudo)
install-system: build
ifeq ($(OS),Windows_NT)
	@echo "install-system is not supported on Windows. Use 'make install' instead."
else
	@echo "Installing to /usr/local/bin..."
	sudo install -m 755 bin/agentmgr /usr/local/bin/
	sudo install -m 755 bin/agentmgr-helper /usr/local/bin/
endif

# Run the CLI
run: build-cli
	./bin/agentmgr$(EXE_EXT) $(ARGS)

# Generate code (protobufs, etc.)
generate:
	@echo "Generating code..."
	go generate ./...

# Check everything
check: fmt vet lint test

# Development helpers
dev: deps build
	@echo "Development build complete"
	./bin/agentmgr$(EXE_EXT) version

# Show help
help:
	@echo "AgentManager Makefile"
	@echo ""
	@echo "Build Targets:"
	@echo "  all              Build all binaries (default)"
	@echo "  build            Build agentmgr and agentmgr-helper"
	@echo "  build-cli        Build agentmgr only"
	@echo "  build-helper     Build agentmgr-helper only"
	@echo "  build-all        Build for all platforms"
	@echo "  clean            Remove build artifacts"
	@echo ""
	@echo "Test Targets:"
	@echo "  test             Run tests with race detection and coverage"
	@echo "  test-verbose     Run tests with verbose output"
	@echo "  test-pkg PKG=x   Run tests for specific package (e.g., PKG=agent)"
	@echo "  test-unit        Run unit tests (pkg packages only)"
	@echo "  test-coverage    Run tests and generate HTML coverage report"
	@echo "  test-coverage-summary  Run tests and show coverage summary"
	@echo "  test-short       Run short tests (skip slow tests)"
	@echo "  test-integration Run integration tests"
	@echo "  benchmark        Run benchmark tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint             Run linter"
	@echo "  fmt              Format code"
	@echo "  vet              Run go vet"
	@echo "  check            Run all checks (fmt, vet, lint, test)"
	@echo ""
	@echo "Other:"
	@echo "  deps             Download and tidy dependencies"
	@echo "  install          Install to GOBIN"
	@echo "  install-system   Install to /usr/local/bin (Unix only)"
	@echo "  run ARGS=...     Build and run CLI"
	@echo "  dev              Development build"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION          Version string (default: git describe)"
	@echo "  ARGS             Arguments for 'make run'"
	@echo "  PKG              Package name for 'make test-pkg'"
	@echo ""
	@echo "Detected OS: $(DETECTED_OS)"
