-- =============================================================================
-- OAD — Initial Schema (v0.1)
-- Derived from docs/data-model.md
-- PostgreSQL 15+
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Core tables (in dependency order)
-- ---------------------------------------------------------------------------

-- Schema registry: controls what entity types exist and their structure.
-- Dynamic schema without DB migrations — new types are rows, not columns.
CREATE TABLE entity_type_definition (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    type_name        VARCHAR(100) NOT NULL UNIQUE,
    allowed_properties JSONB     NOT NULL,           -- JSON Schema document
    allowed_relations  JSONB     NOT NULL,           -- {"member": {"target_types": ["group","role"]}}
    scope            VARCHAR(20) NOT NULL CHECK (scope IN ('global', 'system_scoped')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Registered applications / services whose authorization data is managed here.
-- Defines the management boundary for product teams.
CREATE TABLE system (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(200) NOT NULL UNIQUE,
    description TEXT,
    active      BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Typed nodes in the authorization graph.
-- Represents subjects, resources, roles, permissions, groups, etc.
CREATE TABLE entity (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    type_id     UUID         NOT NULL REFERENCES entity_type_definition (id),
    external_id VARCHAR(500) NOT NULL,
    properties  JSONB        NOT NULL DEFAULT '{}',  -- validated against type schema
    system_id   UUID         REFERENCES system (id), -- NULL for global entities
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (type_id, external_id)
);

-- Per-system, per-entity-type schema governing allowed overlay properties.
-- Prevents attribute pollution and enforces namespace convention.
-- (See: docs/data-model.md §2.4, docs/spec.md §4.1 System Overlay Schema)
CREATE TABLE system_overlay_schema (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id                   UUID NOT NULL REFERENCES system (id),
    entity_type_id              UUID NOT NULL REFERENCES entity_type_definition (id),
    allowed_overlay_properties  JSONB NOT NULL,  -- JSON Schema; keys must be namespaced
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (system_id, entity_type_id)
);

-- Typed, directed edges between entities — building block for RBAC and ReBAC.
-- system_id IS NULL means the relation is global; otherwise it is system-scoped.
CREATE TABLE relation (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_entity_id UUID        NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    relation_type     VARCHAR(100) NOT NULL,
    target_entity_id  UUID        NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    system_id         UUID        REFERENCES system (id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- System-specific properties layered on top of global entities.
-- Keys are namespaced (e.g., "credit.max_approval") to prevent collisions.
-- Merged with entity.properties at retrieval time via JSONB || operator.
CREATE TABLE property_overlay (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id  UUID        NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    system_id  UUID        NOT NULL REFERENCES system (id),
    properties JSONB       NOT NULL DEFAULT '{}',  -- namespaced keys, schema-validated
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_id, system_id)
);

-- Webhook subscriptions: consumers register a callback URL per system.
CREATE TABLE webhook_subscription (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id    UUID         NOT NULL REFERENCES system (id),
    callback_url VARCHAR(2000) NOT NULL,
    secret       VARCHAR(500) NOT NULL,  -- HMAC-SHA256 signing key; never logged
    active       BOOLEAN      NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Immutable record of every write operation.
-- system_id is stored without a FK constraint so that audit records survive
-- system deactivation/deletion (see: docs/data-model.md §4.5).
CREATE TABLE audit_log (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor         VARCHAR(500) NOT NULL,  -- JWT subject or mTLS CN
    operation     VARCHAR(20) NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    resource_type VARCHAR(100) NOT NULL,
    resource_id   UUID        NOT NULL,
    before_value  JSONB,                  -- NULL on create
    after_value   JSONB,                  -- NULL on delete
    system_id     UUID,                   -- intentionally no FK (see data-model §4.5)
    timestamp     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Immutable record of every retrieval event for compliance.
-- Separate from audit_log because structure differs (query params vs before/after).
CREATE TABLE retrieval_log (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    caller_identity  VARCHAR(500) NOT NULL,
    query_parameters JSONB       NOT NULL,
    returned_refs    JSONB       NOT NULL,
    system_id        UUID,                -- intentionally no FK
    timestamp        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tracks individual delivery attempts for webhook notifications.
-- Enables retry-with-backoff independently of the API request path.
CREATE TABLE webhook_delivery (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id    UUID        NOT NULL REFERENCES webhook_subscription (id),
    audit_log_id       UUID        NOT NULL REFERENCES audit_log (id),
    status             VARCHAR(20) NOT NULL DEFAULT 'pending'
                                   CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts           INTEGER     NOT NULL DEFAULT 0,
    next_retry_at      TIMESTAMPTZ,
    last_response_code INTEGER,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 2. Partial unique indexes for relation
--    PostgreSQL treats NULL != NULL in standard UNIQUE constraints, which would
--    allow duplicate global relations. Two partial indexes solve this correctly.
--    (See: docs/data-model.md §4.6)
-- ---------------------------------------------------------------------------

CREATE UNIQUE INDEX uq_relation_global
    ON relation (subject_entity_id, relation_type, target_entity_id)
    WHERE system_id IS NULL;

CREATE UNIQUE INDEX uq_relation_scoped
    ON relation (subject_entity_id, relation_type, target_entity_id, system_id)
    WHERE system_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 3. Performance indexes
--    (See: docs/data-model.md §3)
-- ---------------------------------------------------------------------------

-- Entity: primary retrieval path and property filter queries
CREATE INDEX idx_entity_system_id  ON entity (system_id);
CREATE INDEX idx_entity_properties ON entity USING GIN (properties);

-- Relation: subject/target lookup for graph traversal by PDPs
CREATE INDEX idx_relation_subject ON relation (subject_entity_id, relation_type);
CREATE INDEX idx_relation_target  ON relation (target_entity_id,  relation_type);

-- Audit log: changelog endpoint and audit queries
CREATE INDEX idx_audit_log_timestamp        ON audit_log (timestamp);
CREATE INDEX idx_audit_log_resource         ON audit_log (resource_type, resource_id);
CREATE INDEX idx_audit_log_actor            ON audit_log (actor);
CREATE INDEX idx_audit_log_system_timestamp ON audit_log (system_id, timestamp);

-- Retrieval log: compliance queries by time and caller
CREATE INDEX idx_retrieval_log_timestamp ON retrieval_log (timestamp);
CREATE INDEX idx_retrieval_log_caller    ON retrieval_log (caller_identity, timestamp);

-- Webhook delivery: retry worker queue
CREATE INDEX idx_webhook_delivery_retry        ON webhook_delivery (status, next_retry_at);
CREATE INDEX idx_webhook_delivery_subscription ON webhook_delivery (subscription_id, created_at);

-- ---------------------------------------------------------------------------
-- 4. updated_at trigger
--    Keeps updated_at current on every row update without relying on the
--    application layer to set it correctly.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entity_type_definition_updated_at
    BEFORE UPDATE ON entity_type_definition
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER system_updated_at
    BEFORE UPDATE ON system
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER entity_updated_at
    BEFORE UPDATE ON entity
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER system_overlay_schema_updated_at
    BEFORE UPDATE ON system_overlay_schema
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER property_overlay_updated_at
    BEFORE UPDATE ON property_overlay
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER webhook_subscription_updated_at
    BEFORE UPDATE ON webhook_subscription
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 5. Audit immutability triggers
--    Enforces append-only semantics on audit_log and retrieval_log at the
--    database level. Application roles should also have UPDATE/DELETE revoked.
--    (See: docs/data-model.md §2.9, FR-AUD-003, NFR-AUD-001)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION
        'Audit records are immutable: UPDATE and DELETE are not permitted on %',
        TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();

CREATE TRIGGER retrieval_log_immutable
    BEFORE UPDATE OR DELETE ON retrieval_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();

-- ---------------------------------------------------------------------------
-- 6. Row-Level Security (RLS)
--    Defense-in-depth: the application layer is the primary authorization
--    boundary; RLS is a database-level enforcement layer.
--
--    Design:
--    • app.current_system_id is SET LOCAL per request (see internal/db/rls.go)
--    • Empty string (not set) = admin mode → no restriction
--    • Set to a UUID string = system-scoped mode → restrict to that system
--
--    FORCE ROW LEVEL SECURITY ensures policies apply even to the table owner,
--    making RLS testable in development with the default superuser.
--
--    (See: docs/data-model.md §4.7, NFR-SEC-002)
-- ---------------------------------------------------------------------------

ALTER TABLE entity              ENABLE ROW LEVEL SECURITY;
ALTER TABLE entity              FORCE  ROW LEVEL SECURITY;

ALTER TABLE relation            ENABLE ROW LEVEL SECURITY;
ALTER TABLE relation            FORCE  ROW LEVEL SECURITY;

ALTER TABLE property_overlay    ENABLE ROW LEVEL SECURITY;
ALTER TABLE property_overlay    FORCE  ROW LEVEL SECURITY;

ALTER TABLE webhook_subscription ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_subscription FORCE  ROW LEVEL SECURITY;

-- entity: system-scoped entities are only visible within their system context.
-- Global entities (system_id IS NULL) are always visible.
CREATE POLICY entity_system_isolation ON entity
    USING (
        system_id IS NULL
        OR current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );

-- relation: global relations visible always; system-scoped relations visible
-- only when no context is set (admin) or the system matches.
CREATE POLICY relation_system_isolation ON relation
    USING (
        system_id IS NULL
        OR current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );

-- property_overlay: only visible to the owning system.
-- When no context is set (admin mode), all overlays are visible.
CREATE POLICY property_overlay_system_isolation ON property_overlay
    USING (
        current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );

-- webhook_subscription: only visible to the subscribing system.
CREATE POLICY webhook_subscription_system_isolation ON webhook_subscription
    USING (
        current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );
