# OAD — Sequence Diagrams & Use Cases (v0.1)

> Derived from the [Product Specification](spec.md), [Requirements](requirements.md), and [Data Model](data-model.md). All diagrams use Mermaid syntax for GitHub-native rendering.

## Conventions

- **Solid arrows** (`->>`) represent synchronous requests.
- **Dashed arrows** (`-->>`) represent responses.
- **`alt` blocks** show conditional / error paths.
- **`opt` blocks** show optional steps.
- **`par` blocks** show parallel / asynchronous operations.
- All API endpoints require authentication (JWT or mTLS) — NFR-SEC-001.
- All communication is encrypted in transit (TLS 1.2+) — NFR-SEC-005.
- All requests carry a correlation ID for distributed tracing — NFR-OPS-002.

---

## 1. Use Cases

### 1.1 Actor–Use Case Map

| ID | Use Case | Platform Engineer | Product Team | PDP / Control Plane | Compliance / Audit |
|---|---|:---:|:---:|:---:|:---:|
| UC-01 | Manage entity type definitions | ● | | | |
| UC-02 | Register and manage systems | ● | | | |
| UC-03 | Define system overlay schemas | ● | | | |
| UC-04 | Manage global entities | ● | | | |
| UC-05 | Manage property overlays | | ● | | |
| UC-06 | Manage system-scoped relations | | ● | | |
| UC-07 | Bulk import entities | ● | ● | | |
| UC-08 | Lookup entity (with merged view) | | | ● | |
| UC-09 | Query entity relations | | | ● | |
| UC-10 | Filter entities by properties | | | ● | |
| UC-11 | Retrieve changelog (incremental sync) | | | ● | |
| UC-12 | Bulk export (cold start / DR) | | | ● | |
| UC-13 | Manage webhook subscriptions | | | ● | |
| UC-14 | Query audit log | ● | ● | | ● |
| UC-15 | Browse entities and relations (read-only) | | | | ● |

### 1.2 Use Case Descriptions

#### UC-01 — Manage Entity Type Definitions

| Field | Detail |
|---|---|
| **Actor** | Platform Engineer |
| **Description** | Create, update, or delete entity type definitions that control what entity types exist and constrain their structure (properties via JSON Schema, allowed relations, scope). |
| **Preconditions** | Authenticated with platform admin role. |
| **Postconditions** | Type definition persisted; subsequent entity operations validated against it. Audit log entry recorded. |
| **Error paths** | Invalid JSON Schema → 400. Delete with existing entities → 400 (FR-ETD-003). |
| **Requirements** | FR-ETD-001, FR-ETD-002, FR-ETD-003, FR-ETD-004 |

#### UC-02 — Register and Manage Systems

| Field | Detail |
|---|---|
| **Actor** | Platform Engineer |
| **Description** | Register a new system (application/service), update its metadata, or deactivate it. Deactivated systems have their overlays excluded from retrieval responses. |
| **Preconditions** | Authenticated with platform admin role. |
| **Postconditions** | System record created/updated. Audit log entry recorded. |
| **Error paths** | Duplicate system name → 409. |
| **Requirements** | FR-SYS-001, FR-SYS-002, FR-SYS-003 |

#### UC-03 — Define System Overlay Schemas

| Field | Detail |
|---|---|
| **Actor** | Platform Engineer |
| **Description** | Declare which overlay properties a system can attach to entities of a given type. Schema includes JSON Schema validation and namespace-prefix enforcement. |
| **Preconditions** | Target system and entity type definition exist. Authenticated with platform admin role. |
| **Postconditions** | Overlay schema persisted; subsequent overlay writes validated against it. Audit log entry recorded. |
| **Error paths** | Invalid JSON Schema → 400 (FR-OVS-004). Non-namespaced keys → 400 (FR-OVS-005). Duplicate (system, type) → 409. |
| **Requirements** | FR-OVS-001, FR-OVS-002, FR-OVS-003, FR-OVS-004, FR-OVS-005 |

#### UC-04 — Manage Global Entities

