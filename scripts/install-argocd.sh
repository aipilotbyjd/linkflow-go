#!/bin/bash

# ArgoCD Installation Script for LinkFlow

set -e

# Configuration
ARGOCD_NAMESPACE=${ARGOCD_NAMESPACE:-argocd}
ARGOCD_VERSION=${ARGOCD_VERSION:-v2.9.3}

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[ARGOCD]${NC} $1"
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

# Check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    print_status "Prerequisites check passed ✓"
}

# Install ArgoCD
install_argocd() {
    print_status "Installing ArgoCD version ${ARGOCD_VERSION}..."
    
    # Create namespace
    kubectl create namespace ${ARGOCD_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
    
    # Install ArgoCD
    kubectl apply -n ${ARGOCD_NAMESPACE} -f https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml
    
    print_status "Waiting for ArgoCD to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/argocd-server -n ${ARGOCD_NAMESPACE}
    kubectl wait --for=condition=available --timeout=300s deployment/argocd-repo-server -n ${ARGOCD_NAMESPACE}
    kubectl wait --for=condition=available --timeout=300s deployment/argocd-redis -n ${ARGOCD_NAMESPACE}
    kubectl wait --for=condition=available --timeout=300s deployment/argocd-dex-server -n ${ARGOCD_NAMESPACE}
    
    print_status "ArgoCD installed successfully ✓"
}

# Configure ArgoCD
configure_argocd() {
    print_status "Configuring ArgoCD..."
    
    # Patch ArgoCD server to use LoadBalancer (optional)
    if [[ "$1" == "expose" ]]; then
        kubectl patch svc argocd-server -n ${ARGOCD_NAMESPACE} -p '{"spec": {"type": "LoadBalancer"}}'
        print_status "ArgoCD server exposed via LoadBalancer"
    fi
    
    # Apply LinkFlow applications
    print_status "Applying LinkFlow ArgoCD applications..."
    kubectl apply -f deployments/argocd/
    
    print_status "ArgoCD configured successfully ✓"
}

# Install ArgoCD CLI
install_cli() {
    print_status "Installing ArgoCD CLI..."
    
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
        ARCH="arm64"
    fi
    
    # Download ArgoCD CLI
    curl -sSL -o /tmp/argocd https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-${OS}-${ARCH}
    
    # Make executable and move to PATH
    chmod +x /tmp/argocd
    
    if [[ -w "/usr/local/bin" ]]; then
        mv /tmp/argocd /usr/local/bin/argocd
    else
        print_warning "Cannot write to /usr/local/bin, installing to ~/bin"
        mkdir -p ~/bin
        mv /tmp/argocd ~/bin/argocd
        print_info "Add ~/bin to your PATH: export PATH=\$PATH:~/bin"
    fi
    
    print_status "ArgoCD CLI installed ✓"
}

# Get initial admin password
get_admin_password() {
    print_status "Getting ArgoCD admin password..."
    
    ARGOCD_PASSWORD=$(kubectl -n ${ARGOCD_NAMESPACE} get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
    
    echo ""
    echo "======================================"
    echo "ArgoCD Admin Credentials:"
    echo "Username: admin"
    echo "Password: ${ARGOCD_PASSWORD}"
    echo "======================================"
    echo ""
    
    print_warning "Please change the admin password after first login!"
}

# Port forward to access ArgoCD UI
port_forward() {
    print_status "Setting up port forwarding to ArgoCD UI..."
    print_info "Access ArgoCD at: https://localhost:8080"
    print_info "Press Ctrl+C to stop port forwarding"
    
    kubectl port-forward svc/argocd-server -n ${ARGOCD_NAMESPACE} 8080:443
}

# Login to ArgoCD
login_argocd() {
    print_status "Logging into ArgoCD..."
    
    ARGOCD_PASSWORD=$(kubectl -n ${ARGOCD_NAMESPACE} get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
    
    # Login via CLI
    argocd login localhost:8080 --insecure --username admin --password "${ARGOCD_PASSWORD}"
    
    print_status "Logged into ArgoCD ✓"
}

# Add Git repository
add_repository() {
    local repo_url=${1:-"https://github.com/your-org/linkflow-go"}
    
    print_status "Adding Git repository: ${repo_url}"
    
    argocd repo add ${repo_url} --insecure-skip-server-verification
    
    print_status "Repository added ✓"
}

# Uninstall ArgoCD
uninstall_argocd() {
    print_warning "This will completely remove ArgoCD from your cluster"
    read -p "Are you sure? (yes/no) " -r
    echo
    
    if [[ $REPLY == "yes" ]]; then
        print_status "Uninstalling ArgoCD..."
        kubectl delete -n ${ARGOCD_NAMESPACE} -f https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml
        kubectl delete namespace ${ARGOCD_NAMESPACE}
        print_status "ArgoCD uninstalled"
    else
        print_info "Uninstall cancelled"
    fi
}

# Print usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  install         Install ArgoCD in the cluster"
    echo "  configure       Configure ArgoCD for LinkFlow"
    echo "  cli             Install ArgoCD CLI tool"
    echo "  password        Get admin password"
    echo "  forward         Port forward to ArgoCD UI"
    echo "  login           Login to ArgoCD via CLI"
    echo "  add-repo URL    Add a Git repository"
    echo "  uninstall       Remove ArgoCD from cluster"
    echo "  help            Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  ARGOCD_NAMESPACE    ArgoCD namespace (default: argocd)"
    echo "  ARGOCD_VERSION      ArgoCD version (default: v2.9.3)"
    echo ""
    echo "Examples:"
    echo "  $0 install                    # Install ArgoCD"
    echo "  $0 configure expose           # Configure with LoadBalancer"
    echo "  $0 forward                    # Access UI via port forward"
}

# Main execution
main() {
    case "$1" in
        install)
            check_prerequisites
            install_argocd
            get_admin_password
            print_status "Installation complete! 🚀"
            print_info "Run '$0 forward' to access the UI"
            ;;
        configure)
            configure_argocd "$2"
            ;;
        cli)
            install_cli
            ;;
        password)
            get_admin_password
            ;;
        forward)
            port_forward
            ;;
        login)
            login_argocd
            ;;
        add-repo)
            add_repository "$2"
            ;;
        uninstall)
            uninstall_argocd
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
