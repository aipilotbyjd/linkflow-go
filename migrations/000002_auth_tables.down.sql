-- ============================================================================
-- Migration: 000002_auth_tables (ROLLBACK)
-- Description: Drop authentication and authorization tables
-- ============================================================================

BEGIN;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_teams_updated_at ON auth.teams;
DROP TRIGGER IF EXISTS trg_oauth_connections_updated_at ON auth.oauth_connections;
DROP TRIGGER IF EXISTS trg_roles_updated_at ON auth.roles;
DROP TRIGGER IF EXISTS trg_users_updated_at ON auth.users;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS auth.password_resets;
DROP TABLE IF EXISTS auth.team_members;
DROP TABLE IF EXISTS auth.teams;
DROP TABLE IF EXISTS auth.oauth_connections;
DROP TABLE IF EXISTS auth.api_keys;
DROP TABLE IF EXISTS auth.sessions;
DROP TABLE IF EXISTS auth.role_permissions;
DROP TABLE IF EXISTS auth.user_roles;
DROP TABLE IF EXISTS auth.permissions;
DROP TABLE IF EXISTS auth.roles;
DROP TABLE IF EXISTS auth.users;

COMMIT;
