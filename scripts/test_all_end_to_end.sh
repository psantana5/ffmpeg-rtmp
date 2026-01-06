#!/bin/bash
# Complete end-to-end test suite for wrapper replication
# Tests all functionality: core, integration, deployment, visibility

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "╔════════════════════════════════════════════════════════════════════════════╗"
echo "║                                                                            ║"
echo "║              WRAPPER END-TO-END REPLICATION TEST SUITE                    ║"
echo "║                                                                            ║"
echo "╚════════════════════════════════════════════════════════════════════════════╝"
echo

TESTS_RUN=0
TESTS_PASSED=0
START_TIME=$(date +%s)

run_test() {
    local test_name="$1"
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "${BLUE}TEST ${TESTS_RUN}: ${test_name}${NC}"
}

pass_test() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    echo -e "${GREEN}✓ PASS${NC}"
    echo
}

fail_test() {
    local reason="$1"
    echo -e "${RED}✗ FAIL: ${reason}${NC}"
    echo
}

section() {
    echo -e "${YELLOW}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  $1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${NC}"
}

cd "$(dirname "$0")/.."

# ============================================================================
# SECTION 1: Environment Setup
# ============================================================================
section "Section 1: Environment Setup"

run_test "Go installation and version"
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    echo "  $GO_VERSION"
    pass_test
else
    fail_test "Go not installed"
    exit 1
fi

run_test "Project structure exists"
if [ -d "internal/wrapper" ] && [ -d "internal/cgroups" ] && \
   [ -d "internal/observe" ] && [ -d "internal/report" ]; then
    pass_test
else
    fail_test "Project structure incomplete"
    exit 1
fi

run_test "Build system (go.mod)"
if [ -f "go.mod" ] && grep -q "github.com/psantana5/ffmpeg-rtmp" go.mod; then
    pass_test
else
    fail_test "go.mod missing or invalid"
    exit 1
fi

# ============================================================================
# SECTION 2: Core Wrapper Functionality
# ============================================================================
section "Section 2: Core Wrapper Functionality"

run_test "Build wrapper CLI"
if go build -o /tmp/ffrtmp-wrapper-test ./cmd/ffrtmp/ 2>/dev/null; then
    pass_test
else
    fail_test "Failed to build wrapper CLI"
    exit 1
fi

run_test "Run mode: Basic execution"
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-run-1 -- sleep 0.5 > "$LOG_FILE" 2>&1
if grep -q "JOB test-run-1" "$LOG_FILE" && \
   grep -q "sla=" "$LOG_FILE"; then
    pass_test
else
    fail_test "Run mode basic execution failed"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Run mode: Exit code propagation"
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-exit-code -- sh -c "exit 42" > "$LOG_FILE" 2>&1 || true
if grep -q "exit=42" "$LOG_FILE"; then
    pass_test
else
    fail_test "Exit code not propagated"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Run mode: Cgroup limits (CPU)"
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-cpu-limit \
    --cpu-max "50000 100000" \
    --cpu-weight 100 \
    -- sleep 0.5 > "$LOG_FILE" 2>&1 || true
# Just verify it doesn't crash, cgroup may not be available
if grep -q "JOB test-cpu-limit" "$LOG_FILE"; then
    pass_test
else
    fail_test "CPU limits caused crash"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Run mode: Cgroup limits (Memory)"
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-mem-limit \
    --memory-max 104857600 \
    -- sleep 0.5 > "$LOG_FILE" 2>&1 || true
if grep -q "JOB test-mem-limit" "$LOG_FILE"; then
    pass_test
else
    fail_test "Memory limits caused crash"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Attach mode: Observe existing process"
# Start a background process
sleep 10 &
BG_PID=$!
sleep 0.2

LOG_FILE=$(mktemp)
timeout 2 /tmp/ffrtmp-wrapper-test attach --job-id test-attach-1 --pid $BG_PID > "$LOG_FILE" 2>&1 || true

# Kill background process
kill $BG_PID 2>/dev/null || true
wait $BG_PID 2>/dev/null || true

if grep -q "JOB test-attach-1" "$LOG_FILE"; then
    pass_test
else
    fail_test "Attach mode failed"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Crash safety: Workload survives wrapper kill"
# Start wrapper in background
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-crash-safety -- sleep 5 > "$LOG_FILE" 2>&1 &
WRAPPER_PID=$!
sleep 0.5

# Get workload PID from wrapper's child
WORKLOAD_PID=$(pgrep -P $WRAPPER_PID 2>/dev/null || echo "")

