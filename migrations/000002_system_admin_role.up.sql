-- migrations/000002_system_admin_role.up.sql
-- Add the oad:system-admin built-in group.
-- Users in this group + has_role_in relation on specific systems get
-- scoped management rights (edit description, CRUD overlay schemas).

INSERT INTO entity (type_id, external_id, properties, system_id, is_builtin)
VALUES (
    (SELECT id FROM entity_type_definition WHERE type_name = 'Group'),
    'oad:system-admin',
    '{"displayName":"OAD System Admin","description":"Scoped management access: edit system description and manage overlay schemas for assigned systems."}'::jsonb,
    NULL,
    true
);