| Field | Detail |
|---|---|
| **Actor** | Platform Engineer |
| **Description** | Create, read, update, or delete global entities. Properties are validated against the entity type definition schema. |
| **Preconditions** | Entity type definition exists. Authenticated with appropriate role. |
| **Postconditions** | Entity persisted. Deletion cascades to overlays and relations. Audit log entry recorded. |
| **Error paths** | Undeclared type → 400 (FR-ENT-008). Invalid properties → 400 (FR-ENT-003). Duplicate external_id → 409 (FR-ENT-002). |
| **Requirements** | FR-ENT-001 through FR-ENT-008 |

#### UC-05 — Manage Property Overlays

| Field | Detail |
|---|---|
| **Actor** | Product Team |
| **Description** | Attach, update, or remove system-specific properties on a global entity. Overlay properties must conform to the system overlay schema and use namespaced keys. |
| **Preconditions** | Entity exists. System overlay schema exists for the system + entity type. Caller authorized for the system. |
| **Postconditions** | Overlay properties persisted under system scope. Audit log entry recorded. |
| **Error paths** | No overlay schema → 400 (FR-OVL-003). Invalid properties → 400 (FR-OVL-002). Non-namespaced keys → 400 (FR-OVL-004). Unauthorized system → 403 (FR-OVL-008). |
| **Requirements** | FR-OVL-001 through FR-OVL-004, FR-OVL-008 |

#### UC-06 — Manage System-Scoped Relations

| Field | Detail |
|---|---|
| **Actor** | Product Team |
| **Description** | Create or delete relations between entities within a system scope. Relations are validated against the subject entity's type definition. |
| **Preconditions** | Both entities exist. Relation type is declared in the subject's type definition. Caller authorized for the system. |
| **Postconditions** | System-scoped relation persisted. Audit log entry recorded. |
| **Error paths** | Undeclared relation type → 400 (FR-REL-002). Invalid target type → 400 (FR-REL-002). Duplicate → 409 (FR-REL-003). |
| **Requirements** | FR-REL-001 through FR-REL-005, FR-OVL-005 |

#### UC-07 — Bulk Import Entities

| Field | Detail |
|---|---|
| **Actor** | Platform Engineer, Product Team |
| **Description** | Import a batch of entities in a single API call for initial loads or batch updates from authoritative sources. Each item is validated independently; individual failures do not block the rest. |
| **Preconditions** | Entity type definitions exist for all types in the batch. |
| **Postconditions** | Successfully validated entities persisted. Summary of successes and per-item failures returned. Audit log entries recorded for each mutation. |
| **Error paths** | Individual items may fail validation without affecting the batch. |
| **Requirements** | FR-ENT-007 |

#### UC-08 — Lookup Entity (Merged View)

| Field | Detail |
|---|---|
| **Actor** | PDP / Control Plane |
| **Description** | Retrieve an entity by type + external_id. When a system context is provided, the response includes global properties merged with namespaced overlay properties, plus global and system-scoped relations. |
| **Preconditions** | Caller authenticated and authorized for the requested system scope. |
| **Postconditions** | Entity data returned. Retrieval log entry recorded. |
| **Error paths** | Entity not found → 404. Unauthorized system → 403. |
| **Requirements** | FR-RET-001, FR-OVL-006, FR-OVL-007, NFR-PRF-001 |

#### UC-09 — Query Entity Relations

| Field | Detail |
|---|---|
| **Actor** | PDP / Control Plane |
| **Description** | Retrieve all relations of an entity, filterable by relation type and system scope. Results are paginated. |
| **Preconditions** | Entity exists. Caller authenticated. |
| **Postconditions** | Matching relations returned. Retrieval log entry recorded. |
| **Requirements** | FR-REL-005, FR-RET-005, NFR-PRF-003 |

#### UC-10 — Filter Entities by Properties

| Field | Detail |
|---|---|
| **Actor** | PDP / Control Plane |
| **Description** | Query entities by property values (e.g., all users with `department=ops`). Leverages GIN index on `entity.properties`. |
| **Preconditions** | Caller authenticated. |
| **Postconditions** | Matching entities returned (paginated). Retrieval log entry recorded. |
| **Requirements** | FR-RET-002, FR-RET-005 |

