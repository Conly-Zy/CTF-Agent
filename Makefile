.PHONY: build build-frontend dev-frontend run test clean fmt lint deps

BINARY=ctf-agent
BUILD_DIR=./bin

# Build everything: frontend + Go binary
build: build-frontend
	@mkdir -p $(BUILD_DIR)
	@rm -rf cmd/ctf-agent/web_dist
	@cp -r web/dist cmd/ctf-agent/web_dist
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/ctf-agent
	@rm -rf cmd/ctf-agent/web_dist
	@mkdir -p cmd/ctf-agent/web_dist && touch cmd/ctf-agent/web_dist/.gitkeep
	@echo "Build complete: $(BUILD_DIR)/$(BINARY)"

# Build frontend only
build-frontend:
	cd web && npm install && npm run build

# Dev mode: run frontend dev server + Go backend separately
dev-frontend:
	cd web && npm run dev

dev-backend:
	go run ./cmd/ctf-agent server --config config.yaml

run: build
	$(BUILD_DIR)/$(BINARY) $(ARGS)

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR) web/dist web/node_modules

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

deps:
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  build          - Build frontend + Go binary"
	@echo "  build-frontend - Build frontend only"
	@echo "  dev-frontend   - Start frontend dev server (with hot reload)"
	@echo "  dev-backend    - Start Go backend server"
	@echo "  run            - Build and run (pass ARGS for arguments)"
	@echo "  test           - Run tests"
	@echo "  clean          - Remove build artifacts"
	@echo "  fmt            - Format code"
	@echo "  lint           - Run linter"
	@echo "  deps           - Tidy dependencies"
