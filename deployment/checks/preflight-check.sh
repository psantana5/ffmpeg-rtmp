#!/bin/bash
# Pre-flight Checks for FFmpeg-RTMP Deployment
# Verifies system requirements before deployment
# Usage: ./preflight-check.sh [--master|--worker]

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
log_step() { echo -e "${CYAN}[CHECK]${NC} $1"; }

# Counters
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNED=0

# Configuration
NODE_TYPE=""
MASTER_URL=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --master) NODE_TYPE="master"; shift ;;
        --worker) NODE_TYPE="worker"; shift ;;
        --master-url) MASTER_URL="$2"; shift 2 ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

if [ -z "$NODE_TYPE" ]; then
    log_error "Node type required: --master or --worker"
    exit 1
fi

echo "═══════════════════════════════════════════════════════"
echo "  FFmpeg-RTMP Pre-flight Checks - $NODE_TYPE Node"
echo "═══════════════════════════════════════════════════════"
echo ""

# Check if running as root
log_step "Checking user permissions"
if [ "$EUID" -ne 0 ]; then
    log_error "Must run as root (use sudo)"
    ((CHECKS_FAILED++))
else
    log_success "Running as root"
    ((CHECKS_PASSED++))
fi

# OS Detection
log_step "Detecting operating system"
if [ -f /etc/os-release ]; then
    . /etc/os-release
    log_success "OS: $NAME $VERSION"
    ((CHECKS_PASSED++))
    
    # Check for supported OS
    case $ID in
        ubuntu|debian|rocky|almalinux|centos|rhel)
            log_success "Supported OS detected"
            ((CHECKS_PASSED++))
            ;;
        *)
            log_warn "OS may not be fully supported: $ID"
            ((CHECKS_WARNED++))
            ;;
    esac
else
    log_error "Cannot detect OS"
    ((CHECKS_FAILED++))
fi

# Kernel version
log_step "Checking kernel version"
KERNEL_VERSION=$(uname -r)
log_info "Kernel: $KERNEL_VERSION"
((CHECKS_PASSED++))

# Architecture
log_step "Checking architecture"
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    log_success "Architecture: $ARCH (supported)"
    ((CHECKS_PASSED++))
else
    log_warn "Architecture: $ARCH (may not be fully tested)"
    ((CHECKS_WARNED++))
fi

# CPU cores
log_step "Checking CPU cores"
CPU_CORES=$(nproc)
MIN_CORES=2
if [ "$CPU_CORES" -ge "$MIN_CORES" ]; then
    log_success "CPU cores: $CPU_CORES (minimum: $MIN_CORES)"
    ((CHECKS_PASSED++))
else
    log_warn "CPU cores: $CPU_CORES (recommended: $MIN_CORES or more)"
    ((CHECKS_WARNED++))
fi

# Memory
log_step "Checking memory"
TOTAL_MEM_GB=$(free -g | awk '/^Mem:/{print $2}')
if [ "$NODE_TYPE" = "master" ]; then
    MIN_MEM=4
else
    MIN_MEM=8
fi

if [ "$TOTAL_MEM_GB" -ge "$MIN_MEM" ]; then
    log_success "Memory: ${TOTAL_MEM_GB}GB (minimum: ${MIN_MEM}GB)"
    ((CHECKS_PASSED++))
else
    log_error "Memory: ${TOTAL_MEM_GB}GB (need at least ${MIN_MEM}GB)"
    ((CHECKS_FAILED++))
fi

# Disk space
log_step "Checking disk space"
if [ "$NODE_TYPE" = "master" ]; then
    ROOT_MIN=20
    VAR_MIN=10
else
    ROOT_MIN=20
    VAR_MIN=100
fi

ROOT_AVAIL=$(df -BG / | tail -1 | awk '{print $4}' | sed 's/G//')
if [ "$ROOT_AVAIL" -ge "$ROOT_MIN" ]; then
    log_success "Root filesystem: ${ROOT_AVAIL}GB available (minimum: ${ROOT_MIN}GB)"
    ((CHECKS_PASSED++))
else
    log_error "Root filesystem: ${ROOT_AVAIL}GB (need ${ROOT_MIN}GB)"
    ((CHECKS_FAILED++))
fi

