# LinkFlow Go - Tools Guide

Quick reference for all development tools and commands.

## Quick Start

```bash
make help           # Show all available commands
make setup          # Complete development setup
make dev            # Start full dev environment (infra + migrations)
```

## Development Commands

| Command | Description |
|---------|-------------|
| `make deps` | Download and tidy Go dependencies |
| `make install-tools` | Install required dev tools (linter, mockgen, etc.) |
| `make run` | Start all services locally via docker-compose |
| `make stop` | Stop all local services |
| `make logs` | Show logs for local services |

## Build Commands

| Command | Description |
|---------|-------------|
| `make build` | Build all services to `bin/` directory |
| `make build-service SERVICE=auth` | Build specific service |
| `make clean` | Clean build artifacts and caches |
| `make build-all` | Clean and rebuild everything |

## Testing Commands

| Command | Description |
|---------|-------------|
| `make test` | Run all unit tests |
| `make test-unit` | Run unit tests with verbose output |
| `make test-integration` | Run integration tests |
| `make test-e2e` | Run end-to-end tests |
| `make test-coverage` | Generate HTML coverage report |
| `make test-race` | Run tests with race detector |
| `make test-short` | Run quick tests only |

## Code Quality

| Command | Description |
|---------|-------------|
| `make lint` | Run golangci-lint |
| `make fmt` | Format code (go fmt + gofumpt) |
| `make vet` | Run go vet |
| `make check` | Run all quality checks (fmt + vet + lint) |
| `make sec` | Run security scanner (gosec) |

## Database Migrations

| Command | Description |
|---------|-------------|
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-status` | Show migration status |
| `make migrate-version` | Show current migration version |
| `make migrate-create NAME=add_users` | Create new migration |
| `make migrate-goto V=10` | Migrate to specific version |
| `make migrate-force V=16` | Force migration version |
| `make migrate-reset` | Reset database (DESTRUCTIVE!) |

## Infrastructure (Docker Compose)

| Command | Description |
|---------|-------------|
| `make infra-up` | Start infrastructure (Postgres, Redis, Kafka, etc.) |
| `make infra-down` | Stop infrastructure |
| `make infra-status` | Check infrastructure status |
| `make infra-logs` | Show infrastructure logs |

## Docker

| Command | Description |
|---------|-------------|
| `make docker-build` | Build Docker images for all services |
| `make docker-build-service SERVICE=auth` | Build specific service image |
| `make docker-push` | Push images to registry |

## Kubernetes

| Command | Description |
|---------|-------------|
| `make k8s-deploy` | Deploy to Kubernetes |
| `make k8s-delete` | Delete from Kubernetes |
| `make k8s-status` | Check deployment status |
| `make k8s-logs SERVICE=auth` | Show logs for a service |

## Helm

| Command | Description |
|---------|-------------|
| `make helm-install` | Install Helm chart |
| `make helm-upgrade` | Upgrade Helm chart |
| `make helm-uninstall` | Uninstall Helm chart |
| `make helm-template` | Render templates locally |
| `make helm-lint` | Lint Helm chart |

## Monitoring & Observability

| Command | Description |
|---------|-------------|
| `make monitoring-deploy` | Deploy Prometheus + Grafana |
| `make logging-deploy` | Deploy Loki logging |
| `make tracing-deploy` | Deploy Jaeger tracing |

## Infrastructure Setup

| Command | Description |
|---------|-------------|
| `make kafka-setup` | Setup Kafka topics |
| `make istio-install` | Install Istio service mesh |
| `make istio-setup` | Configure Istio for LinkFlow |
| `make argocd-install` | Install ArgoCD |
| `make argocd-setup` | Configure ArgoCD |

## Code Generation

| Command | Description |
|---------|-------------|
| `make generate` | Generate all code (proto + mocks) |
| `make proto` | Generate protobuf files |
| `make mocks` | Generate mocks |

## Utility

| Command | Description |
|---------|-------------|
| `make version` | Show version info |
| `make info` | Show project info |
| `make all` | Run full CI pipeline locally |
| `make ci` | Run CI checks |

## Common Workflows

### First-Time Setup
```bash
make setup          # Install tools and dependencies
make dev            # Start infrastructure + run migrations
```

### Daily Development
```bash
make infra-up       # Start infrastructure
make migrate-up     # Apply any new migrations
make run            # Start services
make logs           # Watch logs
```

### Before Committing
```bash
make check          # Format + vet + lint
make test           # Run tests
```

### Adding a Migration
```bash
make migrate-create NAME=add_new_table
# Edit the generated migration files
make migrate-up     # Apply
```

### Building for Production
```bash
make clean
make build
make docker-build
```

## Environment Variables

Override database settings:
```bash
DB_HOST=myhost DB_PORT=5433 make migrate-up
```

Override Docker registry:
```bash
DOCKER_REGISTRY=myregistry.com make docker-build
```
