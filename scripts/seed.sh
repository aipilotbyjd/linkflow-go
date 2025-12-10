#!/bin/bash

# Database Seeding Script for LinkFlow Development

set -e

# Source environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Default values
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-linkflow}
DB_USER=${DB_USER:-linkflow}
DB_PASSWORD=${DB_PASSWORD:-linkflow123}

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[SEED]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to run SQL command
run_sql() {
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "$1" > /dev/null 2>&1
}

# Function to run SQL file
run_sql_file() {
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$1" > /dev/null 2>&1
}

# Check database connection
check_db_connection() {
    print_status "Checking database connection..."
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '\q' 2>/dev/null
    if [ $? -ne 0 ]; then
        print_error "Cannot connect to database"
        exit 1
    fi
    print_status "Database connection successful ✓"
}

# Clear existing data
clear_data() {
    print_warning "Clearing existing development data..."
    
    # Clear in reverse order of dependencies
    run_sql "TRUNCATE TABLE webhooks CASCADE;" || true
    run_sql "TRUNCATE TABLE api_keys CASCADE;" || true
    run_sql "TRUNCATE TABLE audit_logs CASCADE;" || true
    run_sql "TRUNCATE TABLE execution_steps CASCADE;" || true
    run_sql "TRUNCATE TABLE executions CASCADE;" || true
    run_sql "TRUNCATE TABLE credentials CASCADE;" || true
    run_sql "TRUNCATE TABLE workflow_versions CASCADE;" || true
    run_sql "TRUNCATE TABLE workflows CASCADE;" || true
    run_sql "TRUNCATE TABLE projects CASCADE;" || true
    run_sql "TRUNCATE TABLE organization_members CASCADE;" || true
    run_sql "TRUNCATE TABLE organizations CASCADE;" || true
    run_sql "TRUNCATE TABLE users CASCADE;" || true
    
    print_status "Existing data cleared"
}

# Seed users
seed_users() {
    print_status "Seeding users..."
    
    # Password hash for 'password123' (you should use proper bcrypt in production)
    local password_hash='$2a$10$YourHashedPasswordHere'
    
    run_sql "
    INSERT INTO users (id, email, username, full_name, status, metadata)
    VALUES 
        ('550e8400-e29b-41d4-a716-446655440001', 'admin@linkflow.local', 'admin', 'Admin User', 'active', '{\"role\": \"admin\"}'),
        ('550e8400-e29b-41d4-a716-446655440002', 'john.doe@linkflow.local', 'johndoe', 'John Doe', 'active', '{\"role\": \"user\"}'),
        ('550e8400-e29b-41d4-a716-446655440003', 'jane.smith@linkflow.local', 'janesmith', 'Jane Smith', 'active', '{\"role\": \"user\"}'),
        ('550e8400-e29b-41d4-a716-446655440004', 'bob.wilson@linkflow.local', 'bobwilson', 'Bob Wilson', 'active', '{\"role\": \"user\"}'),
        ('550e8400-e29b-41d4-a716-446655440005', 'alice.johnson@linkflow.local', 'alicejohnson', 'Alice Johnson', 'inactive', '{\"role\": \"user\"}')
    ON CONFLICT (id) DO NOTHING;"
    
    print_status "Users seeded ✓"
}

# Seed organizations
seed_organizations() {
    print_status "Seeding organizations..."
    
    run_sql "
    INSERT INTO organizations (id, name, slug, description, owner_id, settings)
    VALUES 
        ('650e8400-e29b-41d4-a716-446655440001', 'Acme Corporation', 'acme-corp', 'Leading technology solutions provider', '550e8400-e29b-41d4-a716-446655440001', '{\"plan\": \"enterprise\", \"max_workflows\": 1000}'),
        ('650e8400-e29b-41d4-a716-446655440002', 'StartupXYZ', 'startup-xyz', 'Innovative startup in the automation space', '550e8400-e29b-41d4-a716-446655440002', '{\"plan\": \"startup\", \"max_workflows\": 100}'),
        ('650e8400-e29b-41d4-a716-446655440003', 'Digital Agency', 'digital-agency', 'Creative digital solutions', '550e8400-e29b-41d4-a716-446655440003', '{\"plan\": \"professional\", \"max_workflows\": 500}')
    ON CONFLICT (id) DO NOTHING;"
    
    # Add members to organizations
    run_sql "
    INSERT INTO organization_members (organization_id, user_id, role, permissions)
    VALUES 
        ('650e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440001', 'owner', '[\"*\"]'),
        ('650e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', 'admin', '[\"read\", \"write\", \"delete\"]'),
        ('650e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440003', 'member', '[\"read\", \"write\"]'),
        ('650e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440002', 'owner', '[\"*\"]'),
        ('650e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440004', 'member', '[\"read\", \"write\"]')
    ON CONFLICT (organization_id, user_id) DO NOTHING;"
    
    print_status "Organizations seeded ✓"
}

