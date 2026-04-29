# OAD — SCIM 2.0 Ingest (v0.1)

> Cross-references: [Spec](../spec.md), [Data Model](../data-model.md), [Backlog](../backlog.md), [DB-Authoritative Auth](db-authoritative-auth.md)
>
> Phase reference: **B** in the User Provisioning roadmap (Foundation A → SCIM B → DB-Authoritative C → Admin UI D).

---

## 1. Goal and non-goals

### 1.1 Goal

Provide a SCIM 2.0 server endpoint that accepts user and group lifecycle events from each connected Identity Provider, materializing them as `entity` rows of type `User` and `Group`, with membership stored as `relation` edges. SCIM is the **authoritative ingestion path** for human identities; OAD's authorization decisions consume the resulting graph (see [DB-Authoritative Auth](db-authoritative-auth.md)).

### 1.2 Non-goals

- **Not a SCIM client.** OAD does not push or pull from external systems via SCIM. IdPs push to OAD.
- **Not a directory mirror.** Only attributes relevant to authorization are persisted (subject, email, displayName, group memberships). Pictures, phone numbers, addresses are dropped at the ingestion boundary.
- **Not in scope: ServiceAccount provisioning.** Machine identities follow a separate admin path (deferred).
- **Not in scope: `/scim/v2/Bulk` and `/scim/v2/.search`.** Deferred until requested.
- **Not in scope: webhooks for SCIM mutations.** SCIM-driven create/update/delete events do not trigger `webhook_subscription` deliveries in v1. The current subscription model is per-system; emitting global SCIM events through it would require a model change (filter-based subscriptions) better deferred until a real consumer requires it.

---

## 2. Glossary

| Term | Definition |
|---|---|
| **Provider** | An entry in `auth.providers[]` (YAML config). Each provider represents one connected IdP and has its own JWKS and SCIM tenant token. |
| **SCIM tenant token** | A bearer token unique to one provider. The token identifies which provider is making a SCIM call. |
| **External subject** | The IdP's stable identifier for a user (the SCIM `id` value, derived from the IdP's internal user ID). Unique within a provider, not globally. |
| **External identity** | A row in `entity_external_identity` linking `(provider_name, external_subject)` to an `entity.id`. |

---

## 3. Schema foundation (Phase A)

This section enumerates the schema changes that must land before SCIM ingest can be implemented. Phase A is a precondition for both this doc and the DB-Authoritative Auth doc. Since OAD is pre-release (no tagged versions, no production deployments), Phase A is delivered as a rewrite of migration `000001_initial_schema` rather than a separate up-migration.

### 3.1 New column: `entity_type_definition.is_builtin`

| Column | Type | Constraints | Description |
|---|---|---|---|
| `is_builtin` | `boolean` | NOT NULL, default `false` | Marks built-in types that cannot be deleted or have their `allowed_properties` / `allowed_relations` modified by API callers. |

Application enforcement: any update or delete on a row where `is_builtin = true` returns `403 Forbidden` from the management API. The DB does not enforce this with a trigger — it is checked at the handler layer. SCIM never modifies type definitions.

### 3.2 New column: `entity.is_builtin`

| Column | Type | Constraints | Description |
|---|---|---|---|
| `is_builtin` | `boolean` | NOT NULL, default `false` | Marks built-in entity rows (currently the three reserved Groups, see §3.6). Cannot be deleted; `properties` cannot be modified through the management API. |

### 3.3 New table: `entity_external_identity`

Links one or more external IdP identities to a single OAD `entity` (typically of type `User`, but the link is type-agnostic so future use cases — e.g., system identities — are not blocked).

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | `uuid` | PK, default `gen_random_uuid()` | Internal identifier. |
| `entity_id` | `uuid` | FK → `entity.id`, NOT NULL, ON DELETE CASCADE | The OAD entity this external identity refers to. |
| `provider_name` | `varchar(100)` | NOT NULL | Matches `auth.providers[].name`. |
| `external_subject` | `varchar(500)` | NOT NULL | The IdP's stable identifier (SCIM `id`). |
| `created_at` | `timestamptz` | NOT NULL, default `now()` | First sight of this external identity. |
| `updated_at` | `timestamptz` | NOT NULL, default `now()` | Last update from the IdP. |

**Unique constraint:** `(provider_name, external_subject)` — same external subject from same provider always resolves to the same OAD entity.

