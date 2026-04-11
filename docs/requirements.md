# OAD — Functional & Non-Functional Requirements (v0.2)

> Derived from the [Product Specification](spec.md). Each requirement is traceable to a spec section and includes verifiable acceptance criteria.

---

## 1. Functional Requirements

### 1.1 Entity Type Definitions (Schema)

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-ETD-001 | The system must allow administrators to create entity type definitions specifying a `type_name`, `allowed_properties` (JSON Schema), `allowed_relations`, and `scope` (global or system-scoped). | A new entity type definition can be created via the Admin API; the definition is persisted and returned on subsequent reads. | 4.1 |
| FR-ETD-002 | The system must allow administrators to update existing entity type definitions without requiring a database schema migration. | Modifying `allowed_properties` or `allowed_relations` of an existing type takes effect immediately for subsequent validations, with no downtime or migration step. | 4.1, 5 |
| FR-ETD-003 | The system must allow administrators to delete entity type definitions that have no associated entities. | Deletion succeeds when zero entities reference the type; deletion is rejected with an error when entities of that type exist. | 4.1 |
| FR-ETD-004 | The system must validate that `allowed_properties` conforms to a valid JSON Schema document. | Creating or updating a type definition with invalid JSON Schema returns a 400 error with a descriptive message. | 4.1 |

### 1.2 Entity Management

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-ENT-001 | The system must allow creation of entities with `type`, `external_id`, and optional `properties`. | A POST request with valid payload returns a 201 response containing the created entity with a generated `id` (UUID). | 4.1, 4.2 |
| FR-ENT-002 | The system must enforce uniqueness of `external_id` within an entity `type`. | Creating a second entity with the same `type` + `external_id` returns a 409 Conflict error. | 4.1 |
| FR-ENT-003 | The system must validate entity `properties` against the corresponding entity type definition's `allowed_properties` schema at creation and update time. | Submitting properties that violate the JSON Schema returns a 400 error listing each validation failure. | 4.1, 4.2 |
| FR-ENT-004 | The system must allow retrieval of an entity by its `type` + `external_id`. | A GET request returns 200 with the entity's full representation; 404 when no match exists. | 4.2 |
| FR-ENT-005 | The system must allow updating an entity's `properties`. | A PATCH/PUT request with valid properties returns the updated entity; the previous property values are preserved in the audit log. | 4.2 |
| FR-ENT-006 | The system must allow deletion of entities. | A DELETE request removes the entity, its property overlays, and its relations; returns 204 on success. | 4.2 |
| FR-ENT-007 | The system must support bulk import of entities in a single API call for initial loads and batch updates. | A bulk endpoint accepts a list of entities (create or upsert), processes them, and returns a summary of successes and failures per item. | 4.2 |
| FR-ENT-008 | The system must reject entities whose `type` does not match any declared entity type definition. | Creating an entity with an undeclared type returns a 400 error. | 4.2 |

### 1.3 Relation Management

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-REL-001 | The system must allow creation of directed relations between two entities, specifying `subject_entity`, `relation_type`, and `target_entity`. | A POST request with valid entity references and relation type returns 201 with the created relation. | 4.1, 4.2 |
| FR-REL-002 | The system must validate that `relation_type` is declared in the subject entity's type definition and that the target entity's type is allowed for that relation. | Creating a relation with an undeclared or incompatible relation type returns a 400 error. | 4.1, 4.2 |
| FR-REL-003 | The system must prevent duplicate relations (same subject, relation type, target, and system scope). | Creating a duplicate relation returns a 409 Conflict error. | 4.1 |
| FR-REL-004 | The system must allow deletion of relations. | A DELETE request removes the relation and returns 204. | 4.2 |
| FR-REL-005 | The system must allow querying all relations of an entity, filterable by `relation_type` and/or `system`. | A GET request returns a paginated list of matching relations. | 4.2 |

### 1.4 System Management

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-SYS-001 | The system must allow administrators to register a new system (application/service). | A POST request creates the system record and returns 201. | 4.1 |
| FR-SYS-002 | The system must allow administrators to update system metadata (name, description). | A PUT/PATCH request updates the system and returns 200. | 4.1 |
| FR-SYS-003 | The system must allow administrators to deactivate a system without deleting its data. | A deactivation operation sets the system as inactive; retrieval APIs for that system's overlay data return 403 or omit the overlay. | 4.1 |

