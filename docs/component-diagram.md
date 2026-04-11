# OAD — Component & Deployment Diagram (v0.1)

> Derived from the [Product Specification](spec.md), [Requirements](requirements.md), [Data Model](data-model.md), and [Sequence Diagrams](sequence-diagrams.md). All diagrams use Mermaid syntax for GitHub-native rendering.

---

## 1. System Context Diagram

Shows OAD as a whole and its relationships with external actors and systems. OAD is the PIP (Policy Information Point) — it stores and serves authorization attributes. It does **not** evaluate policies (PDP), enforce decisions (PEP), or manage policies (PAP).

```mermaid
C4Context
    title OAD — System Context

    Person(platform_eng, "Platform / Security Engineer", "Deploys and operates the PIP; manages entity type definitions, systems, and overlay schemas")
    Person(product_team, "Product / Application Team", "Manages overlays and system-scoped relations for their specific system")
    Person(compliance, "Compliance / Audit Team", "Reviews attribute change history and access logs")

    System(oad, "OAD — Open Authoritative Directory", "Centralized Policy Information Point (PIP). Stores entities, properties, relations, and system overlays for authorization.")

    System_Ext(idp, "Identity Provider", "OIDC / SAML. Authenticates users for the Management UI. Provides JWT with role and system claims.")
    System_Ext(auth_source, "Authoritative Sources", "HR system, CMDB, IdP sync. Push attribute data into OAD via ingestion APIs.")
    System_Ext(pdp, "PDP Ecosystem", "OPA, Cedar, Topaz, Cerbos, or custom PDP. Pulls attributes from OAD at policy evaluation time.")
    System_Ext(control_plane, "PDP Control Plane", "OCP, OPAL, Topaz Director, or custom. Syncs attribute changes to PDP edge caches via changelog and webhooks.")

    Rel(platform_eng, oad, "Manages types, systems, schemas", "Admin API / UI")
    Rel(product_team, oad, "Manages overlays, relations", "Management UI (scoped)")
    Rel(compliance, oad, "Reviews audit log", "Audit API / UI (read-only)")
    Rel(auth_source, oad, "Pushes attribute data", "Ingestion API")
    Rel(pdp, oad, "Pulls entities + relations", "Retrieval API")
    Rel(control_plane, oad, "Syncs changes", "Changelog API / Webhooks")
    Rel(oad, idp, "Authenticates UI users", "OIDC")
    Rel(oad, control_plane, "Notifies changes", "Webhook push")
```

---

## 2. Container Diagram

Zooms into OAD, showing the major containers (deployable units) and their responsibilities.

```mermaid
C4Container
    title OAD — Container Diagram

    Person(platform_eng, "Platform Engineer", "")
    Person(product_team, "Product Team", "")
    Person(compliance, "Compliance / Audit", "")

    System_Boundary(oad, "OAD — Open Authoritative Directory") {
        Container(ui, "Management UI", "SPA (React / Next.js)", "Web interface for entity, relation, overlay, and audit management. Role-based access: admin, editor, viewer.")
        Container(api, "OAD API", "REST API (Go / Node.js / Java)", "Stateless application tier. Handles ingestion, retrieval, admin, and audit endpoints. Enforces authentication, authorization, input validation, and audit logging.")
        Container(webhook_worker, "Webhook Dispatcher", "Background Worker", "Processes webhook delivery queue. Delivers event payloads with HMAC-SHA256 signatures. Implements exponential backoff retries.")
        ContainerDb(db, "PostgreSQL 15+", "Relational Database", "Stores entities, relations, overlays, type definitions, overlay schemas, systems, audit logs, retrieval logs, and webhook state. Enforces RLS for system isolation.")
    }

    System_Ext(idp, "Identity Provider", "OIDC / SAML")
    System_Ext(pdp, "PDP Ecosystem", "OPA, Cedar, Topaz, etc.")
    System_Ext(control_plane, "PDP Control Plane", "OCP, OPAL, etc.")
    System_Ext(auth_source, "Authoritative Sources", "HR, CMDB, IdP sync")

    Rel(platform_eng, ui, "Manages types, systems, schemas", "HTTPS")
    Rel(product_team, ui, "Manages overlays, relations", "HTTPS")
    Rel(compliance, ui, "Reviews audit log", "HTTPS")

    Rel(ui, api, "API calls", "HTTPS + JWT")
    Rel(auth_source, api, "Pushes attribute data", "HTTPS + mTLS")
    Rel(pdp, api, "Retrieves entities + relations", "HTTPS + JWT / mTLS")
    Rel(control_plane, api, "Changelog queries", "HTTPS + JWT / mTLS")

    Rel(api, db, "Reads / writes", "TCP + TLS")
    Rel(webhook_worker, db, "Reads delivery queue", "TCP + TLS")

    Rel(ui, idp, "OIDC authentication", "HTTPS")
    Rel(webhook_worker, control_plane, "Delivers event payloads", "HTTPS + HMAC-SHA256")
```

