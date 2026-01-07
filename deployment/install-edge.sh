#!/bin/bash
# Production-Ready Edge Node Deployment Script
# Installs FFmpeg-RTMP worker agent and watch daemon with all dependencies
# Run as root: sudo ./deployment/install-edge.sh

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/ffrtmp/bin"
CONFIG_DIR="/etc/ffrtmp"
DATA_DIR="/var/lib/ffrtmp"
LOG_DIR="/var/log/ffrtmp"
STREAM_DIR="/opt/ffrtmp/streams"
APP_LOG_DIR="/opt/ffrtmp/logs"
USER="ffrtmp"
GROUP="ffrtmp"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Error handler
error_exit() {
    log_error "$1"
    exit 1
}

# Banner
echo ""
echo "=========================================="
echo "  FFmpeg-RTMP Edge Node Deployment"
echo "  Production-Ready Installation"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
   error_exit "This script must be run as root (use sudo)"
fi

# Check if running from project root
if [ ! -f "deployment/install-edge.sh" ]; then
    error_exit "Please run this script from the project root directory"
fi

# ============================================
# STEP 1: Dependency Check
# ============================================
echo ""
log_info "[1/9] Checking dependencies..."

MISSING_DEPS=0

# Check for required binaries
if ! command -v systemctl &> /dev/null; then
    log_error "systemctl not found (systemd required)"
    MISSING_DEPS=1
fi

if ! command -v useradd &> /dev/null; then
    log_error "useradd not found"
    MISSING_DEPS=1
fi

# Check for FFmpeg (optional but recommended)
if command -v ffmpeg &> /dev/null; then
    FFMPEG_VER=$(ffmpeg -version 2>&1 | head -1)
    log_success "FFmpeg found: $FFMPEG_VER"
else
    log_warn "FFmpeg not found (required for transcoding workloads)"
fi

# Check cgroup v2
if mount | grep -q "cgroup2 on /sys/fs/cgroup"; then
    log_success "Cgroup v2 mounted"
else
    log_error "Cgroup v2 not mounted (required for resource limits)"
    MISSING_DEPS=1
fi

# Check kernel version
KERNEL_VERSION=$(uname -r)
KERNEL_MAJOR=$(echo $KERNEL_VERSION | cut -d. -f1)
KERNEL_MINOR=$(echo $KERNEL_VERSION | cut -d. -f2)

if [ "$KERNEL_MAJOR" -gt 4 ] || [ "$KERNEL_MAJOR" -eq 4 -a "$KERNEL_MINOR" -ge 15 ]; then
    log_success "Kernel version: $KERNEL_VERSION (cgroup v2 supported)"
else
    log_error "Kernel version $KERNEL_VERSION < 4.15 (cgroup v2 may not work)"
    MISSING_DEPS=1
fi

if [ $MISSING_DEPS -ne 0 ]; then
    error_exit "Missing required dependencies. Please install them first."
fi

log_success "All dependencies satisfied"

# ============================================
# STEP 2: Build Binaries
# ============================================
echo ""
log_info "[2/9] Building binaries..."

# Check if binaries already exist
if [ ! -f "bin/agent" ] || [ ! -f "bin/ffrtmp" ]; then
    log_info "Binaries not found, building from source..."
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        error_exit "Go not found. Install Go 1.24+ or build binaries manually."
    fi
    
    # Build
    log_info "Building worker agent..."
    make build-agent || error_exit "Failed to build worker agent"
    
    log_info "Building CLI wrapper..."
    make build-cli || error_exit "Failed to build CLI wrapper"
    
    log_success "Binaries built successfully"
else
    log_success "Binaries already exist (bin/agent, bin/ffrtmp)"
fi

# Verify binaries exist
if [ ! -f "bin/agent" ]; then
    error_exit "Worker agent binary not found: bin/agent"
fi

if [ ! -f "bin/ffrtmp" ]; then
    error_exit "CLI wrapper binary not found: bin/ffrtmp"
fi

log_success "Binary verification passed"

# ============================================
# STEP 3: Create User and Group
# ============================================
echo ""
log_info "[3/9] Creating system user..."

if id "$USER" &>/dev/null; then
    log_success "User '$USER' already exists"
else
    useradd -r -s /bin/false -d "$DATA_DIR" -c "FFmpeg RTMP Service" "$USER" || error_exit "Failed to create user"
    log_success "User '$USER' created"
fi

if getent group "$GROUP" &>/dev/null; then
    log_success "Group '$GROUP' exists"
else
    log_warn "Group '$GROUP' should have been created with user"
fi

# ============================================
# STEP 4: Create Directory Structure
# ============================================
echo ""
log_info "[4/9] Creating directory structure..."

# Create all directories
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$INSTALL_DIR"
mkdir -p "$STREAM_DIR"
mkdir -p "$APP_LOG_DIR"
mkdir -p "$LOG_DIR"

