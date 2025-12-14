# Variables
DOCKER_REGISTRY ?= linkflow
VERSION ?= latest
SERVICES = auth user workflow execution executor node credential webhook schedule notification audit analytics storage search billing variable websocket
GO_FLAGS = -v -ldflags="-s -w"

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: deps
deps: ## Download dependencies
	@echo "${GREEN}Downloading dependencies...${NC}"
	@go mod download
	@go mod tidy

.PHONY: build
build: ## Build all services
	@echo "${GREEN}Building all services...${NC}"
	@for service in $(SERVICES); do \
		echo "Building $$service-service..."; \
		go build $(GO_FLAGS) -o bin/$$service-service cmd/services/$$service/main.go || true; \
	done

.PHONY: build-service
build-service: ## Build specific service (SERVICE=auth make build-service)
	@echo "${GREEN}Building $(SERVICE)-service...${NC}"
	@go build $(GO_FLAGS) -o bin/$(SERVICE)-service cmd/services/$(SERVICE)/main.go

.PHONY: docker-build
docker-build: ## Build Docker images for all services
	@echo "${GREEN}Building Docker images...${NC}"
	@for service in $(SERVICES); do \
		echo "Building $$service-service image..."; \
		docker build -t $(DOCKER_REGISTRY)/$$service-service:$(VERSION) \
			--build-arg SERVICE_NAME=$$service \
			-f deployments/docker/Dockerfile . || true; \
	done

.PHONY: docker-push
docker-push: ## Push Docker images to registry
	@echo "${GREEN}Pushing Docker images...${NC}"
	@for service in $(SERVICES); do \
		echo "Pushing $$service-service image..."; \
		docker push $(DOCKER_REGISTRY)/$$service-service:$(VERSION) || true; \
	done

# Infrastructure Commands
.PHONY: infra-up
infra-up: ## Start local infrastructure with docker-compose
	@echo "${GREEN}Starting infrastructure services...${NC}"
	@docker-compose up -d postgres redis zookeeper kafka elasticsearch prometheus grafana jaeger kong
	@echo "${GREEN}Infrastructure is starting up...${NC}"
	@echo "Run 'make infra-status' to check status"

.PHONY: infra-down
infra-down: ## Stop local infrastructure
	@echo "${YELLOW}Stopping infrastructure services...${NC}"
	@docker-compose down

.PHONY: infra-status
infra-status: ## Check infrastructure status
	@echo "${GREEN}Infrastructure Status:${NC}"
	@docker-compose ps

.PHONY: infra-logs
infra-logs: ## Show infrastructure logs
	@docker-compose logs -f

.PHONY: dev-setup
dev-setup: ## Complete development environment setup
	@echo "${GREEN}Setting up development environment...${NC}"
	@./scripts/dev/dev-setup.sh

.PHONY: kafka-setup
kafka-setup: ## Setup Kafka topics
	@echo "${GREEN}Setting up Kafka topics...${NC}"
	@./scripts/install/kafka.sh setup

.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "${GREEN}Running database migrations...${NC}"
	@./scripts/db/migrate.sh up

.PHONY: db-seed
db-seed: ## Seed database with test data
	@echo "${GREEN}Seeding database...${NC}"
	@./scripts/db/seed.sh

.PHONY: k8s-deploy
k8s-deploy: ## Deploy to Kubernetes
	@echo "${GREEN}Deploying to Kubernetes...${NC}"
	@./scripts/deploy/k8s.sh deploy

.PHONY: k8s-status
k8s-status: ## Check Kubernetes deployment status
	@./scripts/deploy/k8s.sh status

.PHONY: argocd-install
argocd-install: ## Install ArgoCD
	@echo "${GREEN}Installing ArgoCD...${NC}"
	@./scripts/install/argocd.sh install

.PHONY: argocd-setup
argocd-setup: ## Configure ArgoCD for LinkFlow
	@echo "${GREEN}Configuring ArgoCD...${NC}"
	@./scripts/install/argocd.sh configure

.PHONY: istio-install
istio-install: ## Install Istio service mesh
	@echo "${GREEN}Installing Istio...${NC}"
	@./scripts/install/istio.sh install

