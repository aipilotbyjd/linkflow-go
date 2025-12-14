#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo -e "${GREEN}Seeding LinkFlow database...${NC}"

# Database connection
DB_HOST=${LINKFLOW_DB_HOST:-localhost}
DB_PORT=${LINKFLOW_DB_PORT:-5432}
DB_NAME=${LINKFLOW_DB_NAME:-linkflow}
DB_USER=${LINKFLOW_DB_USER:-linkflow}
DB_PASSWORD=${LINKFLOW_DB_PASSWORD:-linkflow123}

export PGPASSWORD=$DB_PASSWORD

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo "psql not found. Using docker..."
    PSQL="docker exec -i linkflow-go-postgres-1 psql -U $DB_USER -d $DB_NAME"
else
    PSQL="psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"
fi

echo -e "${YELLOW}Creating admin user...${NC}"
$PSQL << 'EOF'
-- Insert admin user (password: admin123)
INSERT INTO auth.users (id, email, username, password, first_name, last_name, email_verified, status)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'admin@linkflow.io',
    'admin',
    '$2a$10$rQEY7xQxB7xQxB7xQxB7xOQxB7xQxB7xQxB7xQxB7xQxB7xQxB7xQ',
    'Admin',
    'User',
    true,
    'active'
) ON CONFLICT (email) DO NOTHING;

-- Assign admin role
INSERT INTO auth.user_roles (user_id, role_id)
SELECT 'a0000000-0000-0000-0000-000000000001', id FROM auth.roles WHERE name = 'admin'
ON CONFLICT DO NOTHING;

-- Insert demo user (password: demo123)
INSERT INTO auth.users (id, email, username, password, first_name, last_name, email_verified, status)
VALUES (
    'a0000000-0000-0000-0000-000000000002',
    'demo@linkflow.io',
    'demo',
    '$2a$10$rQEY7xQxB7xQxB7xQxB7xOQxB7xQxB7xQxB7xQxB7xQxB7xQxB7xQ',
    'Demo',
    'User',
    true,
    'active'
) ON CONFLICT (email) DO NOTHING;

-- Assign user role to demo
INSERT INTO auth.user_roles (user_id, role_id)
SELECT 'a0000000-0000-0000-0000-000000000002', id FROM auth.roles WHERE name = 'user'
ON CONFLICT DO NOTHING;
EOF

echo -e "${YELLOW}Creating sample workflows...${NC}"
$PSQL << 'EOF'
-- Sample HTTP Request workflow
INSERT INTO workflow.workflows (id, name, description, user_id, nodes, connections, settings, status, is_active, version)
VALUES (
    'w0000000-0000-0000-0000-000000000001',
    'HTTP Request Example',
    'A simple workflow that makes an HTTP request',
    'a0000000-0000-0000-0000-000000000002',
    '[
        {"id": "trigger_1", "name": "Manual Trigger", "type": "manualTrigger", "position": {"x": 100, "y": 200}, "parameters": {}},
        {"id": "http_1", "name": "HTTP Request", "type": "http", "position": {"x": 350, "y": 200}, "parameters": {"url": "https://api.example.com/data", "method": "GET"}}
    ]'::jsonb,
    '[{"id": "conn_1", "source": "trigger_1", "target": "http_1", "sourcePort": "output", "targetPort": "input"}]'::jsonb,
    '{"timeout": 300, "retryOnFailure": false, "maxRetries": 3, "timezone": "UTC"}'::jsonb,
    'inactive',
    false,
    1
) ON CONFLICT DO NOTHING;