**Indexes:**
- `(provider_name, external_subject)` UNIQUE — primary lookup path during JWT identity resolution.
- `(entity_id)` — reverse lookup ("all external identities for this user").

**Many-to-one:** a single OAD entity can have multiple external identities (cross-IdP merge, supported via Phase D admin UI).

### 3.4 Drop `system` table; promote `System` to an entity type

The `system` table is removed. Existing rows (none in production) are not migrated; the new initial migration creates `System` rows directly inside `entity`.

- `entity.system_id`, `relation.system_id`, `property_overlay.system_id`, `system_overlay_schema.system_id`, `webhook_subscription.system_id` continue to exist and continue to scope RLS. Their FK target changes from `system.id` to `entity.id`.
- A trigger on insert/update of these `system_id` columns asserts that the referenced `entity` has `type_id` corresponding to the `System` type. Violations raise `foreign_key_violation`.

### 3.5 Seeded built-in types

The migration seeds three rows in `entity_type_definition`:

| `type_name` | `scope` | `is_builtin` | `allowed_properties` (summary) | `allowed_relations` |
|---|---|---|---|---|
| `User` | `global` | `true` | `userName`, `displayName`, `email`, `active` | `member_of: [Group]`, `has_role_in: [System]` |
| `Group` | `global` | `true` | `displayName`, `description` | `has_role_in: [System]`, `has_permission: [Permission]` |
| `System` | `global` | `true` | `name`, `description`, `active` | (none) |
| `Permission` | `global` | `true` | `name`, `description` | (none) |

Full JSON Schema documents for each type live in `migrations/seed/` and are loaded by the migration via `pg_read_file` or as embedded strings in the migration SQL.

#### 3.5.1 Permission scoping

The `Permission` entity type is global — there is one shared catalog of permission identifiers (e.g., `creditcard:readBill`, `account:readBalance`). Per-system effective entitlement is expressed by the `Group --has_permission--> Permission` relation, which itself can be:

- **Global** (`relation.system_id IS NULL`) — the permission applies in any system context where the group is recognized.
- **System-scoped** (`relation.system_id = <uuid>`) — the permission applies only when that system is the active context.

When the PDP requests a user's attributes for system X, OAD returns the union of permissions reached via global relations plus permissions reached via relations scoped to X. The catalog stays reusable while per-system grants remain expressive.

#### 3.5.2 Permission provisioning

`Permission` rows are not ingested via SCIM — RFC 7644 has no native Permission resource. Permissions are managed through the admin REST API used by the Management UI. The admin CRUD for Permission and the UI to assign Group↔Permission relations are scheduled for Phase D.

#### 3.5.3 Future: IGA tooling integration

The admin REST API for `Permission` CRUD (and `Group`↔`Permission` relation management) doubles as the integration surface for Identity Governance and Administration tools (SailPoint, Saviynt, Okta IG, Entra ID Governance) that need to push entitlements beyond what SCIM 2.0 covers natively.

Expected mapping for a SailPoint-style integration:

| IGA concept | OAD destination | Transport |
|---|---|---|
| Identity | `User` entity | SCIM `/Users` |
| Group | `Group` entity | SCIM `/Groups` |
| Role (collection of entitlements) | `Group` entity with `has_permission` relations | SCIM `/Groups` + admin API |
| Entitlement | `Permission` entity | Admin REST API (custom connector) |
| Account (system-specific user instance) | `User` + system-scoped relations | Combination |

No model change is anticipated to support this. Provenance tracking (`source: scim:<provider>` / `iga:<connector>` / `admin:<user>`) and an "IGA-managed" mode that disables the admin UI for IGA-owned resources are likely future additions, not blocking v1. A dedicated `docs/design/iga-integration.md` should be authored when the first customer with IGA requirements is engaged.

### 3.6 Seeded built-in groups

Three rows in `entity` of type `Group`, with reserved `external_id` values, marked `is_builtin = true`:

| `external_id` | Purpose |
|---|---|
| `oad:admin` | Full administrative access to the OAD management plane. |
| `oad:editor` | Read/write on entities, relations, overlays. Cannot manage type definitions or system registration. |
| `oad:viewer` | Read-only access to entities, relations, overlays, audit logs. |

These groups have no SCIM external identity. They are referenced by user-membership relations created either via SCIM (mapped from IdP groups — see §6) or via direct admin operations.

### 3.7 RLS adjustments for global entities

