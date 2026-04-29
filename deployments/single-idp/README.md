# Single-IdP deployment (Keycloak only)

Minimal local stack with one OIDC IdP. Use this when you do not need to exercise
multi-provider behavior.

## Services

| Service    | Port | Purpose                          |
|------------|------|----------------------------------|
| `api`      | 8080 | OAD API + embedded Management UI |
| `keycloak` | 8081 | OIDC Identity Provider           |
| `postgres` | 5432 | PostgreSQL database              |

## Run

From the repository root:

```bash
make dev STACK=single-idp
```

Or directly:

```bash
docker compose -f deployments/single-idp/docker-compose.yml up --build
```

Open http://localhost:8080 and sign in via Keycloak.

## Pre-configured users

| Username  | Password  | OAD Role  |
|-----------|-----------|-----------|
| `admin`   | `admin`   | `admin`   |
| `product` | `product` | `editor`  |
| `auditor` | `auditor` | `viewer`  |
| `pdp`     | `pdp`     | `viewer`  |

## Files

- `docker-compose.yml` — service definitions; build context points to repo root.
- `config.yml` — OAD runtime config (single Keycloak provider).
- `keycloak/realm-export.json` — Keycloak realm imported on first start.
