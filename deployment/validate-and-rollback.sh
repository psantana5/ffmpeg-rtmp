#!/bin/bash
# Deployment Validator with Dry-Run and Rollback Capability
# Tests deployment readiness without making changes
# Supports: validation, dry-run, rollback
# Usage: ./deployment/validate-and-rollback.sh [--validate|--dry-run|--rollback] [--master|--worker]

set +e  # Don't exit on errors, we handle them

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
log_check() { echo -e "${CYAN}[CHECK]${NC} $1"; }
log_dryrun() { echo -e "${MAGENTA}[DRY-RUN]${NC} $1"; }

# Modes
MODE="validate"  # validate, dry-run, rollback
DEPLOY_TYPE=""   # master, worker
VERBOSE=false

# State tracking for rollback
BACKUP_DIR="/tmp/ffrtmp-rollback-$(date +%s)"
CHANGES_MADE=()

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --validate)
            MODE="validate"
            shift
            ;;
        --dry-run)
            MODE="dry-run"
            shift
            ;;
        --rollback)
            MODE="rollback"
            shift
            ;;
        --master)
            DEPLOY_TYPE="master"
            shift
            ;;
        --worker|--edge)
            DEPLOY_TYPE="worker"
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            cat << 'EOF'
Deployment Validator with Dry-Run and Rollback

Usage: ./deployment/validate-and-rollback.sh [MODE] [TYPE] [OPTIONS]

Modes:
  --validate       Validate system readiness (default)
  --dry-run        Show what would be done without making changes
  --rollback       Rollback previous deployment

Deployment Type:
  --master         Validate/deploy master node
  --worker         Validate/deploy worker node

Options:
  --verbose, -v    Show detailed output
  --help, -h       Show this help

Examples:
  # Validate system before worker deployment
  ./deployment/validate-and-rollback.sh --validate --worker

  # Dry-run master deployment
  ./deployment/validate-and-rollback.sh --dry-run --master

  # Rollback failed worker deployment
  ./deployment/validate-and-rollback.sh --rollback --worker

EOF
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# ============================================
# VALIDATION CHECKS
# ============================================

check_root() {
    log_check "Checking root privileges..."
    if [ "$MODE" = "validate" ]; then
        # Validation mode doesn't need root
        log_success "Root check skipped (validation mode)"
        return 0
    fi
    
    if [ "$EUID" -ne 0 ]; then
        log_error "Root privileges required for $MODE mode"
        return 1
    fi
    log_success "Running as root"
    return 0
}

check_os() {
    log_check "Checking operating system..."
    
    if [ ! -f /etc/os-release ]; then
        log_warn "Cannot detect OS version"
        return 0
    fi
    
    source /etc/os-release
    log_success "OS: $PRETTY_NAME"
    
    # Check for systemd
    if ! command -v systemctl &> /dev/null; then
        log_error "systemd not found (required)"
        return 1
    fi
    log_success "systemd available"
    
    return 0
}

check_cgroups() {
    log_check "Checking cgroups v2..."
    
    local cgroup_type=$(stat -fc %T /sys/fs/cgroup 2>/dev/null)
    
    if [ "$cgroup_type" = "cgroup2fs" ]; then
        log_success "cgroups v2 enabled"
        return 0
    else
        log_warn "cgroups v2 not enabled (kernel parameter required)"
        log_info "Add to kernel: systemd.unified_cgroup_hierarchy=1"
        return 1
    fi
}

check_go_installed() {
    log_check "Checking Go installation..."
    
    if ! command -v go &> /dev/null; then
        log_warn "Go not installed (needed to build from source)"
        return 1
    fi
    
    local go_version=$(go version | awk '{print $3}')
    log_success "Go installed: $go_version"
    return 0
}