---

## 3. Component Diagram

Breaks down the OAD API container into internal components, showing responsibilities and interactions.

```mermaid
graph TB
    subgraph "External Actors"
        UI["Management UI"]
        PDP["PDP / Control Plane"]
        AS["Authoritative Sources"]
    end

    subgraph "API Gateway / Load Balancer"
        GW["TLS Termination<br/>Rate Limiting<br/>Request Routing"]
    end

    subgraph "OAD API Application"
        subgraph "Middleware Layer"
            AUTH["Authentication<br/>Middleware"]
            AUTHZ["Authorization<br/>Middleware"]
            VALID["Input Validation<br/>Middleware"]
            CORR["Correlation ID<br/>Middleware"]
            METRICS["Metrics<br/>Middleware"]
        end

        subgraph "API Layer"
            ENTITY_API["Entity API<br/>/entities<br/>/entities/bulk"]
            RELATION_API["Relation API<br/>/relations"]
            OVERLAY_API["Overlay API<br/>/systems/{id}/entities/{id}/overlay"]
            SCHEMA_API["Overlay Schema API<br/>/systems/{id}/overlay-schemas"]
            TYPE_API["Entity Type API<br/>/entity-types"]
            SYSTEM_API["System API<br/>/systems"]
            RETRIEVAL_API["Retrieval API<br/>/entities?type&external_id<br/>/changelog<br/>/export"]
            WEBHOOK_API["Webhook API<br/>/systems/{id}/webhooks"]
            AUDIT_API["Audit API<br/>/audit-log<br/>/retrieval-log"]
            OPS_API["Operational API<br/>/health<br/>/metrics"]
        end

        subgraph "Domain Services"
            ENTITY_SVC["Entity Service<br/>• CRUD operations<br/>• Property validation<br/>• Bulk import"]
            RELATION_SVC["Relation Service<br/>• CRUD operations<br/>• Type/target validation"]
            OVERLAY_SVC["Overlay Service<br/>• Property overlay CRUD<br/>• Schema validation<br/>• Namespace enforcement"]
            SCHEMA_SVC["Schema Service<br/>• Type definition CRUD<br/>• Overlay schema CRUD<br/>• JSON Schema validation"]
            SYSTEM_SVC["System Service<br/>• Registration<br/>• Activation/Deactivation"]
            MERGE_SVC["Merge Service<br/>• Global + overlay merge<br/>• Global + scoped relations<br/>• AuthZen response mapping"]
            AUDIT_SVC["Audit Service<br/>• Write audit logging<br/>• Retrieval logging<br/>• Query and filter"]
            WEBHOOK_SVC["Webhook Service<br/>• Subscription CRUD<br/>• Event enqueue"]
        end

        subgraph "Data Access Layer"
            REPO["Repository Layer<br/>• Parameterized queries<br/>• Transaction management<br/>• RLS session setup"]
        end
    end

    subgraph "Background Workers"
        WEBHOOK_DISPATCH["Webhook Dispatcher<br/>• HMAC-SHA256 signing<br/>• Exponential backoff<br/>• Delivery tracking"]
    end

    subgraph "PostgreSQL 15+"
        DB[("Database<br/>• RLS policies<br/>• Audit immutability triggers<br/>• GIN indexes on JSONB<br/>• Partial unique indexes")]
    end

    UI --> GW
    PDP --> GW
    AS --> GW
    GW --> AUTH

    AUTH --> AUTHZ
    AUTHZ --> VALID
    VALID --> CORR
    CORR --> METRICS

    METRICS --> ENTITY_API
    METRICS --> RELATION_API
    METRICS --> OVERLAY_API
    METRICS --> SCHEMA_API
    METRICS --> TYPE_API
    METRICS --> SYSTEM_API
    METRICS --> RETRIEVAL_API
    METRICS --> WEBHOOK_API
    METRICS --> AUDIT_API
    METRICS --> OPS_API

    ENTITY_API --> ENTITY_SVC
    RELATION_API --> RELATION_SVC
    OVERLAY_API --> OVERLAY_SVC
    SCHEMA_API --> SCHEMA_SVC
    TYPE_API --> SCHEMA_SVC
    SYSTEM_API --> SYSTEM_SVC
    RETRIEVAL_API --> MERGE_SVC
    WEBHOOK_API --> WEBHOOK_SVC
    AUDIT_API --> AUDIT_SVC

    ENTITY_SVC --> AUDIT_SVC
    ENTITY_SVC --> WEBHOOK_SVC
    RELATION_SVC --> AUDIT_SVC
    RELATION_SVC --> WEBHOOK_SVC
    OVERLAY_SVC --> AUDIT_SVC
    OVERLAY_SVC --> WEBHOOK_SVC
    SCHEMA_SVC --> AUDIT_SVC
    SYSTEM_SVC --> AUDIT_SVC
    MERGE_SVC --> AUDIT_SVC

    ENTITY_SVC --> REPO
    RELATION_SVC --> REPO
    OVERLAY_SVC --> REPO
    SCHEMA_SVC --> REPO
    SYSTEM_SVC --> REPO
    MERGE_SVC --> REPO
    AUDIT_SVC --> REPO
    WEBHOOK_SVC --> REPO

    REPO --> DB
    WEBHOOK_DISPATCH --> DB
```

