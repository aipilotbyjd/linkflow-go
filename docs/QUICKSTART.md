# LinkFlow Go - Quick Start Guide

## Prerequisites

- Go 1.20+
- Docker & Docker Compose
- Make

## Quick Start Commands

```bash
# First time setup
make setup              # Install dependencies and dev tools

# Start everything locally with Docker
make run-local          # Start all services with docker-compose

# Or start just infrastructure first
make infra-up           # Start postgres, redis, kafka, etc.
make db-migrate         # Run database migrations
make kafka-setup        # Create Kafka topics
```

## Common Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make build` | Build all services |
| `make run-local` | Start all services with Docker |
| `make stop-local` | Stop all services |
| `make logs` | View service logs |
| `make test` | Run unit tests |
| `make lint` | Run code linter |

## Development Workflow

```bash
# 1. Start infrastructure (postgres, redis, kafka)
make infra-up

# 2. Run migrations
make migrate-up

# 3. Check migration status
make migrate-status

# 4. Seed test data (optional)
make db-seed

# 5. Build services
make build

# 6. Start all services
make run-local

# 7. Check status
make infra-status

# 8. View logs
make logs
```

## Database Commands

```bash
make migrate-up         # Apply all migrations
make migrate-down       # Rollback 1 migration
make migrate-status     # Show migration status
make migrate-reset      # Drop all and re-migrate (DANGER!)
make db-migrate         # Run migrations via script
make db-seed            # Add test data
```

## Docker Commands

```bash
make docker-build       # Build all Docker images
make docker-push        # Push images to registry
```

## Kubernetes Deployment

```bash
make helm-install       # Deploy with Helm
make helm-upgrade       # Update deployment
make k8s-deploy         # Deploy with kubectl
```

## Service Ports

| Service | Port |
|---------|------|
| Kong Gateway | 8000 |
| Auth | 8001 |
| User | 8002 |
| Workflow | 8003 |
| Execution | 8004 |
| Webhook | 8007 |
| Schedule | 8008 |
| Variable | 8009 |
| Credential | 8010 |
| Node | 8011 |
| Executor | 8012 |
| Notification | 8013 |
| Analytics | 8014 |
| WebSocket | 8020 |

## Monitoring & Observability

| Service | Port | URL |
|---------|------|-----|
| Grafana | 3000 | http://localhost:3000 |
| Prometheus | 9090 | http://localhost:9090 |
| Jaeger UI | 16686 | http://localhost:16686 |
| Elasticsearch | 9200 | http://localhost:9200 |

## All Make Commands

```bash
# Development
make deps               # Download dependencies
make build              # Build all services
make build-service SERVICE=auth  # Build specific service
make test               # Run unit tests
make test-coverage      # Run tests with coverage
make lint               # Run linter
make fmt                # Format code
make vet                # Run go vet
make clean              # Clean build artifacts

# Infrastructure
make infra-up           # Start infrastructure (postgres, redis, kafka, etc.)
make infra-down         # Stop infrastructure
make infra-status       # Check infrastructure status
make infra-logs         # Show infrastructure logs

# Database
make migrate-up         # Run all migrations up
make migrate-down       # Rollback 1 migration
make migrate-status     # Show migration status
make migrate-reset      # Reset database (drop all + migrate)
make db-migrate         # Run migrations via script
make db-seed            # Seed database with test data

# Docker
make docker-build       # Build Docker images
make docker-push        # Push Docker images
make run-local          # Start all services locally
make stop-local         # Stop local services
make logs               # Show service logs

# Kubernetes
make k8s-deploy         # Deploy to Kubernetes
make k8s-delete         # Delete from Kubernetes
make k8s-status         # Check deployment status

# Helm
make helm-install       # Install Helm chart
make helm-upgrade       # Upgrade Helm chart
make helm-uninstall     # Uninstall Helm chart

# Setup
make setup              # Full development setup
make install-tools      # Install dev tools only
make dev-setup          # Run dev setup script
make kafka-setup        # Setup Kafka topics

# Service Mesh & GitOps
make istio-install      # Install Istio
make istio-setup        # Configure Istio
make argocd-install     # Install ArgoCD
make argocd-setup       # Configure ArgoCD
make logging-deploy     # Deploy logging stack
make tracing-deploy     # Deploy Jaeger tracing
```

## Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Key variables:
- `LINKFLOW_DATABASE_HOST` - PostgreSQL host
- `LINKFLOW_DATABASE_PORT` - PostgreSQL port
- `LINKFLOW_DATABASE_NAME` - Database name
- `LINKFLOW_DATABASE_USER` - Database user
- `LINKFLOW_DATABASE_PASSWORD` - Database password
- `LINKFLOW_REDIS_HOST` - Redis host
- `LINKFLOW_KAFKA_BROKERS` - Kafka broker addresses

## Troubleshooting

```bash
# Check if services are running
docker-compose ps

# View logs for specific service
docker-compose logs -f auth-service

# Restart a service
docker-compose restart workflow-service

# Rebuild and restart
docker-compose up -d --build workflow-service

# Check database connection
docker-compose exec postgres psql -U linkflow -d linkflow

# Check Kafka topics
docker-compose exec kafka kafka-topics --list --bootstrap-server localhost:9092
```

## API Access

Once running, access the API through Kong Gateway:

- Base URL: `http://localhost:8000/api/v1`
- Auth: `http://localhost:8000/api/v1/auth`
- Workflows: `http://localhost:8000/api/v1/workflows`
- Executions: `http://localhost:8000/api/v1/executions`

Run `make help` anytime to see all available commands!
