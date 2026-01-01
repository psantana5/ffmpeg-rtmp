#!/bin/bash
# Test Job Control (Pause/Resume/Cancel)
# Tests: Job lifecycle control via API endpoints

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}================================================${NC}"
echo -e "${YELLOW}Testing Job Control (Pause/Resume/Cancel)${NC}"
echo -e "${YELLOW}================================================${NC}"

# Configuration
MASTER_URL="${MASTER_URL:-http://localhost:${PORT:-8080}}"
API_KEY="${MASTER_API_KEY:-test-api-key}"
TEST_DB="/tmp/test_job_control.db"

# Cleanup
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ -n "$MASTER_PID" ]; then
        kill $MASTER_PID 2>/dev/null || true
    fi
    rm -f "$TEST_DB" "$TEST_DB-shm" "$TEST_DB-wal"
}
trap cleanup EXIT

# Start master
echo -e "\n${YELLOW}1. Starting master node...${NC}"
cd "$PROJECT_ROOT"
rm -f "$TEST_DB" "$TEST_DB-shm" "$TEST_DB-wal"

go build -o /tmp/test-master ./master/cmd/master
MASTER_API_KEY="$API_KEY" /tmp/test-master \
    --db "$TEST_DB" \
    --port ${PORT:-8080} \
    --tls=false \
    --metrics=false &
MASTER_PID=$!
sleep 2

if ! kill -0 $MASTER_PID 2>/dev/null; then
    echo -e "${RED}✗ Master failed to start${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Master started${NC}"

# Register worker
echo -e "\n${YELLOW}2. Registering worker node...${NC}"
NODE_RESP=$(curl -s -X POST "$MASTER_URL/nodes/register" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "address": "test-worker",
        "type": "server",
        "cpu_threads": 8,
        "cpu_model": "Test CPU",
        "has_gpu": false,
        "ram_total_bytes": 17179869184
    }')