check_binaries_exist() {
    log_check "Checking binaries..."
    
    local missing=0
    
    if [ "$DEPLOY_TYPE" = "master" ]; then
        if [ ! -f "bin/master" ]; then
            log_warn "Master binary not found: bin/master"
            missing=1
        else
            log_success "Master binary exists"
        fi
    fi
    
    if [ "$DEPLOY_TYPE" = "worker" ]; then
        if [ ! -f "bin/agent" ]; then
            log_warn "Agent binary not found: bin/agent"
            missing=1
        else
            log_success "Agent binary exists"
        fi
        
        if [ ! -f "bin/ffrtmp" ]; then
            log_warn "CLI binary not found: bin/ffrtmp"
            missing=1
        else
            log_success "CLI binary exists"
        fi
    fi
    
    return $missing
}

check_disk_space() {
    log_check "Checking disk space..."
    
    local available=$(df / | awk 'NR==2 {print $4}')
    local required=1048576  # 1GB in KB
    
    if [ "$available" -lt "$required" ]; then
        log_error "Insufficient disk space (need 1GB, have $(( available / 1024 ))MB)"
        return 1
    fi
    
    log_success "Disk space sufficient: $(( available / 1024 ))MB available"
    return 0
}

check_ports() {
    log_check "Checking required ports..."
    
    local ports_to_check=()
    
    if [ "$DEPLOY_TYPE" = "master" ]; then
        ports_to_check=(8080 9090)
    fi
    
    if [ "$DEPLOY_TYPE" = "worker" ]; then
        ports_to_check=(9091)
    fi
    
    local ports_in_use=0
    for port in "${ports_to_check[@]}"; do
        if ss -tuln | grep -q ":$port "; then
            log_warn "Port $port already in use"
            ports_in_use=1
        else
            log_success "Port $port available"
        fi
    done
    
    return $ports_in_use
}

check_existing_services() {
    log_check "Checking for existing services..."
    
    local services=()
    if [ "$DEPLOY_TYPE" = "master" ]; then
        services=(ffrtmp-master)
    fi
    if [ "$DEPLOY_TYPE" = "worker" ]; then
        services=(ffrtmp-worker ffrtmp-watch)
    fi
    
    local existing=0
    for service in "${services[@]}"; do
        if systemctl list-unit-files | grep -q "^${service}.service"; then
            log_warn "Service already exists: ${service}.service"
            existing=1
        else
            log_success "Service does not exist: ${service}.service"
        fi
    done
    
    return $existing
}

check_idempotency() {
    log_check "Checking idempotency (can re-run safely)..."
    
    local issues=0
    
    # Check if user exists
    if [ "$DEPLOY_TYPE" = "master" ]; then
        if id "ffrtmp-master" &>/dev/null; then
            log_success "User ffrtmp-master exists (will be reused)"
        fi
        
        if [ -f "/etc/ffrtmp-master/api-key" ]; then
            log_success "API key exists (will be preserved)"
        fi
    fi
    
    if [ "$DEPLOY_TYPE" = "worker" ]; then
        if id "ffrtmp" &>/dev/null; then
            log_success "User ffrtmp exists (will be reused)"
        fi
        
        if [ -f "/var/lib/ffrtmp/watch-state.json" ]; then
            log_success "Watch state exists (will be preserved)"
        fi
    fi
    
    return 0
}

# ============================================
# DRY-RUN SIMULATION
# ============================================

