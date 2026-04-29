# OAD — Open Authoritative Directory

OAD is a **Policy Information Point (PIP)** — a centralized attribute repository that serves an AuthZen-enabled **Policy Decision Point (PDP)** during policy evaluation. It stores and exposes subject, resource, and environment attributes that policies need to reach permit/deny decisions, directly addressing Broken Access Control (OWASP Top 10 #1).

The binary is self-contained: it embeds the Management UI SPA and runs database migrations on startup.

## Quick start

### Download

Pre-built binaries for Linux, macOS, and Windows (amd64/arm64) and multi-arch
container images are published with every release:

- Binaries + checksums + SBOMs: [GitHub Releases](https://github.com/danielpadua/oad/releases)
- Container image: `ghcr.io/danielpadua/oad:<tag>` (also `:latest`)

Release artifacts are signed with [cosign](https://docs.sigstore.dev/cosign/overview/)
keyless OIDC. See the release notes for the verification commands.

### Run locally

Two ready-made compose stacks live under [`deployments/`](deployments/). Pick one
and pass it to `make dev` via the `STACK` variable:

```bash
make dev STACK=multi-idp     # Keycloak + Dex/glauth + Postgres
make dev STACK=single-idp    # Keycloak + Postgres
```

Each deployment ships its own `docker-compose.yml`, `config.yml`, and IdP
fixtures. See the per-stack READMEs for credentials and details:

- [`deployments/multi-idp/`](deployments/multi-idp/README.md) — demonstrates multi-provider validation (two IdPs, two JWKS, distinct claim mappings).
- [`deployments/single-idp/`](deployments/single-idp/README.md) — minimal stack with one IdP.

Open `http://localhost:8080` after the stack is up.

## Configuration

OAD is configured via a YAML file (`--config` / `-c` flag). Environment variables (`OAD_*`) and CLI flags override file values.

```yaml
server:
  addr: ":8080"

auth:
  mode: jwt   # jwt | mtls | both | none
  providers:
    - name: my-idp
      display_name: My IdP
      backend:
        jwks_url: https://idp.example.com/.well-known/jwks.json
        issuer:   https://idp.example.com
        audience: oad-api
        # claims_mapping adapts the IdP's native claims to OAD's identity model.
        # Omit this block entirely when the IdP emits oad_roles and oad_system_id natively.
        claims_mapping:
          roles_claim:    groups        # claim holding roles (default: oad_roles)
          system_id_claim: x_system_id  # claim holding system UUID (default: oad_system_id)
          default_roles:               # roles when roles_claim is absent
            - viewer
      webui:
        authority:  https://idp.example.com
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

For single-provider deployments without a config file, the equivalent env vars are:

```bash
OAD_DATABASE=postgresql://...
OAD_JWKS_URL=https://idp.example.com/.well-known/jwks.json
OAD_JWT_ISSUER=https://idp.example.com
OAD_JWT_AUDIENCE=oad-api
OAD_PROVIDER_AUTHORITY=https://idp.example.com
OAD_PROVIDER_CLIENT_ID=oad-web
OAD_WEBUI_REDIRECT_URI=https://oad.example.com/callback
OAD_WEBUI_POST_LOGOUT_URI=https://oad.example.com/
```

## Build

```bash
make build       # web-build + compile binary to ./bin/oad
make docker-build  # build production Docker image
```

The production binary requires `OAD_DATABASE` and an external OIDC IdP.

## API overview

All domain endpoints are under `/api/v1` and require authentication.

| Endpoint                              | Roles         | Description                        |
|---------------------------------------|---------------|------------------------------------|
| `GET /health`                         | —             | Health check                       |
| `GET /metrics`                        | —             | Prometheus metrics                 |
| `GET /config.json`                    | —             | Frontend OIDC bootstrap config     |
| `GET /api/v1/stats`                   | any           | Dashboard aggregate counters       |
| `GET /api/v1/entities`                | any           | List entities                      |
| `POST /api/v1/entities`               | admin, editor | Create entity                      |
| `GET /api/v1/entities/lookup`         | any           | Lookup by type + external ID       |
| `GET /api/v1/entities/search`         | any           | Filter by JSONB properties         |
| `GET /api/v1/relations`               | any           | Entity relations                   |
| `GET /api/v1/entity-types`            | admin         | Schema registry — entity types     |
| `GET /api/v1/systems`                 | admin         | Registered systems                 |
| `GET /api/v1/systems/{id}/webhooks`   | admin         | Webhook subscriptions              |
| `GET /api/v1/changelog`               | any           | Attribute change log               |
| `GET /api/v1/export`                  | any           | Full attribute export              |

## Domain concepts

**OAD in the authorization stack:**

```
[Subject] ──► [PEP] ──► [PDP] ──► [OAD/PIP]
                 ▲         │           │
                 └─ deny / ─┘  attributes
                    permit
```

- **Entity** — a typed node in the authorization graph (user, role, resource, device, …).
- **Relation** — a directed edge between entities (user `has_role` admin, resource `owned_by` team).
- **Overlay** — system-scoped property overrides layered on top of global entity attributes.
- **System** — a registered application whose attribute data OAD manages.
- **Retrieval log** — every PDP query is recorded for auditability and freshness monitoring.

## Development workflow

```bash
make setup          # Install git hooks (run once)
make test           # go test -race ./...
make lint           # golangci-lint
make format         # gofumpt -l -w
make ui-dev         # Vite dev server (hot reload, requires API running)
make migrate-up     # Apply pending DB migrations
```

See [CLAUDE.md](CLAUDE.md) for the full coding conventions, commit format, and architecture reference.

## License

OAD is licensed under the [Apache License 2.0](LICENSE).