### 1.5 System Overlay Schema

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-OVS-001 | The system must allow administrators to create a system overlay schema defining which overlay properties a specific system can attach to entities of a given type. | A POST request with `system_id`, `entity_type_definition_id`, and `allowed_overlay_properties` (JSON Schema) returns 201 and persists the schema. | 4.1 |
| FR-OVS-002 | The system must allow administrators to update an existing system overlay schema. | A PUT/PATCH request modifies the `allowed_overlay_properties`; subsequent overlay writes are validated against the updated schema. | 4.1 |
| FR-OVS-003 | The system must allow administrators to delete a system overlay schema that has no associated property overlays. | Deletion succeeds when no overlays reference the schema; deletion is rejected with an error when overlays exist. | 4.1 |
| FR-OVS-004 | The system must validate that `allowed_overlay_properties` conforms to a valid JSON Schema document. | Creating or updating a schema with invalid JSON Schema returns a 400 error with a descriptive message. | 4.1 |
| FR-OVS-005 | The system must enforce that all property keys declared in `allowed_overlay_properties` are prefixed with the system's name followed by a dot separator (e.g., `credit.max_approval`). | Creating or updating a schema with non-namespaced property keys returns a 400 error listing the offending keys. | 4.1 |

### 1.6 System Overlays

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-OVL-001 | The system must allow product teams to attach additional properties (property overlays) to a global entity within their system scope. | A POST/PUT request associates system-specific properties with a global entity; properties are persisted under the system scope. | 4.1, 4.2 |
| FR-OVL-002 | The system must validate overlay properties against the corresponding system overlay schema at creation and update time. | Submitting overlay properties that violate the system overlay schema's JSON Schema returns a 400 error listing each validation failure. | 4.1, 4.2 |
| FR-OVL-003 | The system must reject overlay property writes when no system overlay schema exists for the given system + entity type combination. | A POST/PUT to create an overlay for a system + entity type pair with no declared schema returns a 400 error. | 4.1, 4.2 |
| FR-OVL-004 | The system must enforce that all overlay property keys are prefixed with the owning system's name (namespace). | A write request with non-namespaced overlay property keys returns a 400 error. | 4.1, 4.2 |
| FR-OVL-005 | The system must allow creation of system-scoped relations between entities. | A relation created with a `system_id` is only visible when queried within that system's context. | 4.1, 4.2 |
| FR-OVL-006 | When an entity is retrieved within a system context, the response must return a merged view: global properties combined with namespaced system overlay properties. | A GET request with a `system` parameter returns global properties plus overlay properties with system-namespaced keys; no key collisions occur between global and overlay properties. | 4.1 |
| FR-OVL-007 | When an entity is retrieved within a system context, the response must include both global relations and system-scoped relations. | The relations list in the response contains relations with no system scope plus relations scoped to the requested system. | 4.1 |
| FR-OVL-008 | Property overlay operations must be restricted to the product team's own system scope. | A request to modify overlays for a system the caller is not authorized for returns 403. | 4.2 |

### 1.7 Retrieval API

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-RET-001 | The system must support entity lookup by `type` + `external_id`, optionally scoped to a system. | A GET request with `type`, `external_id`, and optional `system` returns the appropriate entity view. | 4.2 |
| FR-RET-002 | The system must support filtered queries on entity properties (e.g., all users with `department=ops`). | A query endpoint accepts property filters and returns a paginated list of matching entities. | 4.2 |
| FR-RET-003 | The system must expose a changelog endpoint returning all entity, property, relation, and overlay changes since a given timestamp. | A GET request with a `since` parameter returns an ordered list of change events; each event includes entity reference, change type, and timestamp. | 4.2 |
| FR-RET-004 | The system must support bulk export of all entities and relations (paginated) for cold start and disaster recovery. | A paginated export endpoint returns all data in deterministic order; iterating all pages yields the complete dataset. | 4.2 |
| FR-RET-005 | All list and query endpoints must support pagination. | Responses include pagination metadata (cursor or offset, total count, next page indicator). | 4.2 |

### 1.8 Webhook / Event Notifications

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-WHK-001 | The system must allow consumers to subscribe to attribute change events for a specific system. | A subscription endpoint registers a callback URL and target system; returns 201 with the subscription details. | 4.2 |
| FR-WHK-002 | When an entity, property, relation, or overlay changes within a system scope, the system must send a notification to all active subscribers for that system. | A webhook POST is delivered to each subscriber's URL within a configurable time window after the change. | 4.2 |
| FR-WHK-003 | The system must support subscription management (list, update, delete). | Subscribers can list their active subscriptions, update the callback URL, or delete a subscription. | 4.2 |
| FR-WHK-004 | The system must retry failed webhook deliveries with exponential backoff. | A failed delivery (non-2xx response or timeout) is retried up to a configurable maximum number of attempts with increasing intervals. | 4.2 |