log_success "Directories created:"
log_info "  • $CONFIG_DIR (configuration files)"
log_info "  • $DATA_DIR (state/persistent data)"
log_info "  • $INSTALL_DIR (binaries)"
log_info "  • $STREAM_DIR (stream files)"
log_info "  • $APP_LOG_DIR (application logs)"
log_info "  • $LOG_DIR (system logs)"

# Set ownership
chown -R "$USER:$GROUP" "$DATA_DIR"
chown -R "$USER:$GROUP" /opt/ffrtmp
chown -R "$USER:$GROUP" "$LOG_DIR"

log_success "Ownership set to $USER:$GROUP"

# Set permissions
chmod 755 /opt/ffrtmp
chmod 755 "$INSTALL_DIR"
chmod 775 "$DATA_DIR"
chmod 775 "$STREAM_DIR"
chmod 775 "$APP_LOG_DIR"
chmod 775 "$LOG_DIR"

log_success "Permissions configured"

# ============================================
# STEP 5: Install Binaries
# ============================================
echo ""
log_info "[5/9] Installing binaries..."

# Copy binaries
cp bin/agent "$INSTALL_DIR/ffrtmp-worker"
chmod +x "$INSTALL_DIR/ffrtmp-worker"
log_success "Worker agent → $INSTALL_DIR/ffrtmp-worker"

cp bin/ffrtmp "$INSTALL_DIR/ffrtmp"
chmod +x "$INSTALL_DIR/ffrtmp"
log_success "CLI wrapper → $INSTALL_DIR/ffrtmp"

# Create convenience symlinks in /usr/local/bin
mkdir -p /usr/local/bin
ln -sf "$INSTALL_DIR/ffrtmp-worker" /usr/local/bin/ffrtmp-worker
ln -sf "$INSTALL_DIR/ffrtmp" /usr/local/bin/ffrtmp
log_success "Symlinks created in /usr/local/bin"

# Test binary execution
if "$INSTALL_DIR/ffrtmp" run -- echo "test" &>/dev/null; then
    log_success "Binary execution test passed"
else
    log_warn "Binary execution test failed (may need dependencies)"
fi

# ============================================
# STEP 6: Enable Cgroup Delegation
# ============================================
echo ""
log_info "[6/9] Enabling cgroup delegation..."

mkdir -p /etc/systemd/system/user@.service.d/

if [ -f "deployment/systemd/user@.service.d-delegate.conf" ]; then
    cp deployment/systemd/user@.service.d-delegate.conf /etc/systemd/system/user@.service.d/delegate.conf
    log_success "Cgroup delegation config installed"
else
    log_warn "Cgroup delegation config not found, creating..."
    cat > /etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
Delegate=yes
EOF
    log_success "Cgroup delegation config created"
fi

systemctl daemon-reload
log_success "Systemd reloaded"

# ============================================
# STEP 7: Install Systemd Services
# ============================================
echo ""
log_info "[7/9] Installing systemd services..."

# Install worker service
if [ -f "deployment/systemd/ffrtmp-worker.service" ]; then
    cp deployment/systemd/ffrtmp-worker.service /etc/systemd/system/
    log_success "Worker service installed"
else
    error_exit "Worker service file not found: deployment/systemd/ffrtmp-worker.service"
fi

# Install watch daemon service
if [ -f "deployment/systemd/ffrtmp-watch.service" ]; then
    cp deployment/systemd/ffrtmp-watch.service /etc/systemd/system/
    log_success "Watch daemon service installed"
else
    error_exit "Watch daemon service file not found: deployment/systemd/ffrtmp-watch.service"
fi

systemctl daemon-reload
log_success "Systemd daemon reloaded"

# ============================================
# STEP 8: Install Configuration Files
# ============================================
echo ""
log_info "[8/9] Installing configuration files..."

# Worker configuration
if [ ! -f "$CONFIG_DIR/worker.env" ]; then
    if [ -f "deployment/systemd/worker.env.example" ]; then
        cp deployment/systemd/worker.env.example "$CONFIG_DIR/worker.env"
        log_success "Worker config template → $CONFIG_DIR/worker.env"
        log_warn "IMPORTANT: Edit $CONFIG_DIR/worker.env and set MASTER_URL and MASTER_API_KEY"
    else
        log_error "Worker config template not found"
    fi
else
    log_success "Worker config already exists: $CONFIG_DIR/worker.env"
fi

# Watch daemon YAML configuration
if [ ! -f "$CONFIG_DIR/watch-config.yaml" ]; then
    if [ -f "deployment/config/watch-config.production.yaml" ]; then
        cp deployment/config/watch-config.production.yaml "$CONFIG_DIR/watch-config.yaml"
        log_success "Watch config template → $CONFIG_DIR/watch-config.yaml"
    else
        log_error "Watch config template not found"
    fi
else
    log_success "Watch config already exists: $CONFIG_DIR/watch-config.yaml"
fi