# Seed projects
seed_projects() {
    print_status "Seeding projects..."
    
    run_sql "
    INSERT INTO projects (id, organization_id, name, description, created_by, settings)
    VALUES 
        ('750e8400-e29b-41d4-a716-446655440001', '650e8400-e29b-41d4-a716-446655440001', 'Customer Onboarding', 'Automated customer onboarding workflows', '550e8400-e29b-41d4-a716-446655440001', '{\"notifications\": true}'),
        ('750e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440001', 'Data Processing Pipeline', 'ETL and data processing workflows', '550e8400-e29b-41d4-a716-446655440002', '{\"notifications\": true}'),
        ('750e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440002', 'Marketing Automation', 'Email and social media automation', '550e8400-e29b-41d4-a716-446655440002', '{\"notifications\": false}')
    ON CONFLICT (id) DO NOTHING;"
    
    print_status "Projects seeded ✓"
}

# Seed workflows
seed_workflows() {
    print_status "Seeding workflows..."
    
    run_sql "
    INSERT INTO workflows (id, project_id, name, description, status, definition, trigger_config, tags, created_by)
    VALUES 
        (
            '850e8400-e29b-41d4-a716-446655440001',
            '750e8400-e29b-41d4-a716-446655440001',
            'Welcome Email Workflow',
            'Send welcome email to new customers',
            'published',
            '{
                \"nodes\": [
                    {\"id\": \"trigger-1\", \"type\": \"trigger\", \"config\": {\"type\": \"webhook\"}},
                    {\"id\": \"action-1\", \"type\": \"action\", \"config\": {\"type\": \"send_email\"}},
                    {\"id\": \"action-2\", \"type\": \"action\", \"config\": {\"type\": \"update_crm\"}}
                ],
                \"edges\": [
                    {\"source\": \"trigger-1\", \"target\": \"action-1\"},
                    {\"source\": \"action-1\", \"target\": \"action-2\"}
                ]
            }',
            '{\"type\": \"webhook\", \"path\": \"/webhook/welcome\"}',
            '{automation, email, customer}',
            '550e8400-e29b-41d4-a716-446655440001'
        ),
        (
            '850e8400-e29b-41d4-a716-446655440002',
            '750e8400-e29b-41d4-a716-446655440002',
            'Data Sync Pipeline',
            'Sync data between databases',
            'published',
            '{
                \"nodes\": [
                    {\"id\": \"trigger-1\", \"type\": \"trigger\", \"config\": {\"type\": \"schedule\", \"cron\": \"0 */6 * * *\"}},
                    {\"id\": \"action-1\", \"type\": \"action\", \"config\": {\"type\": \"fetch_data\"}},
                    {\"id\": \"transform-1\", \"type\": \"transform\", \"config\": {\"type\": \"map\"}},
                    {\"id\": \"action-2\", \"type\": \"action\", \"config\": {\"type\": \"save_data\"}}
                ],
                \"edges\": [
                    {\"source\": \"trigger-1\", \"target\": \"action-1\"},
                    {\"source\": \"action-1\", \"target\": \"transform-1\"},
                    {\"source\": \"transform-1\", \"target\": \"action-2\"}
                ]
            }',
            '{\"type\": \"schedule\", \"cron\": \"0 */6 * * *\"}',
            '{data, etl, scheduled}',
            '550e8400-e29b-41d4-a716-446655440002'
        ),
        (
            '850e8400-e29b-41d4-a716-446655440003',
            '750e8400-e29b-41d4-a716-446655440001',
            'Order Processing Flow',
            'Process and fulfill orders',
            'draft',
            '{
                \"nodes\": [
                    {\"id\": \"trigger-1\", \"type\": \"trigger\", \"config\": {\"type\": \"api\"}},
                    {\"id\": \"condition-1\", \"type\": \"condition\", \"config\": {\"expression\": \"order.total > 100\"}},
                    {\"id\": \"action-1\", \"type\": \"action\", \"config\": {\"type\": \"apply_discount\"}},
                    {\"id\": \"action-2\", \"type\": \"action\", \"config\": {\"type\": \"process_payment\"}},
                    {\"id\": \"action-3\", \"type\": \"action\", \"config\": {\"type\": \"send_confirmation\"}}
                ],
                \"edges\": [
                    {\"source\": \"trigger-1\", \"target\": \"condition-1\"},
                    {\"source\": \"condition-1\", \"target\": \"action-1\", \"label\": \"true\"},
                    {\"source\": \"condition-1\", \"target\": \"action-2\", \"label\": \"false\"},
                    {\"source\": \"action-1\", \"target\": \"action-2\"},
                    {\"source\": \"action-2\", \"target\": \"action-3\"}
                ]
            }',
            '{\"type\": \"api\", \"endpoint\": \"/api/orders\"}',
            '{order, payment, ecommerce}',
            '550e8400-e29b-41d4-a716-446655440001'
        )
    ON CONFLICT (id) DO NOTHING;"
    
    # Add workflow versions
    run_sql "
    INSERT INTO workflow_versions (workflow_id, version, definition, changelog, created_by)
    VALUES 
        ('850e8400-e29b-41d4-a716-446655440001', 1, '{\"nodes\": [], \"edges\": []}', 'Initial version', '550e8400-e29b-41d4-a716-446655440001'),
        ('850e8400-e29b-41d4-a716-446655440002', 1, '{\"nodes\": [], \"edges\": []}', 'Initial version', '550e8400-e29b-41d4-a716-446655440002')
    ON CONFLICT (workflow_id, version) DO NOTHING;"
    
    print_status "Workflows seeded ✓"
}

# Seed executions
seed_executions() {
    print_status "Seeding executions..."
    
    run_sql "
    INSERT INTO executions (id, workflow_id, workflow_version, status, input, output, started_at, completed_at, duration_ms, triggered_by)
    VALUES 
        (
            '950e8400-e29b-41d4-a716-446655440001',
            '850e8400-e29b-41d4-a716-446655440001',
            1,
            'completed',
            '{\"customer\": {\"email\": \"test@example.com\", \"name\": \"Test User\"}}',
            '{\"email_sent\": true, \"crm_updated\": true}',
            NOW() - INTERVAL '1 hour',
            NOW() - INTERVAL '59 minutes',
            60000,
            '550e8400-e29b-41d4-a716-446655440001'
        ),
        (
            '950e8400-e29b-41d4-a716-446655440002',
            '850e8400-e29b-41d4-a716-446655440001',
            1,
            'failed',
            '{\"customer\": {\"email\": \"invalid-email\", \"name\": \"Invalid User\"}}',
            NULL,
            NOW() - INTERVAL '2 hours',
            NOW() - INTERVAL '119 minutes',
            60000,
            '550e8400-e29b-41d4-a716-446655440001'
        ),
        (
            '950e8400-e29b-41d4-a716-446655440003',
            '850e8400-e29b-41d4-a716-446655440002',
            1,
            'running',
            '{\"source\": \"database_a\", \"target\": \"database_b\"}',
            NULL,
            NOW() - INTERVAL '30 minutes',
            NULL,
            NULL,
            '550e8400-e29b-41d4-a716-446655440002'
        )
    ON CONFLICT (id) DO NOTHING;"
    
    # Add execution steps
    run_sql "
    INSERT INTO execution_steps (execution_id, node_id, node_type, status, input, output, started_at, completed_at, duration_ms)
    VALUES 
        ('950e8400-e29b-41d4-a716-446655440001', 'trigger-1', 'trigger', 'completed', '{}', '{}', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '59 minutes 50 seconds', 10000),
        ('950e8400-e29b-41d4-a716-446655440001', 'action-1', 'action', 'completed', '{}', '{}', NOW() - INTERVAL '59 minutes 50 seconds', NOW() - INTERVAL '59 minutes 30 seconds', 20000),
        ('950e8400-e29b-41d4-a716-446655440001', 'action-2', 'action', 'completed', '{}', '{}', NOW() - INTERVAL '59 minutes 30 seconds', NOW() - INTERVAL '59 minutes', 30000)
    ON CONFLICT DO NOTHING;"
    
    print_status "Executions seeded ✓"
}

# Seed credentials
seed_credentials() {
    print_status "Seeding credentials..."
    
    # Note: In production, use proper encryption
    run_sql "
    INSERT INTO credentials (id, organization_id, name, type, encrypted_data, created_by)
    VALUES 
        ('a50e8400-e29b-41d4-a716-446655440001', '650e8400-e29b-41d4-a716-446655440001', 'SendGrid API', 'api_key', 'encrypted_sendgrid_key_here', '550e8400-e29b-41d4-a716-446655440001'),
        ('a50e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440001', 'Slack Webhook', 'webhook', 'encrypted_slack_webhook_here', '550e8400-e29b-41d4-a716-446655440001'),
        ('a50e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440002', 'Database Connection', 'database', 'encrypted_db_connection_here', '550e8400-e29b-41d4-a716-446655440002')
    ON CONFLICT (id) DO NOTHING;"
    
    print_status "Credentials seeded ✓"
}

# Seed API keys
seed_api_keys() {
    print_status "Seeding API keys..."
    
    # Note: In production, use proper hashing
    run_sql "
    INSERT INTO api_keys (organization_id, name, key_hash, scopes, created_by)
    VALUES 
        ('650e8400-e29b-41d4-a716-446655440001', 'Production API Key', 'hashed_key_1', '{\"workflows:read\", \"workflows:write\", \"executions:read\"}', '550e8400-e29b-41d4-a716-446655440001'),
        ('650e8400-e29b-41d4-a716-446655440001', 'Read-only API Key', 'hashed_key_2', '{\"workflows:read\", \"executions:read\"}', '550e8400-e29b-41d4-a716-446655440001'),
        ('650e8400-e29b-41d4-a716-446655440002', 'Development API Key', 'hashed_key_3', '{\"*\"}', '550e8400-e29b-41d4-a716-446655440002')
    ON CONFLICT DO NOTHING;"
    
    print_status "API keys seeded ✓"
}

# Seed webhooks
seed_webhooks() {
    print_status "Seeding webhooks..."
    
    run_sql "
    INSERT INTO webhooks (organization_id, url, events, secret, is_active, created_by)
    VALUES 
        ('650e8400-e29b-41d4-a716-446655440001', 'https://example.com/webhook/linkflow', '{\"workflow.created\", \"workflow.updated\", \"execution.completed\"}', 'webhook_secret_1', true, '550e8400-e29b-41d4-a716-446655440001'),
        ('650e8400-e29b-41d4-a716-446655440001', 'https://slack.com/webhook/notifications', '{\"execution.failed\"}', 'webhook_secret_2', true, '550e8400-e29b-41d4-a716-446655440001'),
        ('650e8400-e29b-41d4-a716-446655440002', 'https://startup.com/linkflow/events', '{\"*\"}', 'webhook_secret_3', true, '550e8400-e29b-41d4-a716-446655440002')
    ON CONFLICT DO NOTHING;"
    
    print_status "Webhooks seeded ✓"
}

# Print summary
print_summary() {
    print_status "Querying seeded data..."
    
    local user_count=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM users;")
    local org_count=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM organizations;")
    local workflow_count=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM workflows;")
    local execution_count=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM executions;")
    
    echo ""
    echo "================================"
    echo "Database Seeding Complete! 🌱"
    echo "================================"
    echo "Summary:"
    echo "  • Users:         $user_count"
    echo "  • Organizations: $org_count"
    echo "  • Workflows:     $workflow_count"
    echo "  • Executions:    $execution_count"
    echo ""
    echo "Test Accounts:"
    echo "  • admin@linkflow.local (Admin)"
    echo "  • john.doe@linkflow.local (User)"
    echo "  • jane.smith@linkflow.local (User)"
    echo ""
    echo "Note: All test passwords are 'password123'"
    echo ""
}

# Main execution
main() {
    check_db_connection
    
    # Ask user if they want to clear existing data
    read -p "Do you want to clear existing data before seeding? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        clear_data
    fi
    
    print_status "Starting database seeding..."
    
    seed_users
    seed_organizations
    seed_projects
    seed_workflows
    seed_executions
    seed_credentials
    seed_api_keys
    seed_webhooks
    
    print_summary
}

# Run main function
main "$@"
