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

## Backend
- **Language**: Go 1.25
- **HTTP router**: go-chi/chi/v5
- **Database**: PostgreSQL 15 via jackc/pgx/v5 (connection pool)
- **Migrations**: golang-migrate/v4, embedded via `//go:embed` in `migrations/embed.go`
- **JWT validation**: lestrrat-go/jwx/v2 (JWKS auto-refresh cache)
- **JSON Schema validation**: santhosh-tekuri/jsonschema/v6
- **Metrics**: prometheus/client_golang
- **Linting**: golangci-lint v2 config (`.golangci.yml`), gofumpt formatter
- **CI**: GitHub Actions (lint, test, build, security scans via gosec + trivy)

## Frontend (Management UI)
- **Framework**: React 19 + TypeScript, Vite bundler
- **Styling**: Tailwind CSS
- **UI primitives**: shadcn/ui (accessible, composable base components)
- **Animated components**: ReactBits (reactbits.dev) — curated animated components for polish and UX feedback
- **Server state**: TanStack Query (cache, stale-while-revalidate, optimistic updates)
- **Forms**: react-hook-form + zod resolver
- **Authentication**: oidc-client-ts (OIDC/OAuth 2.0 code flow with PKCE)
- **Routing**: React Router v7 (lazy-loaded route modules)
- **Testing**: Vitest + Testing Library, MSW for API mocking
- **MCP servers**: shadcn (`shadcn@latest mcp`), reactbits (`reactbits-dev-mcp-server`)

### ReactBits component mapping

Components selected from ReactBits for the OAD Management UI, organized by usage context:

| Context | Component | Usage |
|---|---|---|
| Login page | `SoftAurora` (Background) | Animated background for the OIDC login/callback screen |
| Login page | `DecryptedText` (Text) | Cipher-decode animation on the OAD title — reinforces security branding |
| Branding | `GradientText` (Text) | OAD logo/title in the top bar |
| Navigation | `Dock` (Component) | Primary sidebar navigation with hover magnification |
| Page transitions | `AnimatedContent` (Animation) | Content reveal animation on route changes |
| Page transitions | `FadeContent` (Animation) | Smooth fade between views |
| Dashboard | `CountUp` (Text) | Animated counters for key metrics (total entities, active systems, pending webhooks) |
| Dashboard | `SpotlightCard` (Component) | Highlight cards for system overview and health status |
| Data lists | `AnimatedList` (Component) | Animated entry for audit log items and entity lists |
| Multi-step flows | `Stepper` (Component) | Bulk import wizard, entity type definition creation |
| Active scope | `BorderGlow` (Component) | Visual indicator on the active system scope selector |
| Action feedback | `ClickSpark` (Animation) | Spark effect on successful create/save actions |
| Loading states | `BlurText` (Text) | Blur-to-sharp text reveal while data loads |
| Emphasis | `ShinyText` (Text) | Highlight active system name in scope banner |

# Project structure

```
cmd/
  oad/                     # CLI entry point (cobra root + run server sub-command)
internal/
  api/
    handler/               # HTTP handlers (health, configjson, and all domain handlers)
    middleware/             # Chi middleware: auth, authz, correlation, logging, metrics, recovery
    response/              # JSON response helpers (response.go)
    router.go              # Composition root for all HTTP routes and middleware
  apierr/                  # Structured API error types (map to HTTP status codes)
  audit/                   # Audit log service (writes within the caller's DB transaction)
  auth/                    # Authentication: JWT (JWKS), mTLS, Identity model, context helpers
  config/                  # Configuration loader: YAML file + OAD_* env vars + CLI flags
  db/                      # PostgreSQL pool, migrations, RLS-scoped transactions (scope.go)
  logging/                 # Context-aware slog handler (auto-injects correlation_id, actor)
  validation/              # JSON Schema compile + validate engine
  webui/                   # Embedded Management UI SPA handler (//go:embed all:dist)
migrations/
  000001_initial_schema.*  # Full schema: entity graph, RLS, audit, webhooks
web/                       # React 19 + Vite Management UI source (built into internal/webui/dist/)
docs/                      # Design documents: spec, data model, component/sequence diagrams, backlog
deployments/               # Local development stacks (docker compose + IdP fixtures)
  multi-idp/               #   API + Keycloak + Dex + glauth + Postgres
  single-idp/              #   API + Keycloak + Postgres
```

