-- ============================================================================
-- Migration: 000018_seed_data
-- Description: Insert initial seed data for development/production
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- System User (for automated operations)
-- ---------------------------------------------------------------------------
INSERT INTO auth.users (id, email, password_hash, first_name, last_name, status, email_verified)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'system@linkflow.local',
    '$2a$10$DISABLED_ACCOUNT_NO_LOGIN',
    'System',
    'User',
    'active',
    TRUE
) ON CONFLICT (email) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Admin User (password: changeme123)
-- ---------------------------------------------------------------------------
INSERT INTO auth.users (id, email, password_hash, first_name, last_name, status, email_verified)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'admin@linkflow.local',
    '$2a$10$rQnM1.Hs8LdDqT5KjKjKjOeJvKjKjKjKjKjKjKjKjKjKjKjKjKjKj',
    'Admin',
    'User',
    'active',
    TRUE
) ON CONFLICT (email) DO NOTHING;


-- ---------------------------------------------------------------------------
-- Assign admin role to admin user
-- ---------------------------------------------------------------------------
INSERT INTO auth.user_roles (user_id, role_id)
SELECT 
    '00000000-0000-0000-0000-000000000002',
    id
FROM auth.roles 
WHERE name = 'admin'
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Node Types (Core Triggers)
-- ---------------------------------------------------------------------------
INSERT INTO node.node_types (name, type, category, description, icon, config_schema, is_trigger, is_core) VALUES
    ('Manual Trigger', 'trigger.manual', 'triggers', 'Manually trigger workflow execution', 'play', '{}', TRUE, TRUE),
    ('Webhook Trigger', 'trigger.webhook', 'triggers', 'Trigger workflow via HTTP webhook', 'webhook', '{"properties": {"method": {"type": "string", "enum": ["GET", "POST", "PUT", "DELETE"]}}}', TRUE, TRUE),
    ('Schedule Trigger', 'trigger.schedule', 'triggers', 'Trigger workflow on a schedule', 'clock', '{"properties": {"cron": {"type": "string"}}}', TRUE, TRUE),
    ('Event Trigger', 'trigger.event', 'triggers', 'Trigger workflow on system events', 'zap', '{"properties": {"event_type": {"type": "string"}}}', TRUE, TRUE)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Node Types (Core Actions)
-- ---------------------------------------------------------------------------
INSERT INTO node.node_types (name, type, category, description, icon, config_schema, is_trigger, is_core) VALUES
    ('HTTP Request', 'action.http', 'actions', 'Make HTTP requests to external APIs', 'globe', '{"properties": {"url": {"type": "string"}, "method": {"type": "string"}}}', FALSE, TRUE),
    ('Code', 'action.code', 'actions', 'Execute custom JavaScript code', 'code', '{"properties": {"language": {"type": "string"}, "code": {"type": "string"}}}', FALSE, TRUE),
    ('Set', 'action.set', 'actions', 'Set workflow variables', 'edit', '{"properties": {"values": {"type": "object"}}}', FALSE, TRUE),
    ('IF', 'action.if', 'actions', 'Conditional branching', 'git-branch', '{"properties": {"conditions": {"type": "array"}}}', FALSE, TRUE),
    ('Switch', 'action.switch', 'actions', 'Multi-way branching', 'shuffle', '{"properties": {"rules": {"type": "array"}}}', FALSE, TRUE),
    ('Merge', 'action.merge', 'actions', 'Merge multiple inputs', 'git-merge', '{"properties": {"mode": {"type": "string"}}}', FALSE, TRUE),
    ('Loop', 'action.loop', 'actions', 'Loop over items', 'repeat', '{"properties": {"batch_size": {"type": "integer"}}}', FALSE, TRUE),
    ('Wait', 'action.wait', 'actions', 'Wait for specified duration', 'clock', '{"properties": {"duration": {"type": "integer"}}}', FALSE, TRUE),
    ('Error Trigger', 'action.error', 'actions', 'Handle workflow errors', 'alert-triangle', '{}', FALSE, TRUE),
    ('No Operation', 'action.noop', 'actions', 'Do nothing (placeholder)', 'minus', '{}', FALSE, TRUE)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Notification Channels
-- ---------------------------------------------------------------------------
INSERT INTO notification.channels (name, type, config, is_active) VALUES
    ('Email', 'email', '{"smtp_host": "localhost", "smtp_port": 587}', TRUE),
    ('In-App', 'in_app', '{}', TRUE),
    ('Webhook', 'webhook', '{}', TRUE)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- Default Notification Templates
-- ---------------------------------------------------------------------------
INSERT INTO notification.templates (name, channel_id, subject, body_template, variables) 
SELECT 
    'Workflow Execution Failed',
    c.id,
    'Workflow "{{workflow_name}}" failed',
    'Your workflow "{{workflow_name}}" failed at {{timestamp}}. Error: {{error_message}}',
    '["workflow_name", "timestamp", "error_message"]'::JSONB
FROM notification.channels c WHERE c.name = 'Email'
ON CONFLICT DO NOTHING;

INSERT INTO notification.templates (name, channel_id, subject, body_template, variables) 
SELECT 
    'Welcome Email',
    c.id,
    'Welcome to LinkFlow!',
    'Hi {{first_name}}, welcome to LinkFlow! Get started by creating your first workflow.',
    '["first_name"]'::JSONB
FROM notification.channels c WHERE c.name = 'Email'
ON CONFLICT DO NOTHING;

COMMIT;
