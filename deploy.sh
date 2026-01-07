#!/bin/bash
# Unified FFmpeg-RTMP Deployment Script
# Deploys master node, worker/edge nodes, or both
# Usage: ./deploy.sh [--master|--worker|--both] [options]

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
log_step() { echo -e "${CYAN}[STEP]${NC} $1"; }

error_exit() {
    log_error "$1"
    exit 1
}

# Default values
DEPLOY_MODE=""
MASTER_URL=""
API_KEY=""
WORKER_ID=$(hostname)
NON_INTERACTIVE=false
SKIP_BUILD=false
GENERATE_CERTS=false
MASTER_IP=""
MASTER_HOST=""

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --master)
                DEPLOY_MODE="master"
                shift
                ;;
            --worker)
                DEPLOY_MODE="worker"
                shift
                ;;
            --edge)
                DEPLOY_MODE="worker"
                shift
                ;;
            --both)
                DEPLOY_MODE="both"
                shift
                ;;
            --master-url)
                MASTER_URL="$2"
                shift 2
                ;;
            --api-key)
                API_KEY="$2"
                shift 2
                ;;
            --worker-id)
                WORKER_ID="$2"
                shift 2
                ;;
            --non-interactive|-y)
                NON_INTERACTIVE=true
                shift
                ;;
            --skip-build)
                SKIP_BUILD=true
                shift
                ;;
            --generate-certs)
                GENERATE_CERTS=true
                shift
                ;;
            --master-ip)
                MASTER_IP="$2"
                shift 2
                ;;
            --master-host)
                MASTER_HOST="$2"
                shift 2
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << EOF
FFmpeg-RTMP Unified Deployment Script

Usage: ./deploy.sh [OPTIONS]

Deployment Modes:
  --master              Deploy master node only
  --worker, --edge      Deploy worker/edge node only
  --both                Deploy both master and worker on same machine

Configuration Options:
  --master-url URL      Master server URL (for worker deployment)
  --api-key KEY         API key for authentication
  --worker-id ID        Worker identifier (default: hostname)
  
TLS/Security Options:
  --generate-certs      Generate TLS certificates before deployment
  --master-ip IP        Master IP address (for certificate SAN)
  --master-host HOST    Master hostname (for certificate SAN)
  
Script Options:
  --non-interactive, -y Run without prompts
  --skip-build          Skip building binaries
  --help, -h            Show this help message

Examples:
  # Interactive mode (prompts for options)
  ./deploy.sh

  # Deploy master node
  ./deploy.sh --master

  # Deploy worker node
  ./deploy.sh --worker --master-url https://master.example.com:8080 --api-key abc123

  # Deploy both on local machine (development)
  ./deploy.sh --both

  # Non-interactive worker deployment
  ./deploy.sh --worker --master-url https://10.0.0.1:8080 --api-key xyz789 -y

Documentation:
  Master: deployment/README.md
  Worker: deployment/WORKER_DEPLOYMENT.md
  Watch Daemon: deployment/WATCH_DEPLOYMENT.md

EOF
}

# Interactive mode
interactive_mode() {
    echo ""
    echo "========================================"
    echo "  FFmpeg-RTMP Deployment Wizard"
    echo "========================================"
    echo ""
    
    # Ask deployment type
    echo "Select deployment type:"
    echo "  1) Master node (orchestration, monitoring)"
    echo "  2) Worker/Edge node (transcoding, discovery)"
    echo "  3) Both (development/testing)"
    echo ""
    read -p "Choice [1-3]: " choice
    
    case $choice in
        1)
            DEPLOY_MODE="master"
            ;;
        2)
            DEPLOY_MODE="worker"
            ;;
        3)
            DEPLOY_MODE="both"
            ;;
        *)
            error_exit "Invalid choice"
            ;;
    esac
    
    # Worker-specific configuration
    if [[ "$DEPLOY_MODE" == "worker" ]]; then
        echo ""
        read -p "Master URL (e.g., https://10.0.0.1:8080): " MASTER_URL
        read -p "API Key: " API_KEY
        read -p "Worker ID [$WORKER_ID]: " input_id
        if [[ -n "$input_id" ]]; then
            WORKER_ID="$input_id"
        fi
    fi
    
    # Confirmation
    echo ""
    echo "Configuration:"
    echo "  Mode: $DEPLOY_MODE"
    if [[ "$DEPLOY_MODE" == "worker" ]]; then
        echo "  Master URL: $MASTER_URL"
        echo "  Worker ID: $WORKER_ID"
    fi
    echo ""
    read -p "Proceed with deployment? [y/N]: " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        echo "Deployment cancelled"
        exit 0
    fi
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        error_exit "This script must be run as root (use sudo)"
    fi
}

