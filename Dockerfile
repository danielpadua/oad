# =============================================================================
# Stage: web-builder — compiles the React/Vite Management UI.
# Output lands in internal/webui/dist/ so the //go:embed directive picks it up.
# =============================================================================
FROM node:22-alpine AS web-builder

WORKDIR /app/web

# Install dependencies first — cached separately from source changes.
# `npm install` (not `npm ci`) is required because npm 10.x — bundled with
# node:22-alpine — incorrectly flags bundleDependencies under strict ci mode
# (e.g. @tailwindcss/oxide-wasm32-wasi → @emnapi/core).
COPY web/package*.json ./
RUN npm install --no-audit --no-fund

COPY web/ ./
RUN npm run build
# Output: /app/internal/webui/dist/

# =============================================================================
# Stage: go-builder — compiles the Go binary with the embedded UI.
# The migrations directory is also embedded (//go:embed in migrations/embed.go).
# =============================================================================
FROM golang:1.25-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /app/internal/webui/dist/ ./internal/webui/dist/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux \
    go build \
    -ldflags="-w -s -extldflags=-static \
      -X github.com/danielpadua/oad/cmd/oad/cmd.version=${VERSION} \
      -X github.com/danielpadua/oad/cmd/oad/cmd.commit=${COMMIT} \
      -X github.com/danielpadua/oad/cmd/oad/cmd.buildDate=${BUILD_DATE}" \
    -o /oad \
    ./cmd/oad

# =============================================================================
# Target: dev — used by docker compose stacks under deployments/.
# Compiles everything (UI + binary) inside the image so that contributors only
# need Docker installed — no local Node or Go toolchain required.
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS dev

COPY --from=go-builder /oad /oad

EXPOSE 8080

ENTRYPOINT ["/oad"]

# =============================================================================
# Target: release — used by GoReleaser. Expects the `oad` binary to exist in
# the build context (already cross-compiled by GoReleaser, with the Management
# UI embedded via the `before:` hook that runs `npm run build`). No in-image
# compilation occurs here; OCI labels are added by GoReleaser via build flags.
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS release

COPY oad /oad

EXPOSE 8080

ENTRYPOINT ["/oad"]