dry_run_worker() {
    log_dryrun "Worker deployment simulation"
    echo ""
    
    log_dryrun "[1/9] Would check dependencies"
    log_info "  - systemd, cgroups v2, disk space"
    
    log_dryrun "[2/9] Would build binaries"
    log_info "  - make build-agent (bin/agent)"
    log_info "  - make build-cli (bin/ffrtmp)"
    
    log_dryrun "[3/9] Would create user 'ffrtmp'"
    log_info "  - useradd -r -s /bin/false ffrtmp"
    
    log_dryrun "[4/9] Would create directories"
    log_info "  - /opt/ffrtmp/{bin,streams,logs}"
    log_info "  - /var/lib/ffrtmp"
    log_info "  - /var/log/ffrtmp"
    log_info "  - /etc/ffrtmp"
    
    log_dryrun "[5/9] Would install binaries"
    log_info "  - bin/agent → /opt/ffrtmp/bin/agent"
    log_info "  - bin/ffrtmp → /opt/ffrtmp/bin/ffrtmp"
    
    log_dryrun "[6/9] Would configure cgroups"
    log_info "  - Enable CPU and memory controllers"
    
    log_dryrun "[7/9] Would install systemd services"
    log_info "  - ffrtmp-worker.service"
    log_info "  - ffrtmp-watch.service"
    
    log_dryrun "[8/9] Would create configurations"
    log_info "  - /etc/ffrtmp/worker.env"
    log_info "  - /etc/ffrtmp/watch-config.yaml"
    
    log_dryrun "[9/9] Would enable and start services"
    log_info "  - systemctl enable ffrtmp-worker ffrtmp-watch"
    log_info "  - systemctl start ffrtmp-worker ffrtmp-watch"
    
    echo ""
    log_success "Dry-run complete (no changes made)"
}

dry_run_master() {
    log_dryrun "Master deployment simulation"
    echo ""
    
    log_dryrun "[1/8] Would check dependencies"
    log_dryrun "[2/8] Would build binary (bin/master)"
    log_dryrun "[3/8] Would create user 'ffrtmp-master'"
    log_dryrun "[4/8] Would create directories"
    log_info "  - /opt/ffrtmp-master/bin"
    log_info "  - /etc/ffrtmp-master"
    log_info "  - /var/lib/ffrtmp-master"
    log_info "  - /var/log/ffrtmp-master"
    
    log_dryrun "[5/8] Would install binary"
    log_info "  - bin/master → /opt/ffrtmp-master/bin/ffrtmp-master"
    
    log_dryrun "[6/8] Would create configuration"
    log_info "  - /etc/ffrtmp-master/master.env"
    log_info "  - /etc/ffrtmp-master/api-key (generated)"
    
    log_dryrun "[7/8] Would install systemd service"
    log_info "  - ffrtmp-master.service"
    
    log_dryrun "[8/8] Would start service"
    log_info "  - systemctl enable ffrtmp-master"
    log_info "  - systemctl start ffrtmp-master"
    
    echo ""
    log_success "Dry-run complete (no changes made)"
}

# ============================================
# ROLLBACK FUNCTIONALITY
# ============================================

