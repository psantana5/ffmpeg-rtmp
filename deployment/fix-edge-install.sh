#!/bin/bash
# Quick fix script for edge node deployment issues
# Run as root: sudo ./deployment/fix-edge-install.sh

set -e

if [ "$EUID" -ne 0 ]; then
   echo "Error: This script must be run as root"
   exit 1
fi

echo "=========================================="
echo "  FFmpeg-RTMP Edge Node Quick Fix"
echo "=========================================="
echo ""

# 1. Stop services
echo "[1/5] Stopping services..."
systemctl stop ffrtmp-worker 2>/dev/null || true
systemctl stop ffrtmp-watch 2>/dev/null || true
echo "✓ Services stopped"
echo ""

# 2. Create directories
echo "[2/5] Creating required directories..."
mkdir -p /opt/ffrtmp/bin
mkdir -p /opt/ffrtmp/streams
mkdir -p /opt/ffrtmp/logs
mkdir -p /var/log/ffrtmp
mkdir -p /var/lib/ffrtmp
mkdir -p /etc/ffrtmp

echo "✓ Directories created:"
echo "  - /opt/ffrtmp/bin"
echo "  - /opt/ffrtmp/streams"
echo "  - /opt/ffrtmp/logs"
echo "  - /var/log/ffrtmp"
echo "  - /var/lib/ffrtmp"
echo ""

# 3. Set ownership
echo "[3/5] Setting ownership..."
chown -R ffrtmp:ffrtmp /opt/ffrtmp 2>/dev/null || echo "Warning: ffrtmp user not found, skipping ownership"
chown -R ffrtmp:ffrtmp /var/lib/ffrtmp 2>/dev/null || true
chown -R ffrtmp:ffrtmp /var/log/ffrtmp 2>/dev/null || true
echo "✓ Ownership set"
echo ""

# 4. Move binaries if needed
echo "[4/5] Checking binary locations..."
MOVED=0

if [ -f "/usr/local/bin/agent" ] && [ ! -f "/opt/ffrtmp/bin/ffrtmp-worker" ]; then
    echo "Moving agent → /opt/ffrtmp/bin/ffrtmp-worker"
    mv /usr/local/bin/agent /opt/ffrtmp/bin/ffrtmp-worker
    chmod +x /opt/ffrtmp/bin/ffrtmp-worker
    ln -sf /opt/ffrtmp/bin/ffrtmp-worker /usr/local/bin/ffrtmp-worker
    MOVED=1
fi

if [ -f "/usr/local/bin/ffrtmp" ] && [ ! -f "/opt/ffrtmp/bin/ffrtmp" ]; then
    echo "Moving ffrtmp → /opt/ffrtmp/bin/ffrtmp"
    cp /usr/local/bin/ffrtmp /opt/ffrtmp/bin/ffrtmp
    chmod +x /opt/ffrtmp/bin/ffrtmp
    ln -sf /opt/ffrtmp/bin/ffrtmp /usr/local/bin/ffrtmp
    MOVED=1
fi

if [ $MOVED -eq 1 ]; then
    echo "✓ Binaries moved to /opt/ffrtmp/bin/"
else
    echo "✓ Binaries already in correct location"
fi
echo ""

# 5. Update systemd if files available
echo "[5/5] Updating systemd configuration..."
if [ -f "deployment/systemd/ffrtmp-worker.service" ]; then
    cp deployment/systemd/ffrtmp-worker.service /etc/systemd/system/
    echo "✓ Updated ffrtmp-worker.service"
fi

if [ -f "deployment/systemd/ffrtmp-watch.service" ]; then
    cp deployment/systemd/ffrtmp-watch.service /etc/systemd/system/
    echo "✓ Updated ffrtmp-watch.service"
fi

systemctl daemon-reload
echo "✓ Systemd reloaded"
echo ""

# Summary
echo "=========================================="
echo "  Fix Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo ""
echo "1. Verify directories:"
echo "   ls -la /opt/ffrtmp/"
echo "   ls -la /var/log/ffrtmp"
echo ""
echo "2. Start services:"
echo "   systemctl start ffrtmp-worker"
echo "   systemctl start ffrtmp-watch"
echo ""
echo "3. Check status:"
echo "   systemctl status ffrtmp-worker"
echo "   systemctl status ffrtmp-watch"
echo ""
echo "4. Monitor logs:"
echo "   journalctl -u ffrtmp-worker -f"
echo "   journalctl -u ffrtmp-watch -f"
echo ""
echo "If issues persist, see deployment/QUICKFIX.md"
echo ""
