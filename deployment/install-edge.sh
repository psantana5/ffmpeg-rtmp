# Edge Deployment Installation Script
# This script installs and configures the FFmpeg-RTMP worker with wrapper

set -e

echo "=========================================="
echo "  FFmpeg-RTMP Edge Deployment"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
   echo "Error: This script must be run as root"
   exit 1
fi

# Configuration
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/ffrtmp"
DATA_DIR="/var/lib/ffrtmp"
USER="ffrtmp"
GROUP="ffrtmp"

echo "[1/7] Creating ffrtmp user..."
if ! id "$USER" &>/dev/null; then
    useradd -r -s /bin/false -d "$DATA_DIR" "$USER"
    echo "✓ User created: $USER"
else
    echo "✓ User already exists: $USER"
fi

echo ""
echo "[2/7] Creating directories..."
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"
chown "$USER:$GROUP" "$DATA_DIR"
echo "✓ Directories created"

echo ""
echo "[3/7] Installing binaries..."
if [ -f "bin/agent" ]; then
    cp bin/agent "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/agent"
    echo "✓ Worker agent installed"
else
    echo "Error: bin/agent not found. Build it first with: make build-agent"
    exit 1
fi

if [ -f "bin/ffrtmp" ]; then
    cp bin/ffrtmp "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/ffrtmp"
    echo "✓ Wrapper installed"
else
    echo "Error: bin/ffrtmp not found. Build it first with: make build-cli"
    exit 1
fi

echo ""
echo "[4/7] Enabling cgroup delegation..."
mkdir -p /etc/systemd/system/user@.service.d/
cp deployment/systemd/user@.service.d-delegate.conf /etc/systemd/system/user@.service.d/delegate.conf
systemctl daemon-reload
echo "✓ Cgroup delegation enabled"

echo ""
echo "[5/7] Installing systemd service..."
cp deployment/systemd/ffrtmp-worker.service /etc/systemd/system/
systemctl daemon-reload
echo "✓ Systemd service installed"

echo ""
echo "[6/7] Creating configuration..."
if [ ! -f "$CONFIG_DIR/worker.env" ]; then
    cp deployment/systemd/worker.env.example "$CONFIG_DIR/worker.env"
    echo "✓ Configuration template created: $CONFIG_DIR/worker.env"
    echo ""
    echo "⚠️  IMPORTANT: Edit $CONFIG_DIR/worker.env and set:"
    echo "   - MASTER_URL"
    echo "   - MASTER_API_KEY"
else
    echo "✓ Configuration already exists: $CONFIG_DIR/worker.env"
fi

echo ""
echo "[7/7] Testing installation..."
echo "Wrapper version:"
"$INSTALL_DIR/ffrtmp" run -- echo "Installation successful!" || true
echo ""

echo "=========================================="
echo "  Installation Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Edit configuration: nano $CONFIG_DIR/worker.env"
echo "  2. Enable service: systemctl enable ffrtmp-worker"
echo "  3. Start service: systemctl start ffrtmp-worker"
echo "  4. Check status: systemctl status ffrtmp-worker"
echo "  5. View logs: journalctl -u ffrtmp-worker -f"
echo ""
echo "For existing workloads (zero-downtime):"
echo "  ffrtmp attach --pid <PID> --job-id <job-id>"
echo ""
