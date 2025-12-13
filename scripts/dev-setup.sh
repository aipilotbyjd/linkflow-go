#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  LinkFlow Development Setup${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

check_command() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}✗ $1 is not installed${NC}"
        return 1
    else
        echo -e "${GREEN}✓ $1 is installed${NC}"
        return 0
    fi
}

check_command go || exit 1
check_command docker || exit 1
check_command docker-compose || echo -e "${YELLOW}docker-compose not found, using docker compose${NC}"

# Copy environment file
echo ""
echo -e "${YELLOW}Setting up environment...${NC}"
if [ ! -f .env ]; then
    cp .env.example .env
    echo -e "${GREEN}✓ Created .env file${NC}"
else
    echo -e "${GREEN}✓ .env file already exists${NC}"
fi

# Download Go dependencies
echo ""
echo -e "${YELLOW}Downloading Go dependencies...${NC}"
go mod download
go mod tidy
echo -e "${GREEN}✓ Dependencies downloaded${NC}"

# Start infrastructure
echo ""
echo -e "${YELLOW}Starting infrastructure services...${NC}"
docker-compose up -d postgres redis zookeeper kafka elasticsearch prometheus grafana jaeger

# Wait for services to be ready
echo ""
echo -e "${YELLOW}Waiting for services to be ready...${NC}"

wait_for_service() {
    local host=$1
    local port=$2
    local service=$3
    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if nc -z $host $port 2>/dev/null; then
            echo -e "${GREEN}✓ $service is ready${NC}"
            return 0
        fi
        echo "Waiting for $service... (attempt $attempt/$max_attempts)"
        sleep 2
        attempt=$((attempt + 1))
    done

    echo -e "${RED}✗ $service failed to start${NC}"
    return 1
}

wait_for_service localhost 5432 "PostgreSQL"
wait_for_service localhost 6379 "Redis"
wait_for_service localhost 29092 "Kafka"
wait_for_service localhost 9200 "Elasticsearch"

# Run migrations
echo ""
echo -e "${YELLOW}Running database migrations...${NC}"
if [ -f scripts/migrate.sh ]; then
    ./scripts/migrate.sh up || echo -e "${YELLOW}Migration script not fully configured${NC}"
fi

# Setup Kafka topics
echo ""
echo -e "${YELLOW}Setting up Kafka topics...${NC}"
if [ -f scripts/kafka-setup.sh ]; then
    ./scripts/kafka-setup.sh setup || echo -e "${YELLOW}Kafka setup not fully configured${NC}"
fi

# Seed database
echo ""
echo -e "${YELLOW}Seeding database...${NC}"
if [ -f scripts/seed.sh ]; then
    ./scripts/seed.sh || echo -e "${YELLOW}Seed script not fully configured${NC}"
fi

# Build services
echo ""
echo -e "${YELLOW}Building services...${NC}"
make build || echo -e "${YELLOW}Some services may have build issues${NC}"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Setup Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Available services:"
echo "  - PostgreSQL:    localhost:5432"
echo "  - Redis:         localhost:6379"
echo "  - Kafka:         localhost:29092"
echo "  - Elasticsearch: localhost:9200"
echo "  - Prometheus:    localhost:9090"
echo "  - Grafana:       localhost:3000 (admin/admin)"
echo "  - Jaeger:        localhost:16686"
echo ""
echo "To start all services:"
echo "  make run-local"
echo ""
echo "To run a specific service:"
echo "  go run ./cmd/services/auth/main.go"
echo ""
