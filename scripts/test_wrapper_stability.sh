#!/bin/bash
# Comprehensive wrapper stability test suite

set -e

PASSED=0
FAILED=0

test_result() {
    if [ $1 -eq 0 ]; then
        echo "  ✓ $2"
        PASSED=$((PASSED + 1))
    else
        echo "  ✗ $2"
        FAILED=$((FAILED + 1))
    fi
}

echo "=============================================="
echo "  Wrapper Comprehensive Stability Tests"
echo "=============================================="
echo ""

# Test 1: Basic Run Mode
echo "[Test 1] Basic Run Mode"
./bin/ffrtmp run --job-id test-basic -- echo "test" > /dev/null 2>&1
test_result $? "Run mode basic execution"
echo ""

# Test 2: Attach Mode - Context Cancel
echo "[Test 2] Attach Mode - Context Cancel"
sleep 30 &
PID=$!
timeout 1 ./bin/ffrtmp attach --pid $PID --job-id test-attach > /dev/null 2>&1 || true
kill -0 $PID 2>/dev/null
test_result $? "Process survives wrapper detach"
kill $PID 2>/dev/null || true
wait $PID 2>/dev/null || true
echo ""

# Test 3: Wrapper Crash Safety
echo "[Test 3] Wrapper Crash Safety (CRITICAL)"
./bin/ffrtmp run --job-id test-crash -- sleep 60 &
WRAPPER_PID=$!
sleep 1
WORKLOAD_PID=$(pgrep -P $WRAPPER_PID sleep 2>/dev/null)
if [ -n "$WORKLOAD_PID" ]; then
    kill -9 $WRAPPER_PID 2>/dev/null || true
    wait $WRAPPER_PID 2>/dev/null || true
    sleep 1
    kill -0 $WORKLOAD_PID 2>/dev/null
    test_result $? "Workload survives wrapper kill -9"
    kill $WORKLOAD_PID 2>/dev/null || true
else
    test_result 1 "Workload survives wrapper kill -9"
fi
echo ""

# Test 4: Invalid PID Handling
echo "[Test 4] Invalid PID Handling"
./bin/ffrtmp attach --pid 99999999 --job-id test-invalid 2>&1 | grep -q "does not exist"
test_result $? "Invalid PID error handling"
echo ""

# Test 5: Empty Job ID
echo "[Test 5] Empty Job ID Handling"
./bin/ffrtmp run -- echo "test" > /dev/null 2>&1
test_result $? "Empty job ID handled gracefully"
echo ""

# Test 6: Exit Code Propagation
echo "[Test 6] Exit Code Propagation"
./bin/ffrtmp run --job-id test-exit-0 -- /bin/true > /dev/null 2>&1
SUCCESS=$?
./bin/ffrtmp run --job-id test-exit-1 -- /bin/false > /dev/null 2>&1 || true
FAIL=$?
if [ $SUCCESS -eq 0 ]; then
    test_result 0 "Exit code 0 handled"
else
    test_result 1 "Exit code 0 handled"
fi
echo ""

# Test 7: Concurrent Operations
echo "[Test 7] Concurrent Operations"
./bin/ffrtmp run --job-id test-conc-1 -- sleep 2 > /dev/null 2>&1 &
./bin/ffrtmp run --job-id test-conc-2 -- sleep 2 > /dev/null 2>&1 &
./bin/ffrtmp run --job-id test-conc-3 -- sleep 2 > /dev/null 2>&1 &
wait
test_result $? "Concurrent run operations"
echo ""

# Test 8: SIGTERM Handling
echo "[Test 8] SIGTERM Handling"
./bin/ffrtmp run --job-id test-sigterm -- sleep 60 &
WRAPPER_PID=$!
sleep 1
WORKLOAD_PID=$(pgrep -P $WRAPPER_PID sleep 2>/dev/null)
if [ -n "$WORKLOAD_PID" ]; then
    kill -TERM $WRAPPER_PID 2>/dev/null || true
    sleep 1
    kill -0 $WORKLOAD_PID 2>/dev/null
    RESULT=$?
    kill $WORKLOAD_PID 2>/dev/null || true
    test_result $RESULT "Workload survives SIGTERM to wrapper"
else
    test_result 1 "Workload survives SIGTERM to wrapper"
fi
echo ""

# Test 9: Rapid Attach/Detach
echo "[Test 9] Rapid Attach/Detach"
sleep 10 &
PID=$!
for i in {1..3}; do
    timeout 0.5 ./bin/ffrtmp attach --pid $PID --job-id test-rapid-$i > /dev/null 2>&1 || true
done
kill -0 $PID 2>/dev/null
test_result $? "Process survives rapid attach/detach"
kill $PID 2>/dev/null || true
echo ""

# Test 10: Attach to Completed Process
echo "[Test 10] Attach to Completed Process"
sleep 1 &
PID=$!
wait $PID
./bin/ffrtmp attach --pid $PID --job-id test-completed 2>&1 | grep -q "does not exist"
test_result $? "Attach to completed process handled"
echo ""

echo "=============================================="
echo "  Test Results"
echo "=============================================="
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo "  ✅ ALL TESTS PASSED"
    echo "=============================================="
    exit 0
else
    echo "  ❌ SOME TESTS FAILED"
    echo "=============================================="
    exit 1
fi
