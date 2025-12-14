# LinkFlow Database Migrations

## Overview
This directory contains all database migrations for the LinkFlow platform.

## Naming Convention
```
{version}_{description}.{up|down}.sql
```

- **version**: Sequential 6-digit number (e.g., `000001`)
- **description**: Snake_case description of the migration
- **up.sql**: Forward migration (apply changes)
- **down.sql**: Rollback migration (revert changes)

## Migration Structure

| Version | Description | Schema | Tables |
|---------|-------------|--------|--------|
| 000001 | Extensions and schemas | - | Creates 16 schemas + utility functions |
| 000002 | Auth tables | auth | users, roles, user_roles, sessions, api_keys, oauth_providers |
| 000003 | Workflow tables | workflow | workflows, workflow_versions, folders, tags, workflow_tags, shares |
| 000004 | Execution tables | execution | executions, node_executions, execution_queue, execution_data |
| 000005 | Node registry | node | categories, nodes, node_versions, custom_nodes |
| 000006 | Schedule tables | schedule | schedules, schedule_history |
| 000007 | Credential tables | credential | credentials, credential_types, oauth_tokens |
| 000008 | Webhook tables | webhook | webhooks, webhook_logs |
| 000009 | Variable tables | variable | variables, variable_history |
| 000010 | Notification tables | notification | notifications, templates, channels, notification_queue |
| 000011 | Audit tables | audit | audit_logs, audit_retention_policies |
| 000012 | Analytics tables | analytics | metrics, events, dashboards, reports |
| 000013 | Search tables | search | documents, search_history |
| 000014 | Storage tables | storage | buckets, files, file_shares, file_access_logs |
| 000015 | Billing tables | billing | plans, subscriptions, invoices, payments, usage_records |
| 000016 | Template tables | template | templates, template_categories, template_reviews |
| 000017 | Performance indexes | - | 60+ indexes across all schemas |
| 000018 | Seed data | - | Default roles, plans, nodes, templates |

## Running Migrations

### Using golang-migrate
```bash
# Apply all migrations
migrate -path deployments/migrations -database "postgres://user:pass@localhost:5432/linkflow?sslmode=disable" up

# Rollback last migration
migrate -path deployments/migrations -database "postgres://user:pass@localhost:5432/linkflow?sslmode=disable" down 1

# Go to specific version
migrate -path deployments/migrations -database "postgres://user:pass@localhost:5432/linkflow?sslmode=disable" goto 5
```

### Using Make
```bash
make migrate-up
make migrate-down
make migrate-status
```

## Best Practices
1. Always create both up and down migrations
2. Test rollback before deploying
3. Never modify existing migrations in production
4. Use transactions where possible
5. Add comments for complex operations


## Schema Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           LinkFlow Database Schemas                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │   auth   │  │ workflow │  │execution │  │   node   │  │ schedule │      │
│  │          │  │          │  │          │  │          │  │          │      │
│  │ users    │  │workflows │  │executions│  │categories│  │schedules │      │
│  │ roles    │  │versions  │  │node_exec │  │ nodes    │  │ history  │      │
│  │ sessions │  │ folders  │  │ queue    │  │ versions │  │          │      │
│  │ api_keys │  │  tags    │  │  data    │  │ custom   │  │          │      │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘      │
│                                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │credential│  │ webhook  │  │ variable │  │ notif.   │  │  audit   │      │
│  │          │  │          │  │          │  │          │  │          │      │
│  │credentials│ │ webhooks │  │variables │  │notifs    │  │audit_logs│      │
│  │  types   │  │  logs    │  │ history  │  │templates │  │retention │      │
│  │  oauth   │  │          │  │          │  │ queue    │  │          │      │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘      │
│                                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                     │
│  │analytics │  │  search  │  │ storage  │  │ billing  │  ┌──────────┐      │
│  │          │  │          │  │          │  │          │  │ template │      │
│  │ metrics  │  │documents │  │ buckets  │  │  plans   │  │          │      │
│  │ events   │  │ history  │  │  files   │  │  subs    │  │templates │      │
│  │dashboards│  │          │  │ shares   │  │ invoices │  │categories│      │
│  │ reports  │  │          │  │  logs    │  │ payments │  │ reviews  │      │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘      │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Seed Data Contents

The `000018_seed_data` migration includes:

### Default Roles
- `admin` - Full system access
- `user` - Standard user permissions
- `viewer` - Read-only access
- `operator` - Execute workflows only

### Billing Plans
- `free` - 5 workflows, 1K executions/month
- `starter` - 50 workflows, 10K executions/month ($29/mo)
- `professional` - 200 workflows, 50K executions/month ($99/mo)
- `enterprise` - Unlimited ($499/mo)

### Built-in Nodes
- Triggers: Manual, Webhook, Schedule
- Flow: If, Switch, Loop
- Data: Set, Function, Merge
- Actions: HTTP Request, Send Email
- Utilities: Wait, No Operation

### Notification Templates
- Welcome email
- Execution failed
- Execution completed
- Password reset

### Storage Buckets
- workflows, attachments, templates, avatars

## Rollback Strategy

```bash
# Rollback single migration
migrate -path deployments/migrations -database "$DB_URL" down 1

# Rollback to specific version
migrate -path deployments/migrations -database "$DB_URL" goto 10

# Full rollback (DANGER: destroys all data)
migrate -path deployments/migrations -database "$DB_URL" down
```

## Adding New Migrations

```bash
# Create new migration files
touch deployments/migrations/000019_new_feature.up.sql
touch deployments/migrations/000019_new_feature.down.sql

# Or use migrate CLI
migrate create -ext sql -dir deployments/migrations -seq new_feature
```
