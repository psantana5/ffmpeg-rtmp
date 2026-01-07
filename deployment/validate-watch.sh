#!/bin/bash
# Production validation script for watch daemon deployment
# Verifies installation, configuration, and basic functionality

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0
PASSED=0

log_pass() {
    echo -e "${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

log_fail() {
    echo -e "${RED}✗${NC} $1"
    FAILED=$((FAILED + 1))
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

echo "=============================================="
echo "  Watch Daemon Production Validation"
echo "=============================================="
echo ""

# Check 1: Binary exists
echo "[1/10] Checking binary installation..."
if [ -f "/usr/local/bin/ffrtmp" ]; then
    log_pass "Binary found: /usr/local/bin/ffrtmp"
    
    # Check executable
    if [ -x "/usr/local/bin/ffrtmp" ]; then
        log_pass "Binary is executable"
    else
        log_fail "Binary is not executable"
    fi
    
    # Check version
    VERSION=$(/usr/local/bin/ffrtmp watch --help 2>&1 | head -1 || echo "unknown")
    log_pass "Version info: $VERSION"
else
    log_fail "Binary not found: /usr/local/bin/ffrtmp"
fi
echo ""

# Check 2: Systemd service
echo "[2/10] Checking systemd service..."
if [ -f "/etc/systemd/system/ffrtmp-watch.service" ]; then
    log_pass "Service file exists"
    
    # Check if enabled
    if systemctl is-enabled ffrtmp-watch.service &>/dev/null; then
        log_pass "Service is enabled"
    else
        log_warn "Service is not enabled (run: systemctl enable ffrtmp-watch)"
    fi
    
    # Check if running
    if systemctl is-active ffrtmp-watch.service &>/dev/null; then
        log_pass "Service is running"
    else
        log_warn "Service is not running (run: systemctl start ffrtmp-watch)"
    fi
else
    log_fail "Service file not found"
fi
echo ""

# Check 3: Configuration files
echo "[3/10] Checking configuration files..."
if [ -f "/etc/ffrtmp/watch-config.yaml" ]; then
    log_pass "Main config exists: /etc/ffrtmp/watch-config.yaml"
    
    # Validate YAML syntax
    if command -v python3 &>/dev/null; then
        if python3 -c "import yaml; yaml.safe_load(open('/etc/ffrtmp/watch-config.yaml'))" 2>/dev/null; then
            log_pass "YAML syntax is valid"
        else
            log_fail "YAML syntax error in watch-config.yaml"
        fi
    else
        log_warn "Python3 not available, skipping YAML validation"
    fi
else
    log_fail "Main config not found: /etc/ffrtmp/watch-config.yaml"
fi

if [ -f "/etc/ffrtmp/watch.env" ]; then
    log_pass "Environment file exists: /etc/ffrtmp/watch.env"
else
    log_warn "Environment file not found (optional)"
fi
echo ""

# Check 4: Directories
echo "[4/10] Checking directories..."
if [ -d "/var/lib/ffrtmp" ]; then
    log_pass "State directory exists: /var/lib/ffrtmp"
    
    # Check permissions
    OWNER=$(stat -c '%U:%G' /var/lib/ffrtmp)
    if [ "$OWNER" = "ffrtmp:ffrtmp" ]; then
        log_pass "Correct ownership: $OWNER"
    else
        log_fail "Wrong ownership: $OWNER (expected: ffrtmp:ffrtmp)"
    fi
    
    # Check writable
    if sudo -u ffrtmp test -w /var/lib/ffrtmp; then
        log_pass "Directory is writable by ffrtmp user"
    else
        log_fail "Directory not writable by ffrtmp user"
    fi
else
    log_fail "State directory not found: /var/lib/ffrtmp"
fi
echo ""

# Check 5: User and group
echo "[5/10] Checking user and group..."
if id ffrtmp &>/dev/null; then
    log_pass "User 'ffrtmp' exists"
    
    # Check group
    if getent group ffrtmp &>/dev/null; then
        log_pass "Group 'ffrtmp' exists"
    else
        log_fail "Group 'ffrtmp' not found"
    fi
else
    log_fail "User 'ffrtmp' not found"
fi
echo ""

# Check 6: Cgroup v2
echo "[6/10] Checking cgroup v2 support..."
if mount | grep -q "cgroup2 on /sys/fs/cgroup"; then
    log_pass "Cgroup v2 is mounted"
else
    log_fail "Cgroup v2 not mounted (required for resource limits)"
fi

# Check cgroup delegation
if systemctl show -p Delegate ffrtmp-watch 2>/dev/null | grep -q "Delegate=yes"; then
    log_pass "Cgroup delegation enabled"
else
    log_fail "Cgroup delegation not enabled"
fi
echo ""

# Check 7: Kernel version
echo "[7/10] Checking kernel version..."
KERNEL_VERSION=$(uname -r)
log_pass "Kernel version: $KERNEL_VERSION"

KERNEL_MAJOR=$(echo $KERNEL_VERSION | cut -d. -f1)
KERNEL_MINOR=$(echo $KERNEL_VERSION | cut -d. -f2)

if [ "$KERNEL_MAJOR" -gt 4 ] || ([ "$KERNEL_MAJOR" -eq 4 ] && [ "$KERNEL_MINOR" -ge 15 ]); then
    log_pass "Kernel version >= 4.15 (cgroup v2 supported)"
else
    log_fail "Kernel version < 4.15 (cgroup v2 may not work)"
fi
echo ""

# Check 8: Service logs (if running)
echo "[8/10] Checking service logs..."
if systemctl is-active ffrtmp-watch.service &>/dev/null; then
    # Check for errors in last 50 lines
    ERROR_COUNT=$(journalctl -u ffrtmp-watch -n 50 --no-pager | grep -i "error\|failed" | wc -l)
    
    if [ "$ERROR_COUNT" -eq 0 ]; then
        log_pass "No errors in recent logs"
    else
        log_warn "Found $ERROR_COUNT error messages in logs"
    fi
    
    # Check for successful startup
    if journalctl -u ffrtmp-watch --no-pager | grep -q "Auto-attach service started"; then
        log_pass "Service started successfully"
    else
        log_warn "Startup message not found in logs"
    fi
    
    # Check for retry worker
    if journalctl -u ffrtmp-watch --no-pager | grep -q "Retry worker started"; then
        log_pass "Retry worker initialized"
    else
        log_warn "Retry worker not found in logs"
    fi
else
    log_warn "Service not running, skipping log checks"
fi
echo ""

# Check 9: State file (if exists)
echo "[9/10] Checking state file..."
if [ -f "/var/lib/ffrtmp/watch-state.json" ]; then
    log_pass "State file exists"
    
    # Check if valid JSON
    if command -v jq &>/dev/null; then
        if jq . /var/lib/ffrtmp/watch-state.json &>/dev/null; then
            log_pass "State file is valid JSON"
            
            # Show statistics
            TOTAL_SCANS=$(jq -r '.statistics.total_scans // 0' /var/lib/ffrtmp/watch-state.json)
            TOTAL_DISCOVERED=$(jq -r '.statistics.total_discovered // 0' /var/lib/ffrtmp/watch-state.json)
            log_pass "Statistics: $TOTAL_SCANS scans, $TOTAL_DISCOVERED discovered"
        else
            log_fail "State file is not valid JSON"
        fi
    else
        log_warn "jq not available, skipping JSON validation"
    fi
else
    log_warn "State file not found (created on first run)"
fi
echo ""

# Check 10: Functional test (optional, requires FFmpeg)
echo "[10/10] Functional test (optional)..."
if command -v ffmpeg &>/dev/null; then
    log_pass "FFmpeg is installed"
    
    if systemctl is-active ffrtmp-watch.service &>/dev/null; then
        log_warn "Run manual test: ffmpeg -f lavfi -i testsrc -f null - &"
        log_warn "Then check: journalctl -u ffrtmp-watch -f"
    else
        log_warn "Start service first: systemctl start ffrtmp-watch"
    fi
else
    log_warn "FFmpeg not installed, skipping functional test"
fi
echo ""

# Summary
echo "=============================================="
echo "  Validation Summary"
echo "=============================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Start service: systemctl start ffrtmp-watch"
    echo "  2. Enable on boot: systemctl enable ffrtmp-watch"
    echo "  3. Monitor logs: journalctl -u ffrtmp-watch -f"
    echo "  4. Test discovery: Run an FFmpeg process"
    exit 0
else
    echo -e "${RED}✗ Some checks failed${NC}"
    echo ""
    echo "Review the failures above and:"
    echo "  1. Check installation: deployment/install-edge.sh"
    echo "  2. Read docs: deployment/WATCH_DEPLOYMENT.md"
    echo "  3. Check logs: journalctl -u ffrtmp-watch -n 50"
    exit 1
fi
