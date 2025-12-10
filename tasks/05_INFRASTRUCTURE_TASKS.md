# 🏗️ Infrastructure & DevOps Tasks

## Prerequisites
- Docker and Docker Compose installed
- Kubernetes cluster available (local or cloud)
- Helm 3.x installed
- kubectl configured

---

## INFRA-001: Setup Local Development Environment
**Priority**: P0 | **Hours**: 3 | **Dependencies**: None

### Context
Complete local development environment with all services running via docker-compose.

### Implementation
**Files to modify:**
- `docker-compose.yml`
- `docker-compose.override.yml` (create for local overrides)
- `scripts/dev-setup.sh` (create)
- `.env.example` (create)

### Steps
1. Verify docker-compose services:
```yaml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: linkflow
      POSTGRES_USER: linkflow
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U linkflow"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]

  kafka:
    image: confluentinc/cp-kafka:7.4.0
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
```

2. Create development setup script:
```bash
#!/bin/bash
# scripts/dev-setup.sh

# Start infrastructure
docker-compose up -d postgres redis kafka elasticsearch

# Wait for services
./scripts/wait-for-it.sh localhost:5432
./scripts/wait-for-it.sh localhost:6379

# Run migrations
./scripts/migrate.sh up

# Seed development data
./scripts/seed.sh

echo "Development environment ready!"
```

3. Health checks for all services
4. Volume management
5. Network configuration

### Testing
```bash
# Start all services
docker-compose up -d

# Verify health
docker-compose ps

# Check logs
docker-compose logs -f postgres

# Test connectivity
psql -h localhost -U linkflow -d linkflow
redis-cli ping
```

### Acceptance Criteria
- ✅ All services start successfully
- ✅ Health checks pass
- ✅ Data persists between restarts
- ✅ Services accessible from host
- ✅ Setup script works

---

## INFRA-002: Deploy Kafka & Configure Topics
**Priority**: P0 | **Hours**: 3 | **Dependencies**: INFRA-001

### Context
Apache Kafka setup with proper topic configuration for event-driven architecture.

### Implementation
**Files to modify:**
- `deployments/kafka/topics.yaml` (create)
- `scripts/kafka-setup.sh` (create)
- `configs/kafka/server.properties` (create)

### Steps
1. Create topic configuration:
```yaml
# deployments/kafka/topics.yaml
topics:
  - name: workflow.events
    partitions: 10
    replication-factor: 3
    config:
      retention.ms: 604800000  # 7 days
      compression.type: snappy

  - name: execution.events
    partitions: 20
    replication-factor: 3
    config:
      retention.ms: 259200000  # 3 days

  - name: audit.log
    partitions: 5
    replication-factor: 3
    config:
      retention.ms: 2592000000  # 30 days
      compression.type: gzip
```

2. Setup script:
```bash
#!/bin/bash
# scripts/kafka-setup.sh

# Create topics
kafka-topics.sh --create \
  --bootstrap-server localhost:9092 \
  --topic workflow.events \
  --partitions 10 \
  --replication-factor 1

# List topics
kafka-topics.sh --list \
  --bootstrap-server localhost:9092

# Describe topic
kafka-topics.sh --describe \
  --bootstrap-server localhost:9092 \
  --topic workflow.events
```

3. Configure producers/consumers
4. Setup Kafka Connect
5. Monitor lag and throughput

### Testing
```bash
# Produce test message
echo "test message" | kafka-console-producer.sh \
  --broker-list localhost:9092 \
  --topic workflow.events

# Consume messages
kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic workflow.events \
  --from-beginning
```

### Acceptance Criteria
- ✅ Kafka cluster running
- ✅ All topics created
- ✅ Producers can send
- ✅ Consumers can receive
- ✅ Monitoring configured

---

## INFRA-003: Setup Kubernetes Deployments
**Priority**: P0 | **Hours**: 5 | **Dependencies**: None

### Context
Deploy all microservices to Kubernetes with proper configuration.

### Implementation
**Files to modify:**
- `deployments/k8s/namespace.yaml`
- `deployments/k8s/*/deployment.yaml` (update all)
- `deployments/k8s/*/service.yaml` (update all)
- `deployments/k8s/configmap.yaml` (create)