### 1.9 Management Interface

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-MGT-001 | The system must provide a web UI for administrators to view, create, edit, and delete entities, properties, relations, and system overlays. | All CRUD operations available via the API are also accessible through the UI. | 4.2 |
| FR-MGT-002 | The UI must enforce system-level segregation: product teams see global entities but can only modify their own system overlays and system-scoped relations. | A user scoped to System A cannot create, update, or delete overlays or system-scoped relations belonging to System B. | 4.2 |
| FR-MGT-003 | Platform administrators must be able to manage global entities and entity type definitions through the UI. | Users with the platform admin role can perform CRUD on entity type definitions and global entity data. | 4.2 |
| FR-MGT-004 | The UI must support role-based access with at least three roles: admin, editor, and viewer. | An admin can perform all operations; an editor can create/update but not delete type definitions or systems; a viewer has read-only access. | 4.2 |
| FR-MGT-005 | The UI must display the audit log with search and filter capabilities. | The audit log view supports filtering by entity, system, user, operation type, and time range. | 4.2 |

### 1.10 Audit Log

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| FR-AUD-001 | Every write operation (create, update, delete) on entities, properties, relations, and overlays must produce an immutable audit log entry. | After any mutation, an audit record exists containing: actor identity, timestamp, operation type, target resource, and before/after values. | 4.2 |
| FR-AUD-002 | Every retrieval event must be logged with the requesting PDP identity and the entities/relations accessed. | After a retrieval API call, an audit record exists containing: caller identity, timestamp, queried parameters, and returned entity references. | 4.2 |
| FR-AUD-003 | Audit log entries must be immutable — no update or delete operations on audit records. | The API exposes no endpoint to modify or delete audit entries; the database enforces append-only semantics (e.g., revoke DELETE/UPDATE on the audit table). | 4.2 |
| FR-AUD-004 | The audit log must be queryable via API. | An API endpoint supports filtering audit entries by entity, system, actor, operation type, and time range with paginated results. | 4.2 |

---

## 2. Non-Functional Requirements

### 2.1 Performance

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-PRF-001 | Single-entity retrieval API calls must complete within 100ms at the 99th percentile under normal load. | Load test with sustained traffic shows p99 latency <= 100ms for single-entity lookups. | 4.3 |
| NFR-PRF-002 | Bulk import endpoints must process at least 1,000 entities per request without timeout. | A bulk import of 1,000 entities completes successfully within the API timeout window (default 30s). | 4.2 |
| NFR-PRF-003 | Relation queries for a single entity must complete within 200ms at the 99th percentile. | Load test with sustained traffic shows p99 latency <= 200ms for relation queries on entities with up to 500 relations. | 4.3 |
| NFR-PRF-004 | The changelog endpoint must return results within 500ms at the 99th percentile for change windows of up to 1 hour. | Load test confirms p99 <= 500ms for changelog queries spanning a 1-hour window with up to 10,000 change events. | 4.2, 4.3 |

### 2.2 Availability & Reliability

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-AVL-001 | The application tier must be stateless, enabling horizontal scaling behind a load balancer. | No session state or local file dependencies exist; any instance can serve any request. Verified by routing requests to different instances and observing identical behavior. | 4.3 |
| NFR-AVL-002 | The system must target 99.9% uptime for the retrieval API (measured monthly). | Monitoring confirms retrieval API availability >= 99.9% over a calendar month, excluding planned maintenance windows. | 4.3 |
| NFR-AVL-003 | The system must handle database connection failures gracefully with retries and circuit-breaking. | When the database is temporarily unreachable, the system returns 503 with a retry-after header; upon recovery, the system resumes normal operation without manual intervention. | 4.3 |

### 2.3 Security

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-SEC-001 | All API endpoints must require authentication via JWT or mTLS. | Unauthenticated requests receive a 401 response; no endpoint returns data without a valid credential. | 4.3 |
| NFR-SEC-002 | System-scoped access control must be enforced on every operation. | A caller authenticated for System A cannot read or modify System B's overlays or system-scoped relations; verified by automated integration tests. | 4.3 |
| NFR-SEC-003 | All API inputs must be validated at the boundary. Malformed or unexpected data must be rejected with descriptive errors. | Fuzz testing and boundary-value testing confirm that invalid payloads return 400 errors and do not cause server errors or data corruption. | CLAUDE.md |
| NFR-SEC-004 | The system must enforce least-privilege attribute access: PDPs should only retrieve attribute sets relevant to their policy scope. | A PDP credential scoped to System A cannot retrieve overlay data for System B. | CLAUDE.md |
| NFR-SEC-005 | All communication between the PIP and external consumers must be encrypted in transit (TLS 1.2+). | Network capture confirms no plaintext traffic on API ports; TLS 1.1 and below are rejected. | CLAUDE.md |
| NFR-SEC-006 | The system must protect against OWASP Top 10 vulnerabilities, specifically injection attacks on property filters and query parameters. | SAST/DAST scans report no critical or high findings for injection vulnerabilities. Parameterized queries are used for all database operations. | CLAUDE.md |
| NFR-SEC-007 | Secrets (database credentials, JWT signing keys, webhook secrets) must never be stored in source code or application logs. | Code review and log audit confirm no secrets in the codebase or log output; secrets are loaded from environment variables or a secrets manager. | CLAUDE.md |