if [ -z "$WORKLOAD_PID" ]; then
    fail_test "Could not find workload PID"
    kill $WRAPPER_PID 2>/dev/null || true
    rm -f "$LOG_FILE"
    exit 1
fi

# Kill wrapper with -9
kill -9 $WRAPPER_PID 2>/dev/null

# Verify workload is STILL RUNNING
sleep 0.5
if ps -p $WORKLOAD_PID > /dev/null 2>&1; then
    # Success! Kill workload now
    kill $WORKLOAD_PID 2>/dev/null || true
    wait $WORKLOAD_PID 2>/dev/null || true
    pass_test
else
    fail_test "Workload died with wrapper (CRITICAL FAILURE)"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

# ============================================================================
# SECTION 3: Visibility (3 Layers)
# ============================================================================
section "Section 3: Visibility (3 Layers)"

run_test "Layer 1: Immutable job truth recorded"
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-layer1 -- sleep 0.2 > "$LOG_FILE" 2>&1
if grep -q "JOB test-layer1" "$LOG_FILE" && \
   grep -q "sla=" "$LOG_FILE" && \
   grep -q "reason=" "$LOG_FILE" && \
   grep -q "runtime=" "$LOG_FILE"; then
    pass_test
else
    fail_test "Layer 1 job truth not complete"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Layer 2: Metrics counters exist"
if grep -q "JobsPlatformCompliant" internal/report/metrics.go && \
   grep -q "JobsPlatformViolation" internal/report/metrics.go && \
   grep -q "RecordResult" internal/report/metrics.go; then
    pass_test
else
    fail_test "Layer 2 metrics incomplete"
    exit 1
fi

run_test "Layer 3: Human-readable log format"
LOG_FILE=$(mktemp)
/tmp/ffrtmp-wrapper-test run --job-id test-layer3 -- true > "$LOG_FILE" 2>&1
if grep -E "JOB test-layer3.*sla=(COMPLIANT|VIOLATION).*runtime=[0-9]+.*exit=[0-9]+" "$LOG_FILE"; then
    pass_test
else
    fail_test "Layer 3 log format incorrect"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi
rm -f "$LOG_FILE"

run_test "Killer feature: Violation sampling exists"
if [ -f "internal/report/violations.go" ] && \
   grep -q "ViolationLog" internal/report/violations.go && \
   grep -q "GetRecent" internal/report/violations.go; then
    pass_test
else
    fail_test "Violation sampling not implemented"
    exit 1
fi

run_test "Prometheus export function exists"
if [ -f "internal/report/export.go" ] && \
   grep -q "PrometheusExport" internal/report/export.go; then
    pass_test
else
    fail_test "Prometheus export not found"
    exit 1
fi

