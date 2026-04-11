-- =============================================================================
-- OAD — Rollback Initial Schema
-- Drops all objects created by 000001_initial_schema.up.sql in reverse order.
-- =============================================================================

-- Triggers (dropped automatically with their tables, but explicit is cleaner)
DROP TRIGGER IF EXISTS retrieval_log_immutable         ON retrieval_log;
DROP TRIGGER IF EXISTS audit_log_immutable             ON audit_log;
DROP TRIGGER IF EXISTS webhook_subscription_updated_at ON webhook_subscription;
DROP TRIGGER IF EXISTS property_overlay_updated_at     ON property_overlay;
DROP TRIGGER IF EXISTS system_overlay_schema_updated_at ON system_overlay_schema;
DROP TRIGGER IF EXISTS entity_updated_at               ON entity;
DROP TRIGGER IF EXISTS system_updated_at               ON system;
DROP TRIGGER IF EXISTS entity_type_definition_updated_at ON entity_type_definition;

DROP FUNCTION IF EXISTS prevent_audit_modification();
DROP FUNCTION IF EXISTS set_updated_at();

-- Tables (reverse dependency order — children before parents)
DROP TABLE IF EXISTS webhook_delivery       CASCADE;
DROP TABLE IF EXISTS retrieval_log          CASCADE;
DROP TABLE IF EXISTS audit_log              CASCADE;
DROP TABLE IF EXISTS webhook_subscription   CASCADE;
DROP TABLE IF EXISTS property_overlay       CASCADE;
DROP TABLE IF EXISTS relation               CASCADE;
DROP TABLE IF EXISTS system_overlay_schema  CASCADE;
DROP TABLE IF EXISTS entity                 CASCADE;
DROP TABLE IF EXISTS system                 CASCADE;
DROP TABLE IF EXISTS entity_type_definition CASCADE;
