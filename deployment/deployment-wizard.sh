#!/bin/bash
# Interactive Deployment Wizard for FFmpeg-RTMP
# Guides users through deployment with interactive prompts
# Usage: ./deployment-wizard.sh

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
log_step() { echo -e "${CYAN}[STEP]${NC} $1"; }
log_title() { echo -e "${MAGENTA}$1${NC}"; }

# Configuration
DEPLOYMENT_TYPE=""
ENVIRONMENT=""
MASTER_URL=""
API_KEY=""
WORKER_ID=""
GENERATE_CERTS=false
MASTER_IP=""
RUN_PREFLIGHT=true
AUTO_START=true

# Helper function for yes/no prompts
ask_yes_no() {
    local prompt=$1
    local default=${2:-n}
    
    if [ "$default" = "y" ]; then
        prompt="$prompt [Y/n]: "
    else
        prompt="$prompt [y/N]: "
    fi
    
    read -p "$prompt" -r response
    response=${response:-$default}
    
    if [[ "$response" =~ ^[Yy]$ ]]; then
        return 0
    else
        return 1
    fi
}

# Banner
clear
echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║                                                      ║"
echo "║        FFmpeg-RTMP Deployment Wizard                ║"
echo "║                                                      ║"
echo "╚══════════════════════════════════════════════════════╝"
echo ""
log_info "This wizard will guide you through deploying FFmpeg-RTMP"
echo ""

# Step 1: Deployment Type
log_title "═══ Step 1: Deployment Type ═══"
echo ""
echo "What would you like to deploy?"
echo "  1) Master node"
echo "  2) Worker/Edge node"
echo "  3) Both (single server)"
echo ""
read -p "Enter choice [1-3]: " choice

case $choice in
    1) DEPLOYMENT_TYPE="master" ;;
    2) DEPLOYMENT_TYPE="worker" ;;
    3) DEPLOYMENT_TYPE="both" ;;
    *) log_error "Invalid choice"; exit 1 ;;
esac

log_success "Deployment type: $DEPLOYMENT_TYPE"
echo ""

# Step 2: Environment
log_title "═══ Step 2: Environment ═══"
echo ""
echo "Select deployment environment:"
echo "  1) Development"
echo "  2) Staging"
echo "  3) Production"
echo ""
read -p "Enter choice [1-3]: " env_choice

case $env_choice in
    1) ENVIRONMENT="development" ;;
    2) ENVIRONMENT="staging" ;;
    3) ENVIRONMENT="production" ;;
    *) log_error "Invalid choice"; exit 1 ;;
esac

log_success "Environment: $ENVIRONMENT"
echo ""

# Step 3: Pre-flight checks
log_title "═══ Step 3: System Checks ═══"
echo ""
if ask_yes_no "Run pre-flight system checks?" "y"; then
    RUN_PREFLIGHT=true
    log_info "Running pre-flight checks..."
    
    if [ -f "./deployment/checks/preflight-check.sh" ]; then
        if sudo ./deployment/checks/preflight-check.sh --$DEPLOYMENT_TYPE; then
            log_success "Pre-flight checks passed"
        else
            log_error "Pre-flight checks failed"
            if ! ask_yes_no "Continue anyway?" "n"; then
                exit 1
            fi
        fi
    else
        log_warn "Pre-flight check script not found"
    fi
else
    log_warn "Skipping pre-flight checks"
fi
echo ""

# Step 4: Master Configuration (if deploying master or both)
if [ "$DEPLOYMENT_TYPE" = "master" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    log_title "═══ Step 4: Master Configuration ═══"
    echo ""
    
    # Get master IP
    DEFAULT_IP=$(hostname -I | awk '{print $1}')
    read -p "Master server IP address [$DEFAULT_IP]: " MASTER_IP
    MASTER_IP=${MASTER_IP:-$DEFAULT_IP}
    log_success "Master IP: $MASTER_IP"
    
    # TLS/SSL
    echo ""
    if ask_yes_no "Generate TLS certificates?" "y"; then
        GENERATE_CERTS=true
        log_success "Will generate TLS certificates"
    else
        log_info "TLS certificates will not be generated"
    fi
    
    # Database
    echo ""
    echo "Database type:"
    echo "  1) SQLite (development/small deployments)"
    echo "  2) PostgreSQL (production)"
    echo ""
    read -p "Enter choice [1-2]: " db_choice
    
    case $db_choice in
        1) 
            log_success "Using SQLite"
            DB_TYPE="sqlite"
            ;;
        2) 
            log_success "Using PostgreSQL"
            DB_TYPE="postgres"
            log_warn "Ensure PostgreSQL is installed and configured"
            ;;
        *)
            log_warn "Invalid choice, defaulting to SQLite"
            DB_TYPE="sqlite"
            ;;
    esac
fi
echo ""

