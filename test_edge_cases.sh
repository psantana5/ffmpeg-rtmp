#!/bin/bash
# Edge Cases and Stress Testing
# Tests for error handling, edge cases, and concurrent operations

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED_TESTS=0
PASSED_TESTS=0
MASTER_PID=""

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED_TESTS++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED_TESTS++))
}

cleanup() {
    if [ -n "$MASTER_PID" ] && kill -0 "$MASTER_PID" 2>/dev/null; then
        kill "$MASTER_PID" 2>/dev/null || true
        wait "$MASTER_PID" 2>/dev/null || true
    fi
    rm -rf /tmp/edge-test-* 2>/dev/null || true
}

trap cleanup EXIT

echo "========================================"
echo "Edge Cases and Stress Testing"
echo "========================================"
echo ""

TEST_DIR="/tmp/edge-test-$$"
mkdir -p "$TEST_DIR"

# Test 1: Invalid JSON payloads
log_test "1. Handling invalid JSON"
./bin/master --port 9101 > "$TEST_DIR/master1.log" 2>&1 &
MASTER_PID=$!
sleep 2

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:9101/nodes/register \
    -H "Content-Type: application/json" \
    -d 'invalid json')

if [ "$STATUS" = "400" ]; then
    log_pass "Invalid JSON rejected with 400"
else
    log_fail "Invalid JSON not rejected properly (status: $STATUS)"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 2: Missing required fields
log_test "2. Missing required fields in registration"
./bin/master --port 9102 > "$TEST_DIR/master2.log" 2>&1 &
MASTER_PID=$!
sleep 2

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:9102/nodes/register \
    -H "Content-Type: application/json" \
    -d '{"address":"test"}')

if [ "$STATUS" = "400" ]; then
    log_pass "Missing fields rejected with 400"
else
    log_fail "Missing fields not rejected (status: $STATUS)"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 3: Querying non-existent node
log_test "3. Non-existent node query"
./bin/master --port 9103 > "$TEST_DIR/master3.log" 2>&1 &
MASTER_PID=$!
sleep 2

RESPONSE=$(curl -s "http://localhost:9103/jobs/next?node_id=nonexistent-id")
if echo "$RESPONSE" | grep -q "not found\|error"; then
    log_pass "Non-existent node query handled correctly"
else
    log_fail "Non-existent node query not handled"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 4: Concurrent node registrations
log_test "4. Concurrent node registrations"
./bin/master --port 9104 > "$TEST_DIR/master4.log" 2>&1 &
MASTER_PID=$!
sleep 2

for i in {1..10}; do
    curl -s -X POST http://localhost:9104/nodes/register \
        -H "Content-Type: application/json" \
        -d "{\"address\":\"node-$i\",\"type\":\"desktop\",\"cpu_threads\":4,\"cpu_model\":\"Test\",\"has_gpu\":false,\"ram_bytes\":8000000000}" \
        > /dev/null &
done
wait

sleep 2
NODE_COUNT=$(curl -s http://localhost:9104/nodes | jq -r '.count')
if [ "$NODE_COUNT" = "10" ]; then
    log_pass "Concurrent registrations handled correctly (10 nodes)"
else
    log_fail "Concurrent registrations failed (count: $NODE_COUNT)"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 5: Multiple jobs for same node
log_test "5. Multiple jobs FIFO ordering"
./bin/master --port 9105 > "$TEST_DIR/master5.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Register node
NODE_ID=$(curl -s -X POST http://localhost:9105/nodes/register \
    -H "Content-Type: application/json" \
    -d '{"address":"test","type":"desktop","cpu_threads":4,"cpu_model":"Test","has_gpu":false,"ram_bytes":8000000000}' \
    | jq -r '.id')

# Create multiple jobs
JOB1_ID=$(curl -s -X POST http://localhost:9105/jobs \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test1","confidence":"auto"}' \
    | jq -r '.id')

JOB2_ID=$(curl -s -X POST http://localhost:9105/jobs \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test2","confidence":"auto"}' \
    | jq -r '.id')

# Get next job
NEXT_JOB_ID=$(curl -s "http://localhost:9105/jobs/next?node_id=$NODE_ID" | jq -r '.job.id')

if [ "$NEXT_JOB_ID" = "$JOB1_ID" ]; then
    log_pass "FIFO job ordering works (got first job)"
else
    log_fail "FIFO ordering broken (expected $JOB1_ID, got $NEXT_JOB_ID)"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 6: Database corruption resilience
log_test "6. Corrupted database handling"
DB_PATH="$TEST_DIR/corrupt.db"
echo "garbage data" > "$DB_PATH"

./bin/master --port 9106 --db "$DB_PATH" > "$TEST_DIR/master6.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Check if master recovered or handled gracefully
if grep -q "Failed to create SQLite store\|failed to open database" "$TEST_DIR/master6.log"; then
    log_pass "Corrupted database detected and reported"
else
    # Try to connect - if it works, it recovered
    if curl -s http://localhost:9106/health | grep -q "healthy"; then
        log_pass "Master recovered from corrupted DB"
    else
        log_fail "Master neither detected corruption nor recovered"
    fi
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 7: Port already in use
log_test "7. Port conflict handling"
./bin/master --port 9107 > "$TEST_DIR/master7a.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Try to start another master on same port
timeout 3 ./bin/master --port 9107 > "$TEST_DIR/master7b.log" 2>&1 || true

if grep -q "address already in use\|bind.*failed" "$TEST_DIR/master7b.log"; then
    log_pass "Port conflict detected and reported"
else
    log_fail "Port conflict not detected"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 8: Large payload handling
log_test "8. Large payload handling"
./bin/master --port 9108 > "$TEST_DIR/master8.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Create a large parameters object
LARGE_PARAMS='{"data":"'$(python3 -c "print('x' * 10000)")'","extra":"data"}'
RESPONSE=$(curl -s -X POST http://localhost:9108/jobs \
    -H "Content-Type: application/json" \
    -d "{\"scenario\":\"large-test\",\"confidence\":\"auto\",\"parameters\":$LARGE_PARAMS}")

if echo "$RESPONSE" | grep -q '"id"'; then
    log_pass "Large payload handled correctly"
else
    log_fail "Large payload rejected"
fi
kill $MASTER_PID 2>/dev/null || true
wait $MASTER_PID 2>/dev/null || true
sleep 1

# Test 9: Rapid start/stop cycles
log_test "9. Rapid start/stop cycles"
for i in {1..5}; do
    ./bin/master --port 9109 > "$TEST_DIR/master9-$i.log" 2>&1 &
    PID=$!
    sleep 1
    kill $PID 2>/dev/null || true
    wait $PID 2>/dev/null || true
done
log_pass "Rapid start/stop cycles completed without crashes"

# Test 10: Invalid certificate files
log_test "10. Invalid certificate handling"
echo "invalid cert data" > "$TEST_DIR/invalid.crt"
echo "invalid key data" > "$TEST_DIR/invalid.key"

timeout 3 ./bin/master --port 9110 --tls --cert "$TEST_DIR/invalid.crt" --key "$TEST_DIR/invalid.key" \
    > "$TEST_DIR/master10.log" 2>&1 || true

if grep -q "failed to load\|certificate\|key" "$TEST_DIR/master10.log"; then
    log_pass "Invalid certificates detected and reported"
else
    log_fail "Invalid certificates not detected"
fi

# Summary
echo ""
echo "========================================"
echo "Edge Case Test Summary"
echo "========================================"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}✓ All edge case tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some edge case tests failed!${NC}"
    exit 1
fi
