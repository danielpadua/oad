# OAD — Open Authoritative Directory

## Product Specification (MVP)

### 1. Problem Statement

Broken Access Control is the #1 vulnerability in the OWASP Top 10. A root cause is that authorization decisions rely on scattered, stale, or inconsistent attribute data — roles stored in one system, resource metadata in another, environment context nowhere at all.

Policy Decision Points (PDPs) need a reliable, unified source of authorization attributes at evaluation time. Today, most organizations either:

- **Hardcode attributes into application logic** — brittle, unauditable, and impossible to govern at scale.
- **Couple attribute storage to a specific PDP** — creates vendor lock-in and prevents multi-PDP architectures.
- **Build ad-hoc attribute resolvers per application** — duplicated effort, inconsistent data, no central audit trail.

There is no widely adopted, open-source, PDP-agnostic attribute repository with a management interface designed for enterprise multi-system environments.

### 2. Target Audience

| Persona | Role | Interaction |
|---|---|---|
| **Platform / Security Engineer** | Deploys and operates the PIP; configures ingestion connectors and API access policies | Admin API, Management UI |
| **Product / Application Team** | Manages attributes for their specific system (subjects, resources, actions) | Management UI (scoped to their system) |
| **PDP / Control Plane** | Consumes attributes at policy evaluation time or syncs them to edge caches | Retrieval API, Changelog API |
| **Compliance / Audit Team** | Reviews attribute change history and access logs | Audit API, Management UI (read-only) |

### 3. Product Vision

OAD is a **standardized Policy Information Point (PIP)** — an open-source, centralized attribute repository that:

