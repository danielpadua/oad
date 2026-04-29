.DEFAULT_GOAL := build

# ──────────────────────────────────────────────────────────────────────────────
# Configuration
# ──────────────────────────────────────────────────────────────────────────────
BINARY         := bin/oad
VERSION        := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LD_FLAGS       := -w -s \
                  -X github.com/danielpadua/oad/cmd/oad/cmd.version=$(VERSION) \
                  -X github.com/danielpadua/oad/cmd/oad/cmd.commit=$(COMMIT) \
                  -X github.com/danielpadua/oad/cmd/oad/cmd.buildDate=$(BUILD_DATE)
GO_BUILD_FLAGS := -ldflags="$(LD_FLAGS)"
DATABASE_URL   ?= postgresql://oad:oad@localhost:5432/oad?sslmode=disable

.PHONY: build web-build dev test test-cover lint setup clean \
        migrate-up migrate-down migrate-status \
        docker-build ui-dev ui-install format format-check pre-commit help

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────

## web-build: Build the Management UI (output: internal/webui/dist/)
web-build:
	cd web && npm run build

## build: Build the Management UI then compile the OAD binary to ./bin/oad
build: web-build
	@mkdir -p bin
	go build $(GO_BUILD_FLAGS) -o $(BINARY) ./cmd/oad

# ──────────────────────────────────────────────────────────────────────────────
# Development
# ──────────────────────────────────────────────────────────────────────────────

## dev: Start a development stack. Usage: make dev STACK=multi-idp|single-idp
##   - multi-idp:  API + Keycloak + Dex + glauth + PostgreSQL
##   - single-idp: API + Keycloak + PostgreSQL
STACK ?=
DEV_STACKS := multi-idp single-idp
dev:
	@if [ -z "$(STACK)" ]; then \
		echo "ERROR: STACK is required. Usage: make dev STACK=<name>"; \
		echo "Available stacks: $(DEV_STACKS)"; \
		exit 2; \
	fi
	@if [ ! -f "deployments/$(STACK)/docker-compose.yml" ]; then \
		echo "ERROR: unknown stack '$(STACK)'. Available: $(DEV_STACKS)"; \
		exit 2; \
	fi
	docker compose -f deployments/$(STACK)/docker-compose.yml up --build


# ──────────────────────────────────────────────────────────────────────────────
# Testing
# ──────────────────────────────────────────────────────────────────────────────

## test: Run all tests with race detector
test:
	go test -race ./...

## test-cover: Run tests and open the HTML coverage report
test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# ──────────────────────────────────────────────────────────────────────────────
# Code quality
# ──────────────────────────────────────────────────────────────────────────────

## format: Format Go code with gofumpt
format:
	gofumpt -l -w ./cmd/ ./internal/

## format-check: Check if Go code is formatted (used by pre-commit)
format-check:
	@if ! command -v gofumpt &>/dev/null; then \
		echo "WARNING: gofumpt not found, skipping format check."; \
		echo "  Install with: go install mvdan.cc/gofumpt@latest"; \
	else \
		unformatted=$$(gofumpt -l ./cmd/ ./internal/); \
		if [ -n "$$unformatted" ]; then \
			echo "FAIL: gofumpt found unformatted files:"; \
			echo "$$unformatted" | sed 's/^/  /'; \
			echo "Run 'make format' to fix."; \
			exit 1; \
		fi \
	fi

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run ./cmd/... ./internal/...

## pre-commit: Run code quality checks (called by git hook)
pre-commit: format-check lint

# ──────────────────────────────────────────────────────────────────────────────
# Setup
# ──────────────────────────────────────────────────────────────────────────────

## setup: Install git hooks via core.hooksPath (run once after cloning)
setup:
	git config core.hooksPath .githooks
	@echo "Git hooks installed: .githooks/pre-commit is now active."
	@echo "Note: .git/hooks/pre-commit is superseded and can be removed."

# ──────────────────────────────────────────────────────────────────────────────
# Migrations (requires golang-migrate CLI)
# Usage: make migrate-up DATABASE_URL=postgresql://...
# ──────────────────────────────────────────────────────────────────────────────

## migrate-up: Apply all pending migrations
migrate-up:
	migrate -path ./migrations -database "$(DATABASE_URL)" up

## migrate-down: Roll back the last migration
migrate-down:
	migrate -path ./migrations -database "$(DATABASE_URL)" down 1

## migrate-status: Show current migration version
migrate-status:
	migrate -path ./migrations -database "$(DATABASE_URL)" version

# ──────────────────────────────────────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────────────────────────────────────

## docker-build: Build the production Docker image
docker-build:
	docker build -t oad:local .

# ──────────────────────────────────────────────────────────────────────────────
# Cleanup
# ──────────────────────────────────────────────────────────────────────────────

## clean: Remove compiled artifacts and test output
clean:
	rm -rf bin/ coverage.out coverage.html

# ──────────────────────────────────────────────────────────────────────────────
# Management UI (web/)
# ──────────────────────────────────────────────────────────────────────────────

## ui-install: Install frontend dependencies
ui-install:
	cd web && npm ci

## ui-dev: Start the Vite dev server for the Management UI (requires API running)
ui-dev:
	cd web && npm run dev

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