# Build and run commands

```bash
make setup              # Install git hooks (run once after cloning)
make web-build          # Build the Management UI into internal/webui/dist/
make build              # web-build + compile binary to ./bin/oad
make dev STACK=<name>   # Start a dev stack (multi-idp | single-idp)
make dev-db STACK=<name> # Start only the PostgreSQL container of a stack
make test               # go test -race ./...
make test-cover         # Tests + HTML coverage report
make lint               # golangci-lint run ./...
make migrate-up         # Apply pending migrations (requires OAD_DATABASE)
make migrate-down       # Roll back last migration
make docker-build       # Build production Docker image
```

# Local development setup

Each compose stack lives in its own directory under `deployments/`. The Makefile
selects a stack via the `STACK` variable; the choice is explicit (no default).

## Stacks

### `deployments/multi-idp/` — Keycloak + Dex/glauth + Postgres

Demonstrates multi-provider validation: two independent IdPs, two JWKS, distinct
claim mappings, served by the same OAD instance.

| Service    | Image / Build                       | Port | Purpose                            |
|------------|-------------------------------------|------|------------------------------------|
| `api`      | `Dockerfile` (repo root)            | 8080 | OAD API + embedded Management UI  |
| `keycloak` | `quay.io/keycloak/keycloak:24.0.0`  | 8081 | OIDC Identity Provider #1          |
| `dex`      | `ghcr.io/dexidp/dex:v2.41.1`        | 5556 | OIDC Identity Provider #2 (LDAP)   |
| `glauth`   | `glauth/glauth:v2.5.0`              | 3893 | Static LDAP directory for Dex      |
| `postgres` | `postgres:15-alpine`                | 5432 | PostgreSQL database                |

```bash
make dev STACK=multi-idp
```

Pre-configured Keycloak users (realm imported automatically on first start):

| Username  | Password  | OAD Role  |
|-----------|-----------|-----------|
| `admin`   | `admin`   | `admin`   |
| `product` | `product` | `editor`  |
| `auditor` | `auditor` | `viewer`  |
| `pdp`     | `pdp`     | `viewer`  |

Pre-configured Dex users (roles via `groups` claim from glauth LDAP; see `deployments/multi-idp/dex/config.yml` and `deployments/multi-idp/glauth/config.cfg`):

| Email            | Password  | LDAP Group | OAD Role  |
|------------------|-----------|------------|-----------|
| `admin@oad.dev`  | `admin`   | `admin`    | `admin`   |
| `editor@oad.dev` | `editor`  | `editor`   | `editor`  |
| `viewer@oad.dev` | `viewer`  | `viewer`   | `viewer`  |
| `pdp@oad.dev`    | `pdp`     | `viewer`   | `viewer`  |

### `deployments/single-idp/` — Keycloak + Postgres

Minimal stack with a single OIDC provider. Same Keycloak users as above.

```bash
make dev STACK=single-idp
```

The Management UI is served by the API at `http://localhost:8080` for both
stacks.

## Configuration file format

OAD is configured via a YAML file passed with `--config` (or `-c`). Precedence:
**CLI flag > `OAD_*` env var > YAML file > built-in defaults**.