- Stores **entities** (subjects, resources, roles, groups, and any typed object) with **properties** (flexible attributes for ABAC) and **relations** (typed edges for RBAC and ReBAC), aligned with the [OpenID AuthZen](https://openid.net/wg/authzen/) data model.
- Supports **global entities with system-scoped overlays** — identity attributes from authoritative sources are stored once; each system adds only system-specific properties and relations.
- Exposes well-defined **ingestion and retrieval APIs** so any PDP ecosystem (OPA, Cedar, Topaz, Cerbos, custom) can consume attributes without coupling.
- Provides a **management interface** where product teams can govern their own entities, properties, and relations, segregated by system.
- Maintains an **immutable audit log** of every data change and retrieval event.

OAD does **not** evaluate policies, enforce access decisions, or push/sync data to PDPs. It is the **authoritative source** that PDP ecosystems pull from using their own distribution mechanisms (OPA bundles via OCP, Topaz edge sync, OPAL data fetchers, etc.). Relationship graph traversal (ReBAC-style `check` or `expand`) is PDP responsibility — the PIP stores the graph, the PDP walks it.

### 4. MVP Scope

#### 4.1 Core Entities

The data model uses a **unified entity–relation graph** inspired by Google Zanzibar's approach, aligned with AuthZen's request structure. This design supports RBAC, ABAC, and ReBAC with a single model.

##### Entity

A typed node in the authorization graph. Entities replace the separate Subject/Resource/Action concepts with a single, flexible structure:

- `id` (UUID) — internal primary key.
- `type` (string) — the kind of entity: `user`, `group`, `role`, `service_account`, `document`, `account`, `action`, etc. Types are declared in the Entity Type Definition.
- `external_id` (string) — identifier from the source system (e.g., employee ID, resource ARN). Unique within a type.
- `properties` (JSONB) — flexible key-value attributes for ABAC (e.g., `{"department": "ops", "clearance": "L3"}`).

Entities can represent anything the authorization model needs: principals, resources, roles, permissions, groups, organizational units.

##### Relation

A typed, directed edge between two entities — the building block for RBAC and ReBAC:

- `subject_entity` → Entity (the source of the relation).
- `relation_type` (string) — the kind of edge: `member`, `owner`, `viewer`, `parent`, `grants`, `assignee`, etc. Allowed types are declared in the Entity Type Definition.
- `target_entity` → Entity (the target of the relation).

Relations are always directional: "Daniel (`subject`) --member--> Approver Role (`target`)".

##### System

A registered application or service whose authorization data is managed in OAD:

- Systems define the **management boundary** — each product team governs entities, properties, and relations within their system scope.
- Systems do **not** define the data ownership boundary for entities themselves — entities are global.

##### System Overlay

System-specific properties and relations layered on top of global entities:

- **Property overlays** — a system can attach additional properties to a global entity (e.g., System "Credit" adds `max_approval: 500000` to user "Daniel" without modifying his global properties).
- **System-scoped relations** — a relation can optionally be scoped to a system (e.g., Daniel --member--> Approver is true only within System "Credit").

When a PDP requests an entity in a system context, the response is a **merged view**: global properties + system overlay properties, global relations + system-scoped relations.

##### Entity Type Definition (Schema)

Declares what entity types exist and constrains their structure:

- `type_name` (string) — e.g., `user`, `role`, `document`.
- `allowed_properties` (JSON Schema) — validates which properties an entity of this type can have and their data types.
- `allowed_relations` — which `relation_type` values are valid, and which target entity types each relation can point to.
- `scope` — whether entities of this type are `global` (shared across systems) or `system-scoped` (exist only within a single system).

##### How each authorization model is served

**ABAC** — "Deny access if user.clearance < document.sensitivity":
```
Entity(type=user, id=daniel, properties={clearance: "L3"})
Entity(type=document, id=doc-123, properties={sensitivity: "L2"})
→ PDP queries properties of both entities and evaluates the policy.
```

**RBAC** — "Allow if subject has role approver with permission approve_credit":
```
Entity(type=user, id=daniel)
Entity(type=role, id=approver)
Entity(type=permission, id=approve_credit)
Relation(daniel --member--> approver)          [system: credit]
Relation(approver --grants--> approve_credit)  [system: credit]
→ PDP queries relations and resolves the chain.
```

**ReBAC** — "Allow if subject is owner or viewer of the resource":
```
Entity(type=user, id=daniel)
Entity(type=document, id=doc-123)
Relation(daniel --owner--> doc-123)            [system: credit]
→ PDP queries direct relations and evaluates.
```

##### AuthZen mapping

| AuthZen request field | OAD source |
|---|---|
| `subject.type` + `subject.id` | Entity lookup by `type` + `external_id` |
| `subject.properties` | Entity global properties merged with system overlay |
| `resource.type` + `resource.id` | Entity lookup by `type` + `external_id` |
| `resource.properties` | Entity global properties merged with system overlay |
| `action.name` | Entity of type `action`, or plain string — PDP's choice |
| `context` | Relevant relations + environment attributes |

#### 4.2 Functional Scope

**Entity & Relation Ingestion**
- CRUD APIs for entities, properties, and relations.
- Global entities and properties are managed by platform/identity teams.
- System overlays (additional properties and system-scoped relations) are managed by product teams within their system scope.
- Bulk import endpoint for initial loads and batch updates from authoritative sources.
- Validation against Entity Type Definitions — reject entities with undeclared properties or invalid relations at the boundary.

**Entity & Relation Retrieval**
- Lookup by entity type + external ID, optionally within a system context (returns merged global + overlay view).
- Relation queries (e.g., "all entities related to Daniel via `member` in system Credit").
- Filtered queries on properties (e.g., "all entities of type `user` with department=ops").
- Changelog endpoint — "what changed since timestamp T?" — for incremental sync by external control planes.
- Bulk export (paginated) for cold start and disaster recovery of edge caches.

**Webhook / Event Notifications**
- Consumers can subscribe to attribute change events per system.
- Enables real-time integration with PDP control planes without polling.

**Management Interface**
- Web UI for administrators to view, create, edit, and delete entities, properties, relations, and system overlays.
- System-level segregation — each product team sees global entities but can only modify their own system overlays and system-scoped relations.
- Platform administrators can manage global entities and Entity Type Definitions.
- Role-based access within the UI (admin, editor, viewer).

**Audit Log**
- Immutable record of every write operation on entities, properties, relations, and overlays (who changed what, when, previous value).
- Record of retrieval events (which PDP requested which entities/relations, when).
- Queryable via API and visible in the management UI.

#### 4.3 Non-Functional Intent (MVP)

These will be detailed in the requirements document, but the MVP targets:

- **Latency**: Retrieval API p99 < 100ms for single-entity lookups.
- **Availability**: Stateless application tier behind a load balancer; PostgreSQL as the durable store.
- **Security**: All API endpoints authenticated (JWT or mTLS). System-scoped access control on every operation.
- **Auditability**: Zero write operations without an audit trail entry.
- **Extensibility**: Entity Type Definitions are dynamic (no schema migration needed to add a new property or relation type).

#### 4.4 Explicitly Out of Scope (MVP)

- Policy evaluation or decision-making (PDP responsibility).
- Policy authoring or management (PAP responsibility).
- Request interception or enforcement (PEP responsibility).
- Data sync/push to PDPs — the PIP exposes APIs; sync is the consumer's responsibility.
- Identity provider functionality (authentication of end-users).
- Relationship graph traversal (ReBAC-style `check` or `expand`) — the PIP stores the graph; the PDP walks it.

### 5. Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| **PDP coupling** | None — PDP-agnostic | Maximizes adoption; avoids competing with PDP-specific ecosystems |
| **Data model** | Unified entity–relation graph (Zanzibar-inspired) mapped to AuthZen | Single model serves RBAC, ABAC, and ReBAC; AuthZen mapping is a view layer, not the storage schema |
| **Multi-tenancy model** | Global entities + system-scoped overlays | Avoids data duplication for shared entities (users, groups); product teams manage only their system-specific properties and relations |
| **Storage engine** | PostgreSQL | Relational model supports ad-hoc queries, secondary indexes, row-level security, and proven HA/replication; avoids bbolt single-process limitation |
| **Entity schema** | Dynamic Entity Type Definitions with JSON Schema validation + JSONB properties | New properties and relation types can be declared without schema migrations; essential for a multi-tenant attribute store |
| **Sync mechanism** | Not owned — expose changelog + webhooks + bulk export | Each PDP ecosystem has its own sync tooling; coupling to one kills agnosticity |
| **Audit strategy** | Append-only audit table with retrieval logging | Compliance requirement; immutable by design |

### 6. Key Risks

| Risk | Impact | Mitigation |
|---|---|---|
| AuthZen spec changes before 1.0 | Data model misalignment | Internal model is a superset (entity–relation graph); AuthZen mapping is a view layer that can adapt without storage changes |
| Low adoption due to "just another auth tool" perception | Wasted effort | Clear positioning as PIP-only (not a PDP); target teams already running OPA/Cedar/Topaz who lack a central attribute store |
| Property schema flexibility vs. query performance | Slow filtered queries on dynamic properties | JSONB with GIN indexes for flexible properties; dedicated columns for high-cardinality lookups |
| Multi-tenant data leakage via overlays | Security incident | Row-level security in PostgreSQL; mandatory system_id on overlay/relation queries; integration tests that assert cross-system isolation |
| Overlay merge complexity | Conflicting or ambiguous property resolution | Clear merge rule: system overlay wins over global for same key; documented and enforced in the retrieval API |

### 7. Success Criteria (MVP)

- A PDP (e.g., OPA via OCP) can pull entity properties and relations from OAD's retrieval API and use them in ABAC, RBAC, or ReBAC policy evaluation.
- Two independent systems can manage their overlays and system-scoped relations in isolation through the management UI, while sharing global entities.
- Every mutation (entity, property, relation, overlay) is recorded in the audit log with before/after values.
- The changelog API enables incremental sync without full data reload.