-- Sample Conditional workflow
INSERT INTO workflow.workflows (id, name, description, user_id, nodes, connections, settings, status, is_active, version)
VALUES (
    'w0000000-0000-0000-0000-000000000002',
    'Conditional Logic Example',
    'Demonstrates IF/ELSE branching',
    'a0000000-0000-0000-0000-000000000002',
    '[
        {"id": "trigger_1", "name": "Webhook Trigger", "type": "webhookTrigger", "position": {"x": 100, "y": 200}, "parameters": {}},
        {"id": "if_1", "name": "Check Value", "type": "if", "position": {"x": 350, "y": 200}, "parameters": {"condition": {"field": "body.status", "operator": "equals", "value": "success"}}},
        {"id": "slack_1", "name": "Send Success", "type": "slack", "position": {"x": 600, "y": 100}, "parameters": {"text": "Success!"}},
        {"id": "email_1", "name": "Send Alert", "type": "email", "position": {"x": 600, "y": 300}, "parameters": {"subject": "Alert"}}
    ]'::jsonb,
    '[
        {"id": "conn_1", "source": "trigger_1", "target": "if_1"},
        {"id": "conn_2", "source": "if_1", "target": "slack_1", "sourcePort": "true"},
        {"id": "conn_3", "source": "if_1", "target": "email_1", "sourcePort": "false"}
    ]'::jsonb,
    '{"timeout": 300, "retryOnFailure": true, "maxRetries": 3, "timezone": "UTC"}'::jsonb,
    'inactive',
    false,
    1
) ON CONFLICT DO NOTHING;
EOF

echo -e "${YELLOW}Creating sample node types...${NC}"
$PSQL << 'EOF'
-- Insert built-in node types
INSERT INTO node.node_types (id, type, name, description, category, icon, version, is_builtin, is_public, status) VALUES
('n0000001-0000-0000-0000-000000000001', 'manualTrigger', 'Manual Trigger', 'Manually trigger workflow execution', 'trigger', 'play', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000002', 'webhookTrigger', 'Webhook Trigger', 'Trigger workflow via HTTP webhook', 'trigger', 'webhook', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000003', 'scheduleTrigger', 'Schedule Trigger', 'Trigger workflow on a schedule', 'trigger', 'clock', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000004', 'http', 'HTTP Request', 'Make HTTP requests to external APIs', 'action', 'globe', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000005', 'if', 'IF', 'Conditional branching based on conditions', 'control', 'git-branch', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000006', 'switch', 'Switch', 'Route to different branches based on value', 'control', 'shuffle', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000007', 'loop', 'Loop', 'Iterate over array items', 'control', 'repeat', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000008', 'set', 'Set', 'Set or transform data values', 'transform', 'edit', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000009', 'merge', 'Merge', 'Merge data from multiple branches', 'control', 'git-merge', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000010', 'email', 'Email', 'Send emails via SMTP', 'action', 'mail', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000011', 'slack', 'Slack', 'Send messages to Slack', 'action', 'slack', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000012', 'database', 'Database', 'Execute database queries', 'action', 'database', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000013', 'wait', 'Wait', 'Pause execution for a duration', 'control', 'clock', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000014', 'json', 'JSON', 'Parse and manipulate JSON data', 'transform', 'code', '1.0.0', true, true, 'active'),
('n0000001-0000-0000-0000-000000000015', 'crypto', 'Crypto', 'Encryption and hashing operations', 'transform', 'lock', '1.0.0', true, true, 'active')
ON CONFLICT (type) DO NOTHING;
EOF

echo -e "${YELLOW}Creating sample variables...${NC}"
$PSQL << 'EOF'
INSERT INTO variable.variables (key, value, type, description) VALUES
('API_BASE_URL', 'https://api.example.com', 'string', 'Base URL for external API'),
('DEFAULT_TIMEOUT', '30', 'number', 'Default timeout in seconds'),
('DEBUG_MODE', 'false', 'boolean', 'Enable debug mode'),
('API_KEY', 'sk-xxxx-xxxx-xxxx', 'secret', 'API key for external service')
ON CONFLICT (key) DO NOTHING;
EOF

echo -e "${GREEN}Database seeding completed!${NC}"
echo ""
echo "Created:"
echo "  - Admin user: admin@linkflow.io (password: admin123)"
echo "  - Demo user: demo@linkflow.io (password: demo123)"
echo "  - 2 sample workflows"
echo "  - 15 built-in node types"
echo "  - 4 global variables"
