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

12. [x] Authentication middleware: JWT validation (lestrrat-go/jwx) + mTLS support
13. [x] Authorization middleware: role extraction and system-scope verification from JWT claims
14. [x] RLS session setup helper: `set_config('app.current_system_id', $1, true)` transactional scope
15. [x] Audit log service: transactional write audit within the same DB transaction as the business operation
16. [x] Structured logging: slog (JSON output) with context-aware handler (correlation ID, actor, system_id)
17. [x] Input validation framework: reusable JSON Schema validation engine (santhosh-tekuri/jsonschema)
18. [x] Prometheus metrics middleware: request count, latency histograms, error rates on `/metrics`

### Phase 2 — Schema Registry (Entity Types, Systems, Overlay Schemas)

19. [x] CRUD `entity_type_definition` — FR-ETD-001..004 (type_name, allowed_properties JSON Schema validation, allowed_relations, scope)
20. [x] CRUD `system` — FR-SYS-001..003 (register, update metadata, deactivate without data deletion)
21. [x] CRUD `system_overlay_schema` — FR-OVS-001..005 (JSON Schema validation, namespace prefix enforcement per system name)

### Phase 3 — Entity & Relation Management

22. [x] CRUD `entity` — FR-ENT-001..006, FR-ENT-008 (property validation against type definition, external_id uniqueness within type, system_id for system-scoped types)
23. [x] CRUD `relation` — FR-REL-001..005 (relation_type validation against subject type definition, target type validation, duplicate prevention, system-scoped relations)
24. [x] Bulk import endpoint — FR-ENT-007 (batch create/upsert, per-item validation, partial failure handling, summary response)

### Phase 4 — Overlay System

25. [x] CRUD `property_overlay` — FR-OVL-001..004, FR-OVL-008 (overlay schema lookup, namespace enforcement, JSON Schema validation, system-scope authorization)

### Phase 5 — Retrieval API

26. [x] Entity lookup with merged view — FR-RET-001, FR-OVL-006, FR-OVL-007 (global properties + namespaced overlay merge, global + system-scoped relations, AuthZen-compatible response)
27. [x] Relation query with filters and pagination — FR-REL-005, FR-RET-005
28. [x] Property filter query — FR-RET-002 (GIN-indexed JSONB containment queries)
29. [x] Changelog endpoint — FR-RET-003 (audit_log query by timestamp + system, paginated)
30. [x] Bulk export — FR-RET-004 (paginated, deterministic order)
31. [x] Retrieval logging — FR-AUD-002 (caller identity, query parameters, returned references)

### Phase 6 — Webhooks

32. [x] CRUD `webhook_subscription` — FR-WHK-001, FR-WHK-003 (callback URL, HMAC secret, activation)
33. [x] Webhook dispatcher background worker — FR-WHK-002, FR-WHK-004 (event delivery, HMAC-SHA256 signing, exponential backoff retries, delivery status tracking)

### Phase 7 — Management UI

#### Phase 7.1 — Foundation & App Shell

34. [x] Vite + React 19 + TypeScript scaffolding, ESLint/Prettier, path aliases
35. [x] Tailwind CSS setup with design tokens (colors, spacing, typography)
36. [x] Routing with React Router v7 (lazy-loaded route modules, `AnimatedContent`/`FadeContent` page transitions)
37. [x] HTTP client: `fetch` wrapper with correlation-ID propagation, error mapping to `apierr` shape
38. [x] TanStack Query setup (cache, retries, stale-while-revalidate defaults)
39. [x] Environment config loader (`VITE_API_BASE_URL`, `VITE_OIDC_*`) with runtime validation
40. [x] App shell layout: top bar with `GradientText` branding, `Dock` sidebar navigation, main content area, breadcrumbs

41. [x] Dashboard home view: `CountUp` metric cards (total entities, active systems, pending webhooks), `SpotlightCard` for system health overview

#### Phase 7.2 — Authentication (OIDC)

42. [x] `oidc-client-ts` provider with silent and interactive token renewal
43. [x] Login page with `SoftAurora` background and `DecryptedText` title; callback and logout routes
44. [x] Token storage strategy (in-memory + refresh) and bearer injection on every API call
45. [x] `AuthContext` exposing identity, `oad_roles`, `oad_system_id`
46. [x] Protected route wrapper and session-expiry handling (auto-redirect on 401)