Current RLS on `entity` filters by `system_id`. Since `User`, `Group`, and `System` are global (`system_id IS NULL`), the policy must permit reads of global entities regardless of the session's `app.current_system_id`. Updated policy:

```sql
ALTER TABLE entity ENABLE ROW LEVEL SECURITY;
CREATE POLICY entity_scope ON entity
  USING (
    system_id IS NULL OR
    system_id = current_setting('app.current_system_id', true)::uuid
  );
```

The same adjustment applies to `relation` (where `system_id IS NULL` means a global relation, e.g., `User --member_of--> Group`).

---

## 4. SCIM 2.0 endpoint surface

### 4.1 Routes

Mounted at `/scim/v2` on the same HTTP server as the rest of the API. All resource routes require a SCIM tenant token (see §5). Discovery routes are unauthenticated per RFC 7644 §4.

| Method | Path | Auth | Operation |
|---|---|---|---|
| `GET` | `/scim/v2/Users` | Yes | List/filter users (paginated). |
| `POST` | `/scim/v2/Users` | Yes | Create user. |
| `GET` | `/scim/v2/Users/{id}` | Yes | Read user. |
| `PUT` | `/scim/v2/Users/{id}` | Yes | Replace user. |
| `PATCH` | `/scim/v2/Users/{id}` | Yes | Partial update (RFC 7644 §3.5.2). |
| `DELETE` | `/scim/v2/Users/{id}` | Yes | Delete user. |
| `GET` | `/scim/v2/Groups` | Yes | List/filter groups. |
| `POST` | `/scim/v2/Groups` | Yes | Create group. |
| `GET` | `/scim/v2/Groups/{id}` | Yes | Read group. |
| `PUT` | `/scim/v2/Groups/{id}` | Yes | Replace group. |
| `PATCH` | `/scim/v2/Groups/{id}` | Yes | Partial update (membership add/remove). |
| `DELETE` | `/scim/v2/Groups/{id}` | Yes | Delete group. |
| `GET` | `/scim/v2/ServiceProviderConfig` | No | Capability discovery. |
| `GET` | `/scim/v2/Schemas` | No | Schema introspection. |
| `GET` | `/scim/v2/ResourceTypes` | No | Resource type listing. |

`/scim/v2/Me`, `/scim/v2/Bulk`, and `/scim/v2/.search` are deferred to a later phase.

### 4.2 ServiceProviderConfig advertisement

```json
{
  "schemas": ["urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"],
  "patch":           {"supported": true},
  "bulk":            {"supported": false},
  "filter":          {"supported": true, "maxResults": 200},
  "changePassword":  {"supported": false},
  "sort":            {"supported": false},
  "etag":            {"supported": true},
  "authenticationSchemes": [{
    "type": "oauthbearertoken",
    "name": "OAuth Bearer Token",
    "description": "Per-provider tenant token",
    "primary": true
  }]
}
```

---

## 5. Authentication: per-provider tenant token

### 5.1 Token model

Each provider declared in `auth.providers[]` may include a `scim` block:

```yaml
auth:
  providers:
    - name: keycloak
      jwks_url: https://idp.example.com/realms/oad/protocol/openid-connect/certs
      issuer:   https://idp.example.com/realms/oad
      audience: oad-api
      scim:
        enabled: true
        token: env:OAD_SCIM_TOKEN_KEYCLOAK   # never inlined in YAML
```

Tokens are loaded from environment variables (recommended) or files. On startup, OAD computes `sha256(token)` and stores the hash in an in-memory map keyed by `provider_name`. Plaintext tokens are not retained after startup.

### 5.2 Authentication flow

1. Incoming request: `Authorization: Bearer <token>`.
2. Server computes `sha256(token)`.
3. Lookup in the `(hash → provider_name)` map.
4. On hit: request is authenticated as provider `<provider_name>`. Audit actor = `scim:<provider_name>`.
5. On miss: `401 Unauthorized` with SCIM error body.

### 5.3 Why config-driven, not DB-managed

For v1, tokens live in config. Rationale:

- **Bootstrap.** No circular dependency with DB state. SCIM provisioning can run before any admin user exists.
- **Ops alignment.** IdP SCIM provisioner setup is an ops task (configuring the IdP side requires the same secret), not a runtime admin task.
- **Simplicity.** Rotation = environment variable change + restart. Acceptable for v1; production deployments running multiple replicas can rotate via rolling restart.

