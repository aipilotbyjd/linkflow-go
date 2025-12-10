#!/bin/bash

# Istio Installation and Configuration Script for LinkFlow

set -e

# Configuration
ISTIO_VERSION=${ISTIO_VERSION:-1.20.0}
NAMESPACE=${NAMESPACE:-linkflow}

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[ISTIO]${NC} $1"
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

# Download and install istioctl
install_istioctl() {
    print_status "Installing istioctl version ${ISTIO_VERSION}..."
    
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
        ARCH="arm64"
    fi
    
    # Download Istio
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=${ISTIO_VERSION} sh -
    
    # Move istioctl to PATH
    if [[ -w "/usr/local/bin" ]]; then
        sudo mv istio-${ISTIO_VERSION}/bin/istioctl /usr/local/bin/
    else
        mkdir -p ~/bin
        mv istio-${ISTIO_VERSION}/bin/istioctl ~/bin/
        print_info "Add ~/bin to your PATH: export PATH=\$PATH:~/bin"
    fi
    
    # Cleanup
    rm -rf istio-${ISTIO_VERSION}
    
    print_status "istioctl installed ✓"
}

# Install Istio
install_istio() {
    print_status "Installing Istio with demo profile..."
    
    # Pre-check
    istioctl x precheck
    
    # Install Istio with production settings
    istioctl install --set profile=production \
        --set values.gateways.istio-ingressgateway.type=LoadBalancer \
        --set values.pilot.resources.requests.memory=512Mi \
        --set values.pilot.resources.requests.cpu=250m \
        --set values.global.proxy.resources.requests.memory=128Mi \
        --set values.global.proxy.resources.requests.cpu=100m \
        --set values.global.defaultPodDisruptionBudget.enabled=true \
        --set values.telemetry.v2.prometheus.wasmEnabled=true \
        --set meshConfig.defaultConfig.proxyStatsMatcher.inclusionRegexps[0]=".*outlier_detection.*" \
        --set meshConfig.defaultConfig.proxyStatsMatcher.inclusionRegexps[1]=".*circuit_breakers.*" \
        --set meshConfig.defaultConfig.proxyStatsMatcher.inclusionRegexps[2]=".*upstream_rq_retry.*" \
        --set meshConfig.defaultConfig.proxyStatsMatcher.inclusionRegexps[3]=".*upstream_rq_pending.*" \
        --set meshConfig.accessLogFile=/dev/stdout \
        -y
    
    print_status "Waiting for Istio to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/istiod -n istio-system
    kubectl wait --for=condition=available --timeout=300s deployment/istio-ingressgateway -n istio-system
    
    print_status "Istio installed successfully ✓"
}

# Enable sidecar injection
enable_injection() {
    print_status "Enabling automatic sidecar injection for namespace ${NAMESPACE}..."
    
    kubectl label namespace ${NAMESPACE} istio-injection=enabled --overwrite
    
    print_status "Sidecar injection enabled ✓"
}

# Install Istio addons
install_addons() {
    print_status "Installing Istio addons (Kiali, Jaeger, Prometheus, Grafana)..."
    
    # Apply addon manifests
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-${ISTIO_VERSION%.*}/samples/addons/prometheus.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-${ISTIO_VERSION%.*}/samples/addons/grafana.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-${ISTIO_VERSION%.*}/samples/addons/jaeger.yaml
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-${ISTIO_VERSION%.*}/samples/addons/kiali.yaml
    
    print_status "Addons installed ✓"
}

# Apply LinkFlow Istio configurations
apply_linkflow_config() {
    print_status "Applying LinkFlow Istio configurations..."
    
    # Apply all Istio configurations
    kubectl apply -f deployments/istio/
    
    print_status "LinkFlow Istio configurations applied ✓"
}

# Restart deployments to inject sidecars
restart_deployments() {
    print_status "Restarting deployments to inject Istio sidecars..."
    
    kubectl rollout restart deployment -n ${NAMESPACE}
    
    print_status "Deployments restarted ✓"
}

# Verify installation
verify_installation() {
    print_status "Verifying Istio installation..."
    
    istioctl verify-install
    
    print_status "Checking proxy status..."
    istioctl proxy-status
    
    print_status "Analyzing configuration..."
    istioctl analyze -n ${NAMESPACE}
    
    print_status "Verification complete ✓"
}

# Open Kiali dashboard
open_kiali() {
    print_status "Opening Kiali dashboard..."
    print_info "Access Kiali at: http://localhost:20001"
    print_info "Press Ctrl+C to stop"
    
    istioctl dashboard kiali
}

# Open Jaeger dashboard
open_jaeger() {
    print_status "Opening Jaeger dashboard..."
    print_info "Access Jaeger at: http://localhost:16686"
    
    istioctl dashboard jaeger
}

# Open Grafana dashboard
open_grafana() {
    print_status "Opening Grafana dashboard..."
    print_info "Access Grafana at: http://localhost:3000"
    
    istioctl dashboard grafana
}

# Uninstall Istio
uninstall_istio() {
    print_warning "This will completely remove Istio from your cluster"
    read -p "Are you sure? (yes/no) " -r
    echo
    
    if [[ $REPLY == "yes" ]]; then
        print_status "Uninstalling Istio..."
        istioctl uninstall --purge -y
        kubectl delete namespace istio-system
        kubectl label namespace ${NAMESPACE} istio-injection-
        print_status "Istio uninstalled"
    else
        print_info "Uninstall cancelled"
    fi
}

# Print usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  install         Install Istio in the cluster"
    echo "  install-cli     Install istioctl CLI tool"
    echo "  enable-injection Enable sidecar injection for namespace"
    echo "  addons          Install Istio addons (Kiali, Jaeger, etc.)"
    echo "  apply-config    Apply LinkFlow Istio configurations"
    echo "  restart         Restart deployments to inject sidecars"
    echo "  verify          Verify Istio installation"
    echo "  kiali           Open Kiali dashboard"
    echo "  jaeger          Open Jaeger dashboard"
    echo "  grafana         Open Grafana dashboard"
    echo "  uninstall       Remove Istio from cluster"
    echo "  help            Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  ISTIO_VERSION    Istio version (default: 1.20.0)"
    echo "  NAMESPACE        Target namespace (default: linkflow)"
    echo ""
    echo "Examples:"
    echo "  $0 install                  # Install Istio"
    echo "  $0 apply-config              # Apply LinkFlow configurations"
    echo "  $0 kiali                     # Open Kiali dashboard"
}

# Main execution
main() {
    case "$1" in
        install)
            check_prerequisites
            install_istioctl
            install_istio
            enable_injection
            install_addons
            print_status "Istio installation complete! 🚀"
            print_info "Run '$0 apply-config' to apply LinkFlow configurations"
            ;;
        install-cli)
            install_istioctl
            ;;
        enable-injection)
            enable_injection
            ;;
        addons)
            install_addons
            ;;
        apply-config)
            apply_linkflow_config
            restart_deployments
            ;;
        restart)
            restart_deployments
            ;;
        verify)
            verify_installation
            ;;
        kiali)
            open_kiali
            ;;
        jaeger)
            open_jaeger
            ;;
        grafana)
            open_grafana
            ;;
        uninstall)
            uninstall_istio
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
