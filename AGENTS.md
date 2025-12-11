# LinkFlow Go - Agent Instructions

## Quick Reference

### Creating a New Microservice
1. Use service template in `/cmd/services/{service-name}/`
2. Implement health check endpoints (`/health/live`, `/health/ready`)
3. Add Prometheus metrics endpoint (`/metrics`)
4. Create database schema and migrations
5. Set up event publishers/subscribers
6. Write unit and integration tests
7. Create Helm chart for deployment
8. Document API in OpenAPI format

### Common Tasks
- **Add new endpoint**: Update handlers, routes, and OpenAPI spec
- **Publish event**: Use EventBus interface with correlation ID
- **Database query**: Use repository pattern with context
- **External API call**: Wrap with circuit breaker
- **Configuration**: Use environment variables, never hardcode
- **Logging**: Always include correlation ID and user context
- **Testing**: Unit test coverage > 80%, integration tests for APIs

## Project Overview

LinkFlow Go is an advanced workflow automation platform (n8n clone) transitioning from monolith to microservices architecture. The system is designed to handle enterprise-grade workloads with millions of workflows and billions of executions.

### Architecture Goals
- **Domain-Driven Microservices**: 15+ specialized services with clear boundaries
- **Event-Driven Architecture**: Apache Kafka/NATS for async communication
- **Service Mesh**: Istio for traffic management and observability
- **API Gateway**: Kong for unified API management
- **CQRS + Event Sourcing**: Separate read/write models with event replay
- **Multi-Region Support**: Active-active deployment capabilities

## Core Commands

```bash
# Development
make run          # Start all services
make test         # Run tests
make lint         # Run linters
make build        # Build the application

# Database
make migrate-up   # Run migrations
make migrate-down # Rollback migrations

# Docker
docker-compose up -d  # Start services in background
docker-compose down   # Stop services
```

## Project Layout

```
linkflow-go/
├── cmd/              # Application entrypoints
│   └── services/     # Microservice entrypoints
│       ├── auth/     # Authentication service
│       ├── workflow/ # Workflow management service
│       ├── executor/ # Workflow execution service
│       └── webhook/  # Webhook handling service
├── configs/          # Configuration files (Kong, services)
├── deployments/      # Kubernetes, Helm charts, Docker configs
│   ├── k8s/          # Kubernetes manifests
│   ├── helm/         # Helm charts
│   └── docker/       # Dockerfiles
├── internal/         # Private application code
│   ├── domain/       # Domain models
│   ├── services/     # Service implementations
│   ├── handlers/     # HTTP handlers
│   └── pkg/          # Shared internal packages
│       ├── events/   # Event bus implementation
│       ├── database/ # Database utilities
│       └── cache/    # Caching layer
├── migrations/       # Database migrations
├── pkg/             # Public packages (telemetry, utils)
├── plan/            # Architecture and implementation plans
├── scripts/         # Build and deployment scripts
└── tests/           # Integration and e2e tests
```

## Microservices Architecture

### Core Services
1. **Auth Service**: JWT management, OAuth2, SSO, API key management
2. **Workflow Service**: CRUD operations, versioning, templates, sharing
3. **Execution Service**: Execution lifecycle, state management, retries
4. **Executor Service**: Node execution, parallel processing, error handling
5. **Webhook Service**: Webhook registration, payload processing, retries
6. **Schedule Service**: Cron management, timezone handling, recurring jobs
7. **Node Service**: Node registry, custom nodes, marketplace integration
8. **Credential Service**: Secure storage, encryption, OAuth token refresh
9. **Notification Service**: Email, SMS, push notifications, in-app alerts
10. **Audit Service**: Activity logging, compliance reporting, data retention

## Development Patterns & Constraints

- **Language**: Go 1.20+
- **Database**: PostgreSQL (primary), Redis (cache), TimescaleDB (metrics)
- **Message Queue**: Apache Kafka/NATS for event streaming
- **API Gateway**: Kong for routing, rate limiting, and plugins
- **Service Mesh**: Istio for service-to-service communication
- **Authentication**: JWT-based auth service with OAuth2 support
- **Testing**: Unit tests in `*_test.go`, integration tests in `/tests`
- **Configuration**: Environment variables, YAML configs, ConfigMaps
- **Error Handling**: Always wrap errors with context, use error codes
- **Logging**: Structured logging (JSON), correlation IDs
- **Tracing**: OpenTelemetry with Jaeger backend
- **Metrics**: Prometheus metrics, custom dashboards in Grafana