#### UC-11 — Retrieve Changelog

| Field | Detail |
|---|---|
| **Actor** | PDP / Control Plane |
| **Description** | Retrieve an ordered list of entity, property, relation, and overlay changes since a given timestamp. Used for incremental sync by external control planes. |
| **Preconditions** | Caller authenticated. |
| **Postconditions** | Change events returned (paginated). Retrieval log entry recorded. |
| **Requirements** | FR-RET-003, FR-RET-005, NFR-PRF-004 |

#### UC-12 — Bulk Export

| Field | Detail |
|---|---|
| **Actor** | PDP / Control Plane |
| **Description** | Export all entities and relations in paginated batches for cold start or disaster recovery of edge caches. |
| **Preconditions** | Caller authenticated. |
| **Postconditions** | Complete dataset returned across paginated requests. Retrieval log entry recorded. |
| **Requirements** | FR-RET-004, FR-RET-005 |

#### UC-13 — Manage Webhook Subscriptions

| Field | Detail |
|---|---|
| **Actor** | PDP / Control Plane |
| **Description** | Subscribe to, list, update, or delete webhook event subscriptions for a specific system. Each subscription registers a callback URL and a shared HMAC secret. |
| **Preconditions** | Target system exists. Caller authenticated. |
| **Postconditions** | Subscription persisted. Active subscriptions receive notifications on data changes. |
| **Requirements** | FR-WHK-001, FR-WHK-003 |

#### UC-14 — Query Audit Log

| Field | Detail |
|---|---|
| **Actor** | Platform Engineer, Product Team (own system), Compliance / Audit |
| **Description** | Search and filter the audit log by entity, system, actor, operation type, and time range. Results are paginated and immutable. |
| **Preconditions** | Caller authenticated. Access scoped to authorized systems. |
| **Postconditions** | Matching audit entries returned. |
| **Requirements** | FR-AUD-004, FR-MGT-005 |

#### UC-15 — Browse Entities and Relations (Read-Only)

| Field | Detail |
|---|---|
| **Actor** | Compliance / Audit |
| **Description** | View entities, their properties, relations, and overlays through the Management UI in read-only mode. |
| **Preconditions** | Authenticated with viewer role. |
| **Postconditions** | No data modified. Retrieval log entry recorded. |
| **Requirements** | FR-MGT-001, FR-MGT-004 |

---

## 2. Ingestion Flows

### 2.1 Create Entity

```mermaid
sequenceDiagram
    actor Client
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Client->>GW: POST /entities<br>{type, external_id, properties}
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Client: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    App->>DB: SELECT * FROM entity_type_definition<br>WHERE type_name = {type}

    alt Type not found
        App-->>Client: 400 Undeclared entity type
    end

    Note over App: Validate properties against<br>allowed_properties (JSON Schema)

    alt Validation fails
        App-->>Client: 400 {errors: [...]}
    end

    opt Type scope = system_scoped
        Note over App: Verify system_id is provided<br>and caller is authorized
        alt Unauthorized
            App-->>Client: 403 Forbidden
        end
    end

    App->>DB: BEGIN
    App->>DB: INSERT INTO entity<br>(type_id, external_id, properties, system_id)

    alt Duplicate (type_id, external_id)
        DB-->>App: UNIQUE violation
        App->>DB: ROLLBACK
        App-->>Client: 409 Conflict
    end

    App->>DB: INSERT INTO audit_log<br>(actor, 'create', 'entity',<br>entity_id, NULL, after_value)
    App->>DB: COMMIT

    App-->>Client: 201 Created {entity}

    par Async: webhook notification
        App->>DB: SELECT * FROM webhook_subscription<br>WHERE system_id = {system_id} AND active = true
        Note over App: Queue webhook deliveries<br>(see §5.1)
    end
```

**Requirement traceability:** FR-ENT-001, FR-ENT-002, FR-ENT-003, FR-ENT-008, FR-AUD-001, NFR-SEC-001, NFR-AUD-001.

---

### 2.2 Bulk Import Entities

