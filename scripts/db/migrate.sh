#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'

# Database connection
DB_HOST=${LINKFLOW_DB_HOST:-localhost}
DB_PORT=${LINKFLOW_DB_PORT:-5432}
DB_NAME=${LINKFLOW_DB_NAME:-linkflow}
DB_USER=${LINKFLOW_DB_USER:-linkflow}
DB_PASSWORD=${LINKFLOW_DB_PASSWORD:-linkflow123}

DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
MIGRATIONS_PATH="migrations"

# Check if migrate tool is installed
if ! command -v migrate &> /dev/null; then
    echo -e "${YELLOW}migrate tool not found. Installing...${NC}"
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
fi

up() {
    echo -e "${GREEN}Running migrations up...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" up
    echo -e "${GREEN}Migrations completed!${NC}"
}

down() {
    local steps=${1:-1}
    echo -e "${YELLOW}Rolling back $steps migration(s)...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" down $steps
    echo -e "${GREEN}Rollback completed!${NC}"
}

version() {
    echo -e "${GREEN}Current migration version:${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" version
}

force() {
    local ver=$1
    echo -e "${YELLOW}Forcing migration version to $ver...${NC}"
    migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" force $ver
    echo -e "${GREEN}Version forced!${NC}"
}

create() {
    local name=$1
    if [ -z "$name" ]; then
        echo -e "${RED}Please provide a migration name${NC}"
        exit 1
    fi
    echo -e "${GREEN}Creating migration: $name${NC}"
    migrate create -ext sql -dir $MIGRATIONS_PATH -seq $name
    echo -e "${GREEN}Migration files created!${NC}"
}

reset() {
    echo -e "${RED}WARNING: This will drop all tables and re-run migrations!${NC}"
    read -p "Are you sure? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Dropping all tables...${NC}"
        migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" drop -f
        echo -e "${GREEN}Running migrations...${NC}"
        migrate -path $MIGRATIONS_PATH -database "$DATABASE_URL" up
        echo -e "${GREEN}Reset completed!${NC}"
    fi
}

status() {
    echo -e "${GREEN}Migration status:${NC}"
    echo "Database: $DB_HOST:$DB_PORT/$DB_NAME"
    echo "Migrations path: $MIGRATIONS_PATH"
    echo ""
    version
}

case "${1:-up}" in
    up)
        up
        ;;
    down)
        down $2
        ;;
    version)
        version
        ;;
    force)
        force $2
        ;;
    create)
        create $2
        ;;
    reset)
        reset
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {up|down [steps]|version|force <version>|create <name>|reset|status}"
        exit 1
        ;;
esac
