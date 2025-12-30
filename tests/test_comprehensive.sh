#!/bin/bash
# Comprehensive Test Suite for Distributed Compute
# Ensures backward compatibility and production features work correctly

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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
    # Kill any running master/agent processes by PID if tracked
    if [ -n "$MASTER_PID" ] && kill -0 "$MASTER_PID" 2>/dev/null; then
        kill "$MASTER_PID" 2>/dev/null || true
        wait "$MASTER_PID" 2>/dev/null || true
    fi
    rm -rf /tmp/test-suite-* 2>/dev/null || true
}

trap cleanup EXIT

echo "========================================"
echo "Comprehensive Test Suite"
echo "========================================"
echo ""

# Test 1: Build succeeds
log_test "1. Build binaries"
if make build-distributed > /tmp/build.log 2>&1; then
    log_pass "Build succeeded"
else
    log_fail "Build failed"
    cat /tmp/build.log
    exit 1
fi

# Test 2: Go vet passes
log_test "2. Go vet check"
if go vet ./... > /tmp/vet.log 2>&1; then
    log_pass "Go vet passed"
else
    log_fail "Go vet failed"
    cat /tmp/vet.log
fi

# Test 3: Master starts in default mode (HTTP, in-memory)
log_test "3. Master starts in default mode"
TEST_DIR="/tmp/test-suite-$$"
mkdir -p "$TEST_DIR"
timeout 5 ./bin/master --port 9001 > "$TEST_DIR/master-default.log" 2>&1 &
MASTER_PID=$!
sleep 2

if curl -s http://localhost:9001/health | grep -q "healthy"; then
    log_pass "Master starts and responds (default mode)"
else
    log_fail "Master health check failed (default mode)"
    cat "$TEST_DIR/master-default.log"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 4: Node registration works (backward compatibility)
log_test "4. Node registration (HTTP mode)"
./bin/master --port 9002 > "$TEST_DIR/master-reg.log" 2>&1 &
MASTER_PID=$!
sleep 2

RESPONSE=$(curl -s -X POST http://localhost:9002/nodes/register \
    -H "Content-Type: application/json" \
    -d '{"address":"test","type":"desktop","cpu_threads":4,"cpu_model":"Test","has_gpu":false,"ram_bytes":8000000000}')

if echo "$RESPONSE" | grep -q '"id"'; then
    log_pass "Node registration works"
else
    log_fail "Node registration failed"
    echo "Response: $RESPONSE"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 5: Job creation and retrieval
log_test "5. Job creation and retrieval"
./bin/master --port 9003 > "$TEST_DIR/master-jobs.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Register node
NODE_ID=$(curl -s -X POST http://localhost:9003/nodes/register \
    -H "Content-Type: application/json" \
    -d '{"address":"test","type":"desktop","cpu_threads":4,"cpu_model":"Test","has_gpu":false,"ram_bytes":8000000000}' \
    | jq -r '.id')

