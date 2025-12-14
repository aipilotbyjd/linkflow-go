-- ============================================================================
-- Migration: 000018_seed_data (ROLLBACK)
-- Description: Remove seed data
-- WARNING: This will remove all seed data including default admin user
-- ============================================================================

BEGIN;

-- Remove system variables
DELETE FROM variable.variables 
WHERE scope = 'global' 
  AND key LIKE 'system.%';

-- Remove default credential types
DELETE FROM credential.credential_types 
WHERE is_builtin = TRUE 
  AND type IN ('http.basic', 'http.bearer', 'http.api_key', 'oauth2.generic', 'database.postgres', 'database.mysql');

-- Remove built-in node types
DELETE FROM node.node_types 
WHERE is_builtin = TRUE;

-- Remove admin user role assignment
DELETE FROM auth.user_roles 
WHERE user_id = '00000000-0000-0000-0000-000000000002'
  AND role_id = '00000000-0000-0000-0000-000000000010';

-- Remove default users (system and admin)
DELETE FROM auth.users 
WHERE id IN (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000002'
);

-- Remove role permissions
DELETE FROM auth.role_permissions 
WHERE role_id IN (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000012',
    '00000000-0000-0000-0000-000000000013'
);

-- Remove default permissions
DELETE FROM auth.permissions 
WHERE id LIKE '00000000-0000-0000-0001-%';

-- Remove default roles (must be after role_permissions due to FK)
DELETE FROM auth.roles 
WHERE is_system = TRUE 
  AND id IN (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000012',
    '00000000-0000-0000-0000-000000000013'
);

COMMIT;