# Check prerequisites
check_prerequisites() {
    log_step "Checking prerequisites..."
    
    local missing=0
    
    # Check systemd
    if ! command -v systemctl &> /dev/null; then
        log_error "systemctl not found (systemd required)"
        missing=1
    else
        log_success "systemd found"
    fi
    
    # Check Go (if not skipping build)
    if [[ "$SKIP_BUILD" == false ]]; then
        if ! command -v go &> /dev/null; then
            log_warn "Go not found (required for building binaries)"
            log_info "Install Go 1.24+ or use --skip-build with pre-built binaries"
            missing=1
        else
            GO_VERSION=$(go version | awk '{print $3}')
            log_success "Go found: $GO_VERSION"
        fi
    fi
    
    # Check cgroups (for worker)
    if [[ "$DEPLOY_MODE" == "worker" ]] || [[ "$DEPLOY_MODE" == "both" ]]; then
        if mount | grep -q "cgroup2 on /sys/fs/cgroup"; then
            log_success "Cgroup v2 mounted"
        else
            log_error "Cgroup v2 not mounted (required for resource limits)"
            missing=1
        fi
    fi
    
    if [ $missing -ne 0 ]; then
        error_exit "Missing required prerequisites"
    fi
    
    log_success "All prerequisites satisfied"
}

# ============================================
# Certificate Generation
# ============================================
generate_certificates() {
    local mode="$1"
    
    log_info "Generating TLS certificates..."
    
    if [ ! -f "deployment/generate-certs.sh" ]; then
        log_error "Certificate generation script not found"
        return 1
    fi
    
    local cert_args="--$mode --output certs"
    
    # Add master IP/host if provided
    if [ -n "$MASTER_IP" ]; then
        cert_args="$cert_args --master-ip $MASTER_IP"
    fi
    if [ -n "$MASTER_HOST" ]; then
        cert_args="$cert_args --master-host $MASTER_HOST"
    fi
    
    # Generate CA for production
    if [ "$NON_INTERACTIVE" = false ]; then
        read -p "Generate CA certificate for mTLS? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            cert_args="$cert_args --ca"
        fi
    fi
    
    log_info "Running: ./deployment/generate-certs.sh $cert_args"
    ./deployment/generate-certs.sh $cert_args || return 1
    
    log_success "Certificates generated in certs/ directory"
    return 0
}

# Deploy master node
deploy_master() {
    log_step "Deploying Master Node..."
    
    # Generate certificates if requested
    if [ "$GENERATE_CERTS" = true ]; then
        generate_certificates "master" || error_exit "Certificate generation failed"
        echo ""
    fi
    
    # Check if master deployment script exists
    if [ -f "master/deployment/install-master.sh" ]; then
        log_info "Running master installation script..."
        bash master/deployment/install-master.sh
    else
        log_warn "Master deployment script not found, using manual steps..."
        
        # Build master binary
        if [[ "$SKIP_BUILD" == false ]]; then
            log_info "Building master binary..."
            make build-master || error_exit "Failed to build master"
        fi
        
        # TODO: Add master installation steps
        log_warn "Master installation not yet automated"
        log_info "Please refer to deployment/README.md for manual installation"
    fi
    
    log_success "Master deployment complete"
}

# Deploy worker node
deploy_worker() {
    log_step "Deploying Worker/Edge Node..."
    
    # Generate certificates if requested
    if [ "$GENERATE_CERTS" = true ]; then
        generate_certificates "worker" || error_exit "Certificate generation failed"
        echo ""
    fi
    
    # Run edge installation script
    if [ -f "deployment/install-edge.sh" ]; then
        log_info "Running edge installation script..."
        bash deployment/install-edge.sh
        
        # Configure worker
        if [[ -n "$MASTER_URL" ]]; then
            log_info "Configuring worker connection..."
            configure_worker
        fi
    else
        error_exit "Edge installation script not found: deployment/install-edge.sh"
    fi
    
    log_success "Worker deployment complete"
}

# Configure worker with master connection
configure_worker() {
    local config_file="/etc/ffrtmp/worker.env"
    
    if [ ! -f "$config_file" ]; then
        log_error "Worker config not found: $config_file"
        return 1
    fi
    
    log_info "Updating worker configuration..."
    
    # Update MASTER_URL
    if [[ -n "$MASTER_URL" ]]; then
        sed -i "s|^MASTER_URL=.*|MASTER_URL=$MASTER_URL|" "$config_file"
        log_success "Set MASTER_URL=$MASTER_URL"
    fi
    
    # Update API_KEY
    if [[ -n "$API_KEY" ]]; then
        if grep -q "^MASTER_API_KEY=" "$config_file"; then
            sed -i "s|^MASTER_API_KEY=.*|MASTER_API_KEY=$API_KEY|" "$config_file"
        else
            echo "MASTER_API_KEY=$API_KEY" >> "$config_file"
        fi
        log_success "Set MASTER_API_KEY"
    fi
    
    # Update WORKER_ID
    if [[ -n "$WORKER_ID" ]]; then
        sed -i "s|^WORKER_ID=.*|WORKER_ID=$WORKER_ID|" "$config_file"
        log_success "Set WORKER_ID=$WORKER_ID"
    fi
}

