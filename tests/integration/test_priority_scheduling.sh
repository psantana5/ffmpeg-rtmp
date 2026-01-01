#!/bin/bash
# Test Priority Scheduling System
# Tests: Queue priority (live > default > batch), Priority (high > medium > low), FIFO

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}================================================${NC}"
echo -e "${YELLOW}Testing Priority Scheduling System${NC}"
echo -e "${YELLOW}================================================${NC}"

# Configuration
MASTER_URL="${MASTER_URL:-http://localhost:${PORT:-8080}}"
API_KEY="${MASTER_API_KEY:-test-api-key}"
TEST_DB="/tmp/test_priority_scheduling.db"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ -n "$MASTER_PID" ]; then
        kill $MASTER_PID 2>/dev/null || true
    fi
    rm -f "$TEST_DB" "$TEST_DB-shm" "$TEST_DB-wal"
}
trap cleanup EXIT

# Start master node
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

# Wait for master to start
sleep 2

if ! kill -0 $MASTER_PID 2>/dev/null; then
    echo -e "${RED}✗ Master failed to start${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Master started (PID: $MASTER_PID)${NC}"

# Register test nodes
echo -e "\n${YELLOW}2. Registering worker nodes...${NC}"

for i in {1..3}; do
    NODE_RESP=$(curl -s -X POST "$MASTER_URL/nodes/register" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"address\": \"worker-$i\",
            \"type\": \"server\",
            \"cpu_threads\": 8,
            \"cpu_model\": \"Test CPU\",
            \"has_gpu\": false,
            \"ram_total_bytes\": 17179869184
        }")
    
    NODE_ID=$(echo "$NODE_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$NODE_ID" ]; then
        echo -e "${GREEN}✓ Registered worker-$i (ID: $NODE_ID)${NC}"
        eval "WORKER${i}_ID=$NODE_ID"
    else
        echo -e "${RED}✗ Failed to register worker-$i${NC}"
        exit 1
    fi
done

# Test 1: Queue Priority (live > default > batch)
echo -e "\n${YELLOW}3. Testing Queue Priority (live > default > batch)${NC}"

# Create jobs in reverse priority order
echo "   Creating batch job..."
BATCH_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","queue":"batch","priority":"medium"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

echo "   Creating default job..."
DEFAULT_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","queue":"default","priority":"medium"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

echo "   Creating live job..."
LIVE_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","queue":"live","priority":"medium"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# Get next job - should be live
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$WORKER1_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$LIVE_JOB" ]; then
    echo -e "${GREEN}✓ Queue priority works: Live job selected first${NC}"
else
    echo -e "${RED}✗ Queue priority failed: Expected $LIVE_JOB, got $ASSIGNED_ID${NC}"
    exit 1
fi

# Get next job - should be default
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$WORKER2_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$DEFAULT_JOB" ]; then
    echo -e "${GREEN}✓ Queue priority works: Default job selected second${NC}"
else
    echo -e "${RED}✗ Queue priority failed: Expected $DEFAULT_JOB, got $ASSIGNED_ID${NC}"
    exit 1
fi

# Test 2: Priority within Queue (high > medium > low)
echo -e "\n${YELLOW}4. Testing Priority within Queue (high > medium > low)${NC}"

# Create default queue jobs with different priorities
echo "   Creating low priority job..."
LOW_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","queue":"default","priority":"low"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

echo "   Creating high priority job..."
HIGH_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","queue":"default","priority":"high"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

echo "   Creating medium priority job..."
MED_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"test","queue":"default","priority":"medium"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# Complete the batch job so we have a free worker
curl -s -X POST "$MASTER_URL/results" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"job_id\":\"$BATCH_JOB\",\"node_id\":\"$WORKER3_ID\",\"status\":\"completed\",\"completed_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" > /dev/null

# Get next job - should be high priority
sleep 1
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$WORKER3_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$HIGH_JOB" ]; then
    echo -e "${GREEN}✓ Priority works: High priority job selected first${NC}"
else
    echo -e "${RED}✗ Priority failed: Expected $HIGH_JOB, got $ASSIGNED_ID${NC}"
    exit 1
fi

# Test 3: FIFO within same priority class
echo -e "\n${YELLOW}5. Testing FIFO within same priority class${NC}"

# Create multiple jobs with same queue and priority
JOBS=()
for i in {1..3}; do
    JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"scenario\":\"test-$i\",\"queue\":\"default\",\"priority\":\"low\"}" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    JOBS+=("$JOB")
    echo "   Created job $i: $JOB"
    sleep 0.1  # Ensure different timestamps
done

# The first created should be assigned first (already LOW_JOB is first)
# We need to complete current jobs to get workers free
curl -s -X POST "$MASTER_URL/results" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"job_id\":\"$HIGH_JOB\",\"node_id\":\"$WORKER3_ID\",\"status\":\"completed\",\"completed_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" > /dev/null

sleep 1
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$WORKER3_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$LOW_JOB" ]; then
    echo -e "${GREEN}✓ FIFO works: First created low-priority job selected${NC}"
else
    echo -e "${RED}✗ FIFO failed: Expected $LOW_JOB (oldest), got $ASSIGNED_ID${NC}"
    exit 1
fi

# Summary
echo -e "\n${YELLOW}================================================${NC}"
echo -e "${GREEN}✓ All Priority Scheduling Tests Passed!${NC}"
echo -e "${YELLOW}================================================${NC}"
echo -e "Tested:"
echo -e "  • Queue priority (live > default > batch)"
echo -e "  • Priority within queue (high > medium > low)"
echo -e "  • FIFO ordering within same priority class"