```yaml
server:
  addr: ":8080"            # default

auth:
  mode: jwt                # jwt | mtls | both | none
  providers:
    - name: keycloak
      display_name: Keycloak
      backend:
        jwks_url: https://idp.example.com/realms/oad/protocol/openid-connect/certs
        issuer:   https://idp.example.com/realms/oad
        audience: oad-api
        # claims_mapping is optional; omit when the IdP emits oad_roles natively.
        claims_mapping:
          roles_claim:     groups       # default: oad_roles
          system_id_claim: x_system_id # default: oad_system_id
          default_roles:               # applied when roles_claim is absent
            - viewer
      webui:
        authority:  https://idp.example.com/realms/oad
        client_id:  oad-web
        scope:      openid profile email

webui:
  redirect_uri:    https://oad.example.com/callback
  post_logout_uri: https://oad.example.com/

database:
  dsn: postgresql://user:pass@host:5432/oad?sslmode=require

log:
  level:  info   # debug | info | warn | error
  format: json   # json | text
```

# Environment variables

All environment variables use the `OAD_` prefix. The legacy unprefixed names (e.g. `DATABASE_URL`, `JWKS_URL`) still work but emit a deprecation warning at startup.

## Server

| Variable               | Required | Default | Description                              |
|------------------------|----------|---------|------------------------------------------|
| `OAD_DATABASE`         | Yes      | —       | PostgreSQL DSN                           |
| `OAD_ADDR`             | No       | `:8080` | Bind address (`[host]:port`)             |
| `OAD_SHUTDOWN_TIMEOUT` | No       | `30s`   | Graceful shutdown deadline (Go duration) |
| `OAD_DB_MAX_CONNS`     | No       | `25`    | Max pool connections                     |
| `OAD_DB_MIN_CONNS`     | No       | `5`     | Min pool connections                     |
| `OAD_LOG_LEVEL`        | No       | `info`  | `debug` \| `info` \| `warn` \| `error`  |
| `OAD_LOG_FORMAT`       | No       | `json`  | `json` \| `text`                         |

## Authentication

| Variable                    | Required      | Default          | Description                                  |
|-----------------------------|---------------|------------------|----------------------------------------------|
| `OAD_AUTH_MODE`             | No            | `jwt`            | `jwt` \| `mtls` \| `both` \| `none`         |
| `OAD_MTLS_HEADER`           | No            | `X-Client-Cert`  | Header for LB-terminated mTLS cert           |
| `OAD_JWKS_URL`              | If jwt / both | —                | JWKS endpoint URL                            |
| `OAD_JWT_ISSUER`            | If jwt / both | —                | Expected `iss` claim                         |
| `OAD_JWT_AUDIENCE`          | If jwt / both | —                | Expected `aud` claim                         |
| `OAD_PROVIDER_NAME`         | No            | `default`        | Provider name (single-provider shortcut)     |
| `OAD_PROVIDER_DISPLAY_NAME` | No            | —                | Provider display name shown in the UI        |
| `OAD_PROVIDER_AUTHORITY`    | No            | —                | OIDC authority URL served via `/config.json` |
| `OAD_PROVIDER_CLIENT_ID`    | No            | —                | OIDC client ID served via `/config.json`     |
| `OAD_PROVIDER_SCOPE`        | No            | —                | OIDC scope served via `/config.json`         |

## Management UI

| Variable                    | Required | Default | Description                                   |
|-----------------------------|----------|---------|-----------------------------------------------|
| `OAD_WEBUI_REDIRECT_URI`    | No       | —       | OIDC redirect URI served via `/config.json`   |
| `OAD_WEBUI_POST_LOGOUT_URI` | No       | —       | Post-logout redirect URI via `/config.json`   |

# Authentication model

The API supports four auth modes configured via `OAD_AUTH_MODE`:

- **`jwt`** (default) — validates Bearer tokens against a JWKS endpoint. Extracts `sub`, `oad_roles` (custom claim), and `oad_system_id` (custom claim) into an `Identity`.
- **`mtls`** — extracts identity from client certificate CN (direct TLS or LB-terminated via header). Maps certificate OUs to roles.
- **`both`** — tries JWT first, falls back to mTLS.
- **`none`** — disables authentication (development only; never use in production).

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