#### Phase 7.3 — Authorization & Scope UX

47. [x] Role-based component gates (`<RequireRole>`, `<RequireAnyRole>`)
48. [x] System scope selector with `BorderGlow` active indicator for platform admins; fixed scope for product team users
49. [x] Hide/disable write actions for viewer role; hide delete for editor role
50. [x] Global banner indicating active system scope with `ShinyText` emphasis on system name
51. [x] 403 fallback page with actionable guidance

#### Phase 7.4 — Design System & Feedback Primitives

52. [x] Component library: Button, Input, Select, Textarea, Checkbox, Badge, Tag
53. [x] DataTable with column config, server-side pagination, sort, and row actions
54. [x] Modal, Drawer, and ConfirmDialog (for destructive actions)
55. [x] Form stack: `react-hook-form` + `zod` resolver with field-level error display
56. [x] Toast/notification system mapped to `apierr` codes; `ClickSpark` on successful create/save actions
57. [x] Empty state, loading skeletons (`BlurText` for text placeholders), and error boundary components
58. [x] JSON Schema editor/viewer component (syntax-highlighted, live validation)

#### Phase 7.5 — Platform Admin: Schema Registry

59. [x] Entity Type Definition list with filter by scope (global/system)
60. [x] Entity Type Definition create/edit form via `Stepper` — `allowed_properties` (JSON Schema) and `allowed_relations`
61. [x] Entity Type Definition detail view with usage count and delete guard
62. [x] System list with `SpotlightCard` overview, register form, edit metadata, deactivate with audit note
63. [x] System Overlay Schema list per system with namespace prefix preview and validation
64. [x] System Overlay Schema create/edit with live schema validation

#### Phase 7.6 — Entities & Relations (Core CRUD)

65. [x] Entity list: filter by type, external_id search, JSONB property filter builder, server pagination
66. [x] Entity detail with tabs: Properties, Relations, Overlays, Audit
67. [x] Entity create/edit — dynamic form generated from type definition `allowed_properties`
68. [x] Entity delete with relation-dependency warning
69. [x] Relation creation (subject picker, target picker, type restricted to `allowed_relations`)
70. [x] Relation list on entity detail with filters and removal
71. [x] Bulk import view via `Stepper` wizard: file upload (JSON), validation preview, per-item summary, partial-failure report

#### Phase 7.7 — Product Team: Overlays

72. [x] Overlay list for current system scope
73. [x] Overlay create: entity search, dynamic form from overlay schema, namespaced key display
74. [x] Overlay edit and delete within system scope
75. [x] Merged-view preview (global + overlay) on entity detail

#### Phase 7.8 — Observability Views

76. [x] Audit log viewer with `AnimatedList` entries: filters (entity, system, actor, operation, time range), paginated — FR-MGT-005
77. [x] Audit detail drawer with before/after diff renderer
78. [x] Retrieval log viewer (compliance persona, read-only) — FR-AUD-002
79. [x] Webhook subscription CRUD per system (callback URL, HMAC secret generation, activation toggle) — FR-WHK-003
80. [x] Webhook delivery history view (recent attempts, retry counter, last status)

#### Phase 7.9 — Polish & UI Testing

81. [x] i18n scaffolding (pt-BR / en) with lazy-loaded message bundles
82. [x] Responsive pass (tablet and mobile breakpoints for read-only views)
83. [x] Accessibility audit (axe-core, keyboard navigation, ARIA on tables/modals)
84. [x] Unit tests (Vitest + Testing Library) for hooks, forms, and guards
85. [x] Contract tests against mocked API (MSW handlers per endpoint)

### Phase 8 — Hardening & Operability

86. [ ] Load testing with k6: p99 retrieval < 100ms, p99 relations < 200ms, p99 changelog < 500ms
87. [ ] Security integration tests: cross-system isolation, RLS bypass attempts, unauthorized overlay access
88. [ ] E2E tests with Playwright: OIDC login, CRUD flows, role enforcement in Management UI
89. [ ] Production deployment documentation (Dockerfile, environment variables, TLS configuration)
