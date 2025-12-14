#!/bin/bash
set -e

# ============================================================================
# LinkFlow Database Migration Script
# ============================================================================

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Database connection from environment or defaults
DB_HOST=${LINKFLOW_DB_HOST:-localhost}
DB_PORT=${LINKFLOW_DB_PORT:-5432}
DB_NAME=${LINKFLOW_DB_NAME:-linkflow}
DB_USER=${LINKFLOW_DB_USER:-linkflow}
DB_PASSWORD=${LINKFLOW_DB_PASSWORD:-linkflow123}

DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
MIGRATIONS_PATH="migrations"

# Add Go bin to PATH
export PATH="$PATH:$(go env GOPATH 2>/dev/null)/bin:$HOME/go/bin:/usr/local/bin"

# ============================================================================
# Helper Functions
# ============================================================================

print_header() {
    echo ""
    echo -e "${BLUE}============================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}============================================${NC}"
    echo ""
}

check_migrate_tool() {
    if ! command -v migrate &> /dev/null; then
        echo -e "${YELLOW}migrate tool not found. Installing...${NC}"
        go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
        
        # Re-check after installation
        if ! command -v migrate &> /dev/null; then
            echo -e "${RED}Failed to install migrate tool.${NC}"
            echo ""
            echo "Please install manually:"
            echo "  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
            echo ""
            echo "Then add Go bin to your PATH:"
            echo "  export PATH=\"\$PATH:\$(go env GOPATH)/bin\""
            exit 1
        fi
        echo -e "${GREEN}migrate tool installed successfully!${NC}"
    fi
}

check_psql() {
    if ! command -v psql &> /dev/null; then
        echo -e "${RED}psql not found. Please install PostgreSQL client.${NC}"
        exit 1
    fi
}

check_db_connection() {
    echo -e "${YELLOW}Checking database connection...${NC}"
    if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT 1" &> /dev/null; then
        echo -e "${GREEN}Database connection OK${NC}"
    else
        echo -e "${RED}Cannot connect to database${NC}"
        echo "Host: $DB_HOST:$DB_PORT"
        echo "Database: $DB_NAME"
        echo "User: $DB_USER"
        exit 1
    fi
}

# ============================================================================
# Migration Commands
# ============================================================================

cmd_up() {
    print_header "Running Migrations UP"
    check_migrate_tool
    
    local steps=${1:-}
    if [ -n "$steps" ]; then
        echo -e "${GREEN}Applying $steps migration(s)...${NC}"
        migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" up $steps
    else
        echo -e "${GREEN}Applying all pending migrations...${NC}"
        migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" up
    fi
    echo -e "${GREEN}Migrations completed!${NC}"
}

cmd_down() {
    print_header "Running Migrations DOWN"
    check_migrate_tool
    
    local steps=${1:-1}
    echo -e "${YELLOW}Rolling back $steps migration(s)...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" down $steps
    echo -e "${GREEN}Rollback completed!${NC}"
}

cmd_version() {
    print_header "Migration Version"
    check_migrate_tool
    
    echo -e "${GREEN}Current migration version:${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" version
}

cmd_force() {
    local ver=$1
    if [ -z "$ver" ]; then
        echo -e "${RED}Please provide a version number${NC}"
        echo "Usage: $0 force <version>"
        exit 1
    fi
    
    print_header "Force Migration Version"
    check_migrate_tool
    
    echo -e "${YELLOW}Forcing migration version to $ver...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" force $ver
    echo -e "${GREEN}Version forced to $ver${NC}"
}

cmd_goto() {
    local ver=$1
    if [ -z "$ver" ]; then
        echo -e "${RED}Please provide a version number${NC}"
        echo "Usage: $0 goto <version>"
        exit 1
    fi
    
    print_header "Goto Migration Version"
    check_migrate_tool
    
    echo -e "${YELLOW}Migrating to version $ver...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" goto $ver
    echo -e "${GREEN}Migrated to version $ver${NC}"
}

cmd_create() {
    local name=$1
    if [ -z "$name" ]; then
        echo -e "${RED}Please provide a migration name${NC}"
        echo "Usage: $0 create <name>"
        exit 1
    fi
    
    print_header "Create New Migration"
    check_migrate_tool
    
    echo -e "${GREEN}Creating migration: $name${NC}"
    migrate create -ext sql -dir $MIGRATIONS_PATH -seq $name
    echo -e "${GREEN}Migration files created!${NC}"
    echo ""
    echo "Files created:"
    ls -la $MIGRATIONS_PATH | tail -2
}

