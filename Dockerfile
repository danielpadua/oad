# =============================================================================
# Stage 1 — Builder
# Compiles the Go binary with all optimizations.
# The migrations directory is embedded into the binary (//go:embed),
# so no additional COPY is needed in the runtime stage.
# =============================================================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first — this layer is cached separately from source code
# so a source change does not re-download all modules.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags=-static" \
    -o /oad \
    ./cmd/api

# =============================================================================
# Stage 2 — Runtime (distroless)
# Minimal attack surface: no shell, no package manager, no unnecessary binaries.
# Runs as non-root user (UID 65532) by default in the nonroot variant.
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /oad /oad

EXPOSE 8080

ENTRYPOINT ["/oad"]