---

## 4. Deployment Diagram

Shows the physical deployment topology for a production environment.

```mermaid
graph TB
    subgraph "Client Tier"
        BROWSER["Browser<br/>(Management UI)"]
        PDP_CLIENT["PDP / Control Plane<br/>(API Client)"]
        SOURCE_CLIENT["Authoritative Source<br/>(API Client)"]
    end

    subgraph "Edge / Ingress"
        LB["Load Balancer<br/>• TLS termination<br/>• Rate limiting<br/>• Health checks"]
    end

    subgraph "Identity"
        IDP["Identity Provider<br/>• OIDC / SAML<br/>• JWT issuance<br/>• MFA"]
    end

    subgraph "Application Tier (Stateless)"
        subgraph "Instance 1"
            API1["OAD API<br/>Instance"]
        end
        subgraph "Instance 2"
            API2["OAD API<br/>Instance"]
        end
        subgraph "Instance N"
            APIN["OAD API<br/>Instance"]
        end
        subgraph "Workers"
            WH1["Webhook<br/>Dispatcher"]
        end
    end

    subgraph "Static Assets"
        CDN["CDN / Static Host<br/>• Management UI SPA<br/>• JS / CSS / Assets"]
    end

    subgraph "Data Tier"
        subgraph "PostgreSQL Cluster"
            PG_PRIMARY[("Primary<br/>• Read / Write<br/>• RLS enforced<br/>• Audit triggers")]
            PG_REPLICA[("Replica<br/>• Read-only<br/>• Retrieval queries<br/>• Audit queries")]
        end
    end

    subgraph "Observability"
        PROM["Prometheus<br/>• Metrics scraping"]
        LOG_AGG["Log Aggregator<br/>• Structured JSON logs<br/>• Correlation ID tracing"]
    end

    BROWSER -->|"HTTPS"| CDN
    BROWSER -->|"HTTPS"| LB
    BROWSER -->|"OIDC"| IDP
    PDP_CLIENT -->|"HTTPS + JWT/mTLS"| LB
    SOURCE_CLIENT -->|"HTTPS + mTLS"| LB

    LB --> API1
    LB --> API2
    LB --> APIN

    API1 -->|"TCP + TLS"| PG_PRIMARY
    API2 -->|"TCP + TLS"| PG_PRIMARY
    APIN -->|"TCP + TLS"| PG_PRIMARY

    API1 -.->|"Read replicas<br/>(retrieval queries)"| PG_REPLICA
    API2 -.->|"Read replicas<br/>(retrieval queries)"| PG_REPLICA
    APIN -.->|"Read replicas<br/>(retrieval queries)"| PG_REPLICA

    PG_PRIMARY -->|"Streaming<br/>Replication"| PG_REPLICA

    WH1 -->|"TCP + TLS"| PG_PRIMARY
    WH1 -->|"HTTPS + HMAC"| PDP_CLIENT

    PROM -.->|"Scrape /metrics"| API1
    PROM -.->|"Scrape /metrics"| API2
    PROM -.->|"Scrape /metrics"| APIN

    API1 -.->|"Structured logs"| LOG_AGG
    API2 -.->|"Structured logs"| LOG_AGG
    APIN -.->|"Structured logs"| LOG_AGG
    WH1 -.->|"Structured logs"| LOG_AGG
```

