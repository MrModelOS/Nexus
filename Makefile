.PHONY: build install clean test lint dev help

BINARY=nex
INSTALL_DIR=$(HOME)/.local/bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build for current platform
build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) .
	@echo "Built $(BINARY) ($(VERSION))"

# Install to ~/.local/bin
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"

# Build for all platforms
build-all:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Built all platforms in dist/"

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run

# Development build and run
dev: build
	./$(BINARY)

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

# Show help
help:
	@echo "Nexus - The Most Powerful AI CLI Tool"
	@echo ""
	@echo "Targets:"
	@echo "  build      - Build for current platform"
	@echo "  install    - Install to ~/.local/bin"
	@echo "  build-all  - Build for all platforms"
	@echo "  test       - Run tests"
	@echo "  lint       - Run linter"
	@echo "  dev        - Build and run"
	@echo "  clean      - Remove build artifacts"
	@echo "  help       - Show this help"
	@echo ""
	@echo "Version: $(VERSION)"
