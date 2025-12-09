#!/bin/bash

# Setup script for LinkFlow development environment

set -e

echo "🚀 Setting up LinkFlow development environment..."

# Check prerequisites
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "❌ $1 is not installed. Please install it first."
        exit 1
    fi
    echo "✅ $1 is installed"
}

echo "Checking prerequisites..."
check_command go
check_command docker
check_command docker-compose
check_command make

# Install Go tools
echo "Installing Go development tools..."
go install github.com/golang/mock/mockgen@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install mvdan.cc/gofumpt@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Download Go dependencies
echo "Downloading Go dependencies..."
go mod download
go mod tidy

# Create necessary directories
echo "Creating project directories..."
mkdir -p bin tmp logs data

# Generate RSA keys for JWT (development only)
echo "Generating RSA keys for JWT..."
mkdir -p configs/keys
openssl genrsa -out configs/keys/private.pem 2048
openssl rsa -in configs/keys/private.pem -pubout -out configs/keys/public.pem

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo "Creating .env file..."
    cat > .env <<EOF
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=linkflow
DB_PASSWORD=linkflow123
DB_NAME=linkflow

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Kafka
KAFKA_BROKERS=localhost:9092

# JWT
JWT_SECRET=your-super-secret-key
EOF
fi

# Start infrastructure services
echo "Starting infrastructure services..."
docker-compose up -d postgres redis kafka zookeeper

# Wait for services to be ready
echo "Waiting for services to be ready..."
sleep 10

# Run database migrations
echo "Running database migrations..."
# Create migration files first if they don't exist
mkdir -p deployments/migrations

# Create initial migration
cat > deployments/migrations/000001_init_schema.up.sql <<EOF
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE,
    password VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    avatar VARCHAR(500),
    email_verified BOOLEAN DEFAULT FALSE,
    email_verify_token VARCHAR(100),
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    two_factor_secret VARCHAR(100),
    status VARCHAR(50) DEFAULT 'active',
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Workflows table
CREATE TABLE IF NOT EXISTS workflows (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    user_id VARCHAR(36) NOT NULL,
    team_id VARCHAR(36),
    nodes JSONB,
    connections JSONB,
    settings JSONB,
    status VARCHAR(50) DEFAULT 'inactive',
    is_active BOOLEAN DEFAULT FALSE,
    version INTEGER DEFAULT 1,
    tags TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Executions table
CREATE TABLE IF NOT EXISTS workflow_executions (
    id VARCHAR(36) PRIMARY KEY,
    workflow_id VARCHAR(36) NOT NULL,
    version INTEGER,
    status VARCHAR(50) DEFAULT 'pending',
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    execution_time BIGINT,
    data JSONB,
    error TEXT,
    created_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    token VARCHAR(500) UNIQUE NOT NULL,
    refresh_token VARCHAR(500) UNIQUE,
    ip_address VARCHAR(50),
    user_agent TEXT,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create indexes
CREATE INDEX idx_workflows_user_id ON workflows(user_id);
CREATE INDEX idx_workflows_status ON workflows(status);
CREATE INDEX idx_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX idx_executions_status ON workflow_executions(status);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
EOF

cat > deployments/migrations/000001_init_schema.down.sql <<EOF
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS workflow_executions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS users;
EOF

# Build all services
echo "Building services..."
make build

echo "✨ Setup complete! You can now run:"
echo "  make run-local    # Start all services locally"
echo "  make test         # Run tests"
echo "  make help         # Show all available commands"
