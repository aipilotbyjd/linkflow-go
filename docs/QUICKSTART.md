# LinkFlow Go - Quick Start Guide

Get up and running with LinkFlow in minutes.

## Prerequisites

- **Go 1.20+** - [Download](https://golang.org/dl/)
- **Docker & Docker Compose** - [Download](https://docs.docker.com/get-docker/)
- **Make** - Usually pre-installed on macOS/Linux

## 30-Second Quick Start

```bash
# One command to rule them all
make dev
```

This single command will:
1. Start all infrastructure (PostgreSQL, Redis, Kafka, etc.)
2. Run all database migrations
3. Set up everything you need to start developing

## First Time Setup

```bash
# Install dependencies and development tools
make setup

# Start full development environment
make dev

# Check everything is running
make infra-status

# View project info
make info
```

## Common Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make dev` | Start full dev environment (infra + migrations) |
| `make build` | Build all services |
| `make run` | Start all services with Docker |
| `make stop` | Stop all services |
| `make logs` | View service logs |
| `make test` | Run unit tests |
| `make check` | Run all code quality checks (fmt, vet, lint) |
| `make version` | Show version information |
| `make info` | Show project information |

## Development Workflow

### Option 1: Quick Start (Recommended)
```bash
# Start everything with one command
make dev

# Build services
make build

# Run services
make run

# View logs
make logs
```

### Option 2: Step by Step
```bash
# 1. Start infrastructure (postgres, redis, kafka)
make infra-up

# 2. Run database migrations
make migrate-up

# 3. Check migration status
make migrate-status

# 4. Seed test data (optional)
make db-seed

# 5. Build services
make build

# 6. Start all services
make run

# 7. Check status
make infra-status

# 8. View logs
make logs
```

## Database Commands

```bash
# Migrations
make migrate-up                   # Apply all pending migrations
make migrate-down                 # Rollback last migration
make migrate-status               # Show migration status
make migrate-version              # Show current version
make migrate-create NAME=xxx      # Create new migration files
make migrate-goto V=10            # Migrate to specific version
make migrate-force V=16           # Force version (fix dirty state)
make migrate-reset                # Drop all & re-migrate (DANGER!)
make migrate-drop                 # Drop all schemas (DANGER!)

# Data
make db-seed                      # Seed database with test data
```

## Build Commands

```bash
make build                        # Build all services
make build-service SERVICE=auth   # Build specific service
make build-all                    # Clean and rebuild all
make clean                        # Clean build artifacts
```

## Testing Commands

```bash
make test                         # Run unit tests
make test-unit                    # Run unit tests
make test-integration             # Run integration tests
make test-e2e                     # Run end-to-end tests
make test-coverage                # Run tests with coverage report
make test-race                    # Run tests with race detector
make test-short                   # Run quick tests only
```

## Code Quality

```bash
make check                        # Run all checks (fmt, vet, lint)
make fmt                          # Format code
make vet                          # Run go vet
make lint                         # Run golangci-lint
make sec                          # Run security scanner (gosec)
```

## Docker Commands

```bash
make docker-build                 # Build all Docker images
make docker-build-service SERVICE=auth  # Build specific image
make docker-push                  # Push images to registry
```

## Kubernetes & Helm

```bash
# Helm
make helm-install                 # Deploy with Helm
make helm-upgrade                 # Update deployment
make helm-uninstall               # Remove deployment
make helm-template                # Render templates locally
make helm-lint                    # Lint Helm chart

# Kubernetes
make k8s-deploy                   # Deploy with kubectl
make k8s-delete                   # Delete deployment
make k8s-status                   # Check deployment status
make k8s-logs                     # Show all logs
make k8s-logs SERVICE=auth        # Show specific service logs
```

## Service Ports

| Service | Port | Description |
|---------|------|-------------|
| Kong Gateway | 8000 | API Gateway |
| Auth | 8001 | Authentication service |
| User | 8002 | User management |
| Workflow | 8003 | Workflow CRUD |
| Execution | 8004 | Execution management |
| Webhook | 8007 | Webhook handling |
| Schedule | 8008 | Cron scheduling |
| Variable | 8009 | Variable storage |
| Credential | 8010 | Credential management |
| Node | 8011 | Node registry |
| Executor | 8012 | Workflow executor |
| Notification | 8013 | Notifications |
| Analytics | 8014 | Analytics & metrics |
| WebSocket | 8020 | Real-time updates |

## Monitoring & Observability

| Service | Port | URL |
|---------|------|-----|
| Grafana | 3000 | http://localhost:3000 |
| Prometheus | 9090 | http://localhost:9090 |
| Jaeger UI | 16686 | http://localhost:16686 |
| Elasticsearch | 9200 | http://localhost:9200 |
| Kibana | 5601 | http://localhost:5601 |

```bash
# Deploy monitoring stack
make monitoring-deploy            # Prometheus + Grafana
make logging-deploy               # Loki logging
make tracing-deploy               # Jaeger tracing
```

## Infrastructure Setup

```bash
# Service Mesh
make istio-install                # Install Istio
make istio-setup                  # Configure Istio

# GitOps
make argocd-install               # Install ArgoCD
make argocd-setup                 # Configure ArgoCD

# Messaging
make kafka-setup                  # Setup Kafka topics
```

## Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Key variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `LINKFLOW_DB_HOST` | PostgreSQL host | localhost |
| `LINKFLOW_DB_PORT` | PostgreSQL port | 5432 |
| `LINKFLOW_DB_NAME` | Database name | linkflow |
| `LINKFLOW_DB_USER` | Database user | linkflow |
| `LINKFLOW_DB_PASSWORD` | Database password | linkflow123 |
| `LINKFLOW_REDIS_HOST` | Redis host | localhost |
| `LINKFLOW_KAFKA_BROKERS` | Kafka broker addresses | localhost:9092 |

## Troubleshooting

### Check Service Status
```bash
# Check all services
make infra-status

# Or use docker-compose directly
docker-compose ps
```

### View Logs
```bash
# All services
make logs

# Specific service
docker-compose logs -f auth-service

# Kubernetes logs
make k8s-logs SERVICE=auth
```

### Database Issues
```bash
# Check migration status
make migrate-status

# Force fix dirty migration state
make migrate-force V=18

# Connect to database directly
docker-compose exec postgres psql -U linkflow -d linkflow

# Reset database (DANGER - destroys all data!)
make migrate-reset
```

### Restart Services
```bash
# Restart all
make stop && make run

# Restart specific service
docker-compose restart workflow-service

# Rebuild and restart
docker-compose up -d --build workflow-service
```

### Kafka Issues
```bash
# List topics
docker-compose exec kafka kafka-topics --list --bootstrap-server localhost:9092

# Re-setup topics
make kafka-setup
```

## API Access

Once running, access the API through Kong Gateway:

| Endpoint | URL |
|----------|-----|
| Base URL | http://localhost:8000/api/v1 |
| Auth | http://localhost:8000/api/v1/auth |
| Workflows | http://localhost:8000/api/v1/workflows |
| Executions | http://localhost:8000/api/v1/executions |
| Webhooks | http://localhost:8000/api/v1/webhooks |

### Default Admin Credentials

After running `make migrate-up`, a default admin user is created:

- **Email:** admin@linkflow.local
- **Password:** changeme123

> **Warning:** Change this password immediately in production!

## CI/CD

```bash
# Run full CI pipeline locally
make all                          # clean, deps, check, test, build

# Run CI checks only
make ci                           # deps, check, test-coverage, build

# Show version info
make version
```

## Need Help?

```bash
# Show all available commands
make help

# Show project info
make info

# Show version
make version
```

---

**Happy coding!** Run `make help` anytime to see all available commands.