cmd_status() {
    print_header "Migration Status"
    check_migrate_tool
    
    echo -e "${BLUE}Database:${NC} $DB_HOST:$DB_PORT/$DB_NAME"
    echo -e "${BLUE}Migrations Path:${NC} $MIGRATIONS_PATH"
    echo ""
    
    echo -e "${BLUE}Current Version:${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" version 2>&1 || true
    
    echo ""
    echo -e "${BLUE}Available Migrations:${NC}"
    ls -1 $MIGRATIONS_PATH/*.up.sql 2>/dev/null | wc -l | xargs echo "Total UP migrations:"
    ls -1 $MIGRATIONS_PATH/*.down.sql 2>/dev/null | wc -l | xargs echo "Total DOWN migrations:"
}

cmd_reset() {
    print_header "Reset Database"
    check_psql
    check_migrate_tool
    
    echo -e "${RED}WARNING: This will DROP ALL SCHEMAS and re-run all migrations!${NC}"
    echo -e "${RED}All data will be permanently deleted!${NC}"
    echo ""
    read -p "Type 'yes' to confirm: " confirm
    
    if [ "$confirm" != "yes" ]; then
        echo -e "${YELLOW}Reset cancelled.${NC}"
        exit 0
    fi
    
    echo ""
    echo -e "${YELLOW}Dropping all schemas...${NC}"
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME <<EOF
DROP SCHEMA IF EXISTS auth CASCADE;
DROP SCHEMA IF EXISTS workflow CASCADE;
DROP SCHEMA IF EXISTS execution CASCADE;
DROP SCHEMA IF EXISTS node CASCADE;
DROP SCHEMA IF EXISTS schedule CASCADE;
DROP SCHEMA IF EXISTS credential CASCADE;
DROP SCHEMA IF EXISTS webhook CASCADE;
DROP SCHEMA IF EXISTS variable CASCADE;
DROP SCHEMA IF EXISTS notification CASCADE;
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP SCHEMA IF EXISTS search CASCADE;
DROP SCHEMA IF EXISTS storage CASCADE;
DROP SCHEMA IF EXISTS billing CASCADE;
DROP SCHEMA IF EXISTS template CASCADE;
DROP TABLE IF EXISTS schema_migrations CASCADE;
EOF

    echo -e "${GREEN}Schemas dropped.${NC}"
    echo ""
    echo -e "${GREEN}Running all migrations...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" up
    echo ""
    echo -e "${GREEN}Database reset completed!${NC}"
}

cmd_drop() {
    print_header "Drop All Schemas"
    check_psql
    
    echo -e "${RED}WARNING: This will DROP ALL SCHEMAS!${NC}"
    echo -e "${RED}All data will be permanently deleted!${NC}"
    echo ""
    read -p "Type 'yes' to confirm: " confirm
    
    if [ "$confirm" != "yes" ]; then
        echo -e "${YELLOW}Drop cancelled.${NC}"
        exit 0
    fi
    
    echo ""
    echo -e "${YELLOW}Dropping all schemas...${NC}"
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME <<EOF
DROP SCHEMA IF EXISTS auth CASCADE;
DROP SCHEMA IF EXISTS workflow CASCADE;
DROP SCHEMA IF EXISTS execution CASCADE;
DROP SCHEMA IF EXISTS node CASCADE;
DROP SCHEMA IF EXISTS schedule CASCADE;
DROP SCHEMA IF EXISTS credential CASCADE;
DROP SCHEMA IF EXISTS webhook CASCADE;
DROP SCHEMA IF EXISTS variable CASCADE;
DROP SCHEMA IF EXISTS notification CASCADE;
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP SCHEMA IF EXISTS search CASCADE;
DROP SCHEMA IF EXISTS storage CASCADE;
DROP SCHEMA IF EXISTS billing CASCADE;
DROP SCHEMA IF EXISTS template CASCADE;
DROP TABLE IF EXISTS schema_migrations CASCADE;
EOF

    echo -e "${GREEN}All schemas dropped!${NC}"
}

cmd_help() {
    echo ""
    echo -e "${BLUE}LinkFlow Database Migration Tool${NC}"
    echo ""
    echo "Usage: $0 <command> [arguments]"
    echo ""
    echo "Commands:"
    echo "  up [N]        Apply all or N pending migrations"
    echo "  down [N]      Rollback N migrations (default: 1)"
    echo "  goto V        Migrate to version V"
    echo "  force V       Force set version V (fix dirty state)"
    echo "  version       Show current migration version"
    echo "  status        Show migration status and info"
    echo "  create NAME   Create new migration files"
    echo "  reset         Drop all schemas and re-run migrations"
    echo "  drop          Drop all schemas (no re-migrate)"
    echo "  help          Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  LINKFLOW_DB_HOST      Database host (default: localhost)"
    echo "  LINKFLOW_DB_PORT      Database port (default: 5432)"
    echo "  LINKFLOW_DB_NAME      Database name (default: linkflow)"
    echo "  LINKFLOW_DB_USER      Database user (default: linkflow)"
    echo "  LINKFLOW_DB_PASSWORD  Database password (default: linkflow123)"
    echo ""
    echo "Examples:"
    echo "  $0 up              # Apply all pending migrations"
    echo "  $0 up 1            # Apply next 1 migration"
    echo "  $0 down 2          # Rollback 2 migrations"
    echo "  $0 force 16        # Force version to 16 (fix dirty state)"
    echo "  $0 create users    # Create new migration files"
    echo ""
}

# ============================================================================
# Main
# ============================================================================

case "${1:-help}" in
    up)
        cmd_up $2
        ;;
    down)
        cmd_down $2
        ;;
    goto)
        cmd_goto $2
        ;;
    force)
        cmd_force $2
        ;;
    version)
        cmd_version
        ;;
    status)
        cmd_status
        ;;
    create)
        cmd_create $2
        ;;
    reset)
        cmd_reset
        ;;
    drop)
        cmd_drop
        ;;
    help|--help|-h)
        cmd_help
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        cmd_help
        exit 1
        ;;
esac
