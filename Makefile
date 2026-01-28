.PHONY: build test lint clean run

# Variables
BINARY_NAME := maxam
BINARY_DIR := bin
CMD_DIR := ./cmd/maxam

# Build the binary
build:
	go build -o $(BINARY_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Run tests
test:
	go test ./...

# Run linter (go vet)
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)

# Build and run
run: build
	./$(BINARY_DIR)/$(BINARY_NAME)
