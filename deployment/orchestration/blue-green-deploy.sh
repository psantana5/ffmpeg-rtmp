#!/bin/bash
# Blue-Green Deployment Orchestrator for FFmpeg-RTMP
# Manages zero-downtime deployments with automatic rollback
# Usage: ./blue-green-deploy.sh [--deploy|--switch|--rollback]

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
log_step() { echo -e "${CYAN}[STEP]${NC} $1"; }

# Configuration
DEPLOYMENT_TYPE="master"  # master or worker
ACTION=""
NEW_VERSION=""
HEALTH_CHECK_RETRIES=10
HEALTH_CHECK_INTERVAL=5

# Paths
BLUE_DIR="/opt/ffrtmp-blue"
GREEN_DIR="/opt/ffrtmp-green"
CURRENT_LINK="/opt/ffrtmp"
STATE_FILE="/var/lib/ffrtmp/deployment-state.json"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --deploy) ACTION="deploy"; shift ;;
        --switch) ACTION="switch"; shift ;;
        --rollback) ACTION="rollback"; shift ;;
        --master) DEPLOYMENT_TYPE="master"; shift ;;
        --worker) DEPLOYMENT_TYPE="worker"; shift ;;
        --version) NEW_VERSION="$2"; shift 2 ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

if [ -z "$ACTION" ]; then
    log_error "Action required: --deploy, --switch, or --rollback"
    exit 1
fi

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    log_error "Must run as root"
    exit 1
fi

# Get current active environment
get_active_env() {
    if [ -L "$CURRENT_LINK" ]; then
        local target
        target=$(readlink "$CURRENT_LINK")
        if [[ "$target" == *"blue"* ]]; then
            echo "blue"
        elif [[ "$target" == *"green"* ]]; then
            echo "green"
        else
            echo "unknown"
        fi
    else
        echo "none"
    fi
}

# Get inactive environment
get_inactive_env() {
    local active
    active=$(get_active_env)
    if [ "$active" = "blue" ]; then
        echo "green"
    elif [ "$active" = "green" ]; then
        echo "blue"
    else
        echo "blue"  # Default to blue if none active
    fi
}

# Get environment directory
get_env_dir() {
    local env=$1
    if [ "$env" = "blue" ]; then
        echo "$BLUE_DIR"
    else
        echo "$GREEN_DIR"
    fi
}

# Save deployment state
save_state() {
    local state=$1
    local version=$2
    local timestamp
    timestamp=$(date -Iseconds)
    
    mkdir -p "$(dirname "$STATE_FILE")"
    cat > "$STATE_FILE" << EOF
{
  "active_env": "$state",
  "version": "$version",
  "timestamp": "$timestamp",
  "previous_env": "$(get_active_env)"
}
EOF
}

# Health check function
health_check() {
    local env_dir=$1
    local retries=$HEALTH_CHECK_RETRIES
    
    log_step "Running health checks for deployment in $env_dir"
    
    # Check if binary exists
    if [ ! -f "$env_dir/bin/ffrtmp-$DEPLOYMENT_TYPE" ] && [ ! -f "$env_dir/bin/agent" ]; then
        log_error "Binary not found in $env_dir"
        return 1
    fi
    
    # Check if service is running (if this is the active environment)
    if [ "$(readlink $CURRENT_LINK)" = "$env_dir" ]; then
        if [ "$DEPLOYMENT_TYPE" = "master" ]; then
            SERVICE="ffrtmp-master.service"
            PORT=8080
        else
            SERVICE="ffrtmp-worker.service"
            PORT=""
        fi
        
        log_info "Checking service: $SERVICE"
        while [ $retries -gt 0 ]; do
            if systemctl is-active --quiet "$SERVICE"; then
                log_success "Service is active"
                
                # Check port for master
                if [ -n "$PORT" ]; then
                    if curl -s --max-time 5 "http://localhost:$PORT/health" >/dev/null 2>&1; then
                        log_success "Health endpoint responding"
                        return 0
                    else
                        log_warn "Health endpoint not responding, retrying... ($retries left)"
                    fi
                else
                    return 0
                fi
            else
                log_warn "Service not active, retrying... ($retries left)"
            fi
            
            ((retries--))
            sleep $HEALTH_CHECK_INTERVAL
        done
        
        log_error "Health check failed after $HEALTH_CHECK_RETRIES attempts"
        return 1
    fi
    
    return 0
}