## Git Workflow

- Branch from `master` for features
- Use conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Run tests before committing
- Keep commits atomic and focused

## Testing Requirements

Before submitting any code:
1. Run `make test` - all tests must pass
2. Run `make lint` - no linting errors  
3. Ensure new features have tests
4. Check coverage doesn't decrease (minimum 80%)
5. Integration tests for API endpoints
6. Load tests for performance-critical paths

### Testing Strategy
```go
// Unit tests: *_test.go files
func TestServiceMethod(t *testing.T) {
    // Arrange
    // Act  
    // Assert
}

// Integration tests: /tests/integration
func TestAPIEndpoint(t *testing.T) {
    // Test with real database
    // Test with real message queue
}

// E2E tests: /tests/e2e
func TestWorkflowExecution(t *testing.T) {
    // Test complete workflow lifecycle
}
```

## Security Guidelines

- Never commit secrets or API keys
- Use environment variables for sensitive data
- Validate all user inputs
- Use prepared statements for database queries
- Follow OWASP security best practices
- Implement rate limiting on all endpoints
- Use mTLS for service-to-service communication
- Encrypt sensitive data at rest (AES-256)
- JWT tokens expire after 1 hour
- Refresh tokens rotate on each use

## API Design

- RESTful endpoints following standard conventions
- Use proper HTTP status codes
- Include error messages in responses
- Version APIs when making breaking changes (`/api/v1`, `/api/v2`)
- Document all endpoints with OpenAPI/Swagger
- Implement pagination for list endpoints (limit/offset or cursor-based)
- Use consistent error response format:
  ```json
  {
    "error": {
      "code": "VALIDATION_ERROR",
      "message": "Invalid input",
      "details": {}
    }
  }
  ```

## Service Communication Patterns

### Synchronous (REST/gRPC)
- Service-to-service auth via mTLS
- Circuit breakers for resilience
- Retry with exponential backoff
- Request timeouts (30s default)

### Asynchronous (Events)
- Event naming: `{service}.{entity}.{action}` (e.g., `workflow.execution.started`)
- Idempotent event handlers
- Dead letter queues for failed messages
- Event sourcing for audit trail

## Performance Considerations

- Use connection pooling for database (max 25 connections per service)
- Implement caching where appropriate (Redis with TTL)
- Monitor resource usage (CPU < 70%, Memory < 80%)
- Profile code for bottlenecks (pprof enabled in dev)
- Use pagination for list endpoints (default: 50, max: 200)
- Batch database operations where possible
- Use read replicas for query-heavy operations
- Implement request coalescing for duplicate requests
- Set appropriate timeouts (HTTP: 30s, Database: 5s)

## Resilience Patterns

### Retry Strategy
```yaml
Exponential Backoff:
  - Initial delay: 100ms
  - Max delay: 10s
  - Max attempts: 3
  - Jitter: ±10%
  - Retryable errors: 429, 502, 503, 504
```

### Health Checks
```go
// Required endpoints for each service
GET /health/live    // Kubernetes liveness probe
GET /health/ready   // Kubernetes readiness probe
GET /metrics        // Prometheus metrics endpoint
```

