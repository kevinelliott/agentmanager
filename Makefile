# AgentManager Makefile

.PHONY: all build build-cli build-helper build-macos-app clean test test-verbose test-pkg test-unit test-coverage test-coverage-summary test-short test-integration benchmark lint install fmt vet deps sync-catalog check-catalog-sync

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go settings
GOBIN ?= $(shell go env GOPATH)/bin

# Pinned tool versions (keep in sync with .github/workflows/ci.yml)
GOLANGCI_LINT_VERSION ?= v2.11.4

# Default target
all: build

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build both binaries
build: sync-catalog build-cli build-helper

# Keep the //go:embed copy in sync with the canonical catalog at repo root.
# The repo-root catalog.json is the contributor-facing source AND the URL
# endpoint served from the `main` branch; pkg/catalog/catalog.json is the
# build-time copy baked into the binary via //go:embed. We sync here rather
# than symlinking because go:embed does not follow symlinks outside its
# package directory.
sync-catalog:
	@cp catalog.json pkg/catalog/catalog.json

# CI guard: errors if the two copies drift. Run as its own step so
# contributors who edit only one get a fast, deterministic nudge.
check-catalog-sync:
	@if ! diff -q catalog.json pkg/catalog/catalog.json >/dev/null; then \
		echo "ERROR: catalog.json and pkg/catalog/catalog.json differ. Run 'make sync-catalog' and commit."; \
		diff -u catalog.json pkg/catalog/catalog.json | head -60; \
		exit 1; \
	fi

# Build CLI binary
build-cli:
	@echo "Building agentmgr..."
	go build $(LDFLAGS) -o bin/agentmgr ./cmd/agentmgr

# Build helper binary
build-helper:
	@echo "Building agentmgr-helper..."
	go build $(LDFLAGS) -o bin/agentmgr-helper ./cmd/agentmgr-helper

# Build macOS app bundle for helper (systray support)
build-macos-app: build-helper
	@echo "Building macOS app bundle..."
	@./scripts/build-macos-app.sh "$(VERSION)" "$(COMMIT)" "$(DATE)"

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/agentmgr-darwin-amd64 ./cmd/agentmgr
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/agentmgr-darwin-arm64 ./cmd/agentmgr
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/agentmgr-linux-amd64 ./cmd/agentmgr
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/agentmgr-linux-arm64 ./cmd/agentmgr
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/agentmgr-windows-amd64.exe ./cmd/agentmgr

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf dist/

# Run tests
test:
	go test -race -cover ./...

# Run tests with verbose output
test-verbose:
	go test -race -cover -v ./...

# Run tests for specific package
test-pkg:
	@if [ -z "$(PKG)" ]; then echo "Usage: make test-pkg PKG=agent"; exit 1; fi
	go test -race -cover -v ./pkg/$(PKG)/...

# Run unit tests only (pkg packages)
test-unit:
	go test -race -cover ./pkg/...

# Run tests with coverage report
test-coverage:
	@mkdir -p coverage
	go test -race -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

# Run tests with coverage summary
test-coverage-summary:
	go test -race -cover ./... 2>&1 | grep -E "^ok|coverage"

# Run short tests (skip slow tests)
test-short:
	go test -race -short ./...

# Run integration tests
test-integration:
	go test -race -v -tags=integration ./...

# Benchmark tests
benchmark:
	go test -bench=. -benchmem ./...

# Run linter
# Pins to the same golangci-lint version CI uses; reinstalls if the local
# binary is missing or on a different version, so `make lint` matches CI.
lint:
	@installed=$$($(GOBIN)/golangci-lint version --format=short 2>/dev/null | sed 's/^v//'); \
	want=$$(echo $(GOLANGCI_LINT_VERSION) | sed 's/^v//'); \
	if [ "$$installed" != "$$want" ]; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) (had: $${installed:-missing})..."; \
		GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	$(GOBIN)/golangci-lint run --timeout=5m

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Install to system
install: build
	@echo "Installing to $(GOBIN)..."
	install -m 755 bin/agentmgr $(GOBIN)/
	install -m 755 bin/agentmgr-helper $(GOBIN)/

# Install to /usr/local/bin (requires sudo)
install-system: build
	@echo "Installing to /usr/local/bin..."
	sudo install -m 755 bin/agentmgr /usr/local/bin/
	sudo install -m 755 bin/agentmgr-helper /usr/local/bin/

# Run the CLI
run: build-cli
	./bin/agentmgr $(ARGS)

# Generate code (protobufs, etc.)
generate:
	@echo "Generating code..."
	go generate ./...

# Check everything
check: fmt vet lint test

# Development helpers
dev: deps build
	@echo "Development build complete"
	./bin/agentmgr version

# Show help
help:
	@echo "AgentManager Makefile"
	@echo ""
	@echo "Build Targets:"
	@echo "  all              Build all binaries (default)"
	@echo "  build            Build agentmgr and agentmgr-helper"
	@echo "  build-cli        Build agentmgr only"
	@echo "  build-helper     Build agentmgr-helper only"
	@echo "  build-macos-app  Build macOS .app bundle for helper (systray)"
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
	@echo "  install-system   Install to /usr/local/bin"
	@echo "  run ARGS=...     Build and run CLI"
	@echo "  dev              Development build"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION          Version string (default: git describe)"
	@echo "  ARGS             Arguments for 'make run'"
	@echo "  PKG              Package name for 'make test-pkg'"
