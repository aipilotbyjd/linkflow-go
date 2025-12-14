-- ============================================================================
-- Migration: 000018_seed_data
-- Description: Insert initial seed data for development/production
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Default Roles (RBAC)
-- ---------------------------------------------------------------------------
INSERT INTO auth.roles (id, name, description, is_system) VALUES
    ('00000000-0000-0000-0000-000000000010', 'admin', 'System administrator with full access', TRUE),
    ('00000000-0000-0000-0000-000000000011', 'user', 'Standard user with basic access', TRUE),
    ('00000000-0000-0000-0000-000000000012', 'viewer', 'Read-only access to workflows', TRUE),
    ('00000000-0000-0000-0000-000000000013', 'developer', 'Developer with API and custom node access', TRUE)
ON CONFLICT (name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Permissions
-- ---------------------------------------------------------------------------
INSERT INTO auth.permissions (id, name, resource, action, description) VALUES
    -- Workflow permissions
    ('00000000-0000-0000-0001-000000000001', 'workflow:create', 'workflow', 'create', 'Create new workflows'),
    ('00000000-0000-0000-0001-000000000002', 'workflow:read', 'workflow', 'read', 'View workflows'),
    ('00000000-0000-0000-0001-000000000003', 'workflow:update', 'workflow', 'update', 'Edit workflows'),
    ('00000000-0000-0000-0001-000000000004', 'workflow:delete', 'workflow', 'delete', 'Delete workflows'),
    ('00000000-0000-0000-0001-000000000005', 'workflow:execute', 'workflow', 'execute', 'Execute workflows'),
    -- Credential permissions
    ('00000000-0000-0000-0001-000000000010', 'credential:create', 'credential', 'create', 'Create credentials'),
    ('00000000-0000-0000-0001-000000000011', 'credential:read', 'credential', 'read', 'View credentials'),
    ('00000000-0000-0000-0001-000000000012', 'credential:update', 'credential', 'update', 'Edit credentials'),
    ('00000000-0000-0000-0001-000000000013', 'credential:delete', 'credential', 'delete', 'Delete credentials'),
    -- Team permissions
    ('00000000-0000-0000-0001-000000000020', 'team:create', 'team', 'create', 'Create teams'),
    ('00000000-0000-0000-0001-000000000021', 'team:manage', 'team', 'manage', 'Manage team members'),
    -- Admin permissions
    ('00000000-0000-0000-0001-000000000030', 'admin:users', 'admin', 'users', 'Manage all users'),
    ('00000000-0000-0000-0001-000000000031', 'admin:system', 'admin', 'system', 'Manage system settings'),
    ('00000000-0000-0000-0001-000000000032', 'admin:billing', 'admin', 'billing', 'Manage billing settings')
ON CONFLICT (name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Assign permissions to roles
-- ---------------------------------------------------------------------------
-- Admin role gets all permissions
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT 
    '00000000-0000-0000-0000-000000000010'::UUID,
    id
FROM auth.permissions
ON CONFLICT DO NOTHING;

-- User role gets standard permissions
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT 
    '00000000-0000-0000-0000-000000000011'::UUID,
    id
FROM auth.permissions
WHERE name IN ('workflow:create', 'workflow:read', 'workflow:update', 'workflow:delete', 'workflow:execute',
               'credential:create', 'credential:read', 'credential:update', 'credential:delete')
ON CONFLICT DO NOTHING;

-- Viewer role gets read-only permissions
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT 
    '00000000-0000-0000-0000-000000000012'::UUID,
    id
FROM auth.permissions
WHERE name IN ('workflow:read', 'credential:read')
ON CONFLICT DO NOTHING;

-- Developer role gets user permissions plus team management
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT 
    '00000000-0000-0000-0000-000000000013'::UUID,
    id
FROM auth.permissions
WHERE name IN ('workflow:create', 'workflow:read', 'workflow:update', 'workflow:delete', 'workflow:execute',
               'credential:create', 'credential:read', 'credential:update', 'credential:delete',
               'team:create', 'team:manage')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- System User (for automated operations)
-- ---------------------------------------------------------------------------
INSERT INTO auth.users (id, email, password_hash, first_name, last_name, status, email_verified)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'system@linkflow.local',
    '$2a$10$DISABLED_ACCOUNT_NO_LOGIN_ALLOWED_HERE',
    'System',
    'User',
    'inactive',
    TRUE
) ON CONFLICT (email) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Admin User (password: changeme123 - MUST BE CHANGED IN PRODUCTION)
-- Note: This is a bcrypt hash placeholder - generate proper hash in production
-- ---------------------------------------------------------------------------
INSERT INTO auth.users (id, email, password_hash, first_name, last_name, status, email_verified)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'admin@linkflow.local',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    'Admin',
    'User',
    'active',
    TRUE
) ON CONFLICT (email) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Assign admin role to admin user
-- ---------------------------------------------------------------------------
INSERT INTO auth.user_roles (user_id, role_id)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000010'
) ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Node Types (Core Triggers)
-- ---------------------------------------------------------------------------
INSERT INTO node.node_types (type, name, category, description, icon, config_schema, is_builtin, status) VALUES
    ('trigger.manual', 'Manual Trigger', 'triggers', 'Manually trigger workflow execution', 'play', '{}', TRUE, 'active'),
    ('trigger.webhook', 'Webhook Trigger', 'triggers', 'Trigger workflow via HTTP webhook', 'webhook', '{"properties": {"method": {"type": "string", "enum": ["GET", "POST", "PUT", "DELETE"]}}}', TRUE, 'active'),
    ('trigger.schedule', 'Schedule Trigger', 'triggers', 'Trigger workflow on a schedule', 'clock', '{"properties": {"cron": {"type": "string"}}}', TRUE, 'active'),
    ('trigger.event', 'Event Trigger', 'triggers', 'Trigger workflow on system events', 'zap', '{"properties": {"event_type": {"type": "string"}}}', TRUE, 'active')
