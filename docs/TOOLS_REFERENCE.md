# LinkFlow Go - Complete Tools Reference

A comprehensive guide to all development, monitoring, and infrastructure tools.

---

## Table of Contents

1. [Monitoring & Observability](#monitoring--observability)
   - [Grafana](#grafana)
   - [Prometheus](#prometheus)
   - [Jaeger](#jaeger)
   - [Elasticsearch](#elasticsearch)
2. [Infrastructure](#infrastructure)
   - [Kong API Gateway](#kong-api-gateway)
   - [Kafka](#kafka)
   - [Redis](#redis)
   - [PostgreSQL](#postgresql)
   - [Zookeeper](#zookeeper)
3. [Development Tools](#development-tools)
4. [Service Ports Reference](#service-ports-reference)
5. [Make Commands Reference](#make-commands-reference)

---

## Monitoring & Observability

### Grafana

**Purpose**: Metrics visualization, dashboards, and alerting.

| Property | Value |
|----------|-------|
| URL | http://localhost:3000 |
| Username | admin |
| Password | admin |
| Image | grafana/grafana:10.2.0 |

**Features**:
- Pre-configured dashboards for all services
- Redis plugin installed
- Anonymous access enabled (dev mode)
- Auto-provisioned data sources

**Usage**:
```bash
# Start Grafana
docker-compose up -d grafana

# Access dashboard
open http://localhost:3000

# View logs
docker-compose logs -f grafana
```

**Pre-configured Dashboards**:
- LinkFlow Overview - System-wide metrics
- Service Health - Per-service metrics
- Kafka Metrics - Event streaming stats
- Database Performance - PostgreSQL metrics
- Redis Metrics - Cache performance

**Key Metrics to Monitor**:
```promql
# Request rate by service
sum(rate(http_requests_total{namespace="linkflow"}[5m])) by (service)

# Error rate percentage
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100

# P99 latency
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))

# Active workflow executions
sum(workflow_executions_active)
```

---

### Prometheus

**Purpose**: Metrics collection, storage, and alerting rules.

| Property | Value |
|----------|-------|
| URL | http://localhost:9090 |
| Config | deployments/monitoring/prometheus/prometheus.yml |
| Alerts | deployments/monitoring/prometheus/rules/alerts.yml |
| Image | prom/prometheus:v2.48.0 |

**Features**:
- Scrapes all LinkFlow services
- Pre-configured alert rules
- 15-day retention (configurable)
- Web UI for queries

**Usage**:
```bash
# Start Prometheus
docker-compose up -d prometheus

# Access UI
open http://localhost:9090

# Check targets
open http://localhost:9090/targets

# Reload config (dev mode)
curl -X POST http://localhost:9090/-/reload
```

**Useful PromQL Queries**:
```promql
# Service request rate
rate(http_requests_total[5m])

# Memory usage by service
container_memory_usage_bytes{container=~".*-service"}

# CPU usage by service
rate(container_cpu_usage_seconds_total[5m])

# Kafka consumer lag
kafka_consumer_group_lag{group="linkflow-group"}

# Database connections
pg_stat_activity_count

# Redis memory
redis_memory_used_bytes
```

**Pre-configured Alerts**:
| Alert | Condition | Severity |
|-------|-----------|----------|
| HighErrorRate | >5% errors for 5m | Critical |
| ServiceDown | 0 pods for 2m | Critical |
| HighLatency | P99 >2s for 10m | Warning |
| KafkaLag | >1000 messages | Warning |
| PodCrashLooping | >3 restarts in 10m | Warning |
| DiskPressure | >85% used | Warning |
| HighMemoryUsage | >80% for 10m | Warning |
| HighCPUUsage | >70% for 15m | Warning |

---

### Jaeger

**Purpose**: Distributed tracing across microservices.

| Property | Value |
|----------|-------|
| UI URL | http://localhost:16686 |
| Collector HTTP | http://localhost:14268 |
| OTLP gRPC | localhost:4317 |
| OTLP HTTP | localhost:4318 |
| Image | jaegertracing/all-in-one:1.52 |

**Features**:
- OpenTelemetry Protocol (OTLP) support
- In-memory storage (dev mode)
- Service dependency graph
- Trace comparison

**Usage**:
```bash
# Start Jaeger
docker-compose up -d jaeger

# Access UI
open http://localhost:16686

# Search traces by service
# 1. Select service from dropdown
# 2. Set time range
# 3. Click "Find Traces"
```

**Tracing in Code**:
```go
import "go.opentelemetry.io/otel"

func Handler(ctx context.Context) {
    tracer := otel.Tracer("service-name")
    ctx, span := tracer.Start(ctx, "operation-name")
    defer span.End()
    
    // Add attributes
    span.SetAttributes(
        attribute.String("user.id", userID),
        attribute.String("workflow.id", workflowID),
    )
}
```

**Environment Variables for Services**:
```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317
OTEL_SERVICE_NAME=workflow-service
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # 10% sampling
```

---

### Elasticsearch

**Purpose**: Log aggregation, full-text search, and analytics.

| Property | Value |
|----------|-------|
| URL | http://localhost:9200 |
| Transport | localhost:9300 |
| Image | docker.elastic.co/elasticsearch/elasticsearch:8.11.0 |
| Memory | 256MB (dev) / 512MB+ (prod) |

**Features**:
- Single-node mode (dev)
- Security disabled (dev mode)
- Full-text search for workflows
- Log aggregation

**Usage**:
```bash
# Start Elasticsearch
docker-compose up -d elasticsearch

# Check health
curl http://localhost:9200/_cluster/health?pretty

# List indices
curl http://localhost:9200/_cat/indices?v

# Search example
curl -X GET "localhost:9200/workflows/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "match": {
      "name": "my workflow"
    }
  }
}'
```

**Index Patterns**:
| Index | Purpose |
|-------|---------|
| workflows | Workflow documents |
| executions | Execution logs |
| audit-* | Audit trail (daily rotation) |
| logs-* | Application logs |

---

## Infrastructure

### Kong API Gateway

**Purpose**: API gateway for routing, rate limiting, authentication, and plugins.

| Property | Value |
|----------|-------|
| Proxy URL | http://localhost:8000 |
| Admin API | http://localhost:8444 |
| Config | deployments/gateway/kong/kong.yml |
| Image | kong:3.5 |
| Mode | DB-less (declarative) |

**Features**:
- Declarative configuration
- JWT validation
- Rate limiting per consumer
- Request/response transformation
- Circuit breaking
- CORS handling

**Route Configuration**:
| Route | Service |
|-------|---------|
| /api/v1/auth/* | auth-service:8001 |
| /api/v1/users/* | user-service:8002 |
| /api/v1/workflows/* | workflow-service:8003 |
| /api/v1/executions/* | execution-service:8004 |
| /api/v1/nodes/* | node-service:8005 |
| /api/v1/webhooks/* | webhook-service:8006 |
| /api/v1/credentials/* | credential-service:8007 |
| /api/v1/variables/* | variable-service:8008 |
| /api/v1/schedules/* | schedule-service:8009 |

**Rate Limits**:
| Consumer Type | Limit |
|---------------|-------|
| Anonymous | 100 req/min |
| Authenticated | 1000 req/min |
| Premium | 10000 req/min |

**Usage**:
```bash
# Start Kong
docker-compose up -d kong

# Check status
curl http://localhost:8444/status

# List routes
curl http://localhost:8444/routes

# List services
curl http://localhost:8444/services

# Test API through gateway
curl http://localhost:8000/api/v1/workflows

# Add custom plugin
curl -X POST http://localhost:8444/services/workflow-service/plugins \
  --data "name=rate-limiting" \
  --data "config.minute=100"
```

---

### Kafka

**Purpose**: Event streaming and async communication between services.

| Property | Value |
|----------|-------|
| Broker | localhost:29092 |
| Internal | kafka:29092 |
| Image | confluentinc/cp-kafka:7.5.0 |
| Depends On | Zookeeper |

**Topics**:
| Topic | Partitions | Retention | Purpose |
|-------|------------|-----------|---------|
| workflow.events | 10 | 7 days | Workflow lifecycle events |
| workflow.triggers | 5 | 2 days | Trigger events |
| execution.events | 20 | 3 days | Execution state changes |
| execution.commands | 15 | 1 day | Start/stop commands |
| execution.results | 20 | 5 days | Node execution outputs |
| webhook.events | 10 | 3 days | Incoming webhook events |
| schedule.triggers | 5 | 1 day | Scheduled triggers |
| notification.events | 10 | 2 days | Notification requests |
| analytics.events | 15 | 30 days | Usage metrics |
| audit.log | 5 | 30 days | Audit trail |
| dlq.events | 5 | 14 days | Dead letter queue |

**Usage**:
```bash
# Start Kafka
docker-compose up -d zookeeper kafka

# List topics
docker exec kafka kafka-topics --list --bootstrap-server localhost:9092

# Create topic
docker exec kafka kafka-topics --create \
  --topic my.topic \
  --partitions 10 \
  --replication-factor 1 \
  --bootstrap-server localhost:9092

# Describe topic
docker exec kafka kafka-topics --describe \
  --topic workflow.events \
  --bootstrap-server localhost:9092

# Consume messages (debugging)
docker exec kafka kafka-console-consumer \
  --topic workflow.events \
  --bootstrap-server localhost:9092 \
  --from-beginning \
  --max-messages 10

# Produce test message
docker exec -it kafka kafka-console-producer \
  --topic workflow.events \
  --bootstrap-server localhost:9092

# Check consumer lag
docker exec kafka kafka-consumer-groups \
  --describe \
  --group linkflow-group \
  --bootstrap-server localhost:9092
```

**Event Format**:
```json
{
  "id": "uuid",
  "type": "workflow.execution.started",
  "aggregateId": "workflow-id",
  "timestamp": "2024-01-15T10:30:00Z",
  "userId": "user-id",
  "version": 1,
  "payload": {},
  "metadata": {
    "correlationId": "request-id",
    "traceId": "jaeger-trace-id"
  }
}
```

---

### Redis

**Purpose**: Caching, session storage, rate limiting, and pub/sub.

| Property | Value |
|----------|-------|
| Host | localhost |
| Port | 6379 |
| Image | redis:7-alpine |
| Persistence | AOF enabled |

**Use Cases**:
- Session storage
- API response caching
- Rate limiting counters
- Distributed locks
- Pub/sub for real-time updates

**Usage**:
```bash
# Start Redis
docker-compose up -d redis

# Connect to CLI
docker exec -it redis redis-cli

# Basic commands
> PING                    # Test connection
> INFO                    # Server info
> DBSIZE                  # Number of keys
> KEYS *                  # List all keys (dev only)
> GET key                 # Get value
> SET key value EX 3600   # Set with 1hr TTL
> DEL key                 # Delete key
> TTL key                 # Check TTL

# Monitor commands in real-time
docker exec -it redis redis-cli MONITOR

# Check memory usage
docker exec -it redis redis-cli INFO memory
```

**Key Patterns**:
| Pattern | Purpose | TTL |
|---------|---------|-----|
| session:{userId} | User sessions | 24h |
| cache:workflow:{id} | Workflow cache | 5m |
| ratelimit:{ip} | Rate limit counter | 1m |
| lock:{resource} | Distributed lock | 30s |
| token:{refreshToken} | Refresh tokens | 7d |

---

### PostgreSQL

**Purpose**: Primary relational database for all services.

| Property | Value |
|----------|-------|
| Host | localhost |
| Port | 5432 |
| Database | linkflow |
| Username | linkflow |
| Password | linkflow123 |
| Image | postgres:15-alpine |

**Schemas** (per service):
| Schema | Service |
|--------|---------|
| auth | Auth service |
| users | User service |
| workflows | Workflow service |
| executions | Execution service |
| credentials | Credential service |
| webhooks | Webhook service |
| schedules | Schedule service |
| audit | Audit service |

**Usage**:
```bash
# Start PostgreSQL
docker-compose up -d postgres

# Connect via psql
docker exec -it postgres psql -U linkflow -d linkflow

# Common commands
\l              # List databases
\dn             # List schemas
\dt             # List tables
\d+ table_name  # Describe table
\q              # Quit

# Run SQL file
docker exec -i postgres psql -U linkflow -d linkflow < script.sql

# Backup
docker exec postgres pg_dump -U linkflow linkflow > backup.sql

# Restore
docker exec -i postgres psql -U linkflow -d linkflow < backup.sql
```

**Connection String**:
```
postgresql://linkflow:linkflow123@localhost:5432/linkflow?sslmode=disable
```

**Migrations**:
```bash
make migrate-up       # Apply all pending
make migrate-down     # Rollback last
make migrate-status   # Check status
make migrate-create NAME=add_table  # Create new
```

---

### Zookeeper

**Purpose**: Coordination service for Kafka.

| Property | Value |
|----------|-------|
| Port | 2181 |
| Image | confluentinc/cp-zookeeper:7.5.0 |

**Usage**:
```bash
# Start Zookeeper
docker-compose up -d zookeeper

# Check status
docker exec zookeeper zkServer.sh status
```

---

## Development Tools

### Go Tools

```bash
# Install all dev tools
make install-tools

# Installed tools:
# - golangci-lint: Linting
# - gofumpt: Code formatting
# - mockgen: Mock generation
# - migrate: Database migrations
# - protoc-gen-go: Protobuf generation
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run go vet
make vet

# All checks
make check

# Security scan
make sec
```

### Testing

```bash
# Unit tests
make test

# With coverage
make test-coverage

# Integration tests
make test-integration

# E2E tests
make test-e2e

# Race detection
make test-race
```

### Building

```bash
# Build all services
make build

# Build specific service
make build-service SERVICE=auth

# Docker build
make docker-build

# Clean build
make clean && make build
```

---

## Service Ports Reference

### Infrastructure Services

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| PostgreSQL | 5432 | TCP | Database |
| Redis | 6379 | TCP | Cache |
| Kafka | 29092 | TCP | Event streaming |
| Zookeeper | 2181 | TCP | Kafka coordination |
| Elasticsearch | 9200 | HTTP | Search/Logs |
| Elasticsearch | 9300 | TCP | Transport |

### Monitoring Services

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| Grafana | 3000 | HTTP | Dashboards |
| Prometheus | 9090 | HTTP | Metrics |
| Jaeger UI | 16686 | HTTP | Tracing UI |
| Jaeger Collector | 14268 | HTTP | Trace collector |
| Jaeger OTLP | 4317 | gRPC | OpenTelemetry |
| Jaeger OTLP | 4318 | HTTP | OpenTelemetry |

### Gateway Services

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| Kong Proxy | 8000 | HTTP | API Gateway |
| Kong Admin | 8444 | HTTP | Admin API |

### Application Services

| Service | Port | Purpose |
|---------|------|---------|
| Auth | 8001 | Authentication, JWT, OAuth |
| User | 8002 | User management |
| Workflow | 8003 | Workflow CRUD |
| Execution | 8004 | Execution orchestration |
| Node | 8005 | Node registry |
| Webhook | 8006 | Webhook handling |
| Credential | 8007 | Secrets management |
| Variable | 8008 | Global variables |
| Schedule | 8009 | Cron scheduling |
| Notification | 8010 | Notifications |
| Audit | 8011 | Audit logging |
| Analytics | 8012 | Analytics |
| Search | 8013 | Search service |
| Storage | 8014 | File storage |
| Billing | 8015 | Billing/payments |
| WebSocket | 8016 | Real-time updates |
| Executor | 8017 | Node execution |

---

## Make Commands Reference

### Development

| Command | Description |
|---------|-------------|
| `make setup` | Complete dev environment setup |
| `make dev` | Start infrastructure + migrations |
| `make deps` | Download Go dependencies |
| `make install-tools` | Install dev tools |

### Building

| Command | Description |
|---------|-------------|
| `make build` | Build all services |
| `make build-service SERVICE=auth` | Build specific service |
| `make clean` | Clean build artifacts |
| `make docker-build` | Build Docker images |

### Testing

| Command | Description |
|---------|-------------|
| `make test` | Run unit tests |
| `make test-coverage` | Generate coverage report |
| `make test-integration` | Run integration tests |
| `make test-e2e` | Run E2E tests |
| `make test-race` | Run with race detector |

### Code Quality

| Command | Description |
|---------|-------------|
| `make lint` | Run golangci-lint |
| `make fmt` | Format code |
| `make vet` | Run go vet |
| `make check` | All quality checks |
| `make sec` | Security scan |

### Database

| Command | Description |
|---------|-------------|
| `make migrate-up` | Apply migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-status` | Show migration status |
| `make migrate-create NAME=x` | Create new migration |
| `make migrate-reset` | Reset database (DANGER!) |

### Infrastructure

| Command | Description |
|---------|-------------|
| `make infra-up` | Start infrastructure |
| `make infra-down` | Stop infrastructure |
| `make infra-status` | Check status |
| `make infra-logs` | View logs |
| `make run` | Start all services |
| `make stop` | Stop all services |
| `make logs` | View service logs |

### Kubernetes/Helm

| Command | Description |
|---------|-------------|
| `make k8s-deploy` | Deploy to Kubernetes |
| `make k8s-delete` | Delete from Kubernetes |
| `make helm-install` | Install Helm chart |
| `make helm-upgrade` | Upgrade Helm chart |
| `make helm-uninstall` | Uninstall Helm chart |

### Monitoring

| Command | Description |
|---------|-------------|
| `make monitoring-deploy` | Deploy Prometheus + Grafana |
| `make logging-deploy` | Deploy Loki |
| `make tracing-deploy` | Deploy Jaeger |

---

## Quick Start Cheatsheet

```bash
# 1. First-time setup
make setup

# 2. Start everything
make dev              # Infrastructure + migrations
make run              # All services

# 3. Access tools
open http://localhost:3000   # Grafana
open http://localhost:9090   # Prometheus
open http://localhost:16686  # Jaeger
open http://localhost:8000   # Kong Gateway

# 4. Development workflow
make check            # Before commit
make test             # Run tests
make build            # Build services

# 5. Database work
make migrate-create NAME=add_feature
make migrate-up

# 6. Debugging
make logs             # Service logs
make infra-logs       # Infrastructure logs
docker-compose ps     # Check status
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check container status
docker-compose ps

# View logs
docker-compose logs service-name

# Common issues:
# - Port already in use: lsof -i :PORT
# - Database not ready: wait for postgres healthcheck
# - Missing env vars: check docker-compose.yml
```

### Database Connection Issues

```bash
# Test connection
docker exec -it postgres psql -U linkflow -d linkflow -c "SELECT 1"

# Check if running
docker-compose ps postgres

# Reset database
make migrate-reset
```

### Kafka Issues

```bash
# Check broker status
docker exec kafka kafka-broker-api-versions --bootstrap-server localhost:9092

# Check consumer groups
docker exec kafka kafka-consumer-groups --list --bootstrap-server localhost:9092

# Reset consumer offset
docker exec kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --group linkflow-group \
  --reset-offsets \
  --to-earliest \
  --execute \
  --topic workflow.events
```

### High Memory Usage

```bash
# Check container stats
docker stats

# Reduce Elasticsearch memory (edit docker-compose.override.yml)
ES_JAVA_OPTS=-Xms128m -Xmx128m

# Restart with limits
docker-compose up -d
```