---

## 5. API Surface Map

Summary of all API groups, their consumers, and authentication requirements.

| API Group | Base Path | Primary Consumer | Auth Method | Description |
|---|---|---|---|---|
| **Entity Type API** | `/entity-types` | Platform Engineer (UI/API) | JWT (admin role) | CRUD for entity type definitions |
| **System API** | `/systems` | Platform Engineer (UI/API) | JWT (admin role) | Register, update, deactivate systems |
| **Overlay Schema API** | `/systems/{id}/overlay-schemas` | Platform Engineer (UI/API) | JWT (admin role) | Declare overlay schemas per system + type |
| **Entity API** | `/entities`, `/entities/bulk` | Platform Engineer, Product Team, Authoritative Sources | JWT / mTLS | CRUD and bulk import for entities |
| **Relation API** | `/relations` | Product Team, Platform Engineer | JWT / mTLS | CRUD for relations (global and system-scoped) |
| **Overlay API** | `/systems/{id}/entities/{id}/overlay` | Product Team | JWT (system-scoped) | Manage property overlays |
| **Retrieval API** | `/entities?type&external_id&system` | PDP / Control Plane | JWT / mTLS | Entity lookup with merged view |
| **Relation Query API** | `/entities/{id}/relations` | PDP / Control Plane | JWT / mTLS | Query entity relations |
| **Changelog API** | `/changelog` | PDP Control Plane | JWT / mTLS | Incremental sync since timestamp |
| **Export API** | `/export` | PDP Control Plane | JWT / mTLS | Paginated bulk export |
| **Webhook API** | `/systems/{id}/webhooks` | PDP Control Plane | JWT / mTLS | Subscription management |
| **Audit API** | `/audit-log`, `/retrieval-log` | Platform Engineer, Compliance | JWT | Query immutable audit trail |
| **Operational API** | `/health`, `/metrics` | Load Balancer, Prometheus | None / Internal | Health checks, Prometheus metrics |

---

## 6. Security Boundary Map

Visualizes the trust boundaries and security controls at each layer.