.PHONY: istio-setup
istio-setup: ## Configure Istio for LinkFlow
	@echo "${GREEN}Configuring Istio...${NC}"
	@./scripts/install/istio.sh apply-config

.PHONY: logging-deploy
logging-deploy: ## Deploy log aggregation stack (ELK/Loki)
	@echo "${GREEN}Deploying logging stack...${NC}"
	@kubectl apply -f deployments/monitoring/loki/

.PHONY: tracing-deploy
tracing-deploy: ## Deploy distributed tracing (Jaeger)
	@echo "${GREEN}Deploying Jaeger tracing...${NC}"
	@kubectl apply -f deployments/monitoring/jaeger/

.PHONY: test
test: ## Run unit tests
	@echo "${GREEN}Running unit tests...${NC}"
	@go test -v -cover -race ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "${GREEN}Running tests with coverage...${NC}"
	@go test -v -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "${GREEN}Running integration tests...${NC}"
	@go test -v -tags=integration ./tests/integration/...

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "${GREEN}Running e2e tests...${NC}"
	@go test -v -tags=e2e ./tests/e2e/...

.PHONY: lint
lint: ## Run linter
	@echo "${GREEN}Running linter...${NC}"
	@golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	@echo "${GREEN}Formatting code...${NC}"
	@go fmt ./...
	@gofumpt -l -w .

.PHONY: vet
vet: ## Run go vet
	@echo "${GREEN}Running go vet...${NC}"
	@go vet ./...

.PHONY: proto
proto: ## Generate protobuf files
	@echo "${GREEN}Generating protobuf files...${NC}"
	@protoc --go_out=. --go-grpc_out=. proto/*.proto

.PHONY: mocks
mocks: ## Generate mocks
	@echo "${GREEN}Generating mocks...${NC}"
	@mockgen -source=internal/domain/workflow/repository.go -destination=internal/domain/workflow/mocks/repository.go

.PHONY: migrate-up
migrate-up: ## Run database migrations up
	@./scripts/db/migrate.sh up

.PHONY: migrate-down
migrate-down: ## Run database migrations down (1 step)
	@./scripts/db/migrate.sh down 1

.PHONY: migrate-status
migrate-status: ## Show migration status
	@./scripts/db/migrate.sh status

.PHONY: migrate-reset
migrate-reset: ## Reset database (drop all and re-migrate)
	@./scripts/db/migrate.sh reset

.PHONY: run-local
run-local: ## Run services locally with docker-compose
	@echo "${GREEN}Starting services locally...${NC}"
	@docker-compose up -d

.PHONY: stop-local
stop-local: ## Stop local services
	@echo "${YELLOW}Stopping local services...${NC}"
	@docker-compose down

.PHONY: logs
logs: ## Show logs for local services
	@docker-compose logs -f

.PHONY: k8s-delete
k8s-delete: ## Delete from Kubernetes
	@echo "${RED}Deleting from Kubernetes...${NC}"
	@kubectl delete -f deployments/kubernetes/

.PHONY: helm-install
helm-install: ## Install Helm chart
	@echo "${GREEN}Installing Helm chart...${NC}"
	@helm install linkflow deployments/helm/linkflow

.PHONY: helm-upgrade
helm-upgrade: ## Upgrade Helm chart
	@echo "${GREEN}Upgrading Helm chart...${NC}"
	@helm upgrade linkflow deployments/helm/linkflow

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall Helm chart
	@echo "${RED}Uninstalling Helm chart...${NC}"
	@helm uninstall linkflow

.PHONY: clean
clean: ## Clean build artifacts
	@echo "${YELLOW}Cleaning build artifacts...${NC}"
	@rm -rf bin/ dist/ tmp/ coverage.*
	@go clean -cache

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "${GREEN}Installing development tools...${NC}"
	@go install github.com/golang/mock/mockgen@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install mvdan.cc/gofumpt@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: setup
setup: deps install-tools ## Setup development environment
	@echo "${GREEN}Development environment ready!${NC}"

.PHONY: all
all: clean deps fmt vet lint test build ## Run all checks and build
