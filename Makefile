.DEFAULT_GOAL := build

# ──────────────────────────────────────────────────────────────────────────────
# Configuration
# ──────────────────────────────────────────────────────────────────────────────
BINARY         := bin/oad
GO_BUILD_FLAGS := -ldflags="-w -s"
DATABASE_URL   ?= postgresql://oad:oad@localhost:5432/oad?sslmode=disable

.PHONY: build dev dev-db test test-cover lint clean \
        migrate-up migrate-down migrate-status \
        docker-build

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────

## build: Compile the API binary to ./bin/oad
build:
	@mkdir -p bin
	go build $(GO_BUILD_FLAGS) -o $(BINARY) ./cmd/api

# ──────────────────────────────────────────────────────────────────────────────
# Development
# ──────────────────────────────────────────────────────────────────────────────

## dev: Start the full stack (API + PostgreSQL) with docker compose
dev:
	docker compose up --build

## dev-db: Start only the PostgreSQL container
dev-db:
	docker compose up postgres

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

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run ./...

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

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