DB-managed tokens with rotation in the admin UI are deferred to Phase D or beyond.

---

## 6. Resource mappings

### 6.1 SCIM User → OAD entity

| SCIM attribute | OAD destination |
|---|---|
| `id` (response only) | `entity.id` (OAD-generated UUID, NOT the IdP's id) |
| `userName` | `entity.properties.userName` |
| `name.formatted` / `displayName` | `entity.properties.displayName` |
| `emails[primary=true].value` | `entity.properties.email` |
| `active` | `entity.properties.active` |
| `meta.created` | `entity.created_at` (preserved on import) |
| `meta.lastModified` | `entity.updated_at` |
| `groups[]` (response only) | Computed from `relation` rows where this entity is the subject and `relation_type = 'member_of'`. Read-only on the User resource — membership is authoritative on the Group. |
| All other attributes | Dropped (not stored). |

The IdP's original SCIM `id` is stored as `external_subject` in `entity_external_identity`. The `id` returned in SCIM responses is `entity.id` — the IdP must remember this for subsequent operations on the resource. This decouples OAD's internal identifier from the IdP's, which permits cross-IdP merges later.

### 6.2 SCIM Group → OAD entity

| SCIM attribute | OAD destination |
|---|---|
| `id` (response only) | `entity.id` |
| `displayName` | `entity.properties.displayName` |
| `members[]` | One `relation` row per member: `subject_entity_id = member`, `relation_type = 'member_of'`, `target_entity_id = group`, `system_id = NULL` (global). |
| `meta.created` / `meta.lastModified` | Same as User. |

### 6.3 Member references resolution

When a Group is created/updated with `members[].value = "<user-scim-id>"`, the server resolves:

1. Look up `entity_external_identity` by `(provider_name, external_subject = user-scim-id)`.
2. If found, the relation references `entity.id`.
3. If not found, return `400 Bad Request` with SCIM `invalidValue` detail. The IdP must provision the user before referencing them in a group.

Most IdP provisioners (Okta, Azure AD) order operations correctly: users first, groups after.

### 6.4 Mapping IdP groups to `oad:*` reserved groups

By default, an IdP group named `editors` becomes a customer-defined Group with `displayName = editors`. To grant the OAD management plane permissions (admin/editor/viewer), an explicit mapping is required.

**Decision (v1):** mapping is performed via direct admin operation, not SCIM. Steps:

1. SCIM ingests the IdP group (e.g., `oad-platform-admins`).
2. Admin in the OAD UI links the customer group to `oad:admin` by creating a `relation`: `oad-platform-admins --member_of--> oad:admin`. Members of `oad-platform-admins` thus inherit `oad:admin` via two hops.

This avoids hard-coding IdP-side naming conventions and lets the customer use their own group names.

---

## 7. PATCH semantics (RFC 7644 §3.5.2)

PATCH is the dominant update operation from real IdP provisioners (Okta, Azure AD send PATCH for nearly everything). The full path expression syntax is non-trivial; we implement a documented subset.

### 7.1 Supported operations

| Operation | Path forms supported (Users) |
|---|---|
| `add` | `displayName`, `emails`, `active` |
| `replace` | `displayName`, `emails[primary eq true].value`, `active`, `userName` |
| `remove` | `emails[primary eq true]` |

| Operation | Path forms supported (Groups) |
|---|---|
| `add` | `displayName`, `members` |
| `replace` | `displayName` |
| `remove` | `members[value eq "<id>"]`, `members` (clear all) |

Any unsupported path returns `400 Bad Request` with SCIM `noTarget` detail.

### 7.2 Implementation approach: hand-rolled subset

PATCH path evaluation and filter parsing are implemented in-tree under `internal/scim/parser/` rather than via a third-party SCIM framework. Rationale:

- **Closed attribute surface.** User has 4 properties, Group has 2. Adding a SCIM attribute requires a deliberate change to the seeded `entity_type_definition` schema — not an organic expansion. The parser does not need to grow with arbitrary user-defined resources.
- **Small surface to own.** The documented filter subset (eq/ne/co/sw/ew/pr/and/or) and PATCH paths (the explicit list in §7.1) compile to a recursive-descent parser of roughly 200 lines for filter and 150 lines for PATCH paths, plus table-driven tests.
- **Architectural fit.** Existing SCIM frameworks (e.g., `github.com/elimity-com/scim`) are full server frameworks expecting `ResourceHandler` registration and routing. Adapting our `entity` / `relation` model to those abstractions is more code than the parser itself, and the abstractions occlude the direct mapping from SCIM operation to graph mutation.
- **Maintenance posture.** Code in-repo follows the project's `gofumpt` formatting and error-wrapping conventions, has no external dependency churn, and is debugged against the same observability stack as the rest of OAD.

The parser is internal — its API surface is consumed only by `internal/scim/handler/`. Tests are under `internal/scim/parser/*_test.go`.

---

## 8. Filter language

`GET /Users?filter=userName eq "alice"`

Supported subset:

- **Attributes:** `userName`, `displayName`, `emails.value`, `active`, `id`, `externalId`.
- **Operators:** `eq`, `ne`, `co`, `sw`, `ew`, `pr`, `and`, `or`.
- **Not supported:** `gt`, `ge`, `lt`, `le`, `not`, complex paths with grouping beyond two levels.

Translates to indexed queries against `entity.properties` (GIN-indexed JSONB) and `entity_external_identity`.

---

## 9. ETag and concurrency

Every response includes `meta.version` as a weak ETag, computed as `W/"<sha256(entity.id || entity.updated_at)>"` (truncated to 16 hex chars). Mutating requests may include `If-Match: <etag>`; on mismatch, the server returns `412 Precondition Failed`.

IdP provisioners typically do not send `If-Match`; we do not require it. Last-write-wins is the default.

---

## 10. Pagination

Standard SCIM pagination: `GET /Users?startIndex=1&count=50`. Default `count = 50`, max `200`. Response includes `totalResults`, `startIndex`, `itemsPerPage`. Backed by `LIMIT`/`OFFSET` against the indexed query.

---

## 11. Initial bulk import

A new IdP being connected may already contain thousands of users. SCIM has no "give me everything" semantics — the IdP's own SCIM provisioner is responsible for the initial sweep.

| IdP | Initial-import behavior |
|---|---|
| Okta | On creation of an OAD application + push-provisioning, Okta enumerates all assigned users and POSTs each. |
| Azure AD | Similar — initial cycle pushes everything assigned. |
| Keycloak | `scim-for-keycloak` plugin (community); coverage uneven. May require admin-triggered "full sync". |
| Dex | No SCIM client. **Cannot ingest from Dex via SCIM.** |

**Decision recorded:** Dex users in the dev stack are NOT a SCIM ingest case. The dev stack uses the **fake SCIM client** (§12) to simulate provider provisioning.

---

## 12. Local development and test strategy

### 12.1 Fake SCIM client

A small Go binary at `deployments/scim-fakeclient/` that pushes users/groups to OAD's SCIM endpoint based on a YAML fixture. Used for:

- **Local dev** — simulate Keycloak-as-SCIM-provisioner without configuring the plugin.
- **E2E tests** — deterministic provisioning of test fixtures before assertions.

```yaml
# deployments/scim-fakeclient/fixtures/keycloak.yaml
target: http://localhost:8080/scim/v2
token: env:OAD_SCIM_TOKEN_KEYCLOAK
users:
  - id: alice-keycloak-id
    userName: alice@oad.dev
    displayName: Alice
    emails: [{value: alice@oad.dev, primary: true}]
    active: true
groups:
  - id: editors-keycloak-id
    displayName: editors
    members: [{value: alice-keycloak-id}]
```

### 12.2 Compose integration

The `deployments/multi-idp/docker-compose.yml` gains an optional `scim-fakeclient` service that runs once on `make dev STACK=multi-idp` and exits, pre-populating users that match the Keycloak realm fixtures.

### 12.3 Unit tests

- **PATCH path parser** — table-driven tests covering each supported and rejected path.
- **Filter expression parser** — table-driven tests against an in-memory entity set.
- **Resource mappers** (User ↔ entity, Group ↔ entity + relations) — isolated tests with stub repositories.

### 12.4 Integration tests

- Real PostgreSQL service (already in CI).
- Fake SCIM client run against an OAD test instance, asserting that resulting entities, relations, and external identities match expected.
- Round-trip: POST → GET → PATCH → GET → DELETE for both Users and Groups.

---

## 13. Edge cases

| Case | Behavior |
|---|---|
| User created via SCIM, then deleted at IdP | SCIM DELETE removes the `entity_external_identity` row and the entity (CASCADE removes relations). Audit log retains full history. |
| User soft-deleted at IdP (`active = false`) | `entity.properties.active = false`. Entity remains; the DB-Authoritative Auth doc defines that `active = false` denies authentication. |
| Same user provisioned by two IdPs | Two separate `entity` rows initially. Phase D admin UI allows merging by attaching the second `entity_external_identity` to the first entity (and dropping the duplicate). |
| Group with member that has not been provisioned | `400 Bad Request` (SCIM `invalidValue`). IdP must order operations: users first. |
| Cycle in group membership | Not possible: `Group --member_of--> Group` is not declared in `allowed_relations`. Nested groups are not supported in v1. |
| SCIM token leaked | Rotation: ops updates env var, restarts. The token only grants SCIM provisioning for one provider; it cannot read other providers' data, cannot read OAD admin endpoints, cannot bypass RLS on system-scoped data (SCIM never touches system-scoped data — Users/Groups are global). |
| Concurrent PATCH from same provider | Last-write-wins. ETag is advisory. |
| `is_builtin` group (e.g. `oad:admin`) targeted by SCIM PATCH | Rejected with `403 Forbidden`. The reserved groups must not be deleted or renamed via SCIM. |

---

## 14. Security considerations

- **Token isolation.** Each provider has a distinct token. Compromise of one token does not affect other providers' data.
- **Audit.** Every SCIM mutation creates an `audit_log` entry with `actor = "scim:<provider_name>"`. Read operations are recorded in `retrieval_log`.
- **Schema validation.** Every User / Group create or update is validated against the seeded JSON Schema for the type. Unknown attributes are dropped; type mismatches return `400`.
- **Rate limiting.** Out of scope for v1; flagged as a known risk for production. A misbehaving provisioner could overwhelm OAD. Recommend infrastructure-level rate limit (reverse proxy) until in-app limiting is added.
- **No system-scoped writes via SCIM.** All SCIM-managed data is global. SCIM cannot write to `property_overlay`, `relation` rows with `system_id != NULL`, or any system-scoped entity.
- **Constant-time token comparison.** Token hash lookup uses `subtle.ConstantTimeCompare` even though hashes are fixed length (defense in depth).

---

## 15. Phased implementation breakdown

Within Phase B, the rollout is:

| Sub-phase | Scope |
|---|---|
| **B.1** | Schema foundation (Phase A delta). Migration rewrite, seed types, seed groups. |
| **B.2** | Tenant token authentication. SCIM router, `/ServiceProviderConfig`, `/Schemas`, `/ResourceTypes`. |
| **B.3** | Users: CRUD endpoints, mapper, filter (subset), pagination. |
| **B.4** | Groups: CRUD endpoints, mapper, member resolution. |
| **B.5** | PATCH support for both Users and Groups (subset of paths). |
| **B.6** | Fake SCIM client + dev-stack integration. |
| **B.7** | E2E test suite + CI integration. |

Each sub-phase is independently mergeable. After B.5 the system is production-viable for any IdP that runs a SCIM client. B.6 and B.7 harden the dev/test path.

---

## 16. Open questions

| # | Question | Default if unanswered |
|---|---|---|
| Q1 | Use `github.com/elimity-com/scim` or implement subset by hand? | **Resolved:** hand-rolled subset (see §7.2). |
| Q2 | Should Dex deployments get a workaround (admin endpoint to create User entities directly)? | Yes, but defer to Phase D. |
| Q3 | Do we need a `/scim/v2/Bulk` endpoint? | No. Defer until requested. |
| Q4 | How do SCIM-driven changes propagate via webhooks to downstream consumers? | **Resolved:** deferred (see §1.2). The current per-system subscription model does not fit global SCIM events; a filter-based subscription redesign will happen when a real consumer drives the requirement. |
| Q5 | Should the IdP-group-to-`oad:*` mapping be configurable in YAML for ops convenience? | Not initially. Admin UI is sufficient and keeps configuration centralized. |

---

## 17. Revision history

| Version | Date | Changes |
|---|---|---|
| 0.1 | 2026-04-28 | Initial draft. |
| 0.2 | 2026-04-28 | Rename `is_system` → `is_builtin`. Add `Permission` built-in type and scoping model (§3.5.1). Add Permission provisioning (§3.5.2) and IGA integration future (§3.5.3). Resolve Q1 (hand-rolled parser, §7.2). Resolve Q4 (defer SCIM webhooks, §1.2). |
