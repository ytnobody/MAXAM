.PHONY: build test lint clean run dist dist-all

# Variables
BINARY_NAME := maxam
BINARY_DIR := bin
DIST_DIR := dist
CMD_DIR := ./cmd/maxam
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Target platforms
PLATFORMS := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

# Build the binary
build:
	go build -ldflags="-X main.Version=$(VERSION)" -o $(BINARY_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Run tests
test:
	go test ./...

# Run linter (go vet)
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR) $(DIST_DIR)

# Build and run
run: build
	./$(BINARY_DIR)/$(BINARY_NAME)

# Cross-compile for a single platform
# Usage: make dist GOOS=linux GOARCH=amd64
dist:
	@if [ -z "$(GOOS)" ] || [ -z "$(GOARCH)" ]; then \
		echo "Usage: make dist GOOS=<os> GOARCH=<arch>"; \
		exit 1; \
	fi
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	$(eval PKG_NAME := $(BINARY_NAME)-$(VERSION)-$(GOOS)-$(GOARCH))
	@echo "Building $(PKG_NAME)..."
	@mkdir -p $(DIST_DIR)/$(PKG_NAME)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST_DIR)/$(PKG_NAME)/$(BINARY_NAME)$(EXT) $(CMD_DIR)
	@cp CLAUDE.md $(DIST_DIR)/$(PKG_NAME)/
	@cd $(DIST_DIR) && zip -r $(PKG_NAME).zip $(PKG_NAME)
	@rm -rf $(DIST_DIR)/$(PKG_NAME)
	@echo "Created $(DIST_DIR)/$(PKG_NAME).zip"

# Build for all platforms
dist-all:
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} $(MAKE) dist; \
	done
	@echo "All packages created in $(DIST_DIR)/"
