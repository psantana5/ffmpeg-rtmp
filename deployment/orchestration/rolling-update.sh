#!/bin/bash
# Rolling Update Manager for Worker Nodes
# Updates workers one at a time to maintain availability
# Usage: ./rolling-update.sh --workers <worker1,worker2,...> --version <version>

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
WORKERS=()
VERSION=""
MASTER_URL=""
API_KEY=""
MAX_PARALLEL=1
HEALTH_CHECK_WAIT=30
DRAIN_TIMEOUT=300
SSH_USER="root"
SSH_KEY=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --workers)
            IFS=',' read -ra WORKERS <<< "$2"
            shift 2
            ;;
        --version) VERSION="$2"; shift 2 ;;
        --master-url) MASTER_URL="$2"; shift 2 ;;
        --api-key) API_KEY="$2"; shift 2 ;;
        --max-parallel) MAX_PARALLEL="$2"; shift 2 ;;
        --ssh-user) SSH_USER="$2"; shift 2 ;;
        --ssh-key) SSH_KEY="$2"; shift 2 ;;
        --drain-timeout) DRAIN_TIMEOUT="$2"; shift 2 ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

# Validation
if [ ${#WORKERS[@]} -eq 0 ]; then
    log_error "No workers specified. Use --workers worker1,worker2,..."
    exit 1
fi

if [ -z "$VERSION" ]; then
    log_error "Version required: --version <version>"
    exit 1
fi

# SSH command helper
ssh_cmd() {
    local host=$1
    shift
    local cmd="$@"
    
    if [ -n "$SSH_KEY" ]; then
        ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "$SSH_USER@$host" "$cmd"
    else
        ssh -o StrictHostKeyChecking=no "$SSH_USER@$host" "$cmd"
    fi
}

# Check worker connectivity
check_worker_connectivity() {
    local worker=$1
    log_step "Checking connectivity to $worker"
    
    if ssh_cmd "$worker" "echo 'ok'" >/dev/null 2>&1; then
        log_success "Can connect to $worker"
        return 0
    else
        log_error "Cannot connect to $worker"
        return 1
    fi
}

# Drain worker (mark as unavailable for new jobs)
drain_worker() {
    local worker=$1
    log_step "Draining worker: $worker"
    
    if [ -n "$MASTER_URL" ] && [ -n "$API_KEY" ]; then
        # Call master API to drain worker
        local response
        response=$(curl -s -X POST \
            -H "X-API-Key: $API_KEY" \
            -H "Content-Type: application/json" \
            "$MASTER_URL/api/v1/workers/$worker/drain" 2>/dev/null || echo "")
        
        if [ -n "$response" ]; then
            log_success "Worker $worker marked for draining"
        else
            log_warn "Could not drain worker via API, proceeding anyway"
        fi
    fi
    
    # Wait for running jobs to complete
    log_info "Waiting for running jobs to complete (timeout: ${DRAIN_TIMEOUT}s)"
    local elapsed=0
    while [ $elapsed -lt $DRAIN_TIMEOUT ]; do
        local running_jobs
        running_jobs=$(ssh_cmd "$worker" "ps aux | grep -c '[f]fmpeg' || echo 0")
        
        if [ "$running_jobs" -eq 0 ]; then
            log_success "All jobs completed on $worker"
            return 0
        fi
        
        log_info "Worker $worker has $running_jobs running jobs, waiting..."
        sleep 10
        ((elapsed+=10))
    done
    
    log_warn "Drain timeout reached, some jobs may still be running"
    return 1
}

# Update worker
update_worker() {
    local worker=$1
    log_step "Updating worker: $worker"
    
    # Stop services
    log_info "Stopping services on $worker"
    ssh_cmd "$worker" "systemctl stop ffrtmp-worker.service ffrtmp-watch.service 2>/dev/null || true"
    
    # Backup current installation
    log_info "Creating backup on $worker"
    ssh_cmd "$worker" "[ -d /opt/ffrtmp ] && cp -a /opt/ffrtmp /opt/ffrtmp.backup-\$(date +%Y%m%d-%H%M%S) || true"
    
    # Copy new binaries
    log_info "Deploying version $VERSION to $worker"
    
    # Create temporary tarball
    local tarball="/tmp/ffrtmp-${VERSION}.tar.gz"
    if [ ! -f "$tarball" ]; then
        log_info "Creating deployment tarball"
        tar -czf "$tarball" -C ./bin . 2>/dev/null || {
            log_error "Failed to create tarball"
            return 1
        }
    fi
    
    # Copy and extract
    scp -q "$tarball" "$SSH_USER@$worker:/tmp/"
    ssh_cmd "$worker" "mkdir -p /opt/ffrtmp/bin && tar -xzf /tmp/ffrtmp-${VERSION}.tar.gz -C /opt/ffrtmp/bin"
    ssh_cmd "$worker" "chmod +x /opt/ffrtmp/bin/* && chown -R ffrtmp:ffrtmp /opt/ffrtmp"
    
    # Start services
    log_info "Starting services on $worker"
    ssh_cmd "$worker" "systemctl daemon-reload && systemctl start ffrtmp-worker.service"
    
    # Health check
    sleep $HEALTH_CHECK_WAIT
    if ssh_cmd "$worker" "systemctl is-active --quiet ffrtmp-worker.service"; then
        log_success "Worker $worker updated successfully"
        return 0
    else
        log_error "Worker $worker failed health check"
        return 1
    fi
}

# Activate worker (mark as available)
activate_worker() {
    local worker=$1
    log_step "Activating worker: $worker"
    
    if [ -n "$MASTER_URL" ] && [ -n "$API_KEY" ]; then
        curl -s -X POST \
            -H "X-API-Key: $API_KEY" \
            "$MASTER_URL/api/v1/workers/$worker/activate" >/dev/null 2>&1 || true
    fi
    
    log_success "Worker $worker activated"
}

# Rollback worker
rollback_worker() {
    local worker=$1
    log_warn "Rolling back worker: $worker"
    
    ssh_cmd "$worker" "systemctl stop ffrtmp-worker.service"
    
    # Find most recent backup
    local backup
    backup=$(ssh_cmd "$worker" "ls -1dt /opt/ffrtmp.backup-* 2>/dev/null | head -1" || echo "")
    
    if [ -n "$backup" ]; then
        log_info "Restoring from backup: $backup"
        ssh_cmd "$worker" "rm -rf /opt/ffrtmp && mv $backup /opt/ffrtmp"
        ssh_cmd "$worker" "systemctl start ffrtmp-worker.service"
        log_success "Rollback completed for $worker"
    else
        log_error "No backup found for $worker"
        return 1
    fi
}

# Main execution
echo "═══════════════════════════════════════════════════"
echo "  Rolling Update for Worker Nodes"
echo "═══════════════════════════════════════════════════"
echo ""
echo "Workers: ${WORKERS[*]}"
echo "Version: $VERSION"
echo "Max parallel: $MAX_PARALLEL"
echo ""

FAILED_WORKERS=()
UPDATED_WORKERS=()

# Check connectivity to all workers first
log_info "Checking connectivity to all workers..."
for worker in "${WORKERS[@]}"; do
    if ! check_worker_connectivity "$worker"; then
        FAILED_WORKERS+=("$worker")
    fi
done

if [ ${#FAILED_WORKERS[@]} -gt 0 ]; then
    log_error "Cannot connect to: ${FAILED_WORKERS[*]}"
    log_error "Fix connectivity issues before proceeding"
    exit 1
fi

log_success "All workers are reachable"
echo ""

# Update workers one by one (or in batches if MAX_PARALLEL > 1)
BATCH_INDEX=0
for worker in "${WORKERS[@]}"; do
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "Processing worker $((BATCH_INDEX + 1))/${#WORKERS[@]}: $worker"
    echo ""
    
    # Drain worker
    if ! drain_worker "$worker"; then
        log_warn "Drain completed with warnings, continuing..."
    fi
    
    # Update worker
    if update_worker "$worker"; then
        UPDATED_WORKERS+=("$worker")
        activate_worker "$worker"
        log_success "Worker $worker updated successfully"
    else
        log_error "Failed to update worker $worker"
        FAILED_WORKERS+=("$worker")
        
        # Ask if should rollback
        echo ""
        read -p "Rollback $worker? (y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rollback_worker "$worker"
        fi
        
        # Ask if should continue
        echo ""
        read -p "Continue with remaining workers? (y/n): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_warn "Aborting rolling update"
            break
        fi
    fi
    
    ((BATCH_INDEX++))
    
    # Wait between workers
    if [ $BATCH_INDEX -lt ${#WORKERS[@]} ]; then
        log_info "Waiting 10 seconds before next worker..."
        sleep 10
    fi
    
    echo ""
done

# Summary
echo "═══════════════════════════════════════"
echo "  Rolling Update Summary"
echo "═══════════════════════════════════════"
echo -e "${GREEN}Updated:${NC}  ${#UPDATED_WORKERS[@]} workers"
if [ ${#UPDATED_WORKERS[@]} -gt 0 ]; then
    for w in "${UPDATED_WORKERS[@]}"; do
        echo "  ✓ $w"
    done
fi
echo ""
echo -e "${RED}Failed:${NC}   ${#FAILED_WORKERS[@]} workers"
if [ ${#FAILED_WORKERS[@]} -gt 0 ]; then
    for w in "${FAILED_WORKERS[@]}"; do
        echo "  ✗ $w"
    done
fi
echo ""

if [ ${#FAILED_WORKERS[@]} -gt 0 ]; then
    log_error "Rolling update completed with failures"
    exit 1
else
    log_success "Rolling update completed successfully!"
    exit 0
fi
