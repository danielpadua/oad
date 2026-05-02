-- =============================================================================
-- OAD — Rollback Initial Schema
-- Drops all objects created by 000001_initial_schema.up.sql in reverse order.
-- =============================================================================

-- Triggers
DROP TRIGGER IF EXISTS retrieval_log_immutable                       ON retrieval_log;
DROP TRIGGER IF EXISTS audit_log_immutable                           ON audit_log;
DROP TRIGGER IF EXISTS webhook_subscription_updated_at               ON webhook_subscription;
DROP TRIGGER IF EXISTS property_overlay_updated_at                   ON property_overlay;
DROP TRIGGER IF EXISTS system_overlay_schema_updated_at              ON system_overlay_schema;
DROP TRIGGER IF EXISTS entity_external_identity_updated_at           ON entity_external_identity;
DROP TRIGGER IF EXISTS entity_updated_at                             ON entity;
DROP TRIGGER IF EXISTS entity_type_definition_updated_at             ON entity_type_definition;

DROP TRIGGER IF EXISTS webhook_subscription_system_id_type_check     ON webhook_subscription;
DROP TRIGGER IF EXISTS property_overlay_system_id_type_check         ON property_overlay;
DROP TRIGGER IF EXISTS relation_system_id_type_check                 ON relation;
DROP TRIGGER IF EXISTS system_overlay_schema_system_id_type_check    ON system_overlay_schema;
DROP TRIGGER IF EXISTS entity_system_id_type_check                   ON entity;

DROP FUNCTION IF EXISTS prevent_audit_modification();
DROP FUNCTION IF EXISTS assert_system_id_targets_system_entity();
DROP FUNCTION IF EXISTS set_updated_at();

-- Tables (reverse dependency order — children before parents)
DROP TABLE IF EXISTS webhook_delivery         CASCADE;
DROP TABLE IF EXISTS retrieval_log            CASCADE;
DROP TABLE IF EXISTS audit_log                CASCADE;
DROP TABLE IF EXISTS webhook_subscription     CASCADE;
DROP TABLE IF EXISTS property_overlay         CASCADE;
DROP TABLE IF EXISTS relation                 CASCADE;
DROP TABLE IF EXISTS system_overlay_schema    CASCADE;
DROP TABLE IF EXISTS entity_external_identity CASCADE;
DROP TABLE IF EXISTS entity                   CASCADE;
DROP TABLE IF EXISTS entity_type_definition   CASCADE;