```mermaid
sequenceDiagram
    actor Client
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Client->>GW: POST /entities/bulk<br>[{type, external_id, properties}, ...]
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Client: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    App->>DB: BEGIN

    loop For each entity in batch
        App->>DB: SELECT * FROM entity_type_definition<br>WHERE type_name = {type}

        alt Type not found
            Note over App: Record failure for this item;<br>continue to next
        else Type found
            Note over App: Validate properties<br>against allowed_properties
            alt Validation fails
                Note over App: Record failure for this item;<br>continue to next
            else Valid
                App->>DB: INSERT INTO entity<br>ON CONFLICT (type_id, external_id)<br>DO UPDATE SET properties, updated_at
                App->>DB: INSERT INTO audit_log
                Note over App: Record success for this item
            end
        end
    end

    App->>DB: COMMIT

    App-->>Client: 200 OK<br>{successes: N, failures: [{index, error}]}

    par Async: webhook notifications
        Note over App: Queue webhook deliveries<br>for affected systems
    end
```

**Requirement traceability:** FR-ENT-007, FR-ENT-003, FR-ENT-008, FR-AUD-001, NFR-PRF-002.

---

### 2.3 Create Relation

```mermaid
sequenceDiagram
    actor Client
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Client->>GW: POST /relations<br>{subject_entity_id, relation_type,<br>target_entity_id, system_id?}
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Client: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    opt system_id provided
        Note over App: Verify caller is authorized<br>for this system scope
        alt Unauthorized
            App-->>Client: 403 Forbidden
        end
    end

    App->>DB: SELECT e.*, etd.allowed_relations<br>FROM entity e<br>JOIN entity_type_definition etd<br>ON e.type_id = etd.id<br>WHERE e.id = {subject_entity_id}

    alt Subject entity not found
        App-->>Client: 400 Subject entity not found
    end

    Note over App: Validate relation_type exists<br>in subject's allowed_relations

    alt Invalid relation type
        App-->>Client: 400 Undeclared relation type
    end

    App->>DB: SELECT e.*, etd.type_name<br>FROM entity e<br>JOIN entity_type_definition etd<br>ON e.type_id = etd.id<br>WHERE e.id = {target_entity_id}

    alt Target entity not found
        App-->>Client: 400 Target entity not found
    end

    Note over App: Validate target entity type<br>is allowed for this relation_type

    alt Invalid target type
        App-->>Client: 400 Target type not allowed<br>for this relation
    end

    App->>DB: BEGIN
    App->>DB: INSERT INTO relation<br>(subject_entity_id, relation_type,<br>target_entity_id, system_id)

    alt Duplicate relation
        DB-->>App: UNIQUE violation
        App->>DB: ROLLBACK
        App-->>Client: 409 Conflict
    end

    App->>DB: INSERT INTO audit_log<br>(actor, 'create', 'relation',<br>relation_id, NULL, after_value)
    App->>DB: COMMIT

    App-->>Client: 201 Created {relation}
```

**Requirement traceability:** FR-REL-001, FR-REL-002, FR-REL-003, FR-OVL-005, FR-AUD-001, NFR-SEC-001, NFR-SEC-002.

---

### 2.4 Create Property Overlay

The most validation-intensive ingestion flow. Enforces schema validation and namespace prefixing to prevent attribute pollution and key collisions.

```mermaid
sequenceDiagram
    actor Client as Product Team
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Client->>GW: POST /systems/{system_id}/entities/{entity_id}/overlay<br>{properties: {"credit.max_approval": 500000}}
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Client: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    Note over App: Authorize caller for system_id

    alt Unauthorized for this system
        App-->>Client: 403 Forbidden
    end

    App->>DB: SET LOCAL app.current_system_id = {system_id}

    App->>DB: SELECT e.*, etd.type_name<br>FROM entity e<br>JOIN entity_type_definition etd<br>ON e.type_id = etd.id<br>WHERE e.id = {entity_id}

    alt Entity not found
        App-->>Client: 404 Entity not found
    end

    App->>DB: SELECT sos.*, s.name AS system_name<br>FROM system_overlay_schema sos<br>JOIN system s ON sos.system_id = s.id<br>WHERE sos.system_id = {system_id}<br>AND sos.entity_type_id = {entity_type_id}

    alt No overlay schema declared
        App-->>Client: 400 No overlay schema exists<br>for this system + entity type
    end

    Note over App: Validate all property keys<br>are prefixed with "{system_name}."

    alt Non-namespaced key found
        App-->>Client: 400 Property keys must be<br>prefixed with "{system_name}."
    end

    Note over App: Validate properties against<br>allowed_overlay_properties<br>(JSON Schema)

    alt Schema validation fails
        App-->>Client: 400 {errors: [...]}
    end

    App->>DB: BEGIN
    App->>DB: INSERT INTO property_overlay<br>(entity_id, system_id, properties)<br>ON CONFLICT (entity_id, system_id)<br>DO UPDATE SET properties, updated_at

    App->>DB: INSERT INTO audit_log<br>(actor, 'create/update',<br>'property_overlay', overlay_id,<br>before_value, after_value, system_id)
    App->>DB: COMMIT

    App-->>Client: 201 Created / 200 Updated {overlay}

    par Async: webhook notification
        App->>DB: SELECT * FROM webhook_subscription<br>WHERE system_id = {system_id}<br>AND active = true
        Note over App: Queue webhook deliveries<br>(see §5.1)
    end
```

**Requirement traceability:** FR-OVL-001, FR-OVL-002, FR-OVL-003, FR-OVL-004, FR-OVL-008, FR-AUD-001, NFR-SEC-001, NFR-SEC-002.

---

## 3. Retrieval Flows

### 3.1 Entity Lookup (Merged View)

The primary retrieval path for PDPs at policy evaluation time. When a system context is provided, global properties are merged with namespaced overlay properties (disjoint key sets by design), and both global and system-scoped relations are returned.

```mermaid
sequenceDiagram
    actor PDP as PDP / Control Plane
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    PDP->>GW: GET /entities?type=user<br>&external_id=daniel&system=credit
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>PDP: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    opt System context provided
        Note over App: Authorize caller<br>for requested system scope
        alt Unauthorized
            App-->>PDP: 403 Forbidden
        end
        App->>DB: SET LOCAL app.current_system_id<br>= {system_id}
    end

    App->>DB: SELECT e.*, etd.type_name<br>FROM entity e<br>JOIN entity_type_definition etd<br>ON e.type_id = etd.id<br>WHERE etd.type_name = 'user'<br>AND e.external_id = 'daniel'

    alt Entity not found
        App-->>PDP: 404 Not Found
    end

    opt System context provided
        App->>DB: SELECT po.properties<br>FROM property_overlay po<br>JOIN system s ON po.system_id = s.id<br>WHERE po.entity_id = {entity_id}<br>AND po.system_id = {system_id}<br>AND s.active = true

        Note over App: Merge:<br>entity.properties || overlay.properties<br>(disjoint keys guaranteed by namespace)
    end

    App->>DB: SELECT r.* FROM relation r<br>WHERE (r.subject_entity_id = {entity_id}<br>OR r.target_entity_id = {entity_id})<br>AND (r.system_id IS NULL<br>OR r.system_id = {system_id})

    App->>DB: INSERT INTO retrieval_log<br>(caller_identity, query_parameters,<br>returned_refs, system_id)

    App-->>PDP: 200 OK<br>{entity: {id, type, external_id,<br>properties: {merged}},<br>relations: [...]}
```

**AuthZen mapping:** The response maps directly to `subject.type` + `subject.id` + `subject.properties` (or `resource.*`) per the AuthZen evaluation request format (spec §4.1).

**Requirement traceability:** FR-RET-001, FR-OVL-006, FR-OVL-007, FR-AUD-002, NFR-PRF-001, NFR-CMP-001.

---

### 3.2 Relation Query