```mermaid
graph TB
    subgraph "Untrusted Zone"
        EXT["External Clients<br/>(Browsers, PDPs,<br/>Authoritative Sources)"]
    end

    subgraph "DMZ — Trust Boundary 1"
        direction TB
        TLS["TLS 1.2+ Termination"]
        RL["Rate Limiting"]
        JWT_VAL["JWT / mTLS Validation"]
    end

    subgraph "Application Zone — Trust Boundary 2"
        direction TB
        AUTHZ_CHK["Authorization Check<br/>• Role verification<br/>• System scope verification"]
        INPUT_VAL["Input Validation<br/>• JSON Schema validation<br/>• Property namespace enforcement<br/>• Parameterized queries"]
        BIZ["Business Logic<br/>• Domain services<br/>• Merge operations"]
    end

    subgraph "Data Zone — Trust Boundary 3"
        direction TB
        RLS["Row-Level Security<br/>• app.current_system_id<br/>• System isolation"]
        AUDIT_ENF["Audit Enforcement<br/>• Append-only triggers<br/>• Transaction-coupled logging"]
        DATA[("PostgreSQL<br/>• Encrypted at rest<br/>• Encrypted in transit<br/>• REVOKE UPDATE/DELETE<br/>  on audit tables")]
    end

    EXT -->|"HTTPS only"| TLS
    TLS --> RL
    RL --> JWT_VAL
    JWT_VAL -->|"Authenticated identity<br/>+ claims"| AUTHZ_CHK
    AUTHZ_CHK --> INPUT_VAL
    INPUT_VAL --> BIZ
    BIZ -->|"SET LOCAL<br/>app.current_system_id"| RLS
    RLS --> AUDIT_ENF
    AUDIT_ENF --> DATA
```

### Trust Boundary Controls

| Boundary | Controls | Threats Mitigated |
|---|---|---|
| **TB-1: DMZ** | TLS termination, rate limiting, JWT signature / mTLS certificate validation | Eavesdropping, replay attacks, credential stuffing, DDoS |
| **TB-2: Application** | Role-based authorization, system-scope verification, JSON Schema validation, namespace enforcement, parameterized queries | Privilege escalation, unauthorized cross-system access, attribute pollution, injection attacks (SQLi, XSS) |
| **TB-3: Data** | Row-Level Security, append-only audit triggers, REVOKE on audit tables, encryption at rest | Data leakage across systems, audit tampering, unauthorized data modification |

---

## 7. Design Decisions

### 7.1 Stateless application tier

All API instances are stateless — no session state, no local file dependencies. Any instance can serve any request. This enables horizontal scaling behind a load balancer and simplifies zero-downtime deployments (NFR-AVL-001).

### 7.2 Read replicas for retrieval queries

PDP retrieval queries (entity lookup, relation query, changelog) are read-heavy and latency-sensitive (NFR-PRF-001: p99 < 100ms). Routing these queries to read replicas distributes the load away from the primary, which handles writes and audit logging. The application detects query type and routes accordingly.

### 7.3 Webhook dispatcher as a separate worker

The webhook dispatcher runs as a separate process (not inline with API requests) to avoid:
- **Increased write latency** — webhook delivery should not block the API response.
- **Retry complexity in the request path** — exponential backoff requires durable state and asynchronous scheduling.
- **Failure coupling** — a subscriber endpoint timeout should not affect API availability.

The dispatcher reads from the `webhook_delivery` table and processes pending/failed deliveries independently.

### 7.4 CDN for Management UI

The Management UI is a single-page application served from a CDN or static host. All data operations go through the OAD API with JWT authentication. This keeps the UI deployment independent of the API tier and eliminates server-side rendering concerns.

### 7.5 No PDP-specific sync layer

OAD exposes generic APIs (changelog, bulk export, webhooks). Each PDP ecosystem brings its own sync mechanism:
- **OPA** → OCP fetches bundles via changelog API
- **Topaz** → Director syncs via changelog API
- **OPAL** → Data fetchers call retrieval API
- **Cedar** → Custom adapter calls retrieval API
- **Cerbos** → Webhook-triggered refresh

This keeps OAD PDP-agnostic (spec §5, architectural decision).

---

## Revision History

| Version | Date | Changes |
|---|---|---|
| 0.1 | 2026-04-10 | Initial draft — system context, container, component, deployment, and security boundary diagrams; API surface map; design decisions |
