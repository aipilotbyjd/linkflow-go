# LinkFlow Operations Guide

Complete guide for deploying and managing LinkFlow workflow automation platform.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Local Development](#local-development)
3. [Production Deployment](#production-deployment)
4. [Day-to-Day Operations](#day-to-day-operations)
5. [Monitoring & Alerting](#monitoring--alerting)
6. [Troubleshooting](#troubleshooting)
7. [Scaling Guide](#scaling-guide)
8. [Backup & Recovery](#backup--recovery)

---

## Quick Start

### Prerequisites

```bash
# Required tools
- Docker & Docker Compose
- kubectl (Kubernetes CLI)
- helm (Kubernetes package manager)
- istioctl (Istio CLI)
- argocd (ArgoCD CLI) - optional
```

### One-Command Setup (Local)

```bash
# Start everything locally
make run-local

# Check status
make infra-status
```

### One-Command Deploy (Kubernetes)

```bash
# Deploy to Kubernetes
make k8s-deploy
```

---

## Local Development

### Step 1: Start Infrastructure

```bash
# Start databases, message queue, monitoring
make infra-up

# Wait for services to be ready (about 30 seconds)
make infra-status
```

### Step 2: Run Database Migrations

```bash
make db-migrate
```

### Step 3: Start Services

Option A - Docker Compose (all services):
```bash
make run-local
```

Option B - Run specific service locally:
```bash
# Terminal 1 - Auth service
go run ./cmd/services/auth/main.go

# Terminal 2 - Workflow service
go run ./cmd/services/workflow/main.go

# Terminal 3 - Execution service
go run ./cmd/services/execution/main.go
```

### Step 4: Access Services

| Service | URL |
|---------|-----|
| API Gateway | http://localhost:8000 |
| Auth API | http://localhost:8001 |
| Workflow API | http://localhost:8003 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Jaeger UI | http://localhost:16686 |

### Development Workflow

```bash
# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Build all services
make build
```

---

## Production Deployment

### Option 1: Kubernetes with ArgoCD (GitOps) - Recommended

```bash
# 1. Install ArgoCD
make argocd-install

# 2. Configure ArgoCD
make argocd-setup

# 3. Apply ArgoCD application
kubectl apply -f deployments/argocd/application.yaml

# From now on, any push to main branch auto-deploys
```

### Option 2: Kubernetes with Helm

```bash
# 1. Create namespace
kubectl create namespace linkflow

# 2. Create secrets
kubectl apply -f deployments/k8s/secrets.yaml

# 3. Install with Helm
helm install linkflow deployments/helm/linkflow \
  --namespace linkflow \
  --values deployments/helm/linkflow/values.yaml
```

### Option 3: Kubernetes with kubectl

```bash
# Apply all manifests
kubectl apply -k deployments/k8s/

# Or step by step:
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/configmap.yaml
kubectl apply -f deployments/k8s/secrets.yaml
kubectl apply -f deployments/k8s/services/
kubectl apply -f deployments/istio/
```

### Setup Istio Service Mesh

```bash
# Install Istio
make istio-install

# Apply Istio configuration
make istio-setup

# Verify
istioctl analyze -n linkflow
```

---

## Day-to-Day Operations

### Check System Health

```bash
# All pods status
kubectl get pods -n linkflow

# Services status
kubectl get svc -n linkflow

# Check specific service logs
kubectl logs -n linkflow -l app=workflow-service -f

# Check all service health endpoints
for svc in auth user workflow execution; do
  echo "=== $svc-service ==="
  kubectl exec -n linkflow deploy/$svc-service -- curl -s localhost:8080/health
done
```

### Deploy New Version

```bash
# Option 1: GitOps (automatic)
git push origin main
# ArgoCD auto-syncs within 3 minutes

# Option 2: Manual
make docker-build VERSION=v1.2.3
make docker-push VERSION=v1.2.3
kubectl set image deployment/workflow-service \
  workflow-service=linkflow/workflow-service:v1.2.3 \
  -n linkflow
```

### Rollback

```bash
# Check deployment history
kubectl rollout history deployment/workflow-service -n linkflow

# Rollback to previous version
kubectl rollout undo deployment/workflow-service -n linkflow

# Rollback to specific version
kubectl rollout undo deployment/workflow-service -n linkflow --to-revision=2
```

### Scale Services

```bash
# Manual scale
kubectl scale deployment/executor-service --replicas=20 -n linkflow

# Check HPA status
kubectl get hpa -n linkflow

# Edit HPA limits
kubectl edit hpa executor-service-hpa -n linkflow
```

### Database Operations

```bash
# Run migrations
make db-migrate

# Connect to database
kubectl exec -it -n linkflow deploy/postgres -- psql -U linkflow

# Backup database
kubectl exec -n linkflow deploy/postgres -- pg_dump -U linkflow linkflow > backup.sql
```

### Kafka Operations

```bash
# List topics
kubectl exec -n linkflow kafka-0 -- kafka-topics.sh --list --bootstrap-server localhost:9092

# Check consumer lag
kubectl exec -n linkflow kafka-0 -- kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --describe --group linkflow-group

# Create topic
kubectl exec -n linkflow kafka-0 -- kafka-topics.sh \
  --create --topic workflow.events \
  --bootstrap-server localhost:9092 \
  --partitions 10 --replication-factor 3
```

---

## Monitoring & Alerting

### Access Dashboards

| Tool | URL | Credentials |
|------|-----|-------------|
| Grafana | http://grafana.linkflow.io | admin / (from secret) |
| Prometheus | http://prometheus.linkflow.io | - |
| Jaeger | http://jaeger.linkflow.io | - |
| Kibana | http://kibana.linkflow.io | - |

### Key Metrics to Watch

```promql
# Request rate per service
sum(rate(http_requests_total{namespace="linkflow"}[5m])) by (service)

# Error rate
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100

# P99 latency
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))

# Workflow executions per minute
sum(rate(workflow_executions_total[1m]))

# Active executor workers
sum(executor_active_workers)

# Kafka consumer lag
kafka_consumer_group_lag
```

### Alert Rules

Critical alerts are configured in `deployments/k8s/monitoring/prometheus-rules.yaml`:

| Alert | Condition | Action |
|-------|-----------|--------|
| HighErrorRate | >5% errors for 5min | Check logs, scale up |
| ServiceDown | 0 ready pods for 2min | Check pod events |
| HighLatency | P99 >2s for 10min | Check DB, scale up |
| KafkaLag | >1000 messages | Scale consumers |
| DiskFull | >85% used | Expand PVC |

### View Logs

```bash
# Structured logs (JSON)
kubectl logs -n linkflow -l app=workflow-service -f | jq .

# Search logs in Kibana
# Go to: http://kibana.linkflow.io
# Index pattern: linkflow-*
# Query: service:workflow-service AND level:error
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check pod status
kubectl describe pod -n linkflow -l app=SERVICE_NAME

# Check events
kubectl get events -n linkflow --sort-by='.lastTimestamp'

# Common issues:
# - ImagePullBackOff: Check image name/tag, registry credentials
# - CrashLoopBackOff: Check logs, env vars, dependencies
# - Pending: Check node resources, PVC binding
```

### High Latency

```bash
# 1. Check database
kubectl exec -n linkflow deploy/postgres -- \
  psql -U linkflow -c "SELECT * FROM pg_stat_activity WHERE state='active';"

# 2. Check Redis
kubectl exec -n linkflow deploy/redis -- redis-cli info stats

# 3. Check Kafka lag
kubectl exec -n linkflow kafka-0 -- kafka-consumer-groups.sh \
  --describe --group linkflow-group --bootstrap-server localhost:9092

# 4. Check service metrics
curl http://SERVICE:8080/metrics | grep http_request_duration
```

### Workflow Execution Stuck

```bash
# 1. Check execution status
kubectl exec -n linkflow deploy/execution-service -- \
  curl localhost:8080/api/v1/executions/EXEC_ID

# 2. Check executor workers
kubectl logs -n linkflow -l app=executor-service --tail=100 | grep EXEC_ID

# 3. Check Kafka events
kubectl exec -n linkflow kafka-0 -- kafka-console-consumer.sh \
  --topic execution.events \
  --bootstrap-server localhost:9092 \
  --from-beginning | grep EXEC_ID
```

### Database Connection Issues

```bash
# Test connection
kubectl run -n linkflow test-pg --rm -it --image=postgres:15 -- \
  psql -h postgres-service -U linkflow -d linkflow

# Check connection pool
kubectl exec -n linkflow deploy/workflow-service -- \
  curl localhost:8080/metrics | grep db_connections
```

---

## Scaling Guide

### Service Scaling Matrix

| Service | Min | Max | Scale Trigger |
|---------|-----|-----|---------------|
| auth | 2 | 10 | CPU > 70% |
| workflow | 3 | 15 | CPU > 70% |
| execution | 5 | 50 | CPU > 70% |
| **executor** | **10** | **100** | CPU > 70%, Queue depth |
| webhook | 3 | 20 | Request rate |
| schedule | 2 | 2 | Leader election (don't scale) |

### Scale Executor Workers

```bash
# Update HPA
kubectl patch hpa executor-service-hpa -n linkflow -p \
  '{"spec":{"maxReplicas":200}}'

# Or edit Helm values
# deployments/helm/linkflow/values.yaml
services:
  executor:
    autoscaling:
      maxReplicas: 200
```

### Scale Infrastructure

```bash
# Scale PostgreSQL read replicas
kubectl scale statefulset postgres-read -n linkflow --replicas=4

# Scale Redis cluster
kubectl scale statefulset redis -n linkflow --replicas=6

# Scale Kafka brokers (careful!)
# Edit kafka statefulset, then rebalance partitions
```

---

## Backup & Recovery

### Automated Backups

Backups run daily at 2 AM UTC via CronJob:

```bash
# Check backup status
kubectl get cronjobs -n linkflow

# Manual backup
kubectl create job --from=cronjob/linkflow-backup manual-backup-$(date +%Y%m%d) -n linkflow

# List backups
kubectl exec -n linkflow deploy/backup -- ls -la /backups/
```

### Restore from Backup

```bash
# 1. Scale down services
kubectl scale deployment --all --replicas=0 -n linkflow

# 2. Restore PostgreSQL
kubectl exec -n linkflow deploy/postgres -- \
  psql -U linkflow -d linkflow < /backups/linkflow-2024-01-15.sql

# 3. Scale up services
kubectl scale deployment --all --replicas=3 -n linkflow
```

### Disaster Recovery

```bash
# Full cluster restore procedure:

# 1. Create new cluster
eksctl create cluster --name linkflow-dr --region us-west-2

# 2. Install dependencies
helm install postgresql bitnami/postgresql
helm install redis bitnami/redis
helm install kafka bitnami/kafka

# 3. Restore data from S3 backup
aws s3 cp s3://linkflow-backups/latest/ /tmp/backup/ --recursive
kubectl cp /tmp/backup/postgres.sql linkflow/postgres-0:/tmp/
kubectl exec -n linkflow postgres-0 -- psql -U linkflow -f /tmp/postgres.sql

# 4. Deploy LinkFlow
helm install linkflow deployments/helm/linkflow

# 5. Update DNS
# Point linkflow.io to new cluster load balancer
```

---

## Quick Reference

### Make Commands

```bash
make help              # Show all commands
make infra-up          # Start local infrastructure
make run-local         # Start all services locally
make build             # Build all services
make test              # Run tests
make docker-build      # Build Docker images
make k8s-deploy        # Deploy to Kubernetes
make helm-install      # Install Helm chart
make logs              # View logs
make clean             # Clean build artifacts
```

### Useful kubectl Commands

```bash
# Get all resources
kubectl get all -n linkflow

# Watch pods
kubectl get pods -n linkflow -w

# Port forward for debugging
kubectl port-forward -n linkflow svc/workflow-service 8080:8080

# Execute command in pod
kubectl exec -it -n linkflow deploy/auth-service -- /bin/sh

# Copy files
kubectl cp local-file.txt linkflow/pod-name:/path/to/file
```

### Environment Variables

All services use these environment variables (set via ConfigMap):

| Variable | Description | Default |
|----------|-------------|---------|
| DATABASE_HOST | PostgreSQL host | postgres-service |
| REDIS_HOST | Redis host | redis-service |
| KAFKA_BROKERS | Kafka brokers | kafka-service:9092 |
| LOG_LEVEL | Log level | info |
| SERVICE_PORT | HTTP port | 8080 |

---

## Support

- **Logs**: Kibana at http://kibana.linkflow.io
- **Metrics**: Grafana at http://grafana.linkflow.io
- **Traces**: Jaeger at http://jaeger.linkflow.io
- **Alerts**: Check PagerDuty/Slack channels