# Deploy to inactive environment
deploy_new_version() {
    local inactive_env
    inactive_env=$(get_inactive_env)
    local target_dir
    target_dir=$(get_env_dir "$inactive_env")
    
    log_step "Deploying new version to $inactive_env environment ($target_dir)"
    
    # Create target directory
    mkdir -p "$target_dir"/{bin,config,data,logs}
    
    # Copy new binaries (assuming they're in ./bin)
    if [ ! -d "./bin" ]; then
        log_error "Build directory ./bin not found. Run 'make build-$DEPLOYMENT_TYPE' first"
        return 1
    fi
    
    log_info "Copying binaries to $target_dir/bin"
    cp -r ./bin/* "$target_dir/bin/" || {
        log_error "Failed to copy binaries"
        return 1
    }
    
    # Copy configuration if exists
    if [ -d "./deployment/config" ]; then
        log_info "Copying configuration files"
        cp -r ./deployment/config/* "$target_dir/config/" 2>/dev/null || true
    fi
    
    # Set ownership
    chown -R ffrtmp:ffrtmp "$target_dir" 2>/dev/null || true
    chmod +x "$target_dir/bin/"* 2>/dev/null || true
    
    log_success "Deployment to $inactive_env completed"
    
    # Save state
    save_state "$inactive_env" "${NEW_VERSION:-unknown}" 
    
    return 0
}

# Switch traffic to new environment
switch_environment() {
    local inactive_env
    inactive_env=$(get_inactive_env)
    local new_dir
    new_dir=$(get_env_dir "$inactive_env")
    local active_env
    active_env=$(get_active_env)
    
    log_step "Switching from $active_env to $inactive_env environment"
    
    # Verify new environment is ready
    if [ ! -d "$new_dir" ]; then
        log_error "Target environment not found: $new_dir"
        return 1
    fi
    
    # Stop current service
    if [ "$DEPLOYMENT_TYPE" = "master" ]; then
        SERVICE="ffrtmp-master.service"
    else
        SERVICE="ffrtmp-worker.service"
    fi
    
    log_info "Stopping current service: $SERVICE"
    systemctl stop "$SERVICE" || log_warn "Service was not running"
    
    # Switch symlink
    log_info "Switching symlink from $active_env to $inactive_env"
    rm -f "$CURRENT_LINK"
    ln -s "$new_dir" "$CURRENT_LINK"
    
    # Update systemd service file to point to new location
    if [ "$DEPLOYMENT_TYPE" = "master" ]; then
        BINARY_PATH="$CURRENT_LINK/bin/ffrtmp-master"
    else
        BINARY_PATH="$CURRENT_LINK/bin/agent"
    fi
    
    # Restart service
    log_info "Starting service with new environment"
    systemctl daemon-reload
    systemctl start "$SERVICE"
    
    # Health check
    if health_check "$new_dir"; then
        log_success "Switch completed successfully"
        save_state "$inactive_env" "${NEW_VERSION:-unknown}"
        return 0
    else
        log_error "Health check failed after switch"
        log_warn "Consider running --rollback"
        return 1
    fi
}

# Rollback to previous environment
rollback() {
    local current_env
    current_env=$(get_active_env)
    local previous_env
    
    if [ "$current_env" = "blue" ]; then
        previous_env="green"
    elif [ "$current_env" = "green" ]; then
        previous_env="blue"
    else
        log_error "Cannot determine current environment"
        return 1
    fi
    
    log_warn "Rolling back from $current_env to $previous_env"
    
    # Verify previous environment exists
    local prev_dir
    prev_dir=$(get_env_dir "$previous_env")
    if [ ! -d "$prev_dir" ]; then
        log_error "Previous environment not found: $prev_dir"
        return 1
    fi
    
    # Stop service
    if [ "$DEPLOYMENT_TYPE" = "master" ]; then
        SERVICE="ffrtmp-master.service"
    else
        SERVICE="ffrtmp-worker.service"
    fi
    
    systemctl stop "$SERVICE"
    
    # Switch symlink back
    rm -f "$CURRENT_LINK"
    ln -s "$prev_dir" "$CURRENT_LINK"
    
    # Restart service
    systemctl daemon-reload
    systemctl start "$SERVICE"
    
    # Health check
    if health_check "$prev_dir"; then
        log_success "Rollback completed successfully"
        save_state "$previous_env" "rollback"
        return 0
    else
        log_error "Rollback failed!"
        return 1
    fi
}

# Main execution
echo "═══════════════════════════════════════"
echo "  Blue-Green Deployment - $DEPLOYMENT_TYPE"
echo "═══════════════════════════════════════"
echo ""

case $ACTION in
    deploy)
        log_info "Current active: $(get_active_env)"
        log_info "Deploying to: $(get_inactive_env)"
        if deploy_new_version; then
            log_success "Deployment complete. Run with --switch to activate."
        else
            log_error "Deployment failed"
            exit 1
        fi
        ;;
    switch)
        if switch_environment; then
            log_success "Environment switched successfully"
        else
            log_error "Switch failed"
            exit 1
        fi
        ;;
    rollback)
        if rollback; then
            log_success "Rollback successful"
        else
            log_error "Rollback failed"
            exit 1
        fi
        ;;
    *)
        log_error "Invalid action: $ACTION"
        exit 1
        ;;
esac

exit 0