# Step 5: Worker Configuration (if deploying worker or both)
if [ "$DEPLOYMENT_TYPE" = "worker" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    log_title "═══ Step 5: Worker Configuration ═══"
    echo ""
    
    # Master URL
    if [ "$DEPLOYMENT_TYPE" = "both" ]; then
        MASTER_URL="http://${MASTER_IP}:8080"
        log_info "Master URL: $MASTER_URL"
    else
        read -p "Master server URL (e.g., https://master.example.com:8080): " MASTER_URL
        if [ -z "$MASTER_URL" ]; then
            log_error "Master URL is required"
            exit 1
        fi
    fi
    
    # API Key
    echo ""
    read -p "Master API key: " API_KEY
    if [ -z "$API_KEY" ]; then
        log_warn "No API key provided, you'll need to configure this later"
    fi
    
    # Worker ID
    echo ""
    DEFAULT_WORKER_ID=$(hostname)
    read -p "Worker ID [$DEFAULT_WORKER_ID]: " WORKER_ID
    WORKER_ID=${WORKER_ID:-$DEFAULT_WORKER_ID}
    log_success "Worker ID: $WORKER_ID"
    
    # Concurrent jobs
    echo ""
    CPU_CORES=$(nproc)
    DEFAULT_JOBS=$((CPU_CORES / 2))
    [ $DEFAULT_JOBS -lt 1 ] && DEFAULT_JOBS=1
    read -p "Max concurrent jobs [$DEFAULT_JOBS]: " MAX_JOBS
    MAX_JOBS=${MAX_JOBS:-$DEFAULT_JOBS}
    log_success "Max concurrent jobs: $MAX_JOBS"
fi
echo ""

# Step 6: Review Configuration
log_title "═══ Step 6: Review Configuration ═══"
echo ""
echo "Deployment configuration:"
echo "  Type: $DEPLOYMENT_TYPE"
echo "  Environment: $ENVIRONMENT"
if [ "$DEPLOYMENT_TYPE" = "master" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    echo "  Master IP: $MASTER_IP"
    echo "  Generate certs: $GENERATE_CERTS"
    echo "  Database: $DB_TYPE"
fi
if [ "$DEPLOYMENT_TYPE" = "worker" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    echo "  Master URL: $MASTER_URL"
    echo "  Worker ID: $WORKER_ID"
    echo "  Max jobs: $MAX_JOBS"
fi
echo ""

if ! ask_yes_no "Proceed with deployment?" "y"; then
    log_warn "Deployment cancelled"
    exit 0
fi

# Step 7: Execute Deployment
log_title "═══ Step 7: Deploying ═══"
echo ""

# Build binaries
log_step "Building binaries..."
if [ "$DEPLOYMENT_TYPE" = "master" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    make build-master
fi
if [ "$DEPLOYMENT_TYPE" = "worker" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    make build-agent
    make build-cli
fi
log_success "Build complete"

# Generate certificates if requested
if [ "$GENERATE_CERTS" = "true" ]; then
    log_step "Generating TLS certificates..."
    sudo ./deployment/generate-certs.sh \
        --type master \
        --ip "$MASTER_IP" \
        --output /etc/ffrtmp-master/certs
    log_success "Certificates generated"
fi

# Run deployment script
log_step "Running deployment..."

DEPLOY_CMD="sudo ./deploy.sh --$DEPLOYMENT_TYPE --non-interactive"

if [ "$DEPLOYMENT_TYPE" = "worker" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    DEPLOY_CMD="$DEPLOY_CMD --master-url $MASTER_URL"
    [ -n "$API_KEY" ] && DEPLOY_CMD="$DEPLOY_CMD --api-key $API_KEY"
    [ -n "$WORKER_ID" ] && DEPLOY_CMD="$DEPLOY_CMD --worker-id $WORKER_ID"
fi

if [ "$GENERATE_CERTS" = "true" ]; then
    DEPLOY_CMD="$DEPLOY_CMD --generate-certs --master-ip $MASTER_IP"
fi

log_info "Executing: $DEPLOY_CMD"
eval "$DEPLOY_CMD"

log_success "Deployment complete!"
echo ""

# Step 8: Post-Deployment Verification
log_title "═══ Step 8: Verification ═══"
echo ""

if ask_yes_no "Run post-deployment health checks?" "y"; then
    log_step "Running health checks..."
    
    if [ -f "./deployment/checks/health-check.sh" ]; then
        if sudo ./deployment/checks/health-check.sh --$DEPLOYMENT_TYPE ${MASTER_URL:+--url $MASTER_URL}; then
            log_success "Health checks passed!"
        else
            log_warn "Some health checks failed. Please review the output above."
        fi
    fi
fi

# Final Summary
echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║                                                      ║"
echo "║            Deployment Complete!                      ║"
echo "║                                                      ║"
echo "╚══════════════════════════════════════════════════════╝"
echo ""

if [ "$DEPLOYMENT_TYPE" = "master" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    log_success "Master server is running on http://${MASTER_IP}:8080"
    log_info "Health endpoint: http://${MASTER_IP}:8080/health"
    log_info "API endpoint: http://${MASTER_IP}:8080/api/v1"
fi

if [ "$DEPLOYMENT_TYPE" = "worker" ] || [ "$DEPLOYMENT_TYPE" = "both" ]; then
    log_success "Worker agent is running"
    log_info "Worker ID: $WORKER_ID"
    log_info "Connected to: $MASTER_URL"
fi

echo ""
log_info "Next steps:"
echo "  1. Check service status: sudo systemctl status ffrtmp-*"
echo "  2. View logs: sudo journalctl -u ffrtmp-* -f"
echo "  3. Configure monitoring: see docs/ALERTING.md"
echo "  4. Set up backups: see PRODUCTION_CHECKLIST.md"
echo ""
log_success "Happy transcoding! 🎬"
echo ""