### Steps
1. Create namespace and common resources:
```yaml
# deployments/k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: linkflow
---
# deployments/k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: linkflow-config
  namespace: linkflow
data:
  DATABASE_HOST: postgres-service
  REDIS_HOST: redis-service
  KAFKA_BROKERS: kafka-service:9092
```

2. Update deployment manifests:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  namespace: linkflow
spec:
  replicas: 3
  selector:
    matchLabels:
      app: auth-service
  template:
    spec:
      containers:
      - name: auth-service
        image: linkflow/auth-service:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: linkflow-config
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
```

3. Apply all manifests
4. Setup ingress rules
5. Configure HPA

### Testing
```bash
# Apply manifests
kubectl apply -f deployments/k8s/

# Check deployments
kubectl get deployments -n linkflow

# Check pods
kubectl get pods -n linkflow

# Check services
kubectl get services -n linkflow

# Test service
kubectl port-forward -n linkflow svc/auth-service 8080:8080
curl localhost:8080/health
```

### Acceptance Criteria
- ✅ All services deployed
- ✅ Pods running and healthy
- ✅ Services accessible
- ✅ ConfigMaps working
- ✅ Resource limits set

---

## INFRA-004: Configure API Gateway (Kong/Traefik)
**Priority**: P0 | **Hours**: 4 | **Dependencies**: INFRA-003

### Context
API Gateway for routing, rate limiting, authentication, and API management.

### Implementation
**Files to modify:**
- `deployments/kong/kong.yaml` (create)
- `deployments/kong/routes.yaml` (create)
- `deployments/kong/plugins.yaml` (create)

### Steps
1. Deploy Kong:
```yaml
# deployments/kong/kong.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kong-gateway
  namespace: linkflow
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: kong
        image: kong:3.3
        env:
        - name: KONG_DATABASE
          value: "postgres"
        - name: KONG_PG_HOST
          value: postgres-service
        - name: KONG_PROXY_ACCESS_LOG
          value: "/dev/stdout"
        ports:
        - containerPort: 8000  # Proxy
        - containerPort: 8001  # Admin
```

2. Configure routes:
```yaml
# Route configuration
apiVersion: configuration.konghq.com/v1
kind: KongIngress
metadata:
  name: auth-route
route:
  paths:
  - /api/auth
  strip_path: true
upstream:
  host: auth-service.linkflow.svc.cluster.local
```

3. Add plugins:
```yaml
# Rate limiting plugin
apiVersion: configuration.konghq.com/v1
kind: KongPlugin
metadata:
  name: rate-limit
config:
  minute: 100
  policy: local
```

4. JWT validation
5. Request/response transformation

### Testing
```bash
# Test gateway routing
curl http://kong-gateway/api/auth/health

# Test rate limiting
for i in {1..200}; do curl http://kong-gateway/api/test; done

# Check metrics
curl http://kong-gateway:8001/status
```

### Acceptance Criteria
- ✅ Gateway routes correctly
- ✅ Rate limiting works
- ✅ JWT validation active
- ✅ Metrics exposed
- ✅ High availability

---

## INFRA-005: Setup Service Mesh (Istio)
**Priority**: P1 | **Hours**: 5 | **Dependencies**: INFRA-003

### Context
Istio service mesh for traffic management, security, and observability.

### Implementation
**Files to modify:**
- `deployments/istio/namespace.yaml`
- `deployments/istio/virtual-service.yaml`
- `deployments/istio/destination-rule.yaml`
- `deployments/istio/gateway.yaml`

### Steps
1. Install Istio:
```bash
# Install Istio
istioctl install --set profile=production

# Enable sidecar injection
kubectl label namespace linkflow istio-injection=enabled

# Restart pods to inject sidecars
kubectl rollout restart deployment -n linkflow
```

2. Configure traffic management:
```yaml
# deployments/istio/virtual-service.yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: workflow-service
  namespace: linkflow
spec:
  hosts:
  - workflow-service
  http:
  - match:
    - headers:
        canary:
          exact: "true"
    route:
    - destination:
        host: workflow-service
        subset: v2
      weight: 100
  - route:
    - destination:
        host: workflow-service
        subset: v1
      weight: 90
    - destination:
        host: workflow-service
        subset: v2
      weight: 10
