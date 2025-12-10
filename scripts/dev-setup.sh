#!/bin/bash

# LinkFlow Development Environment Setup Script
# This script sets up the complete local development environment

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if Docker is installed and running
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi

    if ! docker info &> /dev/null; then
        print_error "Docker is not running. Please start Docker."
        exit 1
    fi
    print_status "Docker is installed and running ✓"
}

# Check if docker-compose is installed
check_docker_compose() {
    if ! command -v docker-compose &> /dev/null; then
        print_error "docker-compose is not installed. Please install docker-compose."
        exit 1
    fi
    print_status "docker-compose is installed ✓"
}

# Create .env file if it doesn't exist
setup_env_file() {
    if [ ! -f .env ]; then
        print_status "Creating .env file from .env.example..."
        cp .env.example .env
        print_warning "Please review and update .env file with your settings"
    else
        print_status ".env file already exists ✓"
    fi
}

# Stop any running services
cleanup() {
    print_status "Stopping any existing containers..."
    docker-compose down --volumes --remove-orphans 2>/dev/null || true
}

# Start infrastructure services
start_infrastructure() {
    print_status "Starting infrastructure services..."
    
    # Start only essential services first
    docker-compose up -d postgres redis zookeeper
    
    print_status "Waiting for PostgreSQL to be ready..."
    until docker-compose exec -T postgres pg_isready -U linkflow &>/dev/null; do
        printf "."
        sleep 1
    done
    echo ""
    print_status "PostgreSQL is ready ✓"
    
    print_status "Waiting for Redis to be ready..."
    until docker-compose exec -T redis redis-cli ping &>/dev/null; do
        printf "."
        sleep 1
    done
    echo ""
    print_status "Redis is ready ✓"
    
    # Start Kafka after Zookeeper is ready
    print_status "Starting Kafka..."
    docker-compose up -d kafka
    sleep 10  # Give Kafka time to start
    
    # Start remaining services
    print_status "Starting remaining services..."
    docker-compose up -d elasticsearch prometheus grafana jaeger kong
}

# Create Kafka topics
setup_kafka_topics() {
    print_status "Setting up Kafka topics..."
    
    # Wait for Kafka to be fully ready
    sleep 5
    
    # Create topics
    docker-compose exec -T kafka kafka-topics --create \
        --if-not-exists \
        --bootstrap-server localhost:9092 \
        --topic workflow.events \
        --partitions 10 \
        --replication-factor 1 2>/dev/null || true
    
    docker-compose exec -T kafka kafka-topics --create \
        --if-not-exists \
        --bootstrap-server localhost:9092 \
        --topic execution.events \
        --partitions 20 \
        --replication-factor 1 2>/dev/null || true
    
    docker-compose exec -T kafka kafka-topics --create \
        --if-not-exists \
        --bootstrap-server localhost:9092 \
        --topic audit.log \
        --partitions 5 \
        --replication-factor 1 2>/dev/null || true
    
    print_status "Kafka topics created ✓"
}

# Run database migrations
run_migrations() {
    if [ -f ./scripts/migrate.sh ]; then
        print_status "Running database migrations..."
        ./scripts/migrate.sh up
    else
        print_warning "Migration script not found. Skipping migrations."
    fi
}

# Seed development data
seed_data() {
    if [ -f ./scripts/seed.sh ]; then
        print_status "Seeding development data..."
        ./scripts/seed.sh
    else
        print_warning "Seed script not found. Skipping data seeding."
    fi
}

# Verify all services are healthy
verify_services() {
    print_status "Verifying service health..."
    
    # Check PostgreSQL
    if docker-compose exec -T postgres pg_isready -U linkflow &>/dev/null; then
        print_status "PostgreSQL: ✓ Running on port 5432"
    else
        print_error "PostgreSQL: ✗ Not responding"
    fi
    
    # Check Redis
    if docker-compose exec -T redis redis-cli ping &>/dev/null; then
        print_status "Redis: ✓ Running on port 6379"
    else
        print_error "Redis: ✗ Not responding"
    fi
    
    # Check Elasticsearch
    if curl -s http://localhost:9200/_cluster/health &>/dev/null; then
        print_status "Elasticsearch: ✓ Running on port 9200"
    else
        print_warning "Elasticsearch: Still starting up..."
    fi
    
    # Check Kafka
    if docker-compose exec -T kafka kafka-broker-api-versions --bootstrap-server localhost:9092 &>/dev/null; then
        print_status "Kafka: ✓ Running on port 9092"
    else
        print_error "Kafka: ✗ Not responding"
    fi
    
    print_status "Grafana: Available at http://localhost:3000 (admin/admin)"
    print_status "Prometheus: Available at http://localhost:9090"
    print_status "Jaeger UI: Available at http://localhost:16686"
    print_status "Kong Admin: Available at http://localhost:8001"
}

# Print usage instructions
print_usage() {
    echo ""
    echo "================================================================"
    echo "LinkFlow Development Environment Setup Complete! 🚀"
    echo "================================================================"
    echo ""
    echo "Services are running at:"
    echo "  • PostgreSQL:    localhost:5432"
    echo "  • Redis:         localhost:6379"
    echo "  • Kafka:         localhost:9092"
    echo "  • Elasticsearch: localhost:9200"
    echo "  • Prometheus:    http://localhost:9090"
    echo "  • Grafana:       http://localhost:3000 (admin/admin)"
    echo "  • Jaeger:        http://localhost:16686"
    echo "  • Kong Gateway:  http://localhost:8000"
    echo "  • Kong Admin:    http://localhost:8001"
    echo ""
    echo "Useful commands:"
    echo "  • View logs:        docker-compose logs -f [service-name]"
    echo "  • Stop services:    docker-compose down"
    echo "  • Restart service:  docker-compose restart [service-name]"
    echo "  • View status:      docker-compose ps"
    echo ""
    echo "Next steps:"
    echo "  1. Review and update .env file if needed"
    echo "  2. Run 'make build' to build the services"
    echo "  3. Run 'make run-auth' to start the auth service"
    echo ""
}

# Main execution
main() {
    print_status "Starting LinkFlow Development Environment Setup..."
    
    check_docker
    check_docker_compose
    setup_env_file
    
    # Ask user if they want to clean up existing containers
    read -p "Do you want to clean up existing containers? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup
    fi
    
    start_infrastructure
    setup_kafka_topics
    run_migrations
    seed_data
    verify_services
    print_usage
}

# Run main function
main "$@"