### 2.4 Auditability

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-AUD-001 | Zero write operations may occur without a corresponding audit log entry. | Integration tests verify that every successful mutation API call produces exactly one audit record. A mutation that fails to write its audit entry must roll back the data change. | 4.3 |
| NFR-AUD-002 | Audit log entries must include a timestamp with millisecond precision in UTC. | All audit records contain a `timestamp` field in ISO 8601 format with millisecond precision and UTC timezone designator. | 4.3 |
| NFR-AUD-003 | Audit records must be retained for a minimum of 1 year. | Data retention policy and storage capacity support at least 12 months of audit data without manual purging. | 4.3 |

### 2.5 Extensibility

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-EXT-001 | New entity types, properties, and relation types must be declarable at runtime without application restarts or database migrations. | An administrator creates a new entity type definition via the API; entities of that type can be created immediately without redeploying the application. | 4.3, 5 |
| NFR-EXT-002 | The property storage model must support arbitrary key-value attributes without predefined columns. | Entities can store any valid JSON object as properties, constrained only by the entity type definition's JSON Schema. | 4.1, 5 |

### 2.6 Operability

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-OPS-001 | The system must expose a health check endpoint that reports application and database connectivity status. | A GET to `/health` returns 200 with status details when healthy; returns 503 when the database is unreachable. | — |
| NFR-OPS-002 | The system must emit structured logs (JSON) with correlation IDs for request tracing. | Every log entry includes a `correlation_id` field matching the request header; logs are parseable as JSON. | — |
| NFR-OPS-003 | The system must expose metrics (request count, latency histograms, error rates) in a Prometheus-compatible format. | A `/metrics` endpoint returns metrics in Prometheus exposition format; dashboards can visualize request rate and latency percentiles. | — |
| NFR-OPS-004 | The system must support configuration via environment variables for all deployment-sensitive settings (database URL, JWT issuer, TLS certificates). | The application starts successfully with all configuration provided via environment variables; no configuration files with secrets are required. | — |

### 2.7 Compatibility

| ID | Requirement | Acceptance Criteria | Spec Ref |
|---|---|---|---|
| NFR-CMP-001 | The retrieval API response structure must be mappable to AuthZen subject/resource/action/context fields. | Given an AuthZen evaluation request, the PDP can resolve `subject.type`, `subject.id`, `subject.properties`, `resource.type`, `resource.id`, `resource.properties`, and `context` by calling OAD's retrieval API. | 4.1 |
| NFR-CMP-002 | The system must use PostgreSQL as the primary data store. | The application connects to and operates correctly against PostgreSQL 15+. | 5 |

---

## 3. Requirement Traceability Matrix

| Spec Section | Requirement IDs |
|---|---|
| 4.1 Core Entities | FR-ETD-001..004, FR-ENT-001..003, FR-REL-001..003, FR-OVS-001..005, FR-OVL-001..007 |
| 4.2 Functional Scope — Ingestion | FR-ENT-001..008, FR-REL-001..004, FR-OVS-001..005, FR-OVL-001..005, FR-OVL-008 |
| 4.2 Functional Scope — Retrieval | FR-RET-001..005, FR-OVL-006..007 |
| 4.2 Functional Scope — Webhooks | FR-WHK-001..004 |
| 4.2 Functional Scope — Management UI | FR-MGT-001..005 |
| 4.2 Functional Scope — Audit Log | FR-AUD-001..004 |
| 4.3 Non-Functional Intent | NFR-PRF-001..004, NFR-AVL-001..003, NFR-SEC-001..002, NFR-AUD-001..003, NFR-EXT-001..002 |
| 5 Architectural Decisions | NFR-EXT-001..002, NFR-CMP-002 |
| CLAUDE.md Security Principles | NFR-SEC-003..007 |

---

## Revision History

| Version | Date | Changes |
|---|---|---|
| 0.1 | 2026-04-10 | Initial draft derived from Product Specification (MVP) |
| 0.2 | 2026-04-10 | Add System Overlay Schema requirements (FR-OVS-001..005); add overlay namespace enforcement (FR-OVL-002..004); renumber overlay requirements (FR-OVL-005..008); update traceability matrix |