```

3. mTLS configuration
4. Circuit breakers
5. Retry policies

### Testing
```bash
# Check sidecar injection
kubectl get pods -n linkflow -o jsonpath='{.items[*].spec.containers[*].name}'

# Test traffic routing
curl -H "canary: true" http://gateway/api/workflow

# Check metrics
kubectl -n istio-system port-forward svc/prometheus 9090:9090
```

### Acceptance Criteria
- ✅ Sidecars injected
- ✅ mTLS enabled
- ✅ Traffic routing works
- ✅ Circuit breakers active
- ✅ Telemetry collected

---

## INFRA-006: Setup CI/CD Pipeline
**Priority**: P1 | **Hours**: 4 | **Dependencies**: None

### Context
Complete CI/CD pipeline with GitHub Actions for automated testing and deployment.

### Implementation
**Files to modify:**
- `.github/workflows/ci.yml`
- `.github/workflows/cd.yml` (create)
- `.github/workflows/release.yml` (create)
- `scripts/ci/` (create CI scripts)

### Steps
1. Enhance CI pipeline:
```yaml
# .github/workflows/ci.yml
name: CI Pipeline
on:
  push:
    branches: [main, develop]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
    
    steps:
    - uses: actions/checkout@v3
    
    - uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run tests
      run: |
        go test -v -race -coverprofile=coverage.out ./...
        go tool cover -html=coverage.out -o coverage.html
    
    - name: Run linting
      uses: golangci/golangci-lint-action@v3
    
    - name: Security scan
      run: |
        go install github.com/securego/gosec/v2/cmd/gosec@latest
        gosec ./...
    
    - name: Build services
      run: make build-all
```

2. CD pipeline:
```yaml
# .github/workflows/cd.yml
name: CD Pipeline
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - name: Build and push Docker images
      run: |
        docker buildx build --platform linux/amd64,linux/arm64 \
          -t linkflow/auth-service:${{ github.sha }} \
          -f cmd/services/auth/Dockerfile \
          --push .
    
    - name: Deploy to Kubernetes
      run: |
        kubectl set image deployment/auth-service \
          auth-service=linkflow/auth-service:${{ github.sha }} \
          -n linkflow
```

3. Release automation
4. Rollback mechanism
5. Environment promotion

### Acceptance Criteria
- ✅ CI runs on all PRs
- ✅ Tests must pass
- ✅ Security scanning active
- ✅ Auto-deploy to staging
- ✅ Manual promotion to prod

---

## INFRA-007: Setup Monitoring Stack (Prometheus/Grafana)
**Priority**: P1 | **Hours**: 4 | **Dependencies**: INFRA-003

### Context
Complete observability stack with metrics, dashboards, and alerting.

### Implementation
**Files to modify:**
- `deployments/monitoring/prometheus.yaml`
- `deployments/monitoring/grafana.yaml`
- `configs/prometheus/prometheus.yml`
- `configs/grafana/dashboards/*.json`

### Steps
1. Deploy Prometheus:
```yaml
# configs/prometheus/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
    - role: pod
      namespaces:
        names:
        - linkflow
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      action: keep
      regex: true
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
      action: replace
      target_label: __metrics_path__
      regex: (.+)
```

2. Create Grafana dashboards:
```json
{
  "dashboard": {
    "title": "LinkFlow Services",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total{status=~\"5..\"}[5m])"
          }
        ]
      }
    ]
  }
}
```

3. Alert rules
4. Service discovery
5. Long-term storage

### Testing
```bash
# Access Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Access Grafana
kubectl port-forward -n monitoring svc/grafana 3000:3000

# Test metrics endpoint
curl localhost:8080/metrics
```

### Acceptance Criteria
- ✅ All services scraped
- ✅ Dashboards display data
- ✅ Alerts configured
- ✅ Historical data retained
- ✅ Performance acceptable

---

## INFRA-008: Setup Log Aggregation (ELK/Loki)
**Priority**: P2 | **Hours**: 4 | **Dependencies**: INFRA-003

### Context
Centralized logging with search, analysis, and visualization capabilities.

### Implementation
**Files to modify:**
- `deployments/logging/elasticsearch.yaml` (create)
- `deployments/logging/logstash.yaml` (create)
- `deployments/logging/kibana.yaml` (create)
- `deployments/logging/filebeat.yaml` (create)

### Steps
1. Deploy ELK stack:
```yaml
# deployments/logging/filebeat.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: filebeat
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: filebeat
        image: elastic/filebeat:8.8.0
        volumeMounts:
        - name: containers
          mountPath: /var/lib/docker/containers
          readOnly: true
        - name: config
          mountPath: /usr/share/filebeat/filebeat.yml
          subPath: filebeat.yml
```

2. Filebeat configuration:
```yaml
filebeat.inputs:
- type: container
  paths:
    - /var/lib/docker/containers/*/*.log
  processors:
  - add_kubernetes_metadata:
      in_cluster: true