### Graceful Degradation
- Feature flags for progressive rollouts
- Fallback to cache when database unavailable
- Queue overflow handling with backpressure
- Partial response on timeout (return what's available)

## Documentation Standards

- Document all public functions and types
- Include examples in complex functions
- Keep README updated with setup instructions
- Document configuration options
- Maintain API documentation

## Implementation Roadmap

### Current Phase: Foundation & Core Services
1. **Infrastructure Setup** (Weeks 1-2)
   - Kubernetes with Kind/Minikube for local dev
   - Service mesh (Istio) deployment
   - Kong API Gateway configuration
   - Kafka/NATS message queue setup
   - Observability stack (Prometheus, Grafana, Jaeger, ELK)

2. **First Services Extraction** (Weeks 3-4)
   - Auth Service (authentication, JWT, OAuth2)
   - User Service (user management, profiles)
   - Initial event bus implementation

3. **Core Workflow Services** (Weeks 5-6)
   - Workflow Service (CRUD, versioning)
   - Execution Service (state management)
   - Basic executor implementation

### Service Implementation Checklist
- [ ] Base service template with health checks
- [ ] Database migrations per service
- [ ] Event publishing/subscribing
- [ ] OpenAPI documentation
- [ ] Prometheus metrics
- [ ] Distributed tracing
- [ ] Unit and integration tests
- [ ] Helm chart for deployment

## Specific Architecture Patterns

### Base Service Template Structure
```go
// Every microservice follows this pattern
cmd/services/{service-name}/
├── main.go           // Entry point with graceful shutdown
├── server/
│   ├── server.go     // HTTP server setup
│   ├── routes.go     // Route definitions
│   └── middleware.go // Service-specific middleware
├── handlers/         // HTTP handlers
├── service/          // Business logic
├── repository/       // Data access layer
└── events/           // Event publishers/subscribers
```

### Event-Driven Architecture
```go
// Event structure for all services
type Event struct {
    ID            string    // UUID
    Type          string    // e.g., "workflow.execution.started"
    AggregateID   string    // Entity ID
    Timestamp     time.Time
    UserID        string
    Version       int       // For event sourcing
    Payload       map[string]interface{}
    Metadata      struct {
        CorrelationID string  // Track across services
        TraceID       string  // Distributed tracing
    }
}
```

### Circuit Breaker Pattern
- Applied to all external service calls
- Threshold: 5 failures to open circuit
- Timeout: 30 seconds before retry
- Half-open state: Allow 3 test requests

### CQRS Implementation
```go
// Command side (write)
type WorkflowCommandService interface {
    CreateWorkflow(ctx context.Context, cmd CreateWorkflowCommand) error
    UpdateWorkflow(ctx context.Context, cmd UpdateWorkflowCommand) error
    DeleteWorkflow(ctx context.Context, id string) error
}

// Query side (read)
type WorkflowQueryService interface {
    GetWorkflow(ctx context.Context, id string) (*Workflow, error)
    ListWorkflows(ctx context.Context, filter Filter) ([]*Workflow, error)
    SearchWorkflows(ctx context.Context, query string) ([]*Workflow, error)
}
```

### Saga Pattern for Distributed Transactions
```yaml
# Example: Workflow Execution Saga
Steps:
  1. Reserve Resources → Compensate: Release Resources
  2. Initialize Execution → Compensate: Mark Failed
  3. Execute Nodes → Compensate: Rollback Node Changes
  4. Save Results → Compensate: Delete Results
  5. Update Status → Compensate: Revert Status
```

### Database Per Service
- Each service owns its database/schema
- No direct database access between services
- Data synchronization via events
- Materialized views for cross-service queries

### API Gateway Patterns
```yaml
Kong Configuration:
  - Rate Limiting: 
    - Anonymous: 100 req/hour
    - Authenticated: 1000 req/hour  
    - Premium: 10000 req/hour
    - Enterprise: Unlimited
  - JWT Validation: RS256 algorithm
  - Request/Response Transformation
  - Circuit Breaking
  - Request Routing by version
  - CORS handling
  - Request logging
  - WebSocket support
  - GraphQL federation

Route Configuration:
  /api/v1/auth/**      → auth-service
  /api/v1/users/**     → user-service
  /api/v1/workflows/** → workflow-service
  /api/v1/executions/** → execution-service
  /api/v1/nodes/**     → node-service
  /api/v1/webhooks/**  → webhook-service
  /api/graphql         → graphql-gateway
  /ws/**               → websocket-gateway
```

### GraphQL Federation
```graphql
type Query {
  # User Service
  me: User
  users(filter: UserFilter): [User!]!
  
  # Workflow Service  
  workflows(filter: WorkflowFilter): [Workflow!]!
  workflow(id: ID!): Workflow
  
  # Execution Service
  executions(filter: ExecutionFilter): [Execution!]!
  execution(id: ID!): Execution
}

type Subscription {
  executionUpdates(executionId: ID!): ExecutionUpdate!
  workflowChanges(workflowId: ID!): WorkflowChange!
}
```

### Observability Stack
```yaml
Metrics (Prometheus):
  - http_requests_total
  - http_request_duration_seconds
  - workflow_executions_total
  - node_execution_duration_seconds
  - database_connections_active
  - event_bus_messages_total

Tracing (Jaeger):
  - Trace all requests across services
  - Baggage propagation for user context
  - Custom spans for business operations

Logging (ELK):
  - Structured JSON logging
  - Correlation ID in every log
  - Log levels: ERROR, WARN, INFO, DEBUG
  - Centralized in Elasticsearch
```

## Deployment

- **Containerization**: Docker with multi-stage builds
- **Orchestration**: Kubernetes (local: Kind/Minikube, prod: EKS/GKE)
- **GitOps**: ArgoCD for continuous deployment
- **Service Mesh**: Istio for traffic management
- **Manifests**: `/deployments/k8s` for raw manifests
- **Helm Charts**: `/deployments/helm` for templated deployments
- **Environment Configs**: ConfigMaps and Secrets per namespace
- **Health Checks**: Readiness and liveness probes required
- **Auto-scaling**: HPA based on CPU/memory metrics
- **Multi-region**: Active-active with Kafka replication

## Database Strategy

### Migration Approach
```sql
-- Each service has its own schema
CREATE SCHEMA auth_service;
CREATE SCHEMA workflow_service;
CREATE SCHEMA execution_service;

-- Migrations follow pattern: {timestamp}_{service}_{description}.sql
-- Example: 20240101120000_workflow_add_version_table.sql
```

### Core Data Models
```go
// Workflow domain model
type Workflow struct {
    ID          string    `json:"id" db:"id"`
    UserID      string    `json:"userId" db:"user_id"`
    Name        string    `json:"name" db:"name"`
    Description string    `json:"description" db:"description"`
    Version     int       `json:"version" db:"version"`
    Active      bool      `json:"active" db:"active"`
    Nodes       []Node    `json:"nodes" db:"-"`
    Connections []Edge    `json:"connections" db:"-"`
    Settings    Settings  `json:"settings" db:"settings"`
    CreatedAt   time.Time `json:"createdAt" db:"created_at"`
    UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
}

// Execution tracking
type Execution struct {
    ID         string            `json:"id"`
    WorkflowID string            `json:"workflowId"`
    Status     ExecutionStatus   `json:"status"`
    StartedAt  time.Time         `json:"startedAt"`
    FinishedAt *time.Time        `json:"finishedAt"`
    Data       map[string]interface{} `json:"data"`
    Error      *string           `json:"error,omitempty"`
}
```

### Database Connection Management
```yaml
PostgreSQL:
  - Max connections: 100 total
  - Connection pool per service: 25
  - Connection timeout: 5s
  - Idle timeout: 10m
  - Health check interval: 30s

Redis:
  - Max connections: 50
  - Connection pool: 10 per service
  - Read timeout: 3s
  - Write timeout: 3s
```

## Monitoring & Alerting

### Key Metrics to Track
```yaml
Service Level:
  - Request rate (req/s)
  - Error rate (% of 5xx responses)
  - Latency (p50, p95, p99)
  - Saturation (CPU, memory, connections)

Business Level:
  - Workflows created per hour
  - Execution success rate
  - Average execution duration
  - Active users
  - Node execution failures

Infrastructure:
  - Kafka lag per consumer group
  - Database connection pool usage
  - Redis memory usage
  - Pod restarts
  - Network latency between services
```

### Alert Thresholds
```yaml
Critical:
  - Error rate > 5% for 5 minutes
  - Service down for 2 minutes
  - Database connections > 90%
  - Kafka consumer lag > 1000 messages

Warning:
  - Response time p95 > 2s for 10 minutes
  - CPU usage > 70% for 15 minutes
  - Memory usage > 80%
  - Failed workflow executions > 10% for 30 minutes
```
