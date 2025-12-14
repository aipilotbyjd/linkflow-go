# Database Migrations

This directory contains all database migrations for LinkFlow.

## Overview

- **Tool**: [golang-migrate](https://github.com/golang-migrate/migrate)
- **Database**: PostgreSQL
- **Naming**: `{version}_{description}.{up|down}.sql`

## Quick Start

```bash
# Apply all pending migrations
make migrate-up

# Check current version
make migrate-status

# Rollback last migration
make migrate-down
```

## Make Commands

| Command | Description |
|---------|-------------|
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-status` | Show migration status |
| `make migrate-version` | Show current version |
| `make migrate-force V=N` | Force version to N (fix dirty state) |
| `make migrate-goto V=N` | Migrate to version N |
| `make migrate-create NAME=x` | Create new migration |
| `make migrate-reset` | Drop all & re-migrate (DANGER!) |
| `make migrate-drop` | Drop all schemas (DANGER!) |

## Script Commands

You can also use the script directly:

```bash
./scripts/db/migrate.sh help        # Show all commands
./scripts/db/migrate.sh up          # Apply all migrations
./scripts/db/migrate.sh up 1        # Apply next 1 migration
./scripts/db/migrate.sh down 2      # Rollback 2 migrations
./scripts/db/migrate.sh version     # Show current version
./scripts/db/migrate.sh status      # Show detailed status
./scripts/db/migrate.sh force 16    # Force version to 16
./scripts/db/migrate.sh goto 10     # Migrate to version 10
./scripts/db/migrate.sh create xxx  # Create new migration
./scripts/db/migrate.sh reset       # Drop all & re-migrate
./scripts/db/migrate.sh drop        # Drop all schemas
```

## Migration Structure

```
migrations/
├── 000001_extensions_schemas.up.sql      # Extensions & schemas
├── 000001_extensions_schemas.down.sql
├── 000002_auth_tables.up.sql             # Auth tables
├── 000002_auth_tables.down.sql
├── 000003_workflow_tables.up.sql         # Workflow tables
├── ...
├── 000017_performance_indexes.up.sql     # Performance indexes
├── 000017_performance_indexes.down.sql
├── 000018_seed_data.up.sql               # Seed data
├── 000018_seed_data.down.sql
└── README.md
```

## Schemas

Each domain has its own PostgreSQL schema:

| Schema | Description |
|--------|-------------|
| `auth` | Users, roles, sessions, API keys |
| `workflow` | Workflows, nodes, edges |
| `execution` | Workflow executions |
| `node` | Node definitions, marketplace |
| `schedule` | Scheduled triggers |
| `credential` | Encrypted credentials |
| `webhook` | Webhook endpoints |
| `variable` | Environment variables |
| `notification` | Notifications, templates |
| `audit` | Audit logs |
| `analytics` | Events, metrics |
| `search` | Search indexes |
| `storage` | File storage |
| `billing` | Subscriptions, invoices |
| `template` | Workflow templates |

## Creating New Migrations

```bash
# Create migration files
make migrate-create NAME=add_user_preferences

# This creates:
# migrations/000019_add_user_preferences.up.sql
# migrations/000019_add_user_preferences.down.sql
```

### Migration File Guidelines

**DO:**
- Use `IF NOT EXISTS` / `IF EXISTS` for safety
- Include comments describing the changes
- Make down migrations reversible
- Test both up and down migrations

**DON'T:**
- Use `BEGIN;` / `COMMIT;` (migrate handles transactions)
- Use `CONCURRENTLY` (can't run in transactions)
- Make destructive changes without backup plan

### Example Migration

**000019_add_user_preferences.up.sql:**
```sql
-- Add user preferences table
CREATE TABLE IF NOT EXISTS auth.user_preferences (
    user_id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    theme VARCHAR(20) DEFAULT 'light',
    language VARCHAR(10) DEFAULT 'en',
    timezone VARCHAR(50) DEFAULT 'UTC',
    notifications JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_preferences_user 
    ON auth.user_preferences(user_id);
```

**000019_add_user_preferences.down.sql:**
```sql
DROP TABLE IF EXISTS auth.user_preferences;
```

## Troubleshooting

### Dirty Database State

If migrations fail mid-way, the database may be in a "dirty" state:

```bash
# Check current state
make migrate-status

# Force version to last successful migration
make migrate-force V=16

# Re-run migrations
make migrate-up
```

### Reset Everything

To completely reset the database (development only!):

```bash
make migrate-reset
```

### Connection Issues

Set environment variables:

```bash
export LINKFLOW_DB_HOST=localhost
export LINKFLOW_DB_PORT=5432
export LINKFLOW_DB_NAME=linkflow
export LINKFLOW_DB_USER=linkflow
export LINKFLOW_DB_PASSWORD=linkflow123
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LINKFLOW_DB_HOST` | localhost | Database host |
| `LINKFLOW_DB_PORT` | 5432 | Database port |
| `LINKFLOW_DB_NAME` | linkflow | Database name |
| `LINKFLOW_DB_USER` | linkflow | Database user |
| `LINKFLOW_DB_PASSWORD` | linkflow123 | Database password |
