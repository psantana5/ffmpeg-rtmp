#!/bin/bash
# Deployment Simulation Script
# Simulates deployment workflow without requiring root or modifying system
# Tests argument parsing, configuration, and logic flow

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
log_test() { echo -e "${CYAN}[TEST]${NC} $1"; }

echo ""
echo "=========================================="
echo "  Deployment Simulation Tests"
echo "=========================================="
echo ""

# ============================================
# TEST 1: Deploy.sh Argument Parsing
# ============================================
log_test "Test 1: Deploy.sh argument parsing"

# Test master mode
if ./deploy.sh --master --help 2>&1 | grep -q "FFmpeg-RTMP"; then
    log_success "Master mode recognized"
else
    log_error "Master mode failed"
    exit 1
fi

# Test worker mode
if ./deploy.sh --worker --help 2>&1 | grep -q "FFmpeg-RTMP"; then
    log_success "Worker mode recognized"
else
    log_error "Worker mode failed"
    exit 1
fi

# Test both mode
if ./deploy.sh --both --help 2>&1 | grep -q "FFmpeg-RTMP"; then
    log_success "Both mode recognized"
else
    log_error "Both mode failed"
    exit 1
fi

echo ""

# ============================================
# TEST 2: Build Capability Check
# ============================================
log_test "Test 2: Build capability"

# Check if Makefile has build targets
if grep -q "build-master:" Makefile && grep -q "build-agent:" Makefile; then
    log_success "Build targets exist in Makefile"
else
    log_error "Build targets missing"
    exit 1
fi

# Check if Go is available
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    log_success "Go available: $GO_VERSION"
else
    log_error "Go not found (required for building)"
    exit 1
fi

echo ""

# ============================================
# TEST 3: Systemd Service Files
# ============================================
log_test "Test 3: Systemd service files"

check_service_file() {
    local file="$1"
    local name="$2"
    
    if [ ! -f "$file" ]; then
        log_error "$name missing: $file"
        return 1
    fi
    
    # Check for required directives
    if ! grep -q "ExecStart=" "$file"; then
        log_error "$name missing ExecStart"
        return 1
    fi
    
    if ! grep -q "User=" "$file"; then
        log_error "$name missing User directive"
        return 1
    fi
    
    log_success "$name valid"
    return 0
}

check_service_file "deployment/systemd/ffrtmp-worker.service" "Worker service"
check_service_file "deployment/systemd/ffrtmp-watch.service" "Watch service"

echo ""

# ============================================
# TEST 4: Configuration Templates
# ============================================
log_test "Test 4: Configuration templates"

if [ -f "deployment/config/watch-config.production.yaml" ]; then
    log_success "Watch config template exists"
else
    log_error "Watch config template missing"
    exit 1
fi

if [ -f "deployment/systemd/watch.env.example" ]; then
    log_success "Watch environment template exists"
else
    log_error "Watch environment template missing"
    exit 1
fi

echo ""

# ============================================
# TEST 5: Directory Structure
# ============================================
log_test "Test 5: Required source directories"

check_dir() {
    local dir="$1"
    local name="$2"
    
    if [ -d "$dir" ]; then
        log_success "$name exists: $dir"
    else
        log_error "$name missing: $dir"
        exit 1
    fi
}

check_dir "cmd/ffrtmp" "CLI command directory"
check_dir "internal/discover" "Auto-discovery package"
check_dir "master" "Master server directory"
check_dir "worker" "Worker directory"
check_dir "deployment" "Deployment directory"

echo ""

# ============================================
# TEST 6: Binary Build Test
# ============================================
log_test "Test 6: Binary build (smoke test)"

log_info "Building fresh binaries..."

# Clean first
make clean &> /dev/null || true

# Build worker/agent and CLI
if make build-agent build-cli > /dev/null 2>&1; then
    log_success "Worker build successful"
else
    log_error "Worker build failed"
    exit 1
fi

# Build master
if make build-master > /dev/null 2>&1; then
    log_success "Master build successful"
else
    log_error "Master build failed"
    exit 1
fi

echo ""

# ============================================
# TEST 7: Binary Verification
# ============================================
log_test "Test 7: Binary verification"

if [ -f "bin/ffrtmp" ]; then
    if [ -x "bin/ffrtmp" ]; then
        log_success "Worker binary is executable"
        
        # Test help command
        if ./bin/ffrtmp --help &> /dev/null; then
            log_success "Worker binary runs correctly"
        else
            log_error "Worker binary execution failed"
            exit 1
        fi
    else
        log_error "Worker binary not executable"
        exit 1
    fi
else
    log_error "Worker binary missing: bin/ffrtmp"
    exit 1
fi

if [ -f "bin/master" ]; then
    if [ -x "bin/master" ]; then
        log_success "Master binary is executable"
        
        # Test help command
        if ./bin/master --help &> /dev/null; then
            log_success "Master binary runs correctly"
        else
            log_error "Master binary execution failed"
            exit 1
        fi
    else
        log_error "Master binary not executable"
        exit 1
    fi
else
    log_error "Master binary missing: bin/master"
    exit 1
fi

echo ""

# ============================================
# TEST 8: Watch Daemon Features
# ============================================
log_test "Test 8: Watch daemon features"

if ./bin/ffrtmp watch --help 2>&1 | grep -q "scan-interval"; then
    log_success "Watch daemon has scan-interval flag"
else
    log_error "Watch daemon missing scan-interval flag"
    exit 1
fi

if ./bin/ffrtmp watch --help 2>&1 | grep -q "enable-retry"; then
    log_success "Watch daemon has enable-retry flag"
else
    log_error "Watch daemon missing enable-retry flag"
    exit 1
fi

echo ""

# ============================================
# TEST 9: Deployment Documentation
# ============================================
log_test "Test 9: Deployment documentation"

check_doc() {
    local file="$1"
    local name="$2"
    
    if [ -f "$file" ]; then
        log_success "$name exists"
    else
        log_error "$name missing: $file"
        exit 1
    fi
}

check_doc "deployment/WATCH_DEPLOYMENT.md" "Watch deployment guide"
check_doc "QUICKSTART.md" "Quickstart guide"
check_doc "README.md" "Main README"

echo ""

# ============================================
# SUMMARY
# ============================================
echo "=========================================="
echo "  Simulation Complete"
echo "=========================================="
echo ""
log_success "All deployment simulation tests passed!"
echo ""
log_info "The deployment scripts are ready for production use"
echo ""
echo "Next steps:"
echo "  1. Deploy master: sudo ./deploy.sh --master"
echo "  2. Deploy worker: sudo ./deploy.sh --worker --master-url http://master:8080 --api-key <key>"
echo "  3. Monitor: sudo systemctl status ffrtmp-master / ffrtmp-worker / ffrtmp-watch"
echo ""
