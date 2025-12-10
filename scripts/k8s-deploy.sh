#!/bin/bash

# Kubernetes Deployment Script for LinkFlow

set -e

# Configuration
NAMESPACE=${NAMESPACE:-linkflow}
ENVIRONMENT=${ENVIRONMENT:-development}
KUBECTL=${KUBECTL:-kubectl}

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[DEPLOY]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Check if kubectl is available
check_kubectl() {
    if ! command -v $KUBECTL &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if we can connect to cluster
    if ! $KUBECTL cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig"
        exit 1
    fi
    
    print_status "Connected to Kubernetes cluster ✓"
}

# Create namespace if it doesn't exist
create_namespace() {
    if $KUBECTL get namespace $NAMESPACE &> /dev/null; then
        print_info "Namespace '$NAMESPACE' already exists"
    else
        print_status "Creating namespace '$NAMESPACE'"
        $KUBECTL apply -f deployments/k8s/namespace.yaml
    fi
}

# Apply configurations
apply_configs() {
    print_status "Applying ConfigMaps..."
    $KUBECTL apply -f deployments/k8s/configmap.yaml
    
    print_status "Applying Secrets..."
    if [ -f deployments/k8s/secrets.yaml ]; then
        print_warning "Applying secrets from file. Use a secrets manager in production!"
        $KUBECTL apply -f deployments/k8s/secrets.yaml
    else
        print_warning "No secrets file found. Services may not start properly."
    fi
}

# Deploy services
deploy_services() {
    local services=("auth" "user" "workflow" "execution")
    
    for service in "${services[@]}"; do
        if [ -d "deployments/k8s/$service" ]; then
            print_status "Deploying $service service..."
            $KUBECTL apply -f deployments/k8s/$service/
        else
            print_warning "Skipping $service service (directory not found)"
        fi
    done
}

# Deploy infrastructure components
deploy_infrastructure() {
    print_status "Deploying infrastructure components..."
    
    # Deploy database if manifest exists
    if [ -f "deployments/k8s/infrastructure/postgres.yaml" ]; then
        print_status "Deploying PostgreSQL..."
        $KUBECTL apply -f deployments/k8s/infrastructure/postgres.yaml
    fi
    
    # Deploy Redis if manifest exists
    if [ -f "deployments/k8s/infrastructure/redis.yaml" ]; then
        print_status "Deploying Redis..."
        $KUBECTL apply -f deployments/k8s/infrastructure/redis.yaml
    fi
    
    # Deploy Kafka if manifest exists
    if [ -f "deployments/k8s/infrastructure/kafka.yaml" ]; then
        print_status "Deploying Kafka..."
        $KUBECTL apply -f deployments/k8s/infrastructure/kafka.yaml
    fi
}

# Deploy ingress
deploy_ingress() {
    print_status "Deploying Ingress rules..."
    $KUBECTL apply -f deployments/k8s/ingress.yaml
}

# Wait for deployments to be ready
wait_for_deployments() {
    print_status "Waiting for deployments to be ready..."
    
    local deployments=$($KUBECTL get deployments -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}')
    
    for deployment in $deployments; do
        print_info "Waiting for $deployment..."
        $KUBECTL rollout status deployment/$deployment -n $NAMESPACE --timeout=300s || {
            print_warning "$deployment did not become ready in time"
        }
    done
}

# Show deployment status
show_status() {
    print_status "Deployment Status:"
    echo ""
    
    echo "Deployments:"
    $KUBECTL get deployments -n $NAMESPACE
    echo ""
    
    echo "Pods:"
    $KUBECTL get pods -n $NAMESPACE
    echo ""
    
    echo "Services:"
    $KUBECTL get services -n $NAMESPACE
    echo ""
    
    echo "Ingress:"
    $KUBECTL get ingress -n $NAMESPACE
    echo ""
    
    echo "Horizontal Pod Autoscalers:"
    $KUBECTL get hpa -n $NAMESPACE
}

# Rollback deployment
rollback() {
    local deployment=$1
    
    if [ -z "$deployment" ]; then
        print_error "Please specify a deployment to rollback"
        exit 1
    fi
    
    print_warning "Rolling back deployment: $deployment"
    $KUBECTL rollout undo deployment/$deployment -n $NAMESPACE
    $KUBECTL rollout status deployment/$deployment -n $NAMESPACE
}

# Scale deployment
scale() {
    local deployment=$1
    local replicas=$2
    
    if [ -z "$deployment" ] || [ -z "$replicas" ]; then
        print_error "Usage: $0 scale <deployment> <replicas>"
        exit 1
    fi
    
    print_status "Scaling $deployment to $replicas replicas"
    $KUBECTL scale deployment/$deployment --replicas=$replicas -n $NAMESPACE
}

# Delete all resources
cleanup() {
    print_warning "This will delete all LinkFlow resources in namespace '$NAMESPACE'"
    read -p "Are you sure? (yes/no) " -r
    echo
    
    if [[ $REPLY == "yes" ]]; then
        print_status "Cleaning up resources..."
        $KUBECTL delete -f deployments/k8s/ --recursive --ignore-not-found
        print_status "Cleanup complete"
    else
        print_info "Cleanup cancelled"
    fi
}

# Print usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  deploy          Deploy all LinkFlow services"
    echo "  status          Show deployment status"
    echo "  rollback NAME   Rollback a deployment"
    echo "  scale NAME NUM  Scale a deployment"
    echo "  cleanup         Delete all resources"
    echo "  help            Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  NAMESPACE       Kubernetes namespace (default: linkflow)"
    echo "  ENVIRONMENT     Deployment environment (default: development)"
    echo "  KUBECTL         kubectl command (default: kubectl)"
    echo ""
    echo "Examples:"
    echo "  $0 deploy                        # Deploy everything"
    echo "  $0 status                        # Check status"
    echo "  $0 rollback auth-service         # Rollback auth service"
    echo "  $0 scale workflow-service 5      # Scale workflow service to 5 pods"
    echo "  NAMESPACE=linkflow-dev $0 deploy # Deploy to linkflow-dev namespace"
}

# Main execution
main() {
    case "$1" in
        deploy)
            check_kubectl
            create_namespace
            apply_configs
            deploy_infrastructure
            deploy_services
            deploy_ingress
            wait_for_deployments
            show_status
            print_status "Deployment complete! 🚀"
            ;;
        status)
            check_kubectl
            show_status
            ;;
        rollback)
            check_kubectl
            rollback "$2"
            ;;
        scale)
            check_kubectl
            scale "$2" "$3"
            ;;
        cleanup)
            check_kubectl
            cleanup
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
