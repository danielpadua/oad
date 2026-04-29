# Multi-IdP deployment (Keycloak + Dex/glauth)

Local stack demonstrating OAD's multi-provider support: two independent OIDC IdPs
share the same OAD instance, each with its own JWKS and claim mapping.

## Services

| Service    | Port | Purpose                              |
|------------|------|--------------------------------------|
| `api`      | 8080 | OAD API + embedded Management UI     |
| `keycloak` | 8081 | OIDC Identity Provider #1            |
| `dex`      | 5556 | OIDC Identity Provider #2 (LDAP)     |
| `glauth`   | 3893 | Static LDAP directory consumed by Dex |
| `postgres` | 5432 | PostgreSQL database                  |

## Run

From the repository root:

```bash
make dev STACK=multi-idp
```

Or directly:

```bash
docker compose -f deployments/multi-idp/docker-compose.yml up --build
```

Open http://localhost:8080 — the login page exposes both providers.

## Pre-configured users

**Keycloak** — roles via the native `oad_roles` claim:

| Username  | Password  | OAD Role  |
|-----------|-----------|-----------|
| `admin`   | `admin`   | `admin`   |
| `product` | `product` | `editor`  |
| `auditor` | `auditor` | `viewer`  |
| `pdp`     | `pdp`     | `viewer`  |

**Dex** — roles via the `groups` claim emitted by glauth:

| Email            | Password  | LDAP Group | OAD Role  |
|------------------|-----------|------------|-----------|
| `admin@oad.dev`  | `admin`   | `admin`    | `admin`   |
| `editor@oad.dev` | `editor`  | `editor`   | `editor`  |
| `viewer@oad.dev` | `viewer`  | `viewer`   | `viewer`  |
| `pdp@oad.dev`    | `pdp`     | `viewer`   | `viewer`  |

## Files

- `docker-compose.yml` — service definitions; build context points to repo root.
- `config.yml` — OAD runtime config registering both providers.
- `keycloak/realm-export.json` — Keycloak realm imported on first start.
- `dex/config.yml` — Dex configuration (LDAP connector pointing at glauth).
- `glauth/config.cfg` — static LDAP directory.
