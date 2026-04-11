# OAD — Project Backlog

## Documentation

1. [x] Product Specification (`docs/spec.md`) — problem, audience, MVP scope, entities, architectural decisions
2. [x] Functional & Non-Functional Requirements (`docs/requirements.md`) — derived from spec, verifiable
3. [x] Data Model Diagram (`docs/data-model.md`) — ER diagram, table definitions, indexes, design decisions
4. [x] Sequence Diagrams & Use Cases (`docs/sequence-diagrams.md`) — ingestion flow, retrieval flow, admin flow, cross-cutting flows, use case map
5. [x] Component/Deployment Diagram (`docs/component-diagram.md`) — system context, container, component, deployment, security boundaries, API surface map

## Implementation

### Phase 0 — Scaffolding & Infrastructure

6. [x] Initialize Go module with project directory structure (cmd/, internal/, migrations/, docker/)
7. [x] Configure linting (golangci-lint), formatting (gofumpt), and .editorconfig
8. [x] Create Dockerfile (multi-stage, distroless) and docker-compose.yml (API + PostgreSQL 15+)
9. [x] Set up GitHub Actions CI pipeline (lint, test, build, gosec SAST, trivy container scan)
10. [x] Create initial database migration with golang-migrate: all tables from data-model.md, indexes, partial unique indexes, audit_log immutability triggers, RLS policies
11. [x] Implement `/health` endpoint (app + database connectivity check) with Chi router bootstrap

### Phase 1 — Cross-Cutting Middleware

12. [ ] Authentication middleware: JWT validation (lestrrat-go/jwx) + mTLS support
13. [ ] Authorization middleware: role extraction and system-scope verification from JWT claims
14. [ ] RLS session setup middleware: `SET LOCAL app.current_system_id` per request
15. [ ] Audit log service: transactional write audit within the same DB transaction as the business operation
16. [ ] Structured logging: slog (JSON output) with correlation ID middleware
17. [ ] Input validation framework: reusable JSON Schema validation engine (santhosh-tekuri/jsonschema)
18. [ ] Prometheus metrics middleware: request count, latency histograms, error rates on `/metrics`

### Phase 2 — Schema Registry (Entity Types, Systems, Overlay Schemas)

19. [ ] CRUD `entity_type_definition` — FR-ETD-001..004 (type_name, allowed_properties JSON Schema validation, allowed_relations, scope)
20. [ ] CRUD `system` — FR-SYS-001..003 (register, update metadata, deactivate without data deletion)
21. [ ] CRUD `system_overlay_schema` — FR-OVS-001..005 (JSON Schema validation, namespace prefix enforcement per system name)

### Phase 3 — Entity & Relation Management

22. [ ] CRUD `entity` — FR-ENT-001..006, FR-ENT-008 (property validation against type definition, external_id uniqueness within type, system_id for system-scoped types)
23. [ ] CRUD `relation` — FR-REL-001..005 (relation_type validation against subject type definition, target type validation, duplicate prevention, system-scoped relations)
24. [ ] Bulk import endpoint — FR-ENT-007 (batch create/upsert, per-item validation, partial failure handling, summary response)

### Phase 4 — Overlay System

25. [ ] CRUD `property_overlay` — FR-OVL-001..004, FR-OVL-008 (overlay schema lookup, namespace enforcement, JSON Schema validation, system-scope authorization)

### Phase 5 — Retrieval API

26. [ ] Entity lookup with merged view — FR-RET-001, FR-OVL-006, FR-OVL-007 (global properties + namespaced overlay merge, global + system-scoped relations, AuthZen-compatible response)
27. [ ] Relation query with filters and pagination — FR-REL-005, FR-RET-005
28. [ ] Property filter query — FR-RET-002 (GIN-indexed JSONB containment queries)
29. [ ] Changelog endpoint — FR-RET-003 (audit_log query by timestamp + system, paginated)
30. [ ] Bulk export — FR-RET-004 (paginated, deterministic order)
31. [ ] Retrieval logging — FR-AUD-002 (caller identity, query parameters, returned references)

### Phase 6 — Webhooks

32. [ ] CRUD `webhook_subscription` — FR-WHK-001, FR-WHK-003 (callback URL, HMAC secret, activation)
33. [ ] Webhook dispatcher background worker — FR-WHK-002, FR-WHK-004 (event delivery, HMAC-SHA256 signing, exponential backoff retries, delivery status tracking)

### Phase 7 — Management UI

34. [ ] React + Vite + Tailwind scaffolding with OIDC authentication flow (oidc-client-ts)
35. [ ] Entity type definition management views (CRUD)
36. [ ] System management views (register, update, deactivate)
37. [ ] Entity and relation management views (CRUD, search, pagination)
38. [ ] Property overlay management views (system-scoped, namespace display)
39. [ ] Audit log viewer (search, filter by entity/system/actor/operation/time range)
40. [ ] Role-based access enforcement (admin, editor, viewer) and system-scope segregation

### Phase 8 — Hardening & Operability

41. [ ] Load testing with k6: p99 retrieval < 100ms, p99 relations < 200ms, p99 changelog < 500ms
42. [ ] Security integration tests: cross-system isolation, RLS bypass attempts, unauthorized overlay access
43. [ ] E2E tests with Playwright: OIDC login, CRUD flows, role enforcement in Management UI
44. [ ] Production deployment documentation (Dockerfile, environment variables, TLS configuration)
