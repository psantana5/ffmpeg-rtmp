#!/bin/bash
# Integration tests for production readiness features

set -e

echo "======================================"
echo "Integration Tests - Production Features"
echo "======================================"
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((TESTS_FAILED++))
}

# Test 1: Retry logic unit tests
echo "1. Testing retry logic..."
if go test github.com/psantana5/ffmpeg-rtmp/pkg/agent -run "TestSendHeartbeat|TestGetNextJob|TestSendResults" -v > /tmp/retry_tests.log 2>&1; then
    pass "Retry logic tests passed"
else
    fail "Retry logic tests failed (see /tmp/retry_tests.log)"
fi
echo ""

# Test 2: Graceful shutdown (worker)
echo "2. Testing worker graceful shutdown..."
timeout 10 ./bin/agent --metrics-port=19091 > /tmp/worker_shutdown.log 2>&1 &
WORKER_PID=$!
sleep 3

# Send SIGTERM
kill -TERM $WORKER_PID
sleep 2

# Check if process exited cleanly
if ! ps -p $WORKER_PID > /dev/null 2>&1; then
    if grep -q "shutdown complete\|Shutdown signal received" /tmp/worker_shutdown.log; then
        pass "Worker graceful shutdown works"
    else
        fail "Worker did not log shutdown (see /tmp/worker_shutdown.log)"
    fi
else
    fail "Worker did not shut down"
    kill -9 $WORKER_PID 2>/dev/null || true
fi
echo ""

# Test 3: Graceful shutdown (master)
echo "3. Testing master graceful shutdown..."
timeout 10 ./bin/master --tls=false --port=18080 --metrics=false --db="" > /tmp/master_shutdown.log 2>&1 &
MASTER_PID=$!
sleep 3

# Send SIGTERM
kill -TERM $MASTER_PID
sleep 2

# Check if process exited cleanly
if ! ps -p $MASTER_PID > /dev/null 2>&1; then
    if grep -q "shutdown complete\|Shutdown signal received" /tmp/master_shutdown.log; then
        pass "Master graceful shutdown works"
    else
        fail "Master did not log shutdown (see /tmp/master_shutdown.log)"
    fi
else
    fail "Master did not shut down"
    kill -9 $MASTER_PID 2>/dev/null || true
fi
echo ""

# Test 4: Readiness checks
echo "4. Testing readiness endpoint..."
timeout 10 ./bin/agent --metrics-port=19092 > /tmp/ready_test.log 2>&1 &
WORKER_PID=$!
sleep 3

READY_RESPONSE=$(curl -s http://localhost:19092/ready || echo "{}")
kill $WORKER_PID 2>/dev/null || true
wait $WORKER_PID 2>/dev/null || true

if echo "$READY_RESPONSE" | grep -q "ffmpeg"; then
    pass "Readiness endpoint returns detailed checks"
else
    fail "Readiness endpoint not working properly"
fi
echo ""

# Test 5: No panic in scheduler
echo "5. Testing scheduler panic fix..."
if go test github.com/psantana5/ffmpeg-rtmp/pkg/scheduler -v > /tmp/scheduler_tests.log 2>&1; then
    if ! grep -q "panic" /tmp/scheduler_tests.log; then
        pass "Scheduler runs without panics"
    else
        fail "Scheduler still has panics"
    fi
else
    fail "Scheduler tests failed (see /tmp/scheduler_tests.log)"
fi
echo ""

# Summary
echo "======================================"
echo "Results:"
echo "  Passed: $TESTS_PASSED"
echo "  Failed: $TESTS_FAILED"
echo "======================================"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All integration tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed${NC}"
    exit 1
fi
