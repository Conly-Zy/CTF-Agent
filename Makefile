.PHONY: build run test clean

BINARY=ctf-agent
BUILD_DIR=./bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/ctf-agent

run: build
	$(BUILD_DIR)/$(BINARY) $(ARGS)

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

deps:
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  build   - Build the binary"
	@echo "  run     - Build and run (pass ARGS for arguments)"
	@echo "  test    - Run tests"
	@echo "  clean   - Remove build artifacts"
	@echo "  fmt     - Format code"
	@echo "  lint    - Run linter"
	@echo "  deps    - Tidy dependencies"
