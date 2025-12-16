# ==============================================================================
# LinkFlow Go - Makefile
# ==============================================================================
# Usage: make [target]
# Run 'make help' to see all available targets
# ==============================================================================

# ------------------------------------------------------------------------------
# Variables
# ------------------------------------------------------------------------------
APP_NAME        := linkflow
DOCKER_REGISTRY ?= linkflow
VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_HASH     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME      := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Services list
SERVICES := auth user workflow execution executor node credential webhook \
            schedule notification audit analytics storage search billing \
            variable websocket

# Go build flags
GO_VERSION := $(shell go version | cut -d ' ' -f 3)
GO_FLAGS   := -v -trimpath
GO_LDFLAGS := -s -w \
              -X main.Version=$(VERSION) \
              -X main.CommitHash=$(COMMIT_HASH) \
              -X main.BuildTime=$(BUILD_TIME)

# Tools (prefer local PATH, fall back to GOPATH/bin)
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null)
ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
endif

# Directories
BIN_DIR      := bin
DIST_DIR     := dist
COVERAGE_DIR := coverage

# Database configuration (can be overridden)
DB_HOST     ?= localhost
DB_PORT     ?= 5432
DB_NAME     ?= linkflow
DB_USER     ?= linkflow
DB_PASSWORD ?= linkflow123

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
BLUE   := \033[0;34m
CYAN   := \033[0;36m
NC     := \033[0m

# ==============================================================================
# HELP
# ==============================================================================
.PHONY: help
help: ## Show this help message
	@echo ''
	@echo '$(CYAN)LinkFlow Go - Development Commands$(NC)'
	@echo ''
	@echo '$(YELLOW)Usage:$(NC) make [target]'
	@echo ''
	@echo '$(YELLOW)Available targets:$(NC)'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo '$(YELLOW)Examples:$(NC)'
	@echo '  make dev              # Start full development environment'
	@echo '  make build            # Build all services'
	@echo '  make test             # Run all tests'
	@echo '  make migrate-up       # Apply database migrations'
	@echo ''

# ==============================================================================
# DEVELOPMENT SETUP
# ==============================================================================
.PHONY: setup dev dev-setup install-tools deps

setup: deps install-tools ## Complete development environment setup
	@echo "$(GREEN)Development environment ready!$(NC)"
	@echo "Run 'make dev' to start local infrastructure"

dev: infra-up migrate-up ## Start full development environment
	@echo "$(GREEN)Development environment is ready!$(NC)"
	@echo "Database migrations applied."
	@echo "Run 'make run' to start services."

dev-setup: ## Run development setup script
	@echo "$(GREEN)Setting up development environment...$(NC)"
	@./scripts/dev/dev-setup.sh

install-tools: ## Install required development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	@go install github.com/golang/mock/mockgen@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install mvdan.cc/gofumpt@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "$(GREEN)Tools installed successfully!$(NC)"

deps: ## Download and tidy Go dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@go mod verify

# ==============================================================================
# BUILD
# ==============================================================================
.PHONY: build build-service build-all clean

build: ## Build all services
	@echo "$(GREEN)Building all services...$(NC)"
	@mkdir -p $(BIN_DIR)
	@for service in $(SERVICES); do \
		if [ -f cmd/services/$$service/main.go ]; then \
			echo "  Building $$service-service..."; \
			go build $(GO_FLAGS) -ldflags="$(GO_LDFLAGS)" \
				-o $(BIN_DIR)/$$service-service \
				cmd/services/$$service/main.go; \
		fi \
	done
	@echo "$(GREEN)Build complete!$(NC)"

build-service: ## Build specific service (usage: make build-service SERVICE=auth)
ifndef SERVICE
	$(error SERVICE is required. Usage: make build-service SERVICE=auth)
endif
	@echo "$(GREEN)Building $(SERVICE)-service...$(NC)"
	@mkdir -p $(BIN_DIR)
	@go build $(GO_FLAGS) -ldflags="$(GO_LDFLAGS)" \
		-o $(BIN_DIR)/$(SERVICE)-service \
		cmd/services/$(SERVICE)/main.go
	@echo "$(GREEN)Build complete: $(BIN_DIR)/$(SERVICE)-service$(NC)"

build-all: clean build ## Clean and rebuild all services

clean: ## Clean build artifacts and caches
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BIN_DIR) $(DIST_DIR) $(COVERAGE_DIR)
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache
	@echo "$(GREEN)Clean complete!$(NC)"

# ==============================================================================
# TESTING
# ==============================================================================
.PHONY: test test-unit test-integration test-e2e test-coverage test-race test-short

test: test-unit ## Run all unit tests

test-unit: ## Run unit tests
	@echo "$(GREEN)Running unit tests...$(NC)"
	@go test -v -cover ./...

test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	@go test -v -tags=integration ./tests/integration/...

test-e2e: ## Run end-to-end tests
	@echo "$(GREEN)Running e2e tests...$(NC)"
	@go test -v -tags=e2e ./tests/e2e/...

test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	@go test -v -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1
	@echo "$(GREEN)Coverage report: $(COVERAGE_DIR)/coverage.html$(NC)"

test-race: ## Run tests with race detector
	@echo "$(GREEN)Running tests with race detector...$(NC)"
	@go test -v -race ./...

test-short: ## Run quick tests only
	@echo "$(GREEN)Running short tests...$(NC)"
	@go test -v -short ./...

# ==============================================================================
# CODE QUALITY
# ==============================================================================
.PHONY: lint fmt vet check sec

lint: ## Run linter (golangci-lint)
	@echo "$(GREEN)Running linter...$(NC)"
	@$(GOLANGCI_LINT) run ./...

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	@go fmt ./...
	@gofumpt -l -w . 2>/dev/null || true

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...

check: fmt vet lint ## Run all code quality checks

sec: ## Run security scanner (gosec)
	@echo "$(GREEN)Running security scanner...$(NC)"
	@gosec -quiet ./... 2>/dev/null || echo "$(YELLOW)Install gosec: go install github.com/securego/gosec/v2/cmd/gosec@latest$(NC)"

# ==============================================================================
# CODE GENERATION
# ==============================================================================
.PHONY: generate proto mocks

generate: proto mocks ## Generate all code (proto, mocks)
	@go generate ./...

proto: ## Generate protobuf files
	@echo "$(GREEN)Generating protobuf files...$(NC)"
	@if [ -d proto ]; then \
		protoc --go_out=. --go-grpc_out=. proto/*.proto; \
	else \
		echo "$(YELLOW)No proto directory found$(NC)"; \
	fi

mocks: ## Generate mocks
	@echo "$(GREEN)Generating mocks...$(NC)"
	@go generate ./...

# ==============================================================================
# DATABASE MIGRATIONS
# ==============================================================================
.PHONY: migrate-up migrate-down migrate-status migrate-version migrate-create migrate-reset migrate-force migrate-goto

migrate-up: ## Apply all pending migrations
	@echo "$(GREEN)Running migrations up...$(NC)"
	@./scripts/db/migrate.sh up

migrate-down: ## Rollback last migration
	@echo "$(YELLOW)Rolling back last migration...$(NC)"
	@./scripts/db/migrate.sh down 1

migrate-status: ## Show migration status
	@./scripts/db/migrate.sh status

migrate-version: ## Show current migration version
	@./scripts/db/migrate.sh version

migrate-create: ## Create new migration (usage: make migrate-create NAME=add_users)
ifndef NAME
	$(error NAME is required. Usage: make migrate-create NAME=add_users)
endif
	@echo "$(GREEN)Creating migration: $(NAME)$(NC)"
	@./scripts/db/migrate.sh create $(NAME)

migrate-reset: ## Reset database (drop all + migrate) [DANGER!]
	@echo "$(RED)WARNING: This will destroy all data!$(NC)"
	@./scripts/db/migrate.sh reset

migrate-force: ## Force migration version (usage: make migrate-force V=16)
ifndef V
	$(error V is required. Usage: make migrate-force V=16)
endif
	@./scripts/db/migrate.sh force $(V)

migrate-goto: ## Migrate to specific version (usage: make migrate-goto V=10)
ifndef V
	$(error V is required. Usage: make migrate-goto V=10)
endif
	@./scripts/db/migrate.sh goto $(V)

migrate-drop: ## Drop all schemas [DANGER!]
	@echo "$(RED)WARNING: This will destroy all data!$(NC)"
	@./scripts/db/migrate.sh drop

# ==============================================================================
# DOCKER
# ==============================================================================
.PHONY: docker-build docker-push docker-build-service

docker-build: ## Build Docker images for all services
	@echo "$(GREEN)Building Docker images...$(NC)"
	@for service in $(SERVICES); do \
		if [ -f cmd/services/$$service/main.go ]; then \
			echo "  Building $(DOCKER_REGISTRY)/$$service-service:$(VERSION)"; \
			docker build -t $(DOCKER_REGISTRY)/$$service-service:$(VERSION) \
				--build-arg SERVICE_NAME=$$service \
				--build-arg VERSION=$(VERSION) \
				--build-arg COMMIT_HASH=$(COMMIT_HASH) \
				-f deployments/docker/Dockerfile . || true; \
		fi \
	done

docker-push: ## Push Docker images to registry
	@echo "$(GREEN)Pushing Docker images...$(NC)"
	@for service in $(SERVICES); do \
		echo "  Pushing $(DOCKER_REGISTRY)/$$service-service:$(VERSION)"; \
		docker push $(DOCKER_REGISTRY)/$$service-service:$(VERSION) || true; \
	done

docker-build-service: ## Build Docker image for specific service
ifndef SERVICE
	$(error SERVICE is required. Usage: make docker-build-service SERVICE=auth)
endif
	@echo "$(GREEN)Building Docker image: $(DOCKER_REGISTRY)/$(SERVICE)-service:$(VERSION)$(NC)"
	@docker build -t $(DOCKER_REGISTRY)/$(SERVICE)-service:$(VERSION) \
		--build-arg SERVICE_NAME=$(SERVICE) \
		--build-arg VERSION=$(VERSION) \
		-f deployments/docker/Dockerfile .

# ==============================================================================
# LOCAL DEVELOPMENT (Docker Compose)
# ==============================================================================
.PHONY: run stop logs infra-up infra-down infra-status infra-logs

run: ## Start all services locally
	@echo "$(GREEN)Starting services locally...$(NC)"
	@docker-compose up -d

stop: ## Stop all local services
	@echo "$(YELLOW)Stopping local services...$(NC)"
	@docker-compose down

logs: ## Show logs for local services
	@docker-compose logs -f

infra-up: ## Start infrastructure only (DB, Redis, Kafka, etc.)
	@echo "$(GREEN)Starting infrastructure services...$(NC)"
	@docker-compose up -d postgres redis zookeeper kafka elasticsearch prometheus grafana jaeger kong
	@echo "$(GREEN)Infrastructure starting... Run 'make infra-status' to check$(NC)"

infra-down: ## Stop infrastructure
	@echo "$(YELLOW)Stopping infrastructure services...$(NC)"
	@docker-compose down

infra-status: ## Check infrastructure status
	@echo "$(CYAN)Infrastructure Status:$(NC)"
	@docker-compose ps

infra-logs: ## Show infrastructure logs
	@docker-compose logs -f postgres redis kafka

# ==============================================================================
# KUBERNETES
# ==============================================================================
.PHONY: k8s-deploy k8s-delete k8s-status k8s-logs

k8s-deploy: ## Deploy to Kubernetes
	@echo "$(GREEN)Deploying to Kubernetes...$(NC)"
	@./scripts/deploy/k8s.sh deploy

k8s-delete: ## Delete from Kubernetes
	@echo "$(RED)Deleting from Kubernetes...$(NC)"
	@kubectl delete -f deployments/kubernetes/ --ignore-not-found

k8s-status: ## Check Kubernetes deployment status
	@./scripts/deploy/k8s.sh status

k8s-logs: ## Show Kubernetes logs (usage: make k8s-logs SERVICE=auth)
ifdef SERVICE
	@kubectl logs -f -l app=$(SERVICE)-service
else
	@kubectl logs -f -l app.kubernetes.io/part-of=linkflow
endif

# ==============================================================================
# HELM
# ==============================================================================
.PHONY: helm-install helm-upgrade helm-uninstall helm-template helm-lint

helm-install: ## Install Helm chart
	@echo "$(GREEN)Installing Helm chart...$(NC)"
	@helm install $(APP_NAME) deployments/helm/linkflow \
		--set image.tag=$(VERSION)

helm-upgrade: ## Upgrade Helm chart
	@echo "$(GREEN)Upgrading Helm chart...$(NC)"
	@helm upgrade $(APP_NAME) deployments/helm/linkflow \
		--set image.tag=$(VERSION)

helm-uninstall: ## Uninstall Helm chart
	@echo "$(RED)Uninstalling Helm chart...$(NC)"
	@helm uninstall $(APP_NAME)

helm-template: ## Render Helm templates locally
	@helm template $(APP_NAME) deployments/helm/linkflow

helm-lint: ## Lint Helm chart
	@helm lint deployments/helm/linkflow

# ==============================================================================
# INFRASTRUCTURE SETUP
# ==============================================================================
.PHONY: kafka-setup istio-install istio-setup argocd-install argocd-setup

kafka-setup: ## Setup Kafka topics
	@echo "$(GREEN)Setting up Kafka topics...$(NC)"
	@./scripts/install/kafka.sh setup

istio-install: ## Install Istio service mesh
	@echo "$(GREEN)Installing Istio...$(NC)"
	@./scripts/install/istio.sh install

istio-setup: ## Configure Istio for LinkFlow
	@echo "$(GREEN)Configuring Istio...$(NC)"
	@./scripts/install/istio.sh apply-config

argocd-install: ## Install ArgoCD
	@echo "$(GREEN)Installing ArgoCD...$(NC)"
	@./scripts/install/argocd.sh install

argocd-setup: ## Configure ArgoCD for LinkFlow
	@echo "$(GREEN)Configuring ArgoCD...$(NC)"
	@./scripts/install/argocd.sh configure

# ==============================================================================
# MONITORING & OBSERVABILITY
# ==============================================================================
.PHONY: monitoring-deploy logging-deploy tracing-deploy

monitoring-deploy: ## Deploy monitoring stack (Prometheus, Grafana)
	@echo "$(GREEN)Deploying monitoring stack...$(NC)"
	@kubectl apply -f deployments/monitoring/prometheus/
	@kubectl apply -f deployments/monitoring/grafana/

logging-deploy: ## Deploy logging stack (Loki)
	@echo "$(GREEN)Deploying logging stack...$(NC)"
	@kubectl apply -f deployments/monitoring/loki/

tracing-deploy: ## Deploy tracing (Jaeger)
	@echo "$(GREEN)Deploying Jaeger tracing...$(NC)"
	@kubectl apply -f deployments/monitoring/jaeger/

# ==============================================================================
# UTILITY TARGETS
# ==============================================================================
.PHONY: version info all ci

version: ## Show version information
	@echo "$(CYAN)Version Information:$(NC)"
	@echo "  App Version:  $(VERSION)"
	@echo "  Commit Hash:  $(COMMIT_HASH)"
	@echo "  Build Time:   $(BUILD_TIME)"
	@echo "  Go Version:   $(GO_VERSION)"

info: ## Show project information
	@echo "$(CYAN)Project Information:$(NC)"
	@echo "  App Name:     $(APP_NAME)"
	@echo "  Registry:     $(DOCKER_REGISTRY)"
	@echo "  Services:     $(words $(SERVICES)) services"
	@echo "  Migrations:   $(shell ls -1 migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' ') up / $(shell ls -1 migrations/*.down.sql 2>/dev/null | wc -l | tr -d ' ') down"

all: clean deps check test build ## Run full CI pipeline locally

ci: deps check test-coverage build ## Run CI checks (for CI/CD pipelines)
	@echo "$(GREEN)CI checks passed!$(NC)"

# ==============================================================================
# ALIASES (backward compatibility)
# ==============================================================================
.PHONY: run-local stop-local db-migrate db-seed

run-local: run ## Alias for 'run'
stop-local: stop ## Alias for 'stop'
db-migrate: migrate-up ## Alias for 'migrate-up'
db-seed: ## Seed database with test data
	@echo "$(GREEN)Seeding database...$(NC)"
	@./scripts/db/seed.sh

# Default target
.DEFAULT_GOAL := help
