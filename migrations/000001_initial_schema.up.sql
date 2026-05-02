-- =============================================================================
-- OAD — Initial Schema (v0.2)
-- Derived from docs/data-model.md and docs/design/scim-ingest.md (Phase A).
-- PostgreSQL 15+
--
-- Phase A landmark changes (vs v0.1):
--   • is_builtin flag on entity_type_definition and entity
--   • entity_external_identity table for SCIM-driven identity linking
--   • System promoted from a dedicated table to an entity of type 'System';
--     the standalone `system` table is removed
--   • entity_external_identity, system-id type-check trigger
--   • Seeded built-in types: User, Group, System, Permission
--   • Seeded reserved Groups: oad:admin, oad:editor, oad:viewer
--   • RLS preserves the existing "global rows always visible" semantic
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Schema registry
-- ---------------------------------------------------------------------------

-- Schema registry: controls what entity types exist and their structure.
-- Dynamic schema without DB migrations — new types are rows, not columns.
CREATE TABLE entity_type_definition (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    type_name          VARCHAR(100) NOT NULL UNIQUE,
    allowed_properties JSONB        NOT NULL,           -- JSON Schema document
    allowed_relations  JSONB        NOT NULL,           -- {"member_of":{"target_types":["Group"]}}
    scope              VARCHAR(20)  NOT NULL CHECK (scope IN ('global', 'system_scoped')),
    is_builtin         BOOLEAN      NOT NULL DEFAULT false, -- protected from API delete/modify
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 2. Built-in entity types (seeded BEFORE entity table is created so the
--    self-FK on entity.system_id has at least the System type to validate
--    against once any seeded entities are inserted)
-- ---------------------------------------------------------------------------

INSERT INTO entity_type_definition (type_name, allowed_properties, allowed_relations, scope, is_builtin) VALUES
(
    'User',
    $${
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "type": "object",
        "properties": {
            "userName":    {"type": "string", "minLength": 1, "maxLength": 200},
            "displayName": {"type": "string", "maxLength": 200},
            "email":       {"type": "string", "format": "email", "maxLength": 320},
            "active":      {"type": "boolean"}
        },
        "required": ["userName", "active"],
        "additionalProperties": false
    }$$,
    $${"member_of":{"target_types":["Group"]},"has_role_in":{"target_types":["System"]}}$$,
    'global',
    true
),
(
    'Group',
    $${
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "type": "object",
        "properties": {
            "displayName": {"type": "string", "minLength": 1, "maxLength": 200},
            "description": {"type": "string", "maxLength": 2000}
        },
        "required": ["displayName"],
        "additionalProperties": false
    }$$,
    $${"has_role_in":{"target_types":["System"]},"has_permission":{"target_types":["Permission"]}}$$,
    'global',
    true
),
(
    'System',
    $${
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "type": "object",
        "properties": {
            "name":        {"type": "string", "minLength": 1, "maxLength": 200},
            "description": {"type": "string", "maxLength": 2000},
            "active":      {"type": "boolean"}
        },
        "required": ["name", "active"],
        "additionalProperties": false
    }$$,
    $${}$$,
    'global',
    true
),
(
    'Permission',
    $${
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "type": "object",
        "properties": {
            "name":        {"type": "string", "minLength": 1, "maxLength": 500},
            "description": {"type": "string", "maxLength": 2000}
        },
        "required": ["name"],
        "additionalProperties": false
    }$$,
    $${}$$,
    'global',
    true
);

-- ---------------------------------------------------------------------------
-- 3. Entity graph (after seed so the type lookups in seeded rows resolve)
-- ---------------------------------------------------------------------------

-- Typed nodes in the authorization graph.
-- Represents subjects, resources, roles, permissions, groups, systems, etc.
--
-- system_id is a self-reference: when set, it must point to an entity of
-- type 'System'. Enforced by a trigger (see §6) rather than a CHECK constraint
-- because PostgreSQL CHECK cannot query other rows.
CREATE TABLE entity (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    type_id     UUID         NOT NULL REFERENCES entity_type_definition (id),
    external_id VARCHAR(500) NOT NULL,
    properties  JSONB        NOT NULL DEFAULT '{}',
    system_id   UUID         REFERENCES entity (id), -- NULL for global; otherwise a System entity
    is_builtin  BOOLEAN      NOT NULL DEFAULT false, -- protected from API delete/modify
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (type_id, external_id)
);

-- ---------------------------------------------------------------------------
-- 4. Reserved built-in groups (seeded before the type-check triggers attach,
--    so the inserts trivially pass — they have system_id = NULL anyway)
-- ---------------------------------------------------------------------------

INSERT INTO entity (type_id, external_id, properties, system_id, is_builtin) VALUES
(
    (SELECT id FROM entity_type_definition WHERE type_name = 'Group'),
    'oad:admin',
    '{"displayName":"OAD Platform Admin","description":"Full administrative access to the OAD management plane."}'::jsonb,
    NULL,
    true
),
(
    (SELECT id FROM entity_type_definition WHERE type_name = 'Group'),
    'oad:editor',
    '{"displayName":"OAD Editor","description":"Read/write access to entities, relations, and overlays. Cannot manage type definitions or system registration."}'::jsonb,
    NULL,
    true
),
(
    (SELECT id FROM entity_type_definition WHERE type_name = 'Group'),
    'oad:viewer',
    '{"displayName":"OAD Viewer","description":"Read-only access to entities, relations, overlays, and audit logs."}'::jsonb,
    NULL,
    true
);

-- ---------------------------------------------------------------------------
-- 5. External identity mapping (SCIM ingestion target)
--    Links one or more (provider, external_subject) pairs to a single entity.
--    Many-to-one supports cross-IdP merges in Phase D.
-- ---------------------------------------------------------------------------

CREATE TABLE entity_external_identity (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id         UUID         NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    provider_name     VARCHAR(100) NOT NULL,
    external_subject  VARCHAR(500) NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (provider_name, external_subject)
);

CREATE INDEX idx_entity_external_identity_entity ON entity_external_identity (entity_id);

-- ---------------------------------------------------------------------------
-- 6. system_id type-check trigger
--    Enforces that any system_id value references an entity of type 'System'.
--    Applied to all tables with a system_id FK to entity.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION assert_system_id_targets_system_entity()
RETURNS TRIGGER AS $$
DECLARE
    sys_type_id    UUID;
    target_type_id UUID;
BEGIN
    IF NEW.system_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT id INTO sys_type_id
      FROM entity_type_definition WHERE type_name = 'System';
    IF sys_type_id IS NULL THEN
        RAISE EXCEPTION 'built-in System type is not seeded (migration ordering error)';
    END IF;

    SELECT type_id INTO target_type_id
      FROM entity WHERE id = NEW.system_id;
    IF target_type_id IS NULL THEN
        RAISE EXCEPTION 'system_id % does not reference any existing entity', NEW.system_id;
    END IF;

    IF target_type_id <> sys_type_id THEN
        RAISE EXCEPTION 'system_id must reference an entity of type System (got entity with type_id=%)', target_type_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entity_system_id_type_check
    BEFORE INSERT OR UPDATE OF system_id ON entity
    FOR EACH ROW EXECUTE FUNCTION assert_system_id_targets_system_entity();

-- ---------------------------------------------------------------------------
-- 7. Remaining schema graph (relations, overlays, webhooks, audit)
-- ---------------------------------------------------------------------------

-- Per-system, per-entity-type schema governing allowed overlay properties.
CREATE TABLE system_overlay_schema (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id                   UUID NOT NULL REFERENCES entity (id),
    entity_type_id              UUID NOT NULL REFERENCES entity_type_definition (id),
    allowed_overlay_properties  JSONB NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (system_id, entity_type_id)
);

CREATE TRIGGER system_overlay_schema_system_id_type_check
    BEFORE INSERT OR UPDATE OF system_id ON system_overlay_schema
    FOR EACH ROW EXECUTE FUNCTION assert_system_id_targets_system_entity();

-- Typed, directed edges between entities — building block for RBAC and ReBAC.
-- system_id IS NULL means the relation is global; otherwise it is system-scoped.
CREATE TABLE relation (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_entity_id UUID         NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    relation_type     VARCHAR(100) NOT NULL,
    target_entity_id  UUID         NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    system_id         UUID         REFERENCES entity (id),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TRIGGER relation_system_id_type_check
    BEFORE INSERT OR UPDATE OF system_id ON relation
    FOR EACH ROW EXECUTE FUNCTION assert_system_id_targets_system_entity();

-- System-specific properties layered on top of global entities.
CREATE TABLE property_overlay (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id  UUID         NOT NULL REFERENCES entity (id) ON DELETE CASCADE,
    system_id  UUID         NOT NULL REFERENCES entity (id),
    properties JSONB        NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (entity_id, system_id)
);

CREATE TRIGGER property_overlay_system_id_type_check
    BEFORE INSERT OR UPDATE OF system_id ON property_overlay
    FOR EACH ROW EXECUTE FUNCTION assert_system_id_targets_system_entity();

-- Webhook subscriptions: consumers register a callback URL per system.
CREATE TABLE webhook_subscription (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id    UUID          NOT NULL REFERENCES entity (id),
    callback_url VARCHAR(2000) NOT NULL,
    secret       VARCHAR(500)  NOT NULL,
    active       BOOLEAN       NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TRIGGER webhook_subscription_system_id_type_check
    BEFORE INSERT OR UPDATE OF system_id ON webhook_subscription
    FOR EACH ROW EXECUTE FUNCTION assert_system_id_targets_system_entity();

-- Immutable record of every write operation.
-- system_id has no FK (preserved across system entity deletion).
CREATE TABLE audit_log (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    actor         VARCHAR(500) NOT NULL,
    operation     VARCHAR(20)  NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    resource_type VARCHAR(100) NOT NULL,
    resource_id   UUID         NOT NULL,
    before_value  JSONB,
    after_value   JSONB,
    system_id     UUID,                   -- intentionally no FK (see data-model §4.5)
    timestamp     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Immutable record of every retrieval event for compliance.
CREATE TABLE retrieval_log (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    caller_identity  VARCHAR(500) NOT NULL,
    query_parameters JSONB        NOT NULL,
    returned_refs    JSONB        NOT NULL,
    system_id        UUID,                -- intentionally no FK
    timestamp        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Tracks individual delivery attempts for webhook notifications.
-- ON DELETE CASCADE: if the parent subscription is removed, drop pending deliveries.
CREATE TABLE webhook_delivery (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id    UUID        NOT NULL REFERENCES webhook_subscription (id) ON DELETE CASCADE,
    audit_log_id       UUID        NOT NULL REFERENCES audit_log (id),
    status             VARCHAR(20) NOT NULL DEFAULT 'pending'
                                   CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts           INTEGER     NOT NULL DEFAULT 0,
    next_retry_at      TIMESTAMPTZ,
    last_response_code INTEGER,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 8. Partial unique indexes for relation
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
-- 9. Performance indexes
-- ---------------------------------------------------------------------------

-- Entity: primary retrieval path and property filter queries
CREATE INDEX idx_entity_type        ON entity (type_id);
CREATE INDEX idx_entity_system_id   ON entity (system_id);
CREATE INDEX idx_entity_properties  ON entity USING GIN (properties);

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
-- 10. updated_at trigger
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

CREATE TRIGGER entity_updated_at
    BEFORE UPDATE ON entity
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER entity_external_identity_updated_at
    BEFORE UPDATE ON entity_external_identity
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
-- 11. Audit immutability triggers
--     Enforces append-only semantics on audit_log and retrieval_log at the
--     database level. Application roles should also have UPDATE/DELETE revoked.
--     (See: docs/data-model.md §2.9, FR-AUD-003, NFR-AUD-001)
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
-- 12. Row-Level Security (RLS)
--     Defense-in-depth: the application layer is the primary authorization
--     boundary; RLS is a database-level enforcement layer.
--
--     Design:
--     • app.current_system_id is SET LOCAL per request (see internal/db/scope.go)
--     • Empty string (not set) = admin mode → no restriction
--     • Set to a UUID string = system-scoped mode → restrict to that system
--     • Global rows (system_id IS NULL) are always visible
--
--     FORCE ROW LEVEL SECURITY ensures policies apply even to the table owner,
--     making RLS testable in development with the default superuser.
-- ---------------------------------------------------------------------------

ALTER TABLE entity              ENABLE ROW LEVEL SECURITY;
ALTER TABLE entity              FORCE  ROW LEVEL SECURITY;

ALTER TABLE relation            ENABLE ROW LEVEL SECURITY;
ALTER TABLE relation            FORCE  ROW LEVEL SECURITY;

ALTER TABLE property_overlay    ENABLE ROW LEVEL SECURITY;
ALTER TABLE property_overlay    FORCE  ROW LEVEL SECURITY;

ALTER TABLE webhook_subscription ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_subscription FORCE  ROW LEVEL SECURITY;

CREATE POLICY entity_system_isolation ON entity
    USING (
        system_id IS NULL
        OR current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );

CREATE POLICY relation_system_isolation ON relation
    USING (
        system_id IS NULL
        OR current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );

CREATE POLICY property_overlay_system_isolation ON property_overlay
    USING (
        current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );

CREATE POLICY webhook_subscription_system_isolation ON webhook_subscription
    USING (
        current_setting('app.current_system_id', true) = ''
        OR system_id::text = current_setting('app.current_system_id', true)
    );