```mermaid
sequenceDiagram
    actor PDP as PDP / Control Plane
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    PDP->>GW: GET /entities/{entity_id}/relations<br>?relation_type=member&system=credit
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>PDP: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    App->>DB: SELECT r.*,<br>se.external_id AS subject_external_id,<br>setd.type_name AS subject_type,<br>te.external_id AS target_external_id,<br>tetd.type_name AS target_type<br>FROM relation r<br>JOIN entity se ON r.subject_entity_id = se.id<br>JOIN entity_type_definition setd ON se.type_id = setd.id<br>JOIN entity te ON r.target_entity_id = te.id<br>JOIN entity_type_definition tetd ON te.type_id = tetd.id<br>WHERE r.subject_entity_id = {entity_id}<br>AND r.relation_type = 'member'<br>AND (r.system_id IS NULL<br>OR r.system_id = {system_id})<br>ORDER BY r.created_at<br>LIMIT {page_size} OFFSET {offset}

    App->>DB: INSERT INTO retrieval_log<br>(caller_identity, query_parameters,<br>returned_refs, system_id)

    App-->>PDP: 200 OK<br>{relations: [...],<br>pagination: {cursor, total, has_next}}
```

**Requirement traceability:** FR-REL-005, FR-RET-005, FR-AUD-002, NFR-PRF-003.

---

### 3.3 Changelog (Incremental Sync)

Used by PDP control planes to synchronize only changes since their last sync point, avoiding full data reloads.

```mermaid
sequenceDiagram
    actor CP as Control Plane
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    CP->>GW: GET /changelog<br>?since=2026-04-10T00:00:00Z<br>&system=credit
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>CP: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    App->>DB: SELECT * FROM audit_log<br>WHERE timestamp > {since}<br>AND (system_id = {system_id}<br>OR system_id IS NULL)<br>AND resource_type IN<br>('entity', 'relation',<br>'property_overlay')<br>ORDER BY timestamp ASC<br>LIMIT {page_size}

    App->>DB: INSERT INTO retrieval_log<br>(caller_identity, query_parameters,<br>returned_refs, system_id)

    App-->>CP: 200 OK<br>{changes: [{timestamp, operation,<br>resource_type, resource_id,<br>before_value, after_value}],<br>pagination: {cursor, has_next}}

    Note over CP: Control plane processes<br>changes and updates PDP<br>(e.g., OPA bundle via OCP,<br>Topaz edge sync,<br>OPAL data fetcher)
```

**Requirement traceability:** FR-RET-003, FR-RET-005, FR-AUD-002, NFR-PRF-004.

---

## 4. Administration Flows

### 4.1 Create Entity Type Definition

```mermaid
sequenceDiagram
    actor Admin as Platform Engineer
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Admin->>GW: POST /entity-types<br>{type_name, allowed_properties,<br>allowed_relations, scope}
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Admin: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    Note over App: Authorize:<br>platform admin role required

    alt Not platform admin
        App-->>Admin: 403 Forbidden
    end

    Note over App: Validate allowed_properties<br>is a valid JSON Schema document

    alt Invalid JSON Schema
        App-->>Admin: 400 Invalid JSON Schema
    end

    Note over App: Validate allowed_relations:<br>each relation_type maps<br>to valid target types

    App->>DB: BEGIN
    App->>DB: INSERT INTO entity_type_definition<br>(type_name, allowed_properties,<br>allowed_relations, scope)

    alt Duplicate type_name
        DB-->>App: UNIQUE violation
        App->>DB: ROLLBACK
        App-->>Admin: 409 Conflict
    end

    App->>DB: INSERT INTO audit_log<br>(actor, 'create',<br>'entity_type_definition',<br>etd_id, NULL, after_value)
    App->>DB: COMMIT

    App-->>Admin: 201 Created {entity_type_definition}
```

**Requirement traceability:** FR-ETD-001, FR-ETD-004, FR-AUD-001, NFR-EXT-001.

---

### 4.2 Create System Overlay Schema

```mermaid
sequenceDiagram
    actor Admin as Platform Engineer
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Admin->>GW: POST /systems/{system_id}/overlay-schemas<br>{entity_type_id,<br>allowed_overlay_properties}
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Admin: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    Note over App: Authorize:<br>platform admin role required

    alt Not platform admin
        App-->>Admin: 403 Forbidden
    end

    App->>DB: SELECT * FROM system<br>WHERE id = {system_id}

    alt System not found
        App-->>Admin: 404 System not found
    end

    App->>DB: SELECT * FROM entity_type_definition<br>WHERE id = {entity_type_id}

    alt Entity type not found
        App-->>Admin: 404 Entity type not found
    end

    Note over App: Validate allowed_overlay_properties<br>is a valid JSON Schema document

    alt Invalid JSON Schema
        App-->>Admin: 400 Invalid JSON Schema
    end

    Note over App: Validate all property keys<br>are prefixed with<br>"{system.name}."

    alt Non-namespaced key detected
        App-->>Admin: 400 Keys must be prefixed<br>with "{system.name}."
    end

    App->>DB: BEGIN
    App->>DB: INSERT INTO system_overlay_schema<br>(system_id, entity_type_id,<br>allowed_overlay_properties)

    alt Duplicate (system_id, entity_type_id)
        DB-->>App: UNIQUE violation
        App->>DB: ROLLBACK
        App-->>Admin: 409 Conflict
    end

    App->>DB: INSERT INTO audit_log<br>(actor, 'create',<br>'system_overlay_schema',<br>schema_id, NULL, after_value,<br>system_id)
    App->>DB: COMMIT

    App-->>Admin: 201 Created {system_overlay_schema}
```

**Requirement traceability:** FR-OVS-001, FR-OVS-004, FR-OVS-005, FR-AUD-001.

---

### 4.3 Register System

```mermaid
sequenceDiagram
    actor Admin as Platform Engineer
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    Admin->>GW: POST /systems<br>{name, description}
    GW->>GW: Authenticate (JWT / mTLS)

    alt Authentication fails
        GW-->>Admin: 401 Unauthorized
    end

    GW->>App: Forward request + caller identity

    Note over App: Authorize:<br>platform admin role required

    alt Not platform admin
        App-->>Admin: 403 Forbidden
    end

    App->>DB: BEGIN
    App->>DB: INSERT INTO system<br>(name, description)

    alt Duplicate system name
        DB-->>App: UNIQUE violation
        App->>DB: ROLLBACK
        App-->>Admin: 409 Conflict
    end

    App->>DB: INSERT INTO audit_log<br>(actor, 'create', 'system',<br>system_id, NULL, after_value)
    App->>DB: COMMIT

    App-->>Admin: 201 Created {system}
```

**Requirement traceability:** FR-SYS-001, FR-AUD-001.

---

## 5. Cross-Cutting Flows

### 5.1 Webhook Notification & Delivery

Triggered asynchronously after any write operation that produces an audit log entry.

```mermaid
sequenceDiagram
    participant App as OAD Application
    participant DB as PostgreSQL
    participant WD as Webhook Dispatcher
    participant Sub as Subscriber Endpoint

    Note over App: Write operation committed<br>(entity / relation / overlay)

    App->>WD: Enqueue event<br>(audit_log_id, system_id)

    WD->>DB: SELECT ws.*<br>FROM webhook_subscription ws<br>WHERE ws.system_id = {system_id}<br>AND ws.active = true

    loop For each active subscription
        WD->>DB: INSERT INTO webhook_delivery<br>(subscription_id, audit_log_id,<br>status = 'pending', attempts = 0)

        Note over WD: Build event payload<br>from audit_log entry

        Note over WD: Compute HMAC-SHA256<br>using subscription.secret

        WD->>Sub: POST {callback_url}<br>X-OAD-Signature: sha256={hmac}<br>Body: {event payload}

        alt 2xx response
            WD->>DB: UPDATE webhook_delivery<br>SET status = 'delivered',<br>last_response_code = {code}
        else Non-2xx or timeout
            WD->>DB: UPDATE webhook_delivery<br>SET attempts = attempts + 1,<br>last_response_code = {code},<br>next_retry_at = now()<br>+ backoff(attempts)

            loop Retry until delivered or max attempts
                Note over WD: Wait until next_retry_at
                WD->>Sub: POST {callback_url} (retry)
                alt 2xx response
                    WD->>DB: UPDATE webhook_delivery<br>SET status = 'delivered'
                else Max attempts reached
                    WD->>DB: UPDATE webhook_delivery<br>SET status = 'failed'
                end
            end
        end
    end
```

