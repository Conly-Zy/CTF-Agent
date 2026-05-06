.PHONY: build build-frontend dev-frontend dev-backend run test clean fmt lint deps docker docker-build docker-up docker-up-build docker-down docker-logs config setup quickstart help

BINARY ?= ctf-agent
BUILD_DIR ?= ./bin
IMAGE ?= ghcr.io/conly-zy/ctf-agent
TAG ?= latest
COMPOSE ?= docker compose

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
	cd web && npm ci && npm run build

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
	rm -rf $(BUILD_DIR) web/dist web/node_modules cmd/ctf-agent/web_dist

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

deps:
	go mod tidy

# Build local Docker image
docker:
	docker build -t $(IMAGE):$(TAG) .

docker-build:
	$(COMPOSE) -f docker-compose.yml -f docker-compose.build.yml build

# Start published GHCR image. Run `make setup` first on a fresh environment.
docker-up:
	$(COMPOSE) up -d

# Build locally and start with Compose.
docker-up-build:
	$(COMPOSE) -f docker-compose.yml -f docker-compose.build.yml up -d --build

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f ctf-agent

config:
	@test -f config.yaml || cp config.yaml.example config.yaml
	@echo "Config ready: config.yaml"

setup quickstart:
	./scripts/bootstrap.sh

help:
	@echo "Available targets:"
	@echo "  build           - Build frontend + Go binary"
	@echo "  build-frontend  - Build frontend only"
	@echo "  dev-frontend    - Start frontend dev server (hot reload)"
	@echo "  dev-backend     - Start Go backend server"
	@echo "  run             - Build and run (pass ARGS='...' for arguments)"
	@echo "  test            - Run Go tests"
	@echo "  clean           - Remove build artifacts"
	@echo "  fmt             - Format Go code"
	@echo "  lint            - Run golangci-lint"
	@echo "  deps            - Tidy Go dependencies"
	@echo "  docker          - Build Docker image locally"
	@echo "  docker-build    - Build Compose image locally"
	@echo "  docker-up       - Start Compose stack using published image"
	@echo "  docker-up-build - Build locally and start Compose stack"
	@echo "  docker-down     - Stop Compose stack"
	@echo "  docker-logs     - Follow ctf-agent logs"
	@echo "  config          - Generate config.yaml from example"
	@echo "  setup           - One-command fresh environment bootstrap"