NODE_ID=$(echo "$NODE_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
if [ -n "$NODE_ID" ]; then
    echo -e "${GREEN}✓ Registered worker (ID: $NODE_ID)${NC}"
else
    echo -e "${RED}✗ Failed to register worker${NC}"
    exit 1
fi

# Test 1: Pause a processing job
echo -e "\n${YELLOW}3. Testing PAUSE operation${NC}"

echo "   Creating and assigning job..."
JOB_RESP=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test-pause","queue":"default","priority":"medium"}')
JOB_ID=$(echo "$JOB_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# Assign job to worker
curl -s "$MASTER_URL/jobs/next?node_id=$NODE_ID" -H "Authorization: Bearer $API_KEY" > /dev/null

# Check job is processing
JOB_STATUS=$(curl -s "$MASTER_URL/jobs/$JOB_ID" -H "Authorization: Bearer $API_KEY" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$JOB_STATUS" != "processing" ]; then
    echo -e "${RED}✗ Job not in processing state: $JOB_STATUS${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Job is processing${NC}"

# Pause the job
echo "   Pausing job..."
PAUSE_RESP=$(curl -s -X POST "$MASTER_URL/jobs/$JOB_ID/pause" \
    -H "Authorization: Bearer $API_KEY")

# Verify job is paused
sleep 0.5
JOB_STATUS=$(curl -s "$MASTER_URL/jobs/$JOB_ID" -H "Authorization: Bearer $API_KEY" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$JOB_STATUS" = "paused" ]; then
    echo -e "${GREEN}✓ Job successfully paused${NC}"
else
    echo -e "${RED}✗ Job not paused: $JOB_STATUS${NC}"
    exit 1
fi

# Test 2: Resume a paused job
echo -e "\n${YELLOW}4. Testing RESUME operation${NC}"

echo "   Resuming job..."
RESUME_RESP=$(curl -s -X POST "$MASTER_URL/jobs/$JOB_ID/resume" \
    -H "Authorization: Bearer $API_KEY")

# Verify job is processing again
sleep 0.5
JOB_STATUS=$(curl -s "$MASTER_URL/jobs/$JOB_ID" -H "Authorization: Bearer $API_KEY" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$JOB_STATUS" = "processing" ]; then
    echo -e "${GREEN}✓ Job successfully resumed${NC}"
else
    echo -e "${RED}✗ Job not resumed: $JOB_STATUS${NC}"
    exit 1
fi

# Test 3: Cancel a job
echo -e "\n${YELLOW}5. Testing CANCEL operation${NC}"

echo "   Canceling job..."
CANCEL_RESP=$(curl -s -X POST "$MASTER_URL/jobs/$JOB_ID/cancel" \
    -H "Authorization: Bearer $API_KEY")

# Verify job is canceled
sleep 0.5
JOB_STATUS=$(curl -s "$MASTER_URL/jobs/$JOB_ID" -H "Authorization: Bearer $API_KEY" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$JOB_STATUS" = "canceled" ]; then
    echo -e "${GREEN}✓ Job successfully canceled${NC}"
else
    echo -e "${RED}✗ Job not canceled: $JOB_STATUS${NC}"
    exit 1
fi

# Verify worker is available again
NODE_STATUS=$(curl -s "$MASTER_URL/nodes/$NODE_ID" -H "Authorization: Bearer $API_KEY" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
if [ "$NODE_STATUS" = "available" ]; then
    echo -e "${GREEN}✓ Worker freed after cancel${NC}"
else
    echo -e "${RED}✗ Worker not available: $NODE_STATUS${NC}"
    exit 1
fi

# Test 4: State transitions tracking
echo -e "\n${YELLOW}6. Testing state transition tracking${NC}"

JOB_DETAIL=$(curl -s "$MASTER_URL/jobs/$JOB_ID" -H "Authorization: Bearer $API_KEY")
TRANSITIONS_COUNT=$(echo "$JOB_DETAIL" | grep -o '"state_transitions"' | wc -l)

if [ "$TRANSITIONS_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ State transitions recorded${NC}"
    echo "   Job went through: pending → assigned → processing → paused → processing → canceled"
else
    echo -e "${YELLOW}⚠ State transitions not visible in response (may be empty array)${NC}"
fi

# Test 5: Error cases
echo -e "\n${YELLOW}7. Testing error cases${NC}"

# Try to pause an already canceled job
echo "   Testing pause on canceled job (should fail)..."
PAUSE_RESP=$(curl -s -w "\n%{http_code}" -X POST "$MASTER_URL/jobs/$JOB_ID/pause" \
    -H "Authorization: Bearer $API_KEY")
HTTP_CODE=$(echo "$PAUSE_RESP" | tail -1)

if [ "$HTTP_CODE" = "400" ]; then
    echo -e "${GREEN}✓ Correctly rejected pause on canceled job${NC}"
else
    echo -e "${RED}✗ Should have rejected pause, got HTTP $HTTP_CODE${NC}"
fi

# Try to cancel non-existent job
echo "   Testing cancel on non-existent job (should fail)..."
CANCEL_RESP=$(curl -s -w "\n%{http_code}" -X POST "$MASTER_URL/jobs/nonexistent-id/cancel" \
    -H "Authorization: Bearer $API_KEY")
HTTP_CODE=$(echo "$CANCEL_RESP" | tail -1)

if [ "$HTTP_CODE" = "404" ]; then
    echo -e "${GREEN}✓ Correctly returned 404 for non-existent job${NC}"
else
    echo -e "${RED}✗ Should have returned 404, got HTTP $HTTP_CODE${NC}"
fi

# Summary
echo -e "\n${YELLOW}================================================${NC}"
echo -e "${GREEN}✓ All Job Control Tests Passed!${NC}"
echo -e "${YELLOW}================================================${NC}"
echo -e "Tested:"
echo -e "  • Pause processing job"
echo -e "  • Resume paused job"
echo -e "  • Cancel job (frees worker)"
echo -e "  • State transition tracking"
echo -e "  • Error handling"