# Check /var space for master, /opt for worker
if [ "$NODE_TYPE" = "master" ]; then
    VAR_AVAIL=$(df -BG /var 2>/dev/null | tail -1 | awk '{print $4}' | sed 's/G//' || echo "$ROOT_AVAIL")
    if [ "$VAR_AVAIL" -ge "$VAR_MIN" ]; then
        log_success "/var filesystem: ${VAR_AVAIL}GB available (minimum: ${VAR_MIN}GB)"
        ((CHECKS_PASSED++))
    else
        log_error "/var filesystem: ${VAR_AVAIL}GB (need ${VAR_MIN}GB)"
        ((CHECKS_FAILED++))
    fi
else
    OPT_AVAIL=$(df -BG /opt 2>/dev/null | tail -1 | awk '{print $4}' | sed 's/G//' || echo "$ROOT_AVAIL")
    if [ "$OPT_AVAIL" -ge "$VAR_MIN" ]; then
        log_success "/opt filesystem: ${OPT_AVAIL}GB available (minimum: ${VAR_MIN}GB)"
        ((CHECKS_PASSED++))
    else
        log_error "/opt filesystem: ${OPT_AVAIL}GB (need ${VAR_MIN}GB)"
        ((CHECKS_FAILED++))
    fi
fi

# Check ports availability
log_step "Checking port availability"
if [ "$NODE_TYPE" = "master" ]; then
    REQUIRED_PORTS=(8080)
    OPTIONAL_PORTS=(1935 9090 3000 5432)
else
    REQUIRED_PORTS=()
    OPTIONAL_PORTS=(1935)
fi

for port in "${REQUIRED_PORTS[@]}"; do
    if ss -tlnp | grep -q ":$port "; then
        log_error "Port $port is already in use"
        ((CHECKS_FAILED++))
        ss -tlnp | grep ":$port "
    else
        log_success "Port $port is available"
        ((CHECKS_PASSED++))
    fi
done

for port in "${OPTIONAL_PORTS[@]}"; do
    if ss -tlnp | grep -q ":$port "; then
        log_warn "Port $port is in use (optional)"
        ((CHECKS_WARNED++))
    else
        log_success "Port $port is available"
        ((CHECKS_PASSED++))
    fi
done

# Check for existing installations
log_step "Checking for existing installations"
if [ "$NODE_TYPE" = "master" ]; then
    if [ -d "/opt/ffrtmp-master" ]; then
        log_warn "Existing master installation found at /opt/ffrtmp-master"
        ((CHECKS_WARNED++))
    else
        log_success "No existing master installation"
        ((CHECKS_PASSED++))
    fi
else
    if [ -d "/opt/ffrtmp" ]; then
        log_warn "Existing worker installation found at /opt/ffrtmp"
        ((CHECKS_WARNED++))
    else
        log_success "No existing worker installation"
        ((CHECKS_PASSED++))
    fi
fi

# Check systemd
log_step "Checking systemd"
if command -v systemctl >/dev/null 2>&1; then
    log_success "systemd is available"
    ((CHECKS_PASSED++))
else
    log_error "systemd not found"
    ((CHECKS_FAILED++))
fi

# Check for required commands
log_step "Checking required commands"
REQUIRED_CMDS=(curl wget git tar gzip)
for cmd in "${REQUIRED_CMDS[@]}"; do
    if command -v "$cmd" >/dev/null 2>&1; then
        log_success "$cmd is installed"
        ((CHECKS_PASSED++))
    else
        log_error "$cmd is not installed"
        ((CHECKS_FAILED++))
    fi
done

# Check Go version
log_step "Checking Go installation"
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
    
    if [ "$GO_MAJOR" -ge 1 ] && [ "$GO_MINOR" -ge 24 ]; then
        log_success "Go $GO_VERSION installed (required: 1.24+)"
        ((CHECKS_PASSED++))
    else
        log_warn "Go $GO_VERSION installed (recommended: 1.24+)"
        ((CHECKS_WARNED++))
    fi
else
    log_info "Go not installed (will be installed during deployment)"
    ((CHECKS_PASSED++))
fi

