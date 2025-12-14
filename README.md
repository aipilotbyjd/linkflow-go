# LinkFlow - Advanced Microservices Workflow Automation Platform

A cloud-native, enterprise-grade workflow automation platform built with Go, featuring a microservices architecture designed for extreme scalability and reliability.

## 🚀 Features

- **15+ Specialized Microservices** with clear domain boundaries
- **Event-Driven Architecture** using Apache Kafka/NATS
- **CQRS + Event Sourcing** for audit trail and scalability
- **Service Mesh** with Istio for advanced traffic management
- **Multi-Region Support** with active-active deployment
- **Enterprise Security** with OAuth2, JWT, and mTLS
- **Real-time Processing** with WebSocket support
- **Horizontal Scaling** to handle millions of workflows

## 🏗️ Architecture

### Core Services

- **Auth Service** - Authentication, JWT, OAuth2, SSO
- **User Service** - User management, teams, RBAC
- **Workflow Service** - Workflow CRUD, versioning, templates
- **Execution Service** - Execution orchestration, monitoring
- **Executor Service** - Worker pool for actual execution
- **Node Service** - Node registry, marketplace
- **Credential Service** - Secure credential storage
- **Webhook Service** - Webhook ingestion and routing
- **Schedule Service** - Cron scheduling, timezone management
- **Notification Service** - Multi-channel notifications
- **Audit Service** - Immutable audit logging
- **Analytics Service** - Metrics and dashboards
- **Storage Service** - File uploads, S3 integration
- **Search Service** - Full-text search with Elasticsearch
- **Billing Service** - Usage tracking, subscriptions

## 📋 Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Kubernetes (for production)
- PostgreSQL 15+
- Redis 7+
- Apache Kafka
- kubectl (for Kubernetes)
- Helm 3.x (optional)
- Istio (optional, for service mesh)

## 🛠️ Quick Start

### Local Development Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/linkflow-go.git
cd linkflow-go

# Complete development environment setup (automated)
make dev-setup

# Or manually:
# 1. Copy environment file
cp .env.example .env

# 2. Start infrastructure services
make infra-up
# Or specific services:
docker-compose up -d postgres redis zookeeper kafka elasticsearch prometheus grafana jaeger kong

# 3. Setup Kafka topics
make kafka-setup

# 4. Run database migrations
make db-migrate

# 5. Seed development data (optional)
make db-seed

# Build all services
make build

# Run a specific service
make run-auth
# Or: ./bin/auth-service

# Check infrastructure status
make infra-status

# View infrastructure logs
make infra-logs
```

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# End-to-end tests
make test-e2e

# Test coverage
make test-coverage
```

## 🐳 Docker

### Build Images

```bash
# Build all service images
make docker-build

# Build specific service
make docker-build SERVICE=auth

# Push to registry
make docker-push
```

### Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## ☸️ Kubernetes Deployment

### Complete Deployment

```bash
# 1. Deploy core services
make k8s-deploy

# 2. Install Istio service mesh
make istio-install
make istio-setup

# 3. Deploy logging stack (ELK)
make logging-deploy

# 4. Deploy distributed tracing (Jaeger)
make tracing-deploy

# 5. Setup GitOps with ArgoCD
make argocd-install
make argocd-setup

# Check deployment status
make k8s-status
```

### Manual Deployment

```bash
# Create namespace
kubectl create namespace linkflow

# Apply configurations
kubectl apply -f deployments/kubernetes/namespace.yaml
kubectl apply -f deployments/kubernetes/configmap.yaml
kubectl apply -f deployments/kubernetes/secrets.yaml

# Deploy services
kubectl apply -f deployments/kubernetes/auth/
kubectl apply -f deployments/kubernetes/workflow/
kubectl apply -f deployments/kubernetes/execution/

# Deploy ingress
kubectl apply -f deployments/kubernetes/ingress.yaml

# Check deployment status
kubectl get pods -n linkflow
kubectl get svc -n linkflow
kubectl get ingress -n linkflow

# View service logs
kubectl logs -f deployment/auth-service -n linkflow
```

### Using Helm

```bash
# Install
helm install linkflow deployments/helm/linkflow \
  --namespace linkflow \
  --create-namespace

# Upgrade
helm upgrade linkflow deployments/helm/linkflow \
  --namespace linkflow

# Rollback
helm rollback linkflow --namespace linkflow

# Uninstall
helm uninstall linkflow --namespace linkflow
```

### Service Mesh (Istio)

```bash
# Install Istio
./scripts/install-istio.sh install

# Enable sidecar injection
kubectl label namespace linkflow istio-injection=enabled

# Apply Istio configurations
kubectl apply -f deployments/istio/

# Access Istio dashboards
istioctl dashboard kiali    # Service mesh visualization
istioctl dashboard grafana   # Metrics dashboards
istioctl dashboard jaeger    # Distributed tracing
```

### GitOps with ArgoCD