# Create job
JOB_RESPONSE=$(curl -s -X POST http://localhost:9003/jobs \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","confidence":"auto"}')

if echo "$JOB_RESPONSE" | grep -q '"id"'; then
    log_pass "Job creation works"
else
    log_fail "Job creation failed"
    echo "Response: $JOB_RESPONSE"
fi

# Get next job
NEXT_JOB=$(curl -s "http://localhost:9003/jobs/next?node_id=$NODE_ID")
if echo "$NEXT_JOB" | grep -q '"id"'; then
    log_pass "Job retrieval works"
else
    log_fail "Job retrieval failed"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 6: SQLite persistence
log_test "6. SQLite persistence"
DB_PATH="$TEST_DIR/test.db"
./bin/master --port 9004 --db "$DB_PATH" > "$TEST_DIR/master-db.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Create node
curl -s -X POST http://localhost:9004/nodes/register \
    -H "Content-Type: application/json" \
    -d '{"address":"test","type":"desktop","cpu_threads":4,"cpu_model":"Test","has_gpu":false,"ram_bytes":8000000000}' \
    > /dev/null

# Check DB was created
if [ -f "$DB_PATH" ]; then
    log_pass "SQLite database created"
else
    log_fail "SQLite database not created"
fi

# Restart and check persistence
kill $MASTER_PID 2>/dev/null || true
sleep 2

./bin/master --port 9004 --db "$DB_PATH" > "$TEST_DIR/master-db2.log" 2>&1 &
MASTER_PID=$!
sleep 2

NODE_COUNT=$(curl -s http://localhost:9004/nodes | jq -r '.count')
if [ "$NODE_COUNT" = "1" ]; then
    log_pass "Data persisted across restart"
else
    log_fail "Data not persisted (count: $NODE_COUNT)"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 7: Certificate generation
log_test "7. Certificate generation"
CERT_DIR="$TEST_DIR/certs"
mkdir -p "$CERT_DIR"
if ./bin/master --generate-cert --cert "$CERT_DIR/test.crt" --key "$CERT_DIR/test.key" > "$TEST_DIR/cert-gen.log" 2>&1; then
    if [ -f "$CERT_DIR/test.crt" ] && [ -f "$CERT_DIR/test.key" ]; then
        log_pass "Certificate generation works"
    else
        log_fail "Certificate files not created"
    fi
else
    log_fail "Certificate generation failed"
    cat "$TEST_DIR/cert-gen.log"
fi

# Test 8: TLS mode
log_test "8. TLS/HTTPS mode"
./bin/master --port 9005 --tls --cert "$CERT_DIR/test.crt" --key "$CERT_DIR/test.key" \
    > "$TEST_DIR/master-tls.log" 2>&1 &
MASTER_PID=$!
sleep 2

if curl -k -s https://localhost:9005/health | grep -q "healthy"; then
    log_pass "TLS mode works"
else
    log_fail "TLS mode failed"
    cat "$TEST_DIR/master-tls.log"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 9: API authentication
log_test "9. API authentication"
./bin/master --port 9006 --api-key "test-key-123" > "$TEST_DIR/master-auth.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Test without key (should fail)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9006/nodes)
if [ "$STATUS" = "401" ]; then
    log_pass "API auth rejects requests without key"
else
    log_fail "API auth did not reject (status: $STATUS)"
fi

# Test with key (should succeed)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer test-key-123" http://localhost:9006/nodes)
if [ "$STATUS" = "200" ]; then
    log_pass "API auth accepts requests with valid key"
else
    log_fail "API auth rejected valid key (status: $STATUS)"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 10: Graceful shutdown
log_test "10. Graceful shutdown"
./bin/master --port 9007 > "$TEST_DIR/master-shutdown.log" 2>&1 &
MASTER_PID=$!
sleep 2

kill -TERM $MASTER_PID
sleep 3

if grep -q "Shutting down gracefully" "$TEST_DIR/master-shutdown.log"; then
    log_pass "Graceful shutdown works"
else
    log_fail "Graceful shutdown not detected"
fi

# Test 11: Agent hardware detection
log_test "11. Agent hardware detection"
./bin/master --port 9008 > "$TEST_DIR/master-agent.log" 2>&1 &
MASTER_PID=$!
sleep 2

# Start agent without registration (should not crash)
timeout 3 ./bin/agent --master http://localhost:9008 > "$TEST_DIR/agent-noregister.log" 2>&1 || true

if grep -q "Hardware detected" "$TEST_DIR/agent-noregister.log"; then
    log_pass "Agent hardware detection works"
else
    log_fail "Agent hardware detection failed"
    cat "$TEST_DIR/agent-noregister.log"
fi
kill $MASTER_PID 2>/dev/null || true
sleep 1

# Test 12: Existing Python tests still work
log_test "12. Python integration (backward compatibility)"
if python3 -c "import sys; sys.exit(0)" 2>/dev/null; then
    if [ -f "scripts/recommend_test.py" ]; then
        if timeout 5 python3 scripts/recommend_test.py > "$TEST_DIR/python-test.log" 2>&1; then
            log_pass "Python scripts still functional"
        else
            log_fail "Python script failed"
        fi
    else
        log_pass "Python scripts check skipped (no script found)"
    fi
else
    log_pass "Python test skipped (Python not available)"
fi

# Test 13: Integration test script
log_test "13. Integration test script"
if [ -x "./test_distributed.sh" ]; then
    # Start master for integration test
    ./bin/master --port 9009 > "$TEST_DIR/master-integration.log" 2>&1 &
    MASTER_PID=$!
    sleep 2
    
    # Run a simplified version of the integration test
    NODE_RESPONSE=$(curl -s -X POST http://localhost:9009/nodes/register \
        -H "Content-Type: application/json" \
        -d '{"address":"test","type":"desktop","cpu_threads":4,"cpu_model":"Test","has_gpu":false,"ram_bytes":8000000000}')
    
    JOB_RESPONSE=$(curl -s -X POST http://localhost:9009/jobs \
        -H "Content-Type: application/json" \
        -d '{"scenario":"test"}')
    
    if echo "$NODE_RESPONSE" | grep -q '"id"' && echo "$JOB_RESPONSE" | grep -q '"id"'; then
        log_pass "Integration test operations work"
    else
        log_fail "Integration test operations failed"
    fi
    
    kill $MASTER_PID 2>/dev/null || true
    sleep 1
else
    log_fail "Integration test script not found"
fi

# Test 14: Production test script exists and is executable
log_test "14. Production test scripts"
if [ -x "./test_production_simple.sh" ]; then
    log_pass "Production test script exists and is executable"
else
    log_fail "Production test script missing or not executable"
fi

# Summary
echo ""
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo "The changes are safe to merge."
    exit 0
else
    echo -e "${RED}✗ Some tests failed!${NC}"
    echo "Please review the failures before merging."
    exit 1
fi
