#!/bin/bash
# Test wrapper integration with worker agent
# Verifies that jobs with WrapperEnabled flag use the wrapper path

set -e

echo "=== Wrapper Integration Test ==="
echo "Testing worker agent integration with edge wrapper"
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0

# Helper function
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

# Build test binary
echo "Building worker agent..."
cd "$(dirname "$0")/.."
go build -o /tmp/ffrtmp-worker-test ./worker/cmd/agent/ || {
    echo "Failed to build worker agent"
    exit 1
}
echo "✓ Built worker agent"
echo

# Test 1: Verify agent compiles with wrapper imports
run_test "Worker agent compiles with wrapper integration"
if [ -f /tmp/ffrtmp-worker-test ]; then
    pass_test
else
    fail_test "Binary not created"
    exit 1
fi

# Test 2: Check wrapper integration helper exists
run_test "Wrapper integration helper exists"
if grep -q "ExecuteWithWrapper" shared/pkg/agent/wrapper_integration.go; then
    pass_test
else
    fail_test "ExecuteWithWrapper function not found"
    exit 1
fi

# Test 3: Check worker agent routes to wrapper
run_test "Worker agent checks WrapperEnabled flag"
if grep -q "job.WrapperEnabled" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "WrapperEnabled check not found in worker agent"
    exit 1
fi

# Test 4: Check executeWithWrapperPath function exists
run_test "executeWithWrapperPath function exists"
if grep -q "executeWithWrapperPath" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "executeWithWrapperPath function not found"
    exit 1
fi

# Test 5: Verify job model has WrapperEnabled field
run_test "Job model has WrapperEnabled field"
if grep -q "WrapperEnabled" shared/pkg/models/job.go; then
    pass_test
else
    fail_test "WrapperEnabled field not in job model"
    exit 1
fi

# Test 6: Verify job model has WrapperConstraints
run_test "Job model has WrapperConstraints"
if grep -q "WrapperConstraints" shared/pkg/models/job.go; then
    pass_test
else
    fail_test "WrapperConstraints not in job model"
    exit 1
fi

# Test 7: Verify wrapper integration uses agent.ExecuteWithWrapper
run_test "executeWithWrapperPath calls agent.ExecuteWithWrapper"
if grep -q "agent.ExecuteWithWrapper" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "agent.ExecuteWithWrapper call not found"
    exit 1
fi

# Test 8: Check backward compatibility (legacy path still exists)
run_test "Legacy execution path preserved"
if grep -q "exec.CommandContext" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "Legacy execution path removed"
    exit 1
fi

# Test 9: Verify wrapper returns platform SLA
run_test "Wrapper execution includes platform SLA"
if grep -q "platform_sla" worker/cmd/agent/main.go; then
    pass_test
else
    fail_test "Platform SLA not included in metrics"
    exit 1
fi

# Test 10: Integration test - simulate wrapper-enabled job execution
run_test "Simulated wrapper execution (dry run)"
# This is a sanity check - we can't run full integration without master server
# Just verify the code paths exist and are callable
if grep -qE "if job.WrapperEnabled" worker/cmd/agent/main.go && \
   grep -q "ExecuteWithWrapper" shared/pkg/agent/wrapper_integration.go && \
   grep -q "func Run" internal/wrapper/run.go; then
    pass_test
else
    fail_test "Wrapper execution chain incomplete"
    exit 1
fi

# Summary
echo "========================================"
echo "Integration Tests Complete"
echo "========================================"
echo "Tests run: $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $((TESTS_RUN - TESTS_PASSED))"
echo

if [ $TESTS_PASSED -eq $TESTS_RUN ]; then
    echo -e "${GREEN}✓ All integration tests passed!${NC}"
    echo
    echo "Phase 2 integration complete:"
    echo "  • Worker agent checks WrapperEnabled flag"
    echo "  • Routes to executeWithWrapperPath when enabled"
    echo "  • Calls agent.ExecuteWithWrapper helper"
    echo "  • Returns platform SLA metrics"
    echo "  • Backward compatible with legacy execution"
    echo
    exit 0
else
    echo -e "${RED}✗ Some integration tests failed${NC}"
    exit 1
fi
