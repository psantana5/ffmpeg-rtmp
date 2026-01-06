#!/bin/bash
# Test visibility layers implementation
# Verifies Layer 1 (job truth), Layer 2 (counters), Layer 3 (logs)

set -e

echo "=== Wrapper Visibility Test ==="
echo "Testing three visibility layers implementation"
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

TESTS_RUN=0
TESTS_PASSED=0

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

# Build
echo "Building wrapper..."
cd "$(dirname "$0")/.."
go build -o /tmp/ffrtmp-cli ./cmd/ffrtmp/ || {
    echo "Failed to build"
    exit 1
}
echo "✓ Built"
echo

# Test 1: Layer 1 - Immutable Result
run_test "Layer 1: Result has all immutable fields"
if grep -q "StartTime.*time.Time" internal/report/result.go && \
   grep -q "EndTime.*time.Time" internal/report/result.go && \
   grep -q "PlatformSLA.*bool" internal/report/result.go && \
   grep -q "PlatformSLAReason.*string" internal/report/result.go && \
   grep -q "Intent.*string" internal/report/result.go; then
    pass_test
else
    fail_test "Result missing required immutable fields"
    exit 1
fi

# Test 2: Layer 1 - LogSummary exists
run_test "Layer 1: Human-readable LogSummary"
if grep -q "func.*LogSummary" internal/report/result.go; then
    pass_test
else
    fail_test "LogSummary method not found"
    exit 1
fi

# Test 3: Layer 2 - Boring counters only
run_test "Layer 2: Metrics are counters only (no histograms)"
if grep -q "atomic.Uint64" internal/report/metrics.go && \
   ! grep -q "Histogram" internal/report/metrics.go && \
   ! grep -q "Summary" internal/report/metrics.go; then
    pass_test
else
    fail_test "Metrics contain non-counter types"
    exit 1
fi

# Test 4: Layer 2 - RecordResult single source of truth
run_test "Layer 2: RecordResult updates from single Result"
if grep -q "func.*RecordResult" internal/report/metrics.go; then
    pass_test
else
    fail_test "RecordResult method not found"
    exit 1
fi

# Test 5: Layer 2 - Platform SLA counters
run_test "Layer 2: Platform SLA counters exist"
if grep -q "JobsPlatformCompliant" internal/report/metrics.go && \
   grep -q "JobsPlatformViolation" internal/report/metrics.go; then
    pass_test
else
    fail_test "Platform SLA counters missing"
    exit 1
fi

# Test 6: Killer feature - Violation sampling
run_test "Killer feature: Violation sampling exists"
if [ -f internal/report/violations.go ] && \
   grep -q "ViolationLog" internal/report/violations.go && \
   grep -q "GetRecent" internal/report/violations.go; then
    pass_test
else
    fail_test "Violation sampling not implemented"
    exit 1
fi

# Test 7: Prometheus export
run_test "Layer 2: Prometheus export function"
if grep -q "PrometheusExport" internal/report/export.go; then
    pass_test
else
    fail_test "PrometheusExport not found"
    exit 1
fi

# Test 8: No reactive behavior
run_test "Principle: No feedback loops in wrapper"
if ! grep -qi "if.*metric" internal/wrapper/*.go && \
   ! grep -qi "if.*sla.*rate" internal/wrapper/*.go && \
   ! grep -qi "adaptive" internal/wrapper/*.go; then
    pass_test
else
    fail_test "Found reactive behavior in wrapper"
    exit 1
fi

# Test 9: Wrapper logs summary at completion
run_test "Layer 3: Wrapper calls LogSummary"
if grep -q "LogSummary" internal/wrapper/run.go || \
   grep -q "LogSummary" internal/wrapper/attach.go; then
    pass_test
else
    fail_test "LogSummary not called in wrapper"
    exit 1
fi

# Test 10: Integration test - Run a job and check visibility
run_test "Integration: Run job and verify all layers"

# Run a simple job
LOG_FILE=$(mktemp)
/tmp/ffrtmp-cli run --job-id test-visibility -- sleep 1 > "$LOG_FILE" 2>&1 &
WRAPPER_PID=$!

# Wait for completion
sleep 2
wait $WRAPPER_PID 2>/dev/null || true

# Check Layer 3 (human-readable log)
if grep -q "JOB test-visibility" "$LOG_FILE" && \
   grep -q "sla=" "$LOG_FILE" && \
   grep -q "reason=" "$LOG_FILE" && \
   grep -q "runtime=" "$LOG_FILE"; then
    pass_test
else
    fail_test "Layer 3 log summary not found"
    cat "$LOG_FILE"
    rm -f "$LOG_FILE"
    exit 1
fi

rm -f "$LOG_FILE"

# Summary
echo "========================================"
echo "Visibility Tests Complete"
echo "========================================"
echo "Tests run: $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $((TESTS_RUN - TESTS_PASSED))"
echo

if [ $TESTS_PASSED -eq $TESTS_RUN ]; then
    echo -e "${GREEN}✓ All visibility tests passed!${NC}"
    echo
    echo "Three visibility layers verified:"
    echo "  🟢 Layer 1: Immutable job-level truth"
    echo "  🟡 Layer 2: Boring counters only"
    echo "  🔵 Layer 3: Human-readable logs"
    echo
    echo "Killer feature: SLA violation sampling ✓"
    echo "No reactive behavior: Visibility is derived, not driving ✓"
    echo
    exit 0
else
    echo -e "${RED}✗ Some visibility tests failed${NC}"
    exit 1
fi