find_backup() {
    log_info "Searching for backups..."
    
    local backups=($(ls -dt /tmp/ffrtmp-rollback-* 2>/dev/null))
    
    if [ ${#backups[@]} -eq 0 ]; then
        log_error "No backups found in /tmp/ffrtmp-rollback-*"
        return 1
    fi
    
    log_info "Found ${#backups[@]} backup(s)"
    echo ""
    
    for i in "${!backups[@]}"; do
        local backup="${backups[$i]}"
        local timestamp=$(basename "$backup" | cut -d'-' -f3)
        local date=$(date -d "@$timestamp" 2>/dev/null || echo "unknown date")
        echo "  [$i] $backup ($date)"
    done
    
    echo ""
    read -p "Select backup to restore [0]: " selection
    selection=${selection:-0}
    
    if [ "$selection" -ge 0 ] && [ "$selection" -lt "${#backups[@]}" ]; then
        BACKUP_DIR="${backups[$selection]}"
        log_success "Selected: $BACKUP_DIR"
        return 0
    else
        log_error "Invalid selection"
        return 1
    fi
}

rollback_worker() {
    log_info "Rolling back worker deployment..."
    
    # Stop services
    log_info "Stopping services..."
    systemctl stop ffrtmp-watch 2>/dev/null || true
    systemctl stop ffrtmp-worker 2>/dev/null || true
    
    # Remove service files
    log_info "Removing service files..."
    rm -f /etc/systemd/system/ffrtmp-worker.service
    rm -f /etc/systemd/system/ffrtmp-watch.service
    systemctl daemon-reload
    
    # Restore backed up files if they exist
    if [ -d "$BACKUP_DIR" ]; then
        log_info "Restoring from backup: $BACKUP_DIR"
        
        if [ -f "$BACKUP_DIR/worker.env" ]; then
            cp "$BACKUP_DIR/worker.env" /etc/ffrtmp/worker.env
            log_success "Restored worker.env"
        fi
        
        if [ -f "$BACKUP_DIR/watch-state.json" ]; then
            cp "$BACKUP_DIR/watch-state.json" /var/lib/ffrtmp/watch-state.json
            log_success "Restored watch state"
        fi
    fi
    
    log_success "Rollback complete"
    log_warn "Binaries and directories were NOT removed (manual cleanup required)"
}

rollback_master() {
    log_info "Rolling back master deployment..."
    
    # Stop service
    systemctl stop ffrtmp-master 2>/dev/null || true
    
    # Remove service file
    rm -f /etc/systemd/system/ffrtmp-master.service
    systemctl daemon-reload
    
    # Restore backup if exists
    if [ -d "$BACKUP_DIR" ]; then
        log_info "Restoring from backup: $BACKUP_DIR"
        
        if [ -f "$BACKUP_DIR/master.env" ]; then
            cp "$BACKUP_DIR/master.env" /etc/ffrtmp-master/master.env
            log_success "Restored master.env"
        fi
    fi
    
    log_success "Rollback complete"
    log_warn "Binaries and directories were NOT removed (manual cleanup required)"
}

# ============================================
# MAIN EXECUTION
# ============================================

echo ""
echo "╔════════════════════════════════════════╗"
echo "║  Deployment Validator & Rollback Tool  ║"
echo "╚════════════════════════════════════════╝"
echo ""

log_info "Mode: $MODE"
[ -n "$DEPLOY_TYPE" ] && log_info "Type: $DEPLOY_TYPE"
echo ""

# Execute based on mode
case $MODE in
    validate)
        log_info "Running validation checks..."
        echo ""
        
        FAILURES=0
        
        check_root || ((FAILURES++))
        check_os || ((FAILURES++))
        check_disk_space || ((FAILURES++))
        
        if [ -n "$DEPLOY_TYPE" ]; then
            check_binaries_exist || log_warn "Binaries missing (will build if Go available)"
            check_go_installed || ((FAILURES++))
            check_cgroups || log_warn "Cgroups v2 recommended but not required"
            check_ports || log_warn "Ports in use (services may fail to start)"
            check_existing_services || log_warn "Existing services will be updated"
            check_idempotency
        fi
        
        echo ""
        if [ $FAILURES -eq 0 ]; then
            log_success "✓ All validation checks passed"
            exit 0
        else
            log_error "✗ $FAILURES validation check(s) failed"
            exit 1
        fi
        ;;
        
    dry-run)
        if [ -z "$DEPLOY_TYPE" ]; then
            log_error "Deployment type required for dry-run (--master or --worker)"
            exit 1
        fi
        
        if [ "$DEPLOY_TYPE" = "worker" ]; then
            dry_run_worker
        else
            dry_run_master
        fi
        exit 0
        ;;
        
    rollback)
        check_root || exit 1
        
        if [ -z "$DEPLOY_TYPE" ]; then
            log_error "Deployment type required for rollback (--master or --worker)"
            exit 1
        fi
        
        find_backup || exit 1
        
        echo ""
        read -p "Confirm rollback? [y/N] " confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            log_info "Rollback cancelled"
            exit 0
        fi
        
        if [ "$DEPLOY_TYPE" = "worker" ]; then
            rollback_worker
        else
            rollback_master
        fi
        exit 0
        ;;
esac