# Start services
start_services() {
    local mode=$1
    
    log_step "Starting services..."
    
    if [[ "$mode" == "master" ]] || [[ "$mode" == "both" ]]; then
        if systemctl list-unit-files | grep -q "ffrtmp-master.service"; then
            systemctl enable ffrtmp-master.service
            systemctl start ffrtmp-master.service
            log_success "Master service started"
        fi
    fi
    
    if [[ "$mode" == "worker" ]] || [[ "$mode" == "both" ]]; then
        if systemctl list-unit-files | grep -q "ffrtmp-worker.service"; then
            systemctl enable ffrtmp-worker.service
            systemctl start ffrtmp-worker.service
            log_success "Worker service started"
        fi
        
        if systemctl list-unit-files | grep -q "ffrtmp-watch.service"; then
            systemctl enable ffrtmp-watch.service
            systemctl start ffrtmp-watch.service
            log_success "Watch daemon service started"
        fi
    fi
}

# Show post-deployment info
show_post_deployment() {
    local mode=$1
    
    echo ""
    echo "========================================"
    echo "  Deployment Complete!"
    echo "========================================"
    echo ""
    
    if [[ "$mode" == "master" ]] || [[ "$mode" == "both" ]]; then
        echo -e "${GREEN}Master Node:${NC}"
        echo "  Status: systemctl status ffrtmp-master"
        echo "  Logs: journalctl -u ffrtmp-master -f"
        echo "  Web UI: http://$(hostname -I | awk '{print $1}'):8080"
        echo ""
    fi
    
    if [[ "$mode" == "worker" ]] || [[ "$mode" == "both" ]]; then
        echo -e "${GREEN}Worker Node:${NC}"
        echo "  Worker Status: systemctl status ffrtmp-worker"
        echo "  Watch Daemon: systemctl status ffrtmp-watch"
        echo "  Worker Logs: journalctl -u ffrtmp-worker -f"
        echo "  Watch Logs: journalctl -u ffrtmp-watch -f"
        echo "  Config: /etc/ffrtmp/worker.env"
        echo ""
    fi
    
    echo -e "${BLUE}Documentation:${NC}"
    echo "  Master: deployment/README.md"
    echo "  Worker: deployment/WORKER_DEPLOYMENT.md"
    echo "  Watch: deployment/WATCH_DEPLOYMENT.md"
    echo ""
    
    echo -e "${YELLOW}Next Steps:${NC}"
    if [[ "$mode" == "worker" ]] && [[ -z "$MASTER_URL" ]]; then
        echo "  1. Edit /etc/ffrtmp/worker.env"
        echo "  2. Set MASTER_URL and MASTER_API_KEY"
        echo "  3. Restart: systemctl restart ffrtmp-worker"
    fi
    echo ""
}

# Main deployment flow
main() {
    # Banner
    echo ""
    echo "╔════════════════════════════════════════╗"
    echo "║   FFmpeg-RTMP Unified Deployment      ║"
    echo "╚════════════════════════════════════════╝"
    echo ""
    
    # Parse arguments
    parse_args "$@"
    
    # Check root
    check_root
    
    # Interactive mode if no deployment mode specified
    if [[ -z "$DEPLOY_MODE" ]]; then
        interactive_mode
    fi
    
    # Check prerequisites
    check_prerequisites
    
    # Deploy based on mode
    case "$DEPLOY_MODE" in
        master)
            deploy_master
            ;;
        worker)
            deploy_worker
            ;;
        both)
            deploy_master
            deploy_worker
            ;;
        *)
            error_exit "Invalid deployment mode: $DEPLOY_MODE"
            ;;
    esac
    
    # Start services
    if [[ "$NON_INTERACTIVE" == true ]]; then
        start_services "$DEPLOY_MODE"
    else
        echo ""
        read -p "Start services now? [Y/n]: " start_now
        if [[ ! "$start_now" =~ ^[Nn]$ ]]; then
            start_services "$DEPLOY_MODE"
        fi
    fi
    
    # Show post-deployment info
    show_post_deployment "$DEPLOY_MODE"
    
    log_success "Deployment completed successfully!"
}

# Run main
main "$@"