ON CONFLICT (type) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Node Types (Core Actions)
-- ---------------------------------------------------------------------------
INSERT INTO node.node_types (type, name, category, description, icon, config_schema, is_builtin, status) VALUES
    ('action.http', 'HTTP Request', 'actions', 'Make HTTP requests to external APIs', 'globe', '{"properties": {"url": {"type": "string"}, "method": {"type": "string"}}}', TRUE, 'active'),
    ('action.code', 'Code', 'actions', 'Execute custom JavaScript code', 'code', '{"properties": {"language": {"type": "string"}, "code": {"type": "string"}}}', TRUE, 'active'),
    ('action.set', 'Set', 'actions', 'Set workflow variables', 'edit', '{"properties": {"values": {"type": "object"}}}', TRUE, 'active'),
    ('action.if', 'IF', 'actions', 'Conditional branching', 'git-branch', '{"properties": {"conditions": {"type": "array"}}}', TRUE, 'active'),
    ('action.switch', 'Switch', 'actions', 'Multi-way branching', 'shuffle', '{"properties": {"rules": {"type": "array"}}}', TRUE, 'active'),
    ('action.merge', 'Merge', 'actions', 'Merge multiple inputs', 'git-merge', '{"properties": {"mode": {"type": "string"}}}', TRUE, 'active'),
    ('action.loop', 'Loop', 'actions', 'Loop over items', 'repeat', '{"properties": {"batch_size": {"type": "integer"}}}', TRUE, 'active'),
    ('action.wait', 'Wait', 'actions', 'Wait for specified duration', 'clock', '{"properties": {"duration": {"type": "integer"}}}', TRUE, 'active'),
    ('action.error', 'Error Handler', 'actions', 'Handle workflow errors', 'alert-triangle', '{}', TRUE, 'active'),
    ('action.noop', 'No Operation', 'actions', 'Do nothing (placeholder)', 'minus', '{}', TRUE, 'active')
ON CONFLICT (type) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Node Types (Data Operations)
-- ---------------------------------------------------------------------------
INSERT INTO node.node_types (type, name, category, description, icon, config_schema, is_builtin, status) VALUES
    ('data.transform', 'Transform', 'data', 'Transform data between formats', 'shuffle', '{}', TRUE, 'active'),
    ('data.filter', 'Filter', 'data', 'Filter data based on conditions', 'filter', '{}', TRUE, 'active'),
    ('data.aggregate', 'Aggregate', 'data', 'Aggregate multiple data items', 'layers', '{}', TRUE, 'active'),
    ('data.split', 'Split', 'data', 'Split data into multiple items', 'scissors', '{}', TRUE, 'active')
ON CONFLICT (type) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Credential Types
-- ---------------------------------------------------------------------------
INSERT INTO credential.credential_types (type, name, description, category, schema, is_builtin) VALUES
    ('http.basic', 'HTTP Basic Auth', 'Username and password authentication', 'http', 
     '{"properties": {"username": {"type": "string"}, "password": {"type": "string", "format": "password"}}, "required": ["username", "password"]}', TRUE),
    ('http.bearer', 'Bearer Token', 'Bearer token authentication', 'http',
     '{"properties": {"token": {"type": "string", "format": "password"}}, "required": ["token"]}', TRUE),
    ('http.api_key', 'API Key', 'API key authentication', 'http',
     '{"properties": {"key": {"type": "string", "format": "password"}, "header_name": {"type": "string", "default": "X-API-Key"}}, "required": ["key"]}', TRUE),
    ('oauth2.generic', 'OAuth2', 'Generic OAuth2 authentication', 'oauth',
     '{"properties": {"client_id": {"type": "string"}, "client_secret": {"type": "string", "format": "password"}, "auth_url": {"type": "string"}, "token_url": {"type": "string"}, "scope": {"type": "string"}}, "required": ["client_id", "client_secret"]}', TRUE),
    ('database.postgres', 'PostgreSQL', 'PostgreSQL database connection', 'database',
     '{"properties": {"host": {"type": "string"}, "port": {"type": "integer", "default": 5432}, "database": {"type": "string"}, "username": {"type": "string"}, "password": {"type": "string", "format": "password"}}, "required": ["host", "database", "username", "password"]}', TRUE),
    ('database.mysql', 'MySQL', 'MySQL database connection', 'database',
     '{"properties": {"host": {"type": "string"}, "port": {"type": "integer", "default": 3306}, "database": {"type": "string"}, "username": {"type": "string"}, "password": {"type": "string", "format": "password"}}, "required": ["host", "database", "username", "password"]}', TRUE)
ON CONFLICT (type) DO NOTHING;

-- ---------------------------------------------------------------------------
-- System Variables (Global Configuration)
-- ---------------------------------------------------------------------------
INSERT INTO variable.variables (key, value, type, scope, description, is_secret) VALUES
    ('system.version', '1.0.0', 'string', 'global', 'Current system version', FALSE),
    ('system.maintenance_mode', 'false', 'boolean', 'global', 'Enable maintenance mode', FALSE),
    ('system.max_concurrent_executions', '100', 'number', 'global', 'Maximum concurrent workflow executions', FALSE),
    ('system.default_timeout_seconds', '3600', 'number', 'global', 'Default workflow execution timeout', FALSE),
    ('system.webhook_base_url', 'http://localhost:8080/webhooks', 'string', 'global', 'Base URL for webhooks', FALSE)
ON CONFLICT (user_id, team_id, workflow_id, key) DO NOTHING;

COMMIT;
