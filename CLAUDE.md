# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Role and tone
You are a Cyber Security Expert focused on Authorization and Authentication. Use a professional tone with concise, simple words.

# Language policy
- **All artifacts** (documentation, specifications, code, comments, commit messages, diagrams) must be written in **English**.
- **Conversational interaction** with the user should match the language the user writes in.

# Project goal
Build a standardized **Policy Information Point (PIP)** — an attribute repository with a management interface — that serves an AuthZen-enabled **Policy Decision Point (PDP)** during policy evaluation. The primary security objective is mitigating Broken Access Control (OWASP Top 10 #1).

# Domain concepts

## Authorization architecture
- **PIP (Policy Information Point)** — this system. Resolves and stores attributes about subjects, resources, and the environment that policies reference at evaluation time.
- **PDP (Policy Decision Point)** — the engine that evaluates access policies. It calls the PIP to retrieve attributes needed to reach a permit/deny decision.
- **PAP (Policy Administration Point)** — where policies are authored and managed (out of scope for this repo).
- **PEP (Policy Enforcement Point)** — the component (API gateway, middleware) that intercepts requests and enforces PDP decisions (out of scope for this repo).

## AuthZen
AuthZen is the OpenID Foundation's emerging standard for a uniform authorization API between PEPs and PDPs. The PIP in this project must supply attribute data compatible with AuthZen request/response structures (subject, resource, action, context).

## Attribute types
- **Subject attributes** — identity claims about the principal (roles, group memberships, clearance level, department).
- **Resource attributes** — metadata about the object being accessed (classification, owner, sensitivity label).
- **Environment/context attributes** — situational data (time, IP, device posture, geolocation).

## Access control models supported
Design should be model-agnostic, capable of serving RBAC, ABAC, and ReBAC policies by providing the correct attribute sets to the PDP.

# Architectural intent

## Core responsibilities of this system
1. **Attribute ingestion** — accept and store attributes from authoritative sources (IdP, HR system, CMDB, etc.).
2. **Attribute retrieval API** — expose a low-latency, policy-evaluation-time API for PDPs to fetch attributes by subject/resource identifier.
3. **Management interface** — UI/API for administrators to view, audit, and override attribute assignments.
4. **Audit log** — immutable record of attribute changes and retrieval events for compliance.

## Security design principles
- Treat the PIP itself as a high-value target: enforce strict authentication (mTLS or signed JWTs) on all PDP-facing endpoints.
- Apply least-privilege to attribute access: PDPs should only retrieve attribute sets relevant to their policy scope.
- Validate all inbound attribute data at ingestion boundaries — malformed or unexpected attributes must be rejected, not silently ignored.
- Ensure attribute freshness: stale attributes are a Broken Access Control vector. Cache invalidation strategy is critical.
- All write operations must be authorized and audited.

# Technology stack

- **Language**: Go 1.25
- **HTTP router**: go-chi/chi/v5
- **Database**: PostgreSQL 15 via jackc/pgx/v5 (connection pool)
- **Migrations**: golang-migrate/v4, embedded via `//go:embed` in `migrations/embed.go`
- **JWT validation**: lestrrat-go/jwx/v2 (JWKS auto-refresh cache)
- **JSON Schema validation**: santhosh-tekuri/jsonschema/v6
- **Metrics**: prometheus/client_golang
- **Linting**: golangci-lint v2 config (`.golangci.yml`), gofumpt formatter
- **CI**: GitHub Actions (lint, test, build, security scans via gosec + trivy)

# Project structure

```
cmd/
  api/                     # Application entry point (main.go)
  devtools/jwks-server/    # Lightweight JWKS stub for local IdP simulation
internal/
  api/
    handler/               # HTTP handlers (health.go; domain handlers Phase 2+)
    middleware/             # Chi middleware: auth, authz, correlation, logging, metrics, recovery
    response/              # JSON response helpers (response.go)
    router.go              # Composition root for all HTTP routes and middleware
  apierr/                  # Structured API error types (map to HTTP status codes)
  audit/                   # Audit log service (writes within the caller's DB transaction)
  auth/                    # Authentication: JWT (JWKS), mTLS, Identity model, context helpers
  config/                  # Environment-based configuration (config.go)
  db/                      # PostgreSQL pool, migrations, RLS-scoped transactions (scope.go)
  logging/                 # Context-aware slog handler (auto-injects correlation_id, actor)
  validation/              # JSON Schema compile + validate engine
migrations/
  000001_initial_schema.*  # Full schema: entity graph, RLS, audit, webhooks
docs/                      # Design documents: spec, data model, component/sequence diagrams, backlog
```

# Build and run commands

```bash
make setup              # Install git hooks (run once after cloning)
make build              # Compile API binary to ./bin/oad
make dev                # docker compose up --build (API + PostgreSQL + JWKS stub)
make dev-db             # Start only the PostgreSQL container
make dev-token          # Mint a JWT via the JWKS stub (overridable: SUB, ROLES, SYSTEM_ID)
make test               # go test -race ./...
make test-cover         # Tests + HTML coverage report
make lint               # golangci-lint run ./...
make migrate-up         # Apply pending migrations (requires DATABASE_URL)
make migrate-down       # Roll back last migration
make docker-build       # Build production Docker image
```

# Local development setup

The full local stack is orchestrated via `docker-compose.yml` with three services:

| Service      | Image / Build          | Port  | Purpose                                    |
|--------------|------------------------|-------|--------------------------------------------|
| `api`        | `Dockerfile`           | 8080  | The OAD API server                         |
| `jwks-stub`  | `Dockerfile.devtools`  | 9090  | Lightweight IdP stub (JWKS + token minting)|
| `postgres`   | `postgres:15-alpine`   | 5432  | PostgreSQL database                        |

The API depends on both `postgres` and `jwks-stub` being healthy before starting. Startup order is enforced via `depends_on` with `condition: service_healthy`.

## JWKS stub server

A standalone Go server (`cmd/devtools/jwks-server`) that simulates an IdP for local development:
- Generates an ephemeral RSA keypair at startup (new keys every container restart).
- `GET /.well-known/jwks.json` — serves the public JWKS.
- `POST /token` — mints signed JWTs with configurable claims (`sub`, `oad_roles`, `oad_system_id`, `expires_in`).
- `GET /health` — liveness check.

Minting tokens:
```bash
# Default admin token
make dev-token

# Custom viewer token
make dev-token SUB=viewer@example.com ROLES='["viewer"]' SYSTEM_ID=sys-1
```

# Environment variables

| Variable                   | Required | Default            | Description                                      |
|----------------------------|----------|--------------------|--------------------------------------------------|
| `DATABASE_URL`             | Yes      | —                  | PostgreSQL connection string                     |
| `SERVER_HOST`              | No       | `0.0.0.0`          | Bind address                                     |
| `SERVER_PORT`              | No       | `8080`             | Listen port                                      |
| `SERVER_SHUTDOWN_TIMEOUT`  | No       | `30s`              | Graceful shutdown timeout (Go duration)          |
| `AUTH_MODE`                | No       | `jwt`              | `jwt`, `mtls`, or `both`                         |
| `JWKS_URL`                 | If jwt   | —                  | JWKS endpoint URL (auto-refreshed every 15 min)  |
| `JWT_AUDIENCE`             | If jwt   | —                  | Expected `aud` claim                             |
| `JWT_ISSUER`               | If jwt   | —                  | Expected `iss` claim                             |
| `MTLS_HEADER`              | No       | `X-Client-Cert`    | Header for LB-terminated mTLS                    |
| `DB_MAX_CONNS`             | No       | `25`               | Max pool connections                             |
| `DB_MIN_CONNS`             | No       | `5`                | Min pool connections                             |

# Authentication model

The API supports three auth modes configured via `AUTH_MODE`:

- **`jwt`** (default) — validates Bearer tokens against a JWKS endpoint. Extracts `sub`, `oad_roles` (custom claim), and `oad_system_id` (custom claim) into an `Identity`.
- **`mtls`** — extracts identity from client certificate CN (direct TLS or LB-terminated via header). Maps certificate OUs to roles.
- **`both`** — tries JWT first, falls back to mTLS.

Custom JWT claims used by OAD:
- `oad_roles` — `[]string` of application roles (`admin`, `editor`, `viewer`).
- `oad_system_id` — `string` UUID of the scoped system. Empty means platform admin (unrestricted access).

# Authorization model

Role-based authorization is enforced at two levels:

1. **Middleware** (`middleware/authz.go`) — `RequireRole`, `RequireAnyRole`, `RequireSystemScope` guard route groups.
2. **Database RLS** (`db/scope.go`) — every scoped query runs inside a transaction with `SET LOCAL app.current_system_id`. PostgreSQL Row-Level Security policies enforce system isolation at the DB level as defense-in-depth.

# API routes (current)

| Method | Path       | Auth | Description               |
|--------|------------|------|---------------------------|
| GET    | `/health`  | No   | Health check (DB + app)   |
| GET    | `/metrics` | No   | Prometheus metrics        |
| *      | `/api/v1/` | Yes  | Protected prefix (Phase 2+: domain routes) |

# Database schema highlights

The initial migration (`000001_initial_schema.up.sql`) creates:

- **`entity_type_definition`** — dynamic schema registry (JSON Schema per entity type).
- **`system`** — registered applications whose auth data is managed.
- **`entity`** — typed nodes in the authorization graph (subjects, resources, roles, etc.).
- **`relation`** — directed edges for RBAC/ReBAC graph traversal.
- **`property_overlay`** — system-specific properties layered on global entities.
- **`system_overlay_schema`** — per-system, per-type JSON Schema for overlay validation.
- **`audit_log`** / **`retrieval_log`** — immutable append-only logs (DB triggers prevent UPDATE/DELETE).
- **`webhook_subscription`** / **`webhook_delivery`** — event notification infrastructure.
- **Row-Level Security** on `entity`, `relation`, `property_overlay`, `webhook_subscription` using `app.current_system_id` session variable.

# Commit conventions

All commits must follow the **Conventional Commits** specification (`type(scope): description`).

| Type | When to use |
|---|---|
| `feat` | New feature or endpoint |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `chore` | Build, CI, tooling, config |
| `style` | Formatting, lint fixes |

**Scope** (optional) narrows the context: `feat(overlay)`, `fix(lint)`, `chore(ci)`, `docs(claude)`.

**Subject line rules**: lowercase, imperative mood, no trailing period, max 72 characters.

Examples from this repo:
```
feat: implement phase 4 overlay system (property_overlay CRUD)
fix(lint): replace naked return with explicit return values in parsePagination
chore(hooks): track pre-commit hook and expose make setup for new clones
docs(claude): document conventional commits convention
```

# Coding conventions

- **Error handling**: always wrap errors with `fmt.Errorf("context: %w", err)`. Use `apierr` types for HTTP responses.
- **Logging**: use `slog.*Context` methods — the `ContextHandler` auto-injects `correlation_id` and actor identity.
- **Audit**: every write operation must include an audit log entry within the same DB transaction (via `audit.Service.WriteFromContext`).
- **Validation**: use `validation.Compile` + `Validator.Validate` for JSON Schema checks. Use `ValidateIsJSONSchema` to validate schema-of-schemas.
- **Transactions**: use `db.WithAuthScope` for RLS-scoped DB operations. It extracts `system_id` from the request context automatically.
- **Testing**: unit tests use `httptest.Server` for JWKS simulation. The CI pipeline has a real PostgreSQL service for integration tests.
- **Formatting**: gofumpt (stricter than gofmt). The golangci-lint config enforces this in CI.
  A `.vscode/settings.json` is committed to the repo and configures `gopls` to use gofumpt
  automatically on save (`"formatting.gofumpt": true`). Install the **Go** extension
  (`golang.go`) for this to take effect — no additional setup required.