output.elasticsearch:
  hosts: ['elasticsearch:9200']
  index: "linkflow-%{+yyyy.MM.dd}"
```

3. Log parsing rules
4. Index lifecycle management
5. Kibana dashboards

### Acceptance Criteria
- ✅ Logs collected from all pods
- ✅ Logs searchable in Kibana
- ✅ Log retention policy active
- ✅ Dashboards created
- ✅ Performance acceptable

---

## INFRA-009: Setup Distributed Tracing (Jaeger)
**Priority**: P2 | **Hours**: 3 | **Dependencies**: INFRA-005

### Context
Distributed tracing for request flow visualization and performance analysis.

### Implementation
**Files to modify:**
- `deployments/tracing/jaeger.yaml` (create)
- `pkg/telemetry/tracing.go` (update)
- Service code to add spans

### Steps
1. Deploy Jaeger:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: linkflow
spec:
  template:
    spec:
      containers:
      - name: jaeger
        image: jaegertracing/all-in-one:1.46
        ports:
        - containerPort: 16686  # UI
        - containerPort: 14268  # Collector
        environment:
        - name: COLLECTOR_ZIPKIN_HOST_PORT
          value: ":9411"
```

2. Instrument services:
```go
// pkg/telemetry/tracing.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
)

func InitTracing(endpoint string) (*TracerProvider, error) {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint(endpoint),
        ),
    )
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String("linkflow"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

3. Add spans to critical paths
4. Context propagation
5. Performance analysis

### Acceptance Criteria
- ✅ Traces collected
- ✅ Service dependencies visible
- ✅ Latency analysis works
- ✅ Error traces captured
- ✅ Sampling configured

---

## INFRA-010: Setup GitOps with ArgoCD
**Priority**: P2 | **Hours**: 3 | **Dependencies**: INFRA-003, INFRA-006

### Context
GitOps deployment model with ArgoCD for declarative, versioned deployments.

### Implementation
**Files to modify:**
- `deployments/argocd/install.yaml` (create)
- `deployments/argocd/application.yaml`
- `deployments/argocd/project.yaml` (create)

### Steps
1. Install ArgoCD:
```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

2. Configure application:
```yaml
# deployments/argocd/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: linkflow
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/linkflow-go
    targetRevision: HEAD
    path: deployments/k8s
  destination:
    server: https://kubernetes.default.svc
    namespace: linkflow
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

3. Setup sync policies
4. Configure webhooks
5. Multi-environment setup

### Acceptance Criteria
- ✅ ArgoCD deployed
- ✅ Auto-sync enabled
- ✅ Rollback works
- ✅ Multi-env configured
- ✅ Notifications setup

---

## Summary Stats
- **Total Tasks**: 10
- **Total Hours**: 38
- **Critical (P0)**: 4
- **High (P1)**: 4
- **Medium (P2)**: 2

## Execution Order
1. INFRA-001, INFRA-002 (foundation)
2. INFRA-003, INFRA-006 (parallel)
3. INFRA-004, INFRA-005, INFRA-007 (parallel)
4. INFRA-008, INFRA-009, INFRA-010 (parallel)

## Team Assignment Suggestion
- **DevOps Lead**: INFRA-003, INFRA-005, INFRA-010
- **DevOps Eng 1**: INFRA-001, INFRA-002, INFRA-004
- **DevOps Eng 2**: INFRA-006, INFRA-007
- **SRE**: INFRA-008, INFRA-009