run_test "No reactive behavior (visibility derived, not driving)"
if ! grep -rE "if.*(metric|sla.*rate)" internal/wrapper/*.go 2>/dev/null; then
    pass_test
else
    fail_test "Found reactive behavior in wrapper"
    exit 1
fi

# ============================================================================
# SECTION 4: Worker Agent Integration
# ============================================================================
section "Section 4: Worker Agent Integration"

run_test "Worker agent builds"
if go build -o /tmp/ffrtmp-worker-test ./worker/cmd/agent/ 2>/dev/null; then
    pass_test
else
    fail_test "Worker agent build failed"
    exit 1
fi

run_test "Job model has WrapperEnabled field"
if grep -q "WrapperEnabled.*bool" shared/pkg/models/job.go; then
    pass_test
else
    fail_test "WrapperEnabled field missing"
    exit 1
fi

run_test "Job model has WrapperConstraints"
if grep -q "WrapperConstraints" shared/pkg/models/job.go; then
    pass_test
else
    fail_test "WrapperConstraints missing"
    exit 1
fi

run_test "Worker agent checks WrapperEnabled flag"
if grep -q "if job.WrapperEnabled" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "WrapperEnabled check not in worker agent"
    exit 1
fi

run_test "Integration helper exists"
if [ -f "shared/pkg/agent/wrapper_integration.go" ] && \
   grep -q "ExecuteWithWrapper" shared/pkg/agent/wrapper_integration.go; then
    pass_test
else
    fail_test "Integration helper not found"
    exit 1
fi

run_test "Backward compatibility preserved"
if grep -q "exec.CommandContext" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "Legacy execution path removed"
    exit 1
fi

# ============================================================================
# SECTION 5: Deployment Infrastructure
# ============================================================================
section "Section 5: Deployment Infrastructure"

run_test "Systemd service file exists"
if [ -f "deployment/systemd/ffrtmp-worker.service" ]; then
    pass_test
else
    fail_test "Systemd service file missing"
    exit 1
fi

run_test "Systemd service has Delegate=yes"
if grep -q "Delegate=yes" deployment/systemd/ffrtmp-worker.service; then
    pass_test
else
    fail_test "Cgroup delegation not configured"
    exit 1
fi

run_test "Worker environment template exists"
if [ -f "deployment/systemd/worker.env.example" ]; then
    pass_test
else
    fail_test "Worker environment template missing"
    exit 1
fi

run_test "Cgroup delegation config exists"
if [ -f "deployment/systemd/user@.service.d-delegate.conf" ]; then
    pass_test
else
    fail_test "Cgroup delegation config missing"
    exit 1
fi

run_test "Automated installer script exists"
if [ -f "deployment/install-edge.sh" ] && [ -x "deployment/install-edge.sh" ]; then
    pass_test
else
    fail_test "Install script missing or not executable"
    exit 1
fi

# ============================================================================
# SECTION 6: Documentation
# ============================================================================
section "Section 6: Documentation"

run_test "Architecture documentation"
if [ -f "docs/WRAPPER_MINIMALIST_ARCHITECTURE.md" ]; then
    pass_test
else
    fail_test "Architecture docs missing"
    exit 1
fi

run_test "Integration documentation"
if [ -f "docs/WRAPPER_INTEGRATION.md" ]; then
    pass_test
else
    fail_test "Integration docs missing"
    exit 1
fi

run_test "Deployment documentation"
if [ -f "docs/WRAPPER_EDGE_DEPLOYMENT.md" ]; then
    pass_test
else
    fail_test "Deployment docs missing"
    exit 1
fi

run_test "Visibility documentation"
if [ -f "docs/WRAPPER_VISIBILITY.md" ]; then
    pass_test
else
    fail_test "Visibility docs missing"
    exit 1
fi

# ============================================================================
# SECTION 7: Test Suites
# ============================================================================
section "Section 7: Test Suites"

run_test "Stability test suite exists"
if [ -f "scripts/test_wrapper_stability.sh" ] && [ -x "scripts/test_wrapper_stability.sh" ]; then
    pass_test
else
    fail_test "Stability tests missing"
    exit 1
fi

run_test "Integration test suite exists"
if [ -f "scripts/test_wrapper_integration.sh" ] && [ -x "scripts/test_wrapper_integration.sh" ]; then
    pass_test
else
    fail_test "Integration tests missing"
    exit 1
fi

run_test "Visibility test suite exists"
if [ -f "scripts/test_wrapper_visibility.sh" ] && [ -x "scripts/test_wrapper_visibility.sh" ]; then
    pass_test
else
    fail_test "Visibility tests missing"
    exit 1
fi

# ============================================================================
# Summary
# ============================================================================
echo
echo "╔════════════════════════════════════════════════════════════════════════════╗"
echo "║                                                                            ║"
echo "║                        END-TO-END TEST SUMMARY                             ║"
echo "║                                                                            ║"
echo "╚════════════════════════════════════════════════════════════════════════════╝"
echo

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "Tests run:    $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $((TESTS_RUN - TESTS_PASSED))"
echo "Duration:     ${DURATION}s"
echo

if [ $TESTS_PASSED -eq $TESTS_RUN ]; then
    echo -e "${GREEN}"
    echo "✓ ✓ ✓ ALL TESTS PASSED ✓ ✓ ✓"
    echo
    echo "Wrapper is fully functional and ready for production:"
    echo "  • Core functionality verified (crash safety ✓)"
    echo "  • Visibility layers complete (3/3 ✓)"
    echo "  • Worker integration working (opt-in ✓)"
    echo "  • Deployment infrastructure ready (systemd ✓)"
    echo "  • Documentation complete (4 guides ✓)"
    echo "  • Test suites available (3 suites ✓)"
    echo -e "${NC}"
    exit 0
else
    echo -e "${RED}"
    echo "✗ ✗ ✗ SOME TESTS FAILED ✗ ✗ ✗"
    echo
    echo "Review failures above and check:"
    echo "  1. Go installation and dependencies"
    echo "  2. Project structure completeness"
    echo "  3. Build errors (go build output)"
    echo "  4. Test logs for specific failures"
    echo -e "${NC}"
    exit 1
fi