```bash
# Install ArgoCD
./scripts/install-argocd.sh install

# Apply LinkFlow applications
kubectl apply -f deployments/argocd/

# Access ArgoCD UI (port-forward)
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Get admin password
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

## 📊 Monitoring & Observability

### Metrics (Prometheus + Grafana)
- **Grafana Dashboard**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- Pre-configured dashboards for service metrics
- Alert rules for service health, performance, and resources

### Distributed Tracing (Jaeger)
- **Jaeger UI**: http://localhost:16686
- OpenTelemetry integration
- Service dependency visualization
- Performance bottleneck analysis

### Logging (ELK Stack or Loki)
- **Kibana**: http://localhost:5601 (for ELK)
- **Grafana**: http://localhost:3000 (for Loki)
- Centralized log aggregation from all services
- Log parsing and search capabilities

### Service Mesh (Istio)
- **Kiali Dashboard**: Service mesh topology and health
- **Grafana Dashboards**: Traffic metrics and performance
- **mTLS**: Automatic mutual TLS between services
- **Traffic Management**: Canary deployments, circuit breakers

## 🏗️ Infrastructure Components

### Local Development Stack
- **PostgreSQL 15**: Primary database with replication support
- **Redis 7**: Caching, session store, and pub/sub
- **Apache Kafka**: Event streaming platform with Zookeeper
- **Elasticsearch 8**: Full-text search and log storage
- **Prometheus**: Time-series metrics database
- **Grafana**: Metrics visualization and dashboards
- **Jaeger**: Distributed tracing platform
- **Kong**: API Gateway for routing and rate limiting

### Kubernetes Resources
- **Namespaces**: linkflow, linkflow-staging, linkflow-dev
- **Deployments**: All microservices with rolling updates
- **StatefulSets**: Elasticsearch, Kafka, PostgreSQL
- **Services**: ClusterIP and LoadBalancer services
- **Ingress**: NGINX ingress controller with TLS
- **ConfigMaps & Secrets**: Centralized configuration
- **HPA**: Horizontal Pod Autoscaling for all services
- **PDB**: Pod Disruption Budgets for high availability

### CI/CD Pipeline (GitHub Actions)
- **CI**: Lint, test, security scan, build
- **CD**: Automated deployment to staging/production
- **Release**: Multi-platform builds and Docker images
- **Security**: gosec, Trivy vulnerability scanning

## 🔧 Configuration

Configuration is managed through environment variables and config files. See `.env.example` for all available options.

```yaml
# configs/auth-service.yaml
server:
  port: 8001
  
database:
  host: localhost
  port: 5432
  name: linkflow
  
redis:
  host: localhost
  port: 6379
  
kafka:
  brokers:
    - localhost:9092
```

## 📈 Performance Targets

- **1M+** workflows supported
- **100M+** executions/month
- **10K+** concurrent executions
- **100K+** API requests/second
- **P99 latency < 200ms**

## 🔐 Security

- OAuth2/OIDC authentication
- JWT with RS256 signing
- mTLS between services
- HashiCorp Vault for secrets
- RBAC with Casbin
- Audit logging for compliance

## 🧪 Development

### Project Structure

```
linkflow-go/
├── cmd/services/       # Service entry points
├── internal/           # Private application code
│   ├── domain/        # Domain models
│   ├── services/      # Service implementations
│   └── pkg/          # Shared packages
├── pkg/               # Public packages
├── deployments/       # Deployment configurations
│   ├── docker/        # Docker configurations
│   ├── k8s/          # Kubernetes manifests
│   ├── istio/        # Istio service mesh configs
│   ├── logging/      # ELK/Loki stack configs
│   ├── tracing/      # Jaeger configurations
│   ├── kafka/        # Kafka topics and configs
│   ├── kong/         # API Gateway configs
│   └── argocd/       # GitOps configurations
├── configs/           # Service configurations
├── scripts/           # Utility scripts
├── docs/             # Documentation
└── tests/            # Test suites
```

### Utility Scripts

| Script | Purpose |
|--------|---------|
| `dev-setup.sh` | Complete local environment setup |
| `kafka-setup.sh` | Create and manage Kafka topics |
| `migrate.sh` | Run database migrations |
| `seed.sh` | Seed database with test data |
| `k8s-deploy.sh` | Deploy to Kubernetes cluster |
| `install-istio.sh` | Install and configure Istio |
| `install-argocd.sh` | Install and configure ArgoCD |
| `wait-for-it.sh` | Wait for service availability |

### Adding a New Service

1. Create service directory: `cmd/services/myservice/`
2. Implement service using base template
3. Add Docker configuration
4. Create Kubernetes manifests
5. Update Helm charts
6. Add to Makefile

## 📚 Documentation

- [Architecture Overview](docs/architecture.md)
- [API Documentation](docs/api.md)
- [Service Implementation Guide](plan/SERVICE_IMPLEMENTATION_GUIDE.md)
- [Deployment Guide](docs/deployment.md)
- [Development Guide](docs/development.md)

## 🤝 Contributing

1. Fork the repository
2. Create feature branch
3. Commit changes
4. Push to branch
5. Create Pull Request

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🎯 Roadmap

### ✅ Completed
- [x] Core microservices architecture (15+ services)
- [x] Event-driven communication (Kafka with 20+ topics)
- [x] Service mesh integration (Istio with mTLS)
- [x] Complete CI/CD pipeline (GitHub Actions)
- [x] Monitoring stack (Prometheus + Grafana)
- [x] Distributed tracing (Jaeger + OpenTelemetry)
- [x] Log aggregation (ELK Stack + Loki)
- [x] API Gateway (Kong)
- [x] GitOps deployment (ArgoCD)
- [x] Kubernetes manifests with HPA and PDB
- [x] Docker Compose for local development

### 🚧 In Progress
- [ ] GraphQL federation
- [ ] WebAssembly node support
- [ ] Multi-region deployment
- [ ] Enterprise SSO integrations
- [ ] Advanced analytics dashboard

## 💬 Support

- GitHub Issues: [Create an issue](https://github.com/yourusername/linkflow-go/issues)
- Documentation: [Read the docs](https://docs.linkflow.io)
- Community: [Join our Discord](https://discord.gg/linkflow)

## 🙏 Acknowledgments

Built with inspiration from n8n, Temporal, and other great workflow automation tools.

---

**LinkFlow** - Enterprise-grade workflow automation at scale 🚀
