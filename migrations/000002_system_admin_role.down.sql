-- migrations/000002_system_admin_role.down.sql
DELETE FROM entity WHERE external_id = 'oad:system-admin' AND is_builtin = true;
