# Makefile for smbput - SMB client with binary size optimization

# Binary name
BINARY := smbput

# Build flags for minimal binary size
LDFLAGS := -s -w
GOFLAGS := -trimpath
BUILDTAGS :=

# Go build command with optimizations
GOBUILD := CGO_ENABLED=0 go build $(GOFLAGS) -ldflags="$(LDFLAGS)" $(if $(BUILDTAGS),-tags $(BUILDTAGS))

# Default target: build optimized binary
.PHONY: all
all: clean build

# Build optimized binary
.PHONY: build
build:
	@echo "Building optimized $(BINARY)..."
	@$(GOBUILD) -o $(BINARY)
	@ls -lh $(BINARY)
	@echo "Build complete!"

# Build with even more aggressive optimizations (experimental)
.PHONY: build-aggressive
build-aggressive:
	@echo "Building with aggressive optimizations..."
	@CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags="-s -w -extldflags=-static" \
		-gcflags="all=-l -B" \
		-o $(BINARY)
	@ls -lh $(BINARY)
	@echo "Aggressive build complete!"

# Install UPX compression tool
.PHONY: install-upx
install-upx:
	@echo "Installing UPX..."
	@if command -v upx >/dev/null 2>&1; then \
		echo "UPX is already installed: $$(upx --version | head -1)"; \
	elif command -v curl >/dev/null 2>&1; then \
		echo "Downloading UPX from GitHub releases..."; \
		cd /tmp && \
		curl -L -o upx.tar.xz https://github.com/upx/upx/releases/download/v4.2.4/upx-4.2.4-amd64_linux.tar.xz && \
		tar -xf upx.tar.xz && \
		install -m 755 upx-4.2.4-amd64_linux/upx /usr/local/bin/ && \
		rm -rf upx.tar.xz upx-4.2.4-amd64_linux && \
		echo "UPX installed successfully: $$(upx --version | head -1)"; \
	elif command -v apt-get >/dev/null 2>&1; then \
		echo "Installing via apt-get..."; \
		apt-get update && apt-get install -y upx-ucl; \
	else \
		echo "Cannot install UPX automatically. Please install manually."; \
		echo "Download from: https://github.com/upx/upx/releases"; \
		exit 1; \
	fi

# Build and compress with UPX (requires upx to be installed)
.PHONY: build-upx
build-upx: build
	@if command -v upx >/dev/null 2>&1; then \
		echo "Compressing binary with UPX..."; \
		upx --best --lzma $(BINARY); \
		ls -lh $(BINARY); \
		echo "UPX compression complete!"; \
	else \
		echo "UPX not found. Run 'make install-upx' or install with: apt-get install upx-ucl"; \
		exit 1; \
	fi

# Cross-compile for 32-bit ARM (Raspberry Pi, etc.)
.PHONY: build-arm
build-arm:
	@echo "Building for ARM (32-bit)..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build \
		-trimpath \
		-ldflags="$(LDFLAGS)" \
		-o $(BINARY)-arm
	@ls -lh $(BINARY)-arm
	@echo "ARM build complete!"

# Cross-compile for 64-bit ARM (ARM64)
.PHONY: build-arm64
build-arm64:
	@echo "Building for ARM64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
		-trimpath \
		-ldflags="$(LDFLAGS)" \
		-o $(BINARY)-arm64
	@ls -lh $(BINARY)-arm64
	@echo "ARM64 build complete!"

# Build for Windows
.PHONY: build-windows
build-windows:
	@echo "Building for Windows..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
		-trimpath \
		-ldflags="$(LDFLAGS)" \
		-o $(BINARY).exe
	@ls -lh $(BINARY).exe
	@echo "Windows build complete!"

# Build for macOS
.PHONY: build-macos
build-macos:
	@echo "Building for macOS..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
		-trimpath \
		-ldflags="$(LDFLAGS)" \
		-o $(BINARY)-macos
	@ls -lh $(BINARY)-macos
	@echo "macOS build complete!"

# Build for all platforms
.PHONY: build-all
build-all: build build-arm build-arm64 build-windows build-macos
	@echo "All builds complete!"
	@ls -lh $(BINARY)*

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY) $(BINARY)-* $(BINARY).exe
	@echo "Clean complete!"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# Show build size comparison
.PHONY: size-comparison
size-comparison:
	@echo "Building with different optimization levels..."
	@echo ""
	@echo "1. Standard build (with debug info):"
	@go build -o $(BINARY)-standard
	@ls -lh $(BINARY)-standard | awk '{print $$5 " - Standard build"}'
	@echo ""
	@echo "2. Optimized build (-ldflags=\"-s -w\" -trimpath):"
	@$(GOBUILD) -o $(BINARY)-optimized
	@ls -lh $(BINARY)-optimized | awk '{print $$5 " - Optimized build"}'
	@echo ""
	@echo "3. Aggressive build:"
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -extldflags=-static" -gcflags="all=-l -B" -o $(BINARY)-aggressive
	@ls -lh $(BINARY)-aggressive | awk '{print $$5 " - Aggressive build"}'
	@echo ""
	@if command -v upx >/dev/null 2>&1; then \
		echo "4. UPX compressed:"; \
		cp $(BINARY)-optimized $(BINARY)-upx; \
		upx --best --lzma $(BINARY)-upx 2>/dev/null; \
		ls -lh $(BINARY)-upx | awk '{print $$5 " - UPX compressed"}'; \
	fi
	@echo ""
	@rm -f $(BINARY)-standard $(BINARY)-optimized $(BINARY)-aggressive $(BINARY)-upx

# Install binary to /usr/local/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY) to /usr/local/bin..."
	@install -m 755 $(BINARY) /usr/local/bin/
	@echo "Installation complete!"

# Uninstall binary
.PHONY: uninstall
uninstall:
	@echo "Removing $(BINARY) from /usr/local/bin..."
	@rm -f /usr/local/bin/$(BINARY)
	@echo "Uninstall complete!"

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@echo "Dependencies ready!"

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make build              - Build optimized binary (default)"
	@echo "  make build-aggressive   - Build with aggressive optimizations"
	@echo "  make install-upx        - Install UPX compression tool"
	@echo "  make build-upx          - Build and compress with UPX"
	@echo "  make build-arm          - Cross-compile for 32-bit ARM"
	@echo "  make build-arm64        - Cross-compile for 64-bit ARM"
	@echo "  make build-windows      - Cross-compile for Windows"
	@echo "  make build-macos        - Cross-compile for macOS"
	@echo "  make build-all          - Build for all platforms"
	@echo "  make size-comparison    - Compare different optimization levels"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make test               - Run tests"
	@echo "  make install            - Install binary to /usr/local/bin"
	@echo "  make uninstall          - Remove binary from /usr/local/bin"
	@echo "  make deps               - Download and verify dependencies"
	@echo "  make help               - Show this help message"
