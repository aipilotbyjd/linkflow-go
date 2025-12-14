# Build stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the service
ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w" \
    -o service cmd/services/${SERVICE_NAME}/main.go

# Runtime stage
FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/service /app/service

# Copy RBAC config files
COPY deployments/config/rbac/model.conf /app/deployments/config/rbac/model.conf
COPY deployments/config/rbac/policy.csv /app/deployments/config/rbac/policy.csv

# Create non-root user
RUN addgroup -g 1001 -S linkflow && \
    adduser -u 1001 -S linkflow -G linkflow

USER linkflow

# Health check - try /health/live first (standard), then /health (legacy)
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${LINKFLOW_SERVER_PORT:-8080}/health/live 2>/dev/null || \
        wget --no-verbose --tries=1 --spider http://localhost:${LINKFLOW_SERVER_PORT:-8080}/health || exit 1

# Expose port
EXPOSE 8080

# Run the service
CMD ["/app/service"]
