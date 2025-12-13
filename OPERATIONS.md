# LinkFlow Operations Guide

Complete guide for deploying and managing the LinkFlow workflow automation platform.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Quick Start](#quick-start)
3. [Local Development](#local-development)
4. [Cloud Deployment](#cloud-deployment)
5. [Service Management](#service-management)
6. [Istio Service Mesh](#istio-service-mesh)
7. [Kong API Gateway](#kong-api-gateway)
8. [Kafka Event Streaming](#kafka-event-streaming)
9. [GitOps with ArgoCD](#gitops-with-argocd)
10. [Monitoring & Observability](#monitoring--observability)
11. [Scaling](#scaling)
12. [Troubleshooting](#troubleshooting)
13. [Backup & Recovery](#backup--recovery)

---

## Architecture Overview

```
                                    ┌─────────────────┐
                                    │   Clients       │
                                    │ (Web/Mobile/API)│
                                    └────────┬────────┘
                                             │
                              ┌──────────────▼──────────────┐
                              │     Istio Ingress Gateway   │
                              │    (TLS Termination)        │
                              └──────────────┬──────────────┘
                                             │
                              ┌──────────────▼──────────────┐
                              │      Kong API Gateway       │
                              │ (Rate Limit, Auth, Routing) │
                              └──────────────┬──────────────┘
                                             │
                    ┌────────────────────────┼────────────────────────┐
                    │                        │                        │
         ┌──────────▼──────────┐  ┌─────────▼─────────┐  ┌──────────▼──────────┐
         │    Auth Service     │  │ Workflow Service  │  │  Webhook Service    │
         │    (JWT, OAuth)     │  │   (CRUD, DAG)     │  │ (Incoming webhooks) │
         └──────────┬──────────┘  └─────────┬─────────┘  └──────────┬──────────┘
                    │                       │                        │
                    │              ┌────────▼────────┐               │
                    │              │Execution Service│◄──────────────┘
                    │              │ (Orchestrator)  │
                    │              └────────┬────────┘
                    │                       │
                    │              ┌────────▼────────┐
                    │              │Executor Service │ (Auto-scales 10-100)
                    │              │  (Node Runner)  │
                    │              └────────┬────────┘
                    │                       │
         ┌──────────▼───────────────────────▼──────────────────────────────┐
         │                        Kafka Event Bus                          │
         │  (workflow.events, execution.events, webhook.events, etc.)      │
         └──────────┬───────────────────────┬──────────────────────────────┘
                    │                       │
    ┌───────────────┼───────────────────────┼───────────────────┐
    │               │                       │                   │
┌───▼───┐    ┌──────▼──────┐    ┌──────────▼──────────┐   ┌────▼────┐
│Schedule│    │Notification │    │    Analytics        │   │  Audit  │
│Service │    │  Service    │    │    Service          │   │ Service │
└───┬───┘    └─────────────┘    └─────────────────────┘   └─────────┘
    │
    │         ┌─────────────────────────────────────────────────────┐
    │         │                   Data Layer                        │
    └────────►│  PostgreSQL │ Redis │ Elasticsearch │ S3 Storage   │
              └─────────────────────────────────────────────────────┘
```

### Services (16 Total)

| Service | Port | Purpose | Replicas |
|---------|------|---------|----------|
| auth | 8080 | JWT, OAuth, Sessions | 2-10 |
| user | 8080 | User management, Teams | 2-10 |
| workflow | 8080 | Workflow CRUD, Versioning | 3-15 |
| execution | 8080 | Orchestration, State machine | 5-50 |
| executor | 8080 | Node execution workers | 10-100 |
| node | 8080 | Node registry, Types | 2-10 |
| credential | 8080 | Secrets vault, OAuth tokens | 2 |
| variable | 8080 | Global variables | 2 |
| webhook | 8080 | Incoming webhooks | 3-20 |
| schedule | 8080 | Cron jobs (leader election) | 2 |
| notification | 8080 | Email, Slack, Push | 2 |
| audit | 8080 | Activity logging | 2 |
| analytics | 8080 | Metrics aggregation | 2 |
| search | 8080 | Elasticsearch integration | 2 |
| storage | 8080 | S3 file management | 2 |
| billing | 8080 | Stripe integration | 2 |

---

## Quick Start

### Prerequisites

```bash
# Required CLI tools
kubectl version          # Kubernetes CLI
helm version            # Helm package manager
istioctl version        # Istio CLI
argocd version          # ArgoCD CLI (optional)
docker --version        # Docker

# Optional
k9s                     # Kubernetes TUI
stern                   # Multi-pod log tailing
```

### One-Command Deploy

```bash
# Local (Docker Compose)
make run-local

# Kubernetes (Kustomize)
make k8s-deploy

# Kubernetes (Helm)
make helm-install
```

---

## Local Development

### Step 1: Start Infrastructure

```bash
# Start PostgreSQL, Redis, Kafka, Elasticsearch
make infra-up

# Verify all containers are running
make infra-status

# Expected output:
# postgres      running   0.0.0.0:5432->5432/tcp
# redis         running   0.0.0.0:6379->6379/tcp
# kafka         running   0.0.0.0:29092->29092/tcp
# zookeeper     running   0.0.0.0:2181->2181/tcp
# elasticsearch running   0.0.0.0:9200->9200/tcp
```

### Step 2: Run Migrations

```bash
make db-migrate
```

### Step 3: Start Services

```bash
# Option A: All services via Docker Compose
make run-local

# Option B: Run specific service for debugging
go run ./cmd/services/workflow/main.go
```

### Step 4: Access Services

| Service | Local URL |
|---------|-----------|
| Kong Gateway | http://localhost:8000 |
| Auth API | http://localhost:8001 |
| Grafana | http://localhost:3000 (admin/admin) |
| Prometheus | http://localhost:9090 |
| Jaeger | http://localhost:16686 |
| Kafka UI | http://localhost:8080 |

### Development Commands

```bash
make build          # Build all services
make test           # Run unit tests
make lint           # Run linter
make fmt            # Format code
make clean          # Clean build artifacts
```

---

## Cloud Deployment

### Deployment Options

| Method | Use Case | Command |
|--------|----------|---------|
| **ArgoCD (GitOps)** | Production - auto-deploy on git push | `kubectl apply -f deployments/argocd/` |
| **Helm** | Production - manual control | `helm install linkflow deployments/helm/linkflow` |
| **Kustomize** | Staging/Dev | `kubectl apply -k deployments/k8s/` |
| **kubectl** | Quick testing | `kubectl apply -f deployments/k8s/services/` |

### Option 1: GitOps with ArgoCD (Recommended)

```bash
# 1. Install ArgoCD (if not installed)
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# 2. Get ArgoCD admin password
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d

# 3. Deploy LinkFlow application
kubectl apply -f deployments/argocd/application.yaml

# 4. From now on, just push to git
git push origin main
# ArgoCD syncs automatically within 3 minutes
```

### Option 2: Helm

```bash
# Install
helm install linkflow deployments/helm/linkflow \
  --namespace linkflow \
  --create-namespace \
  -f deployments/helm/linkflow/values.yaml

# Upgrade
helm upgrade linkflow deployments/helm/linkflow \
  --namespace linkflow \
  -f deployments/helm/linkflow/values.yaml

# Rollback
helm rollback linkflow 1 --namespace linkflow

# Uninstall
helm uninstall linkflow --namespace linkflow
```

### Option 3: Kustomize

```bash
# Deploy all
kubectl apply -k deployments/k8s/

# Deploy specific overlay
kubectl apply -k deployments/k8s/overlays/production/

# Delete all
kubectl delete -k deployments/k8s/
```

### Post-Deployment Setup

```bash
# 1. Setup Istio
kubectl apply -f deployments/istio/

# 2. Setup Kafka topics
kubectl apply -f deployments/kafka/topics.yaml

# 3. Setup Kong routes
kubectl apply -f deployments/kong/

# 4. Setup monitoring
kubectl apply -f deployments/k8s/monitoring/
kubectl apply -f deployments/logging/
kubectl apply -f deployments/tracing/
```

---

## Service Management

### Check Service Status

```bash
# All pods
kubectl get pods -n linkflow

# Specific service
kubectl get pods -n linkflow -l app=workflow-service

# Service endpoints
kubectl get svc -n linkflow

# Service health
kubectl exec -n linkflow deploy/workflow-service -- curl -s localhost:8080/health
```

### View Logs

```bash
# Single service
kubectl logs -n linkflow -l app=workflow-service -f

# Multiple services (requires stern)
stern -n linkflow ".*-service" --tail 100

# With JSON parsing
kubectl logs -n linkflow -l app=execution-service -f | jq .
```

### Restart Service

```bash
# Rolling restart (zero downtime)
kubectl rollout restart deployment/workflow-service -n linkflow

# Check rollout status
kubectl rollout status deployment/workflow-service -n linkflow
```

### Deploy New Version

```bash
# Update image tag
kubectl set image deployment/workflow-service \
  workflow-service=linkflow/workflow-service:v1.2.3 \
  -n linkflow

# Or via Helm
helm upgrade linkflow deployments/helm/linkflow \
  --set services.workflow.image.tag=v1.2.3
```

### Rollback

```bash
# View history
kubectl rollout history deployment/workflow-service -n linkflow

# Rollback to previous
kubectl rollout undo deployment/workflow-service -n linkflow

# Rollback to specific revision
kubectl rollout undo deployment/workflow-service -n linkflow --to-revision=2
```

---

## Istio Service Mesh

### Architecture

```
Internet → Istio Gateway → VirtualService → DestinationRule → Service → Pods
```

### Key Files

| File | Purpose |
|------|---------|
| `gateway.yaml` | External traffic entry point, TLS |
| `virtual-service.yaml` | Routing rules for all 16 services |
| `destination-rule.yaml` | Load balancing, circuit breakers |
| `security.yaml` | mTLS, JWT authentication |

### Routing Configuration

All API routes are defined in `deployments/istio/virtual-service.yaml`:

```yaml
/api/v1/auth/*        → auth-service
/api/v1/users/*       → user-service
/api/v1/workflows/*   → workflow-service
/api/v1/executions/*  → execution-service
/api/v1/nodes/*       → node-service
/api/v1/credentials/* → credential-service
/api/v1/variables/*   → variable-service
/api/v1/webhooks/*    → webhook-service
/api/v1/schedules/*   → schedule-service
/webhook/*            → webhook-service (incoming)
/api/v1/billing/*     → billing-service
```

### Traffic Management

```bash
# View current routes
istioctl analyze -n linkflow

# Debug routing
istioctl proxy-config routes deploy/workflow-service -n linkflow

# View service mesh
istioctl dashboard kiali
```

### Canary Deployment

Workflow service has canary configured (95/5 split):

```bash
# Deploy canary version
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workflow-service-canary
  namespace: linkflow
spec:
  replicas: 1
  selector:
    matchLabels:
      app: workflow-service
      version: v2
  template:
    metadata:
      labels:
        app: workflow-service
        version: v2
    spec:
      containers:
      - name: workflow-service
        image: linkflow/workflow-service:v2.0.0
EOF

# Traffic automatically splits 95% v1, 5% v2
# To test canary explicitly:
curl -H "x-canary: true" https://api.linkflow.io/api/v1/workflows
```

### Circuit Breaker

Configured in `destination-rule.yaml`:

```yaml
outlierDetection:
  consecutive5xxErrors: 5    # Eject after 5 errors
  interval: 30s              # Check every 30s
  baseEjectionTime: 30s      # Eject for 30s minimum
  maxEjectionPercent: 50     # Max 50% pods ejected
```

---

## Kong API Gateway

### Configuration

Kong runs in DB-less mode with declarative config.

```bash
# View Kong config
kubectl exec -n linkflow deploy/kong-gateway -- kong config db_export

# Reload config
kubectl exec -n linkflow deploy/kong-gateway -- kong reload
```

### Rate Limiting

```yaml
# Default limits (per consumer)
Anonymous: 100 requests/minute
Authenticated: 1000 requests/minute
```

### Add Custom Plugin

```bash
kubectl exec -n linkflow deploy/kong-gateway -- \
  curl -X POST http://localhost:8001/services/workflow-service/plugins \
  --data "name=rate-limiting" \
  --data "config.minute=100"
```

---

## Kafka Event Streaming

### Topics (18 Total)

| Topic | Partitions | Retention | Purpose |
|-------|------------|-----------|---------|
| workflow.events | 10 | 7 days | Workflow lifecycle |
| workflow.triggers | 5 | 2 days | Trigger events |
| execution.events | 20 | 3 days | Execution state |
| execution.commands | 15 | 1 day | Start/stop commands |
| execution.results | 20 | 5 days | Node outputs |
| webhook.events | 10 | 3 days | Incoming webhooks |
| schedule.triggers | 5 | 1 day | Cron triggers |
| notification.events | 10 | 2 days | Notifications |
| analytics.events | 15 | 30 days | Usage metrics |
| audit.log | 5 | 30 days | Audit trail |
| dlq.events | 5 | 14 days | Dead letter queue |

### Management Commands

```bash
# List topics
kubectl exec -n linkflow kafka-0 -- \
  kafka-topics.sh --list --bootstrap-server localhost:9092

# Describe topic
kubectl exec -n linkflow kafka-0 -- \
  kafka-topics.sh --describe --topic workflow.events --bootstrap-server localhost:9092

# Check consumer lag
kubectl exec -n linkflow kafka-0 -- \
  kafka-consumer-groups.sh --describe --group linkflow-group --bootstrap-server localhost:9092

# Read messages (debugging)
kubectl exec -n linkflow kafka-0 -- \
  kafka-console-consumer.sh --topic workflow.events \
  --bootstrap-server localhost:9092 --from-beginning --max-messages 10
```

### Create Topic

```bash
kubectl exec -n linkflow kafka-0 -- \
  kafka-topics.sh --create \
  --topic new.topic \
  --partitions 10 \
  --replication-factor 3 \
  --bootstrap-server localhost:9092
```

---

## GitOps with ArgoCD

### How It Works

```
1. Developer pushes to main branch
2. ArgoCD detects change (polls every 3 min)
3. ArgoCD syncs K8s manifests from deployments/k8s/
4. Kubernetes applies changes
5. Pods roll out with zero downtime
```

### ArgoCD Commands

```bash
# Login
argocd login argocd.linkflow.io

# List apps
argocd app list

# Sync manually
argocd app sync linkflow

# View app status
argocd app get linkflow

# View sync history
argocd app history linkflow

# Rollback
argocd app rollback linkflow <revision>
```

### ArgoCD Dashboard

```bash
# Port forward to access UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Open https://localhost:8080
# Username: admin
# Password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

---

## Monitoring & Observability

### Stack Overview

| Tool | Purpose | URL |
|------|---------|-----|
| Prometheus | Metrics collection | http://prometheus.linkflow.io |
| Grafana | Dashboards | http://grafana.linkflow.io |
| Jaeger | Distributed tracing | http://jaeger.linkflow.io |
| Loki | Log aggregation | (via Grafana) |

### Key Metrics

```promql
# Request rate
sum(rate(http_requests_total{namespace="linkflow"}[5m])) by (service)

# Error rate
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100

# P99 latency
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))

# Active executions
sum(workflow_executions_active)

# Kafka consumer lag
kafka_consumer_group_lag{group="linkflow-group"}
```

### Alerts (Pre-configured)

| Alert | Condition | Severity |
|-------|-----------|----------|
| HighErrorRate | >5% errors for 5m | Critical |
| ServiceDown | 0 pods for 2m | Critical |
| HighLatency | P99 >2s for 10m | Warning |
| KafkaLag | >1000 messages | Warning |
| PodCrashLooping | >3 restarts in 10m | Warning |
| DiskPressure | >85% used | Warning |

### Access Dashboards

```bash
# Grafana
kubectl port-forward svc/grafana -n linkflow 3000:3000

# Prometheus
kubectl port-forward svc/prometheus -n linkflow 9090:9090

# Jaeger
kubectl port-forward svc/jaeger-query -n linkflow 16686:16686
```

---

## Scaling

### Auto-Scaling (HPA)

Services auto-scale based on CPU/memory:

| Service | Min | Max | Scale Trigger |
|---------|-----|-----|---------------|
| auth | 2 | 10 | CPU > 70% |
| workflow | 3 | 15 | CPU > 70% |
| execution | 5 | 50 | CPU > 70% |
| **executor** | **10** | **100** | CPU > 70% |
| webhook | 3 | 20 | CPU > 70% |

### Manual Scaling

```bash
# Scale deployment
kubectl scale deployment/executor-service --replicas=50 -n linkflow

# View HPA status
kubectl get hpa -n linkflow

# Edit HPA limits
kubectl edit hpa executor-service-hpa -n linkflow
```

### Scale Infrastructure

```bash
# PostgreSQL read replicas
kubectl scale statefulset postgres-read --replicas=4 -n linkflow

# Redis
kubectl scale statefulset redis --replicas=6 -n linkflow

# Kafka (careful - requires partition rebalancing)
# Edit statefulset, then run partition reassignment
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check pod status
kubectl describe pod -n linkflow -l app=SERVICE_NAME

# Check events
kubectl get events -n linkflow --sort-by='.lastTimestamp' | tail -20

# Common issues:
# - ImagePullBackOff: Wrong image name/tag, missing registry credentials
# - CrashLoopBackOff: Check logs, missing env vars, DB connection
# - Pending: Insufficient resources, PVC not bound
```

### Connection Issues

```bash
# Test database
kubectl run -n linkflow test-pg --rm -it --image=postgres:15 -- \
  psql -h postgres-service -U linkflow -d linkflow

# Test Redis
kubectl run -n linkflow test-redis --rm -it --image=redis:7 -- \
  redis-cli -h redis-service ping

# Test Kafka
kubectl exec -n linkflow kafka-0 -- \
  kafka-broker-api-versions.sh --bootstrap-server localhost:9092
```

### High Latency

```bash
# Check database slow queries
kubectl exec -n linkflow deploy/postgres -- \
  psql -U linkflow -c "SELECT * FROM pg_stat_activity WHERE state='active';"

# Check Redis memory
kubectl exec -n linkflow deploy/redis -- redis-cli info memory

# Check Kafka lag
kubectl exec -n linkflow kafka-0 -- \
  kafka-consumer-groups.sh --describe --group linkflow-group --bootstrap-server localhost:9092
```

### Debug Network

```bash
# Test service connectivity
kubectl exec -n linkflow deploy/auth-service -- \
  curl -s workflow-service:8080/health

# Check Istio proxy
istioctl proxy-status -n linkflow

# View Envoy config
istioctl proxy-config cluster deploy/workflow-service -n linkflow
```

---

## Backup & Recovery

### Database Backup

```bash
# Manual backup
kubectl exec -n linkflow deploy/postgres -- \
  pg_dump -U linkflow linkflow > backup-$(date +%Y%m%d).sql

# Automated (CronJob runs daily at 2 AM)
kubectl get cronjobs -n linkflow
```

### Restore Database

```bash
# Scale down services
kubectl scale deployment --all --replicas=0 -n linkflow

# Restore
kubectl exec -i -n linkflow deploy/postgres -- \
  psql -U linkflow -d linkflow < backup-20240115.sql

# Scale up
kubectl scale deployment --all --replicas=3 -n linkflow
```

### Full Disaster Recovery

```bash
# 1. Create new cluster
# 2. Install infrastructure (Kafka, PostgreSQL, Redis)
# 3. Restore database from backup
# 4. Deploy LinkFlow
helm install linkflow deployments/helm/linkflow
# 5. Update DNS to point to new cluster
```

---

## Quick Reference

### Make Commands

```bash
make help              # Show all commands
make infra-up          # Start local infrastructure
make infra-down        # Stop local infrastructure
make run-local         # Start all services locally
make build             # Build all services
make test              # Run tests
make lint              # Run linter
make docker-build      # Build Docker images
make docker-push       # Push to registry
make k8s-deploy        # Deploy to Kubernetes
make helm-install      # Install Helm chart
make helm-upgrade      # Upgrade Helm release
make logs              # View service logs
make clean             # Clean build artifacts
```

### kubectl Shortcuts

```bash
alias k=kubectl
alias kn='kubectl -n linkflow'
alias kgp='kubectl get pods -n linkflow'
alias kgs='kubectl get svc -n linkflow'
alias kl='kubectl logs -n linkflow'
alias ke='kubectl exec -it -n linkflow'
```

### Useful Commands

```bash
# Watch pods
watch kubectl get pods -n linkflow

# Port forward for debugging
kubectl port-forward -n linkflow svc/workflow-service 8080:8080

# Execute shell in pod
kubectl exec -it -n linkflow deploy/auth-service -- /bin/sh

# Copy files
kubectl cp local.txt linkflow/pod-name:/path/

# View resource usage
kubectl top pods -n linkflow
```

---

## Support & Contacts

- **Logs**: Grafana → Explore → Loki
- **Metrics**: Grafana → Dashboards → LinkFlow Overview
- **Traces**: Jaeger UI → Search by service
- **Alerts**: Check Slack #linkflow-alerts channel
