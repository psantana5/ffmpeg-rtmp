#!/bin/bash
# Master Node Installation Script
# Installs FFmpeg-RTMP master server with monitoring stack
# Run as root: sudo ./master/deployment/install-master.sh

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
INSTALL_DIR="/opt/ffrtmp-master/bin"
CONFIG_DIR="/etc/ffrtmp-master"
DATA_DIR="/var/lib/ffrtmp-master"
LOG_DIR="/var/log/ffrtmp-master"
USER="ffrtmp-master"
GROUP="ffrtmp-master"
PORT=8080
METRICS_PORT=9090

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }

error_exit() {
    log_error "$1"
    exit 1
}

# Banner
echo ""
echo "=========================================="
echo "  FFmpeg-RTMP Master Node Deployment"
echo "=========================================="
echo ""

# Check root
if [ "$EUID" -ne 0 ]; then
    error_exit "This script must be run as root (use sudo)"
fi

# Check if running from project root
if [ ! -f "master/deployment/install-master.sh" ]; then
    error_exit "Please run this script from the project root directory"
fi

# ============================================
# STEP 1: Dependencies
# ============================================
echo ""
log_info "[1/8] Checking dependencies..."

if ! command -v systemctl &> /dev/null; then
    log_error "systemd not found"
    exit 1
fi

log_success "Dependencies satisfied"

# ============================================
# STEP 2: Build Binary
# ============================================
echo ""
log_info "[2/8] Building master binary..."

if [ ! -f "bin/master" ]; then
    if ! command -v go &> /dev/null; then
        error_exit "Go not found. Install Go 1.24+ first"
    fi
    
    log_info "Building from source..."
    make build-master || error_exit "Failed to build master"
    log_success "Build complete"
else
    log_success "Binary already exists"
fi

# ============================================
# STEP 3: Create User
# ============================================
echo ""
log_info "[3/8] Creating system user..."

if id "$USER" &>/dev/null; then
    log_success "User '$USER' already exists"
else
    useradd -r -s /bin/false -d "$DATA_DIR" -c "FFmpeg RTMP Master" "$USER" || error_exit "Failed to create user"
    log_success "User '$USER' created"
fi

# ============================================
# STEP 4: Create Directories
# ============================================
echo ""
log_info "[4/8] Creating directories..."

mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$LOG_DIR"

chown -R "$USER:$GROUP" "$DATA_DIR"
chown -R "$USER:$GROUP" "$LOG_DIR"

log_success "Directories created"

# ============================================
# STEP 5: Install Binary
# ============================================
echo ""
log_info "[5/8] Installing binary..."

cp bin/master "$INSTALL_DIR/ffrtmp-master"
chmod +x "$INSTALL_DIR/ffrtmp-master"

# Create symlink
mkdir -p /usr/local/bin
ln -sf "$INSTALL_DIR/ffrtmp-master" /usr/local/bin/ffrtmp-master

log_success "Binary installed → $INSTALL_DIR/ffrtmp-master"

# ============================================
# STEP 6: Create Configuration
# ============================================
echo ""
log_info "[6/8] Creating configuration..."

# Generate API key if not exists
API_KEY_FILE="$CONFIG_DIR/api-key"
if [ ! -f "$API_KEY_FILE" ]; then
    openssl rand -base64 32 > "$API_KEY_FILE"
    chmod 600 "$API_KEY_FILE"
    log_success "Generated API key → $API_KEY_FILE"
else
    log_success "API key already exists"
fi

# Create config file
cat > "$CONFIG_DIR/master.env" <<EOF
# FFmpeg-RTMP Master Configuration

# Server settings
PORT=$PORT
METRICS_PORT=$METRICS_PORT

# Database
DATABASE_PATH=$DATA_DIR/master.db

# API Key (for worker authentication)
API_KEY_FILE=$API_KEY_FILE

# Logging
LOG_LEVEL=info
LOG_DIR=$LOG_DIR

# TLS (optional)
# TLS_ENABLED=true
# TLS_CERT=/etc/ffrtmp-master/certs/server.crt
# TLS_KEY=/etc/ffrtmp-master/certs/server.key
EOF

log_success "Configuration created → $CONFIG_DIR/master.env"

# ============================================
# STEP 7: Install Systemd Service
# ============================================
echo ""
log_info "[7/8] Installing systemd service..."

cat > /etc/systemd/system/ffrtmp-master.service <<EOF
[Unit]
Description=FFmpeg-RTMP Master Server
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$DATA_DIR

# Environment
EnvironmentFile=$CONFIG_DIR/master.env

# Binary
ExecStart=$INSTALL_DIR/ffrtmp-master \\
    --port=\${PORT} \\
    --db=\${DATABASE_PATH} \\
    --api-key-file=\${API_KEY_FILE}

# Restart policy
Restart=on-failure
RestartSec=10s
KillMode=process
TimeoutStopSec=30s

# Security
NoNewPrivileges=true
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$DATA_DIR $LOG_DIR

# Resource limits
LimitNOFILE=65536

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ffrtmp-master

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
log_success "Systemd service installed"

# ============================================
# STEP 8: Validation
# ============================================
echo ""
log_info "[8/8] Validating installation..."

VALIDATION_FAILED=0

if [ -x "$INSTALL_DIR/ffrtmp-master" ]; then
    log_success "Binary executable"
else
    log_error "Binary not executable"
    VALIDATION_FAILED=1
fi

if [ -f "$API_KEY_FILE" ]; then
    log_success "API key exists"
else
    log_error "API key missing"
    VALIDATION_FAILED=1
fi

if [ -f "/etc/systemd/system/ffrtmp-master.service" ]; then
    log_success "Systemd service installed"
else
    log_error "Systemd service missing"
    VALIDATION_FAILED=1
fi

if [ $VALIDATION_FAILED -ne 0 ]; then
    log_error "Validation failed"
    exit 1
fi

log_success "All validation checks passed"

# ============================================
# Complete
# ============================================
echo ""
echo "=========================================="
echo "  Installation Complete!"
echo "=========================================="
echo ""
echo -e "${GREEN}✓ Master node installed${NC}"
echo ""
echo "Configuration:"
echo "  • Config: $CONFIG_DIR/master.env"
echo "  • API Key: $API_KEY_FILE"
echo "  • Database: $DATA_DIR/master.db"
echo "  • Logs: $LOG_DIR"
echo ""
echo -e "${YELLOW}API Key (save this for worker nodes):${NC}"
cat "$API_KEY_FILE"
echo ""
echo ""
echo "Next steps:"
echo ""
echo "1. Start Master:"
echo "   systemctl enable ffrtmp-master"
echo "   systemctl start ffrtmp-master"
echo ""
echo "2. Check Status:"
echo "   systemctl status ffrtmp-master"
echo "   journalctl -u ffrtmp-master -f"
echo ""
echo "3. Access Web UI:"
echo "   http://$(hostname -I | awk '{print $1}'):$PORT"
echo ""
echo "4. Deploy Workers:"
echo "   Use API key above when configuring worker nodes"
echo "   ./deploy.sh --worker --master-url http://$(hostname -I | awk '{print $1}'):$PORT --api-key \$(cat $API_KEY_FILE)"
echo ""
echo "Documentation:"
echo "  • Master: master/README.md"
echo "  • Deployment: deployment/README.md"
echo ""
