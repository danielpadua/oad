# Multi-IdP deployment (Keycloak + Authentik)

Local stack demonstrating OAD's multi-provider support: two independent
real Identity Providers share the same OAD instance.

- **Keycloak** — OIDC IdP for the management plane.
- **Authentik** — second OIDC IdP that ALSO acts as the SCIM source.
  On first start, Authentik provisions its bootstrapped Users and
  Groups into OAD via its SCIM Provider.

Source-of-truth design: [docs/design/scim-ingest.md §11/§12.1](../../docs/design/scim-ingest.md#11-initial-bulk-import).

## Services

| Service              | Port | Purpose                                                |
|----------------------|------|--------------------------------------------------------|
| `api`                | 8080 | OAD API + embedded Management UI                       |
| `keycloak`           | 8081 | OIDC Identity Provider #1                              |
| `authentik-server`   | 9000 | OIDC Identity Provider #2 + SCIM source                |
| `authentik-worker`   | —    | Async task runner (handles SCIM provisioning cycles)   |
| `authentik-redis`    | —    | Authentik task broker                                  |
| `authentik-postgres` | —    | Authentik database (separate from OAD's Postgres)      |
| `postgres`           | 5432 | OAD's PostgreSQL database                              |

## Required setup

The Authentik integration needs three secrets in a `.env` file before
the stack will start:

```bash
cp deployments/multi-idp/.env.example deployments/multi-idp/.env
# then edit:
#   AUTHENTIK_SECRET_KEY        — openssl rand -base64 60
#   AUTHENTIK_OAD_SCIM_TOKEN    — openssl rand -hex 32
#   OAD_SCIM_TOKEN_AUTHENTIK    — same value as AUTHENTIK_OAD_SCIM_TOKEN
```

The two SCIM-token vars carry the same secret on both ends of the
provisioning channel: Authentik signs SCIM requests with it, and OAD
matches it against the SCIM tenant registry to authenticate Authentik
as the `authentik` provider.

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

**Authentik** — Users and Groups seeded by the bootstrap blueprint and
automatically SCIM-provisioned into OAD on first sync. Roles are
derived by OAD from the `groups` claim:

| Username         | Password  | Authentik Group | OAD Role  |
|------------------|-----------|-----------------|-----------|
| `admin@oad.dev`  | `admin`   | `oad-admin`     | `admin`   |
| `editor@oad.dev` | `editor`  | `oad-editor`    | `editor`  |
| `viewer@oad.dev` | `viewer`  | `oad-viewer`    | `viewer`  |
| `pdp@oad.dev`    | `pdp`     | `oad-viewer`    | `viewer`  |

The Authentik admin UI is reachable at http://localhost:9000 with the
default `akadmin` / `${AUTHENTIK_BOOTSTRAP_PASSWORD}` credentials.

## SCIM verification

After the stack is fully up, you can confirm SCIM ingest worked by
querying OAD directly:

```bash
SCIM_TOKEN=$(grep ^OAD_SCIM_TOKEN_AUTHENTIK deployments/multi-idp/.env | cut -d= -f2)
curl -H "Authorization: Bearer $SCIM_TOKEN" \
     http://localhost:8080/scim/v2/Users | jq '.totalResults, .Resources[].userName'
```

You should see four users (the four Authentik fixtures) returned.

## Files

- `docker-compose.yml` — service definitions; build context points to repo root.
- `config.yml` — OAD runtime config registering both providers.
- `.env.example` — template for the required secrets.
- `keycloak/realm-export.json` — Keycloak realm imported on first start.
- `authentik/blueprints/oad.yaml` — Authentik blueprint applied on first
  start (Users, Groups, Application, OIDC Provider, SCIM Provider).