**Exponential backoff:** `delay = base_interval * 2^attempts`, capped at a configurable maximum interval.

**Requirement traceability:** FR-WHK-002, FR-WHK-004.

---

### 5.2 Management UI Authentication & Authorization

The Management UI delegates authentication to an external Identity Provider (OIDC). User roles and system assignments are conveyed as JWT claims, avoiding a circular dependency where OAD would query itself for access control.

```mermaid
sequenceDiagram
    actor User as Admin / Editor / Viewer
    participant UI as Management UI
    participant IdP as Identity Provider
    participant GW as API Gateway
    participant App as OAD Application
    participant DB as PostgreSQL

    User->>UI: Navigate to OAD Management UI

    UI->>IdP: OIDC Authorization Request<br>(redirect to IdP login)

    User->>IdP: Authenticate<br>(credentials + MFA)

    IdP-->>UI: Authorization Code

    UI->>IdP: Exchange code for tokens

    IdP-->>UI: JWT access token<br>Claims: {sub, role, system_ids}

    Note over UI: Store JWT<br>in secure storage

    User->>UI: Perform action<br>(e.g., create overlay for System A)

    UI->>GW: API request +<br>Authorization: Bearer {JWT}

    GW->>GW: Validate JWT<br>signature + expiry

    alt Invalid or expired JWT
        GW-->>UI: 401 Unauthorized
        UI->>IdP: Re-authenticate
    end

    GW->>App: Forward request + JWT claims

    Note over App: Extract role and<br>system_ids from claims

    alt Role check fails (e.g., viewer writes)
        App-->>UI: 403 Forbidden
    end

    alt System scope check fails
        Note over App: Caller for System A<br>attempting System B operation
        App-->>UI: 403 Forbidden
    end

    App->>DB: SET LOCAL app.current_system_id<br>= {system_id}

    Note over App,DB: RLS policies enforce<br>row-level system isolation

    App->>DB: Execute operation

    App-->>UI: Response
    UI-->>User: Display result
```

**Requirement traceability:** FR-MGT-001 through FR-MGT-005, NFR-SEC-001, NFR-SEC-002, NFR-SEC-004.

---

## 6. Cross-Cutting Concerns

The following concerns apply to all flows and are enforced consistently across the system:

| Concern | Enforcement | Requirements |
|---|---|---|
| **Transport security** | TLS 1.2+ on all API endpoints; TLS 1.1 and below rejected. | NFR-SEC-005 |
| **Authentication** | Every API call requires JWT or mTLS. Unauthenticated requests receive 401. | NFR-SEC-001 |
| **Authorization** | System-scoped access control on every operation. Caller credentials determine visible systems. | NFR-SEC-002, NFR-SEC-004 |
| **Input validation** | All inputs validated at the API boundary. Parameterized queries for all database operations. | NFR-SEC-003, NFR-SEC-006 |
| **Audit trail** | Every write produces an immutable `audit_log` entry within the same database transaction. Every read produces a `retrieval_log` entry. | NFR-AUD-001, FR-AUD-001, FR-AUD-002 |
| **Row-Level Security** | RLS policies on `entity`, `relation`, `property_overlay`, `webhook_subscription` restrict access by `app.current_system_id` session variable. | NFR-SEC-002 |
| **Structured logging** | JSON-formatted logs with `correlation_id` per request for distributed tracing. | NFR-OPS-002 |
| **Observability** | Prometheus metrics on `/metrics` endpoint: request count, latency histograms (p50 / p95 / p99), error rates. | NFR-OPS-003 |
| **Secrets management** | No secrets in source code or logs. All secrets loaded from environment variables or a secrets manager. | NFR-SEC-007 |

---

## Revision History

| Version | Date | Changes |
|---|---|---|
| 0.1 | 2026-04-10 | Initial draft — use cases (UC-01..15), ingestion flows (entity, bulk import, relation, overlay), retrieval flows (merged view, relation query, changelog), admin flows (type definition, overlay schema, system registration), cross-cutting flows (webhook delivery, UI authentication) |