# Watch daemon environment
if [ ! -f "$CONFIG_DIR/watch.env" ]; then
    if [ -f "deployment/systemd/watch.env.example" ]; then
        cp deployment/systemd/watch.env.example "$CONFIG_DIR/watch.env"
        log_success "Watch environment template → $CONFIG_DIR/watch.env"
    else
        log_error "Watch environment template not found"
    fi
else
    log_success "Watch environment already exists: $CONFIG_DIR/watch.env"
fi

# Set config file permissions
chmod 644 "$CONFIG_DIR"/*.yaml 2>/dev/null || true
chmod 644 "$CONFIG_DIR"/*.env 2>/dev/null || true

log_success "Configuration files installed"

# ============================================
# STEP 9: Validation
# ============================================
echo ""
log_info "[9/9] Running validation checks..."

VALIDATION_FAILED=0

# Check binaries
if [ -x "$INSTALL_DIR/ffrtmp-worker" ]; then
    log_success "Worker binary executable"
else
    log_error "Worker binary not executable"
    VALIDATION_FAILED=1
fi

if [ -x "$INSTALL_DIR/ffrtmp" ]; then
    log_success "CLI wrapper executable"
else
    log_error "CLI wrapper not executable"
    VALIDATION_FAILED=1
fi

# Check directories
for dir in "$CONFIG_DIR" "$DATA_DIR" "$INSTALL_DIR" "$STREAM_DIR" "$APP_LOG_DIR" "$LOG_DIR"; do
    if [ -d "$dir" ]; then
        log_success "Directory exists: $dir"
    else
        log_error "Directory missing: $dir"
        VALIDATION_FAILED=1
    fi
done

# Check systemd services
if [ -f "/etc/systemd/system/ffrtmp-worker.service" ]; then
    log_success "Worker service file installed"
else
    log_error "Worker service file missing"
    VALIDATION_FAILED=1
fi

if [ -f "/etc/systemd/system/ffrtmp-watch.service" ]; then
    log_success "Watch service file installed"
else
    log_error "Watch service file missing"
    VALIDATION_FAILED=1
fi

# Check cgroup delegation
if systemctl show -p Delegate user@.service 2>/dev/null | grep -q "Delegate=yes"; then
    log_success "Cgroup delegation enabled"
else
    log_warn "Cgroup delegation may not be enabled"
fi

if [ $VALIDATION_FAILED -ne 0 ]; then
    log_error "Some validation checks failed"
    echo ""
    echo "Installation completed with warnings. Review errors above."
    exit 1
fi

log_success "All validation checks passed"

# ============================================
# Installation Complete
# ============================================
echo ""
echo "=========================================="
echo "  Installation Complete!"
echo "=========================================="
echo ""
echo -e "${GREEN}✓ Binaries installed:${NC}"
echo "  • $INSTALL_DIR/ffrtmp-worker"
echo "  • $INSTALL_DIR/ffrtmp"
echo "  • Symlinks in /usr/local/bin/"
echo ""
echo -e "${GREEN}✓ Services installed:${NC}"
echo "  • ffrtmp-worker.service"
echo "  • ffrtmp-watch.service"
echo ""
echo -e "${GREEN}✓ Configuration:${NC}"
echo "  • $CONFIG_DIR/worker.env"
echo "  • $CONFIG_DIR/watch-config.yaml"
echo "  • $CONFIG_DIR/watch.env"
echo ""
echo -e "${YELLOW}⚠ Next Steps:${NC}"
echo ""
echo "1. Configure Worker Agent:"
echo "   sudo nano $CONFIG_DIR/worker.env"
echo "   # Set: MASTER_URL, MASTER_API_KEY"
echo ""
echo "2. Configure Watch Daemon (optional):"
echo "   sudo nano $CONFIG_DIR/watch-config.yaml"
echo "   # Adjust: scan_interval, target_commands, resource limits"
echo ""
echo "3. Enable and Start Services:"
echo "   sudo systemctl enable ffrtmp-worker"
echo "   sudo systemctl start ffrtmp-worker"
echo "   sudo systemctl enable ffrtmp-watch"
echo "   sudo systemctl start ffrtmp-watch"
echo ""
echo "4. Verify Installation:"
echo "   sudo systemctl status ffrtmp-worker"
echo "   sudo systemctl status ffrtmp-watch"
echo "   sudo journalctl -u ffrtmp-worker -f"
echo "   sudo journalctl -u ffrtmp-watch -f"
echo ""
echo "5. Run Validation Script:"
echo "   sudo ./deployment/validate-watch.sh"
echo ""
echo -e "${BLUE}Documentation:${NC}"
echo "  • Worker: deployment/WORKER_DEPLOYMENT.md"
echo "  • Watch Daemon: deployment/WATCH_DEPLOYMENT.md"
echo "  • Quick Fix: deployment/QUICKFIX.md"
echo ""
echo "For issues, check deployment/QUICKFIX.md"
echo ""