# Check FFmpeg (for worker nodes)
if [ "$NODE_TYPE" = "worker" ]; then
    log_step "Checking FFmpeg installation"
    if command -v ffmpeg >/dev/null 2>&1; then
        FFMPEG_VERSION=$(ffmpeg -version 2>/dev/null | head -1 | awk '{print $3}')
        log_success "FFmpeg $FFMPEG_VERSION installed"
        ((CHECKS_PASSED++))
        
        # Check for hardware acceleration support
        log_step "Checking FFmpeg encoders"
        if ffmpeg -encoders 2>/dev/null | grep -q "h264_nvenc"; then
            log_success "NVIDIA hardware encoding available"
            ((CHECKS_PASSED++))
        elif ffmpeg -encoders 2>/dev/null | grep -q "h264_vaapi"; then
            log_success "VAAPI hardware encoding available"
            ((CHECKS_PASSED++))
        else
            log_info "Software encoding only (no hardware acceleration detected)"
            ((CHECKS_PASSED++))
        fi
    else
        log_info "FFmpeg not installed (will be installed during deployment)"
        ((CHECKS_PASSED++))
    fi
fi

# Check cgroups v2
log_step "Checking cgroups version"
if [ -f "/sys/fs/cgroup/cgroup.controllers" ]; then
    log_success "Cgroups v2 is enabled"
    ((CHECKS_PASSED++))
else
    if [ "$NODE_TYPE" = "worker" ]; then
        log_warn "Cgroups v2 not detected (will enable during deployment)"
        ((CHECKS_WARNED++))
    else
        log_info "Cgroups v2 not enabled (not critical for master)"
        ((CHECKS_PASSED++))
    fi
fi

# Network connectivity
log_step "Checking network connectivity"
if ping -c 1 -W 2 8.8.8.8 >/dev/null 2>&1; then
    log_success "Internet connectivity available"
    ((CHECKS_PASSED++))
else
    log_error "No internet connectivity"
    ((CHECKS_FAILED++))
fi

# DNS resolution
log_step "Checking DNS resolution"
if host github.com >/dev/null 2>&1; then
    log_success "DNS resolution working"
    ((CHECKS_PASSED++))
else
    log_error "DNS resolution failed"
    ((CHECKS_FAILED++))
fi

# Master connectivity (for worker nodes)
if [ "$NODE_TYPE" = "worker" ] && [ -n "$MASTER_URL" ]; then
    log_step "Checking master connectivity"
    if curl -s --max-time 5 "${MASTER_URL}/health" >/dev/null 2>&1; then
        log_success "Can reach master at $MASTER_URL"
        ((CHECKS_PASSED++))
    else
        log_error "Cannot reach master at $MASTER_URL"
        ((CHECKS_FAILED++))
    fi
fi

# Firewall check
log_step "Checking firewall status"
if command -v ufw >/dev/null 2>&1; then
    UFW_STATUS=$(ufw status | head -1)
    log_info "UFW: $UFW_STATUS"
    ((CHECKS_PASSED++))
elif command -v firewall-cmd >/dev/null 2>&1; then
    FIREWALLD_STATUS=$(firewall-cmd --state 2>/dev/null || echo "not running")
    log_info "Firewalld: $FIREWALLD_STATUS"
    ((CHECKS_PASSED++))
else
    log_warn "No firewall detected (ufw or firewalld recommended)"
    ((CHECKS_WARNED++))
fi

# SELinux check (for RHEL-based systems)
if command -v getenforce >/dev/null 2>&1; then
    log_step "Checking SELinux status"
    SELINUX_STATUS=$(getenforce)
    if [ "$SELINUX_STATUS" = "Enforcing" ]; then
        log_warn "SELinux is enforcing (may require additional configuration)"
        ((CHECKS_WARNED++))
    else
        log_info "SELinux: $SELINUX_STATUS"
        ((CHECKS_PASSED++))
    fi
fi

# Summary
echo ""
echo "═══════════════════════════════════════"
echo "  Pre-flight Check Summary"
echo "═══════════════════════════════════════"
echo -e "${GREEN}Passed:${NC}   $CHECKS_PASSED"
echo -e "${YELLOW}Warnings:${NC}  $CHECKS_WARNED"
echo -e "${RED}Failed:${NC}   $CHECKS_FAILED"
echo ""

if [ $CHECKS_FAILED -gt 0 ]; then
    log_error "Pre-flight checks failed! Please resolve issues before deployment."
    exit 1
elif [ $CHECKS_WARNED -gt 0 ]; then
    log_warn "Pre-flight checks passed with warnings. Review warnings before proceeding."
    exit 0
else
    log_success "All pre-flight checks passed! System is ready for deployment."
    exit 0
fi
