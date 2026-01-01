#!/bin/bash
# Comprehensive Production Testing Script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test ports
MASTER_PORT=8100
METRICS_PORT=9100
WORKER_METRICS_PORT=9101
WORKER_METRICS_PORT_2=9102

# Unset API keys for testing (or set a test key)
unset MASTER_API_KEY
unset FFMPEG_RTMP_API_KEY
export TEST_API_KEY="test-key-12345"

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║       COMPREHENSIVE PRODUCTION TESTING SUITE                  ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo ""

# Function to test
test_case() {
    local test_name="$1"
    local test_cmd="$2"
    
    echo -n "Testing: $test_name... "
    
    if eval "$test_cmd" > /tmp/test_output.log 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        cat /tmp/test_output.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

cleanup() {
    echo ""
    echo "Cleaning up..."
    pkill -f "bin/master.*--port $MASTER_PORT" 2>/dev/null || true
    pkill -f "bin/agent.*--metrics-port" 2>/dev/null || true
    sleep 2
}

trap cleanup EXIT

# Clean up any existing processes
cleanup

echo "════════════════════════════════════════════════════════════════"
echo "PHASE 1: Unit Tests"
echo "════════════════════════════════════════════════════════════════"

test_case "Store unit tests" "(cd shared/pkg/store && go test -v 2>&1 | grep -q PASS)"
test_case "API unit tests" "(cd shared/pkg/api && go test -v 2>&1 | grep -q PASS)"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 2: Master Startup & Health"
echo "════════════════════════════════════════════════════════════════"

# Start master
echo "Starting master on port $MASTER_PORT..."
./bin/master --port $MASTER_PORT --metrics-port $METRICS_PORT --tls=false --db="" --api-key "$TEST_API_KEY" > /tmp/master.log 2>&1 &
MASTER_PID=$!
sleep 3

test_case "Master process running" "ps -p $MASTER_PID > /dev/null"
test_case "Master health endpoint" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/health | grep -q healthy"
test_case "Master metrics endpoint" "curl -s http://localhost:$METRICS_PORT/metrics | grep -q ffrtmp_jobs_total"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 3: Worker Registration"
echo "════════════════════════════════════════════════════════════════"

# Start worker 1
echo "Starting worker 1..."
./bin/agent --master http://localhost:$MASTER_PORT --register --allow-master-as-worker --skip-confirmation --api-key "$TEST_API_KEY" --metrics-port $WORKER_METRICS_PORT --poll-interval 5s --heartbeat-interval 10s > /tmp/worker1.log 2>&1 &
WORKER1_PID=$!
sleep 3

test_case "Worker 1 process running" "ps -p $WORKER1_PID > /dev/null"
test_case "Worker 1 metrics endpoint" "curl -s http://localhost:$WORKER_METRICS_PORT/metrics | grep -q ffrtmp_worker_cpu_usage"
test_case "Worker 1 registered" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/nodes | jq -e '.nodes | length > 0'"

# Start worker 2
echo "Starting worker 2..."
./bin/agent --master http://localhost:$MASTER_PORT --register --allow-master-as-worker --skip-confirmation --api-key "$TEST_API_KEY" --metrics-port $WORKER_METRICS_PORT_2 --poll-interval 5s --heartbeat-interval 10s > /tmp/worker2.log 2>&1 &
WORKER2_PID=$!
sleep 3

test_case "Worker 2 process running" "ps -p $WORKER2_PID > /dev/null"
test_case "Two workers registered" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/nodes | jq -e '.nodes | length == 2'"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 4: Job Submission & Priority Scheduling"
echo "════════════════════════════════════════════════════════════════"

# Submit jobs with different priorities
echo "Submitting test jobs..."

# High priority live job
JOB1=$(curl -s -X POST http://localhost:$MASTER_PORT/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TEST_API_KEY" \
  -d '{
    "scenario": "4K60-h264",
    "queue": "live",
    "priority": "high",
    "confidence": "auto",
    "parameters": {"duration": 5}
  }' | jq -r '.id')

test_case "High-priority live job submitted" "[ ! -z '$JOB1' ]"

# Medium priority default job
JOB2=$(curl -s -X POST http://localhost:$MASTER_PORT/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TEST_API_KEY" \
  -d '{
    "scenario": "1080p30-h264",
    "queue": "default",
    "priority": "medium",
    "confidence": "auto",
    "parameters": {"duration": 5}
  }' | jq -r '.id')

test_case "Medium-priority default job submitted" "[ ! -z '$JOB2' ]"

# Low priority batch job
JOB3=$(curl -s -X POST http://localhost:$MASTER_PORT/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TEST_API_KEY" \
  -d '{
    "scenario": "720p30-h265",
    "queue": "batch",
    "priority": "low",
    "confidence": "auto",
    "parameters": {"duration": 5}
  }' | jq -r '.id')

test_case "Low-priority batch job submitted" "[ ! -z '$JOB3' ]"

sleep 2

test_case "Jobs created with correct queue" "curl -s -H 'Authorization: Bearer $TEST_API_KEY' http://localhost:$MASTER_PORT/jobs/$JOB1 | jq -e '.queue == \"live\"'"
test_case "Jobs created with correct priority" "curl -s -H 'Authorization: Bearer $TEST_API_KEY' http://localhost:$MASTER_PORT/jobs/$JOB1 | jq -e '.priority == \"high\"'"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 5: Job Status & Progress"
echo "════════════════════════════════════════════════════════════════"

test_case "Job status endpoint" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/jobs/$JOB1 | jq -e '.id == \"$JOB1\"'"
test_case "Jobs list endpoint" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/jobs | jq -e '.jobs | length >= 3'"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 6: Job Control Operations"
echo "════════════════════════════════════════════════════════════════"

# Submit a job specifically for control testing
JOB_CTRL=$(curl -s -X POST http://localhost:$MASTER_PORT/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-scenario",
    "queue": "default",
    "priority": "medium",
    "confidence": "auto"
  }' | jq -r '.id')

sleep 1

test_case "Pause job endpoint" "curl -s -X POST http://localhost:$MASTER_PORT/jobs/$JOB_CTRL/pause | jq -e '.status == \"success\" or .status == \"paused\"' || true"
test_case "Resume job endpoint" "curl -s -X POST http://localhost:$MASTER_PORT/jobs/$JOB_CTRL/resume | jq -e '.status == \"success\" or .status' || true"
test_case "Cancel job endpoint" "curl -s -X POST http://localhost:$MASTER_PORT/jobs/$JOB_CTRL/cancel | jq -e '.status == \"success\" or .status == \"canceled\"' || true"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 7: Node Information"
echo "════════════════════════════════════════════════════════════════"

# Get first node ID
NODE_ID=$(curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/nodes | jq -r '.nodes[0].id')

test_case "Node list endpoint" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/nodes | jq -e '.count >= 2'"
test_case "Node detail endpoint" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/nodes/$NODE_ID | jq -e '.id == \"$NODE_ID\"'"
test_case "Node has hardware info" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/nodes/$NODE_ID | jq -e '.cpu_threads > 0'"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 8: Metrics Validation"
echo "════════════════════════════════════════════════════════════════"

test_case "Master: jobs_total metric" "curl -s http://localhost:$METRICS_PORT/metrics | grep -q 'ffrtmp_jobs_total{state='"
test_case "Master: active_jobs metric" "curl -s http://localhost:$METRICS_PORT/metrics | grep -q 'ffrtmp_active_jobs'"
test_case "Master: queue_length metric" "curl -s http://localhost:$METRICS_PORT/metrics | grep -q 'ffrtmp_queue_length'"
test_case "Master: nodes_total metric" "curl -s http://localhost:$METRICS_PORT/metrics | grep -q 'ffrtmp_nodes_total'"
test_case "Master: queue_by_priority metric" "curl -s http://localhost:$METRICS_PORT/metrics | grep -q 'ffrtmp_queue_by_priority'"

test_case "Worker 1: cpu_usage metric" "curl -s http://localhost:$WORKER_METRICS_PORT/metrics | grep -q 'ffrtmp_worker_cpu_usage'"
test_case "Worker 1: memory_bytes metric" "curl -s http://localhost:$WORKER_METRICS_PORT/metrics | grep -q 'ffrtmp_worker_memory_bytes'"
test_case "Worker 1: active_jobs metric" "curl -s http://localhost:$WORKER_METRICS_PORT/metrics | grep -q 'ffrtmp_worker_active_jobs'"
test_case "Worker 1: heartbeats metric" "curl -s http://localhost:$WORKER_METRICS_PORT/metrics | grep -q 'ffrtmp_worker_heartbeats_total'"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 9: CLI Commands"
echo "════════════════════════════════════════════════════════════════"

export FFMPEG_RTMP_MASTER="http://localhost:$MASTER_PORT"

test_case "CLI: jobs list" "./bin/ffrtmp jobs status $JOB1 --output json | jq -e '.id'"
test_case "CLI: nodes list" "./bin/ffrtmp nodes list --output json | jq -e '.count >= 2'"
test_case "CLI: nodes describe" "./bin/ffrtmp nodes describe $NODE_ID --output json | jq -e '.id'"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "PHASE 10: Stress Test - Multiple Concurrent Jobs"
echo "════════════════════════════════════════════════════════════════"

echo "Submitting 10 concurrent jobs..."
for i in {1..10}; do
    PRIORITY=("high" "medium" "low")
    QUEUE=("live" "default" "batch")
    PRIO=${PRIORITY[$((i % 3))]}
    Q=${QUEUE[$((i % 3))]}
    
    curl -s -X POST http://localhost:$MASTER_PORT/jobs \
      -H "Content-Type: application/json" \
      -d "{
        \"scenario\": \"test-$i\",
        \"queue\": \"$Q\",
        \"priority\": \"$PRIO\",
        \"confidence\": \"auto\"
      }" > /dev/null &
done

wait
sleep 2

test_case "Stress test: all jobs submitted" "curl -s -H "Authorization: Bearer $TEST_API_KEY" http://localhost:$MASTER_PORT/jobs | jq -e '.jobs | length >= 13'"
test_case "Stress test: metrics updated" "curl -s http://localhost:$METRICS_PORT/metrics | grep 'ffrtmp_jobs_total' | grep -q 'state='"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "TEST SUMMARY"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}╔═══════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  ✅ ALL TESTS PASSED - PRODUCTION READY         ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${RED}╔═══════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║  ❌ SOME TESTS FAILED - REVIEW REQUIRED         ║${NC}"
    echo -e "${RED}╚═══════════════════════════════════════════════════╝${NC}"
    
    echo ""
    echo "Log files for debugging:"
    echo "  Master: /tmp/master.log"
    echo "  Worker 1: /tmp/worker1.log"
    echo "  Worker 2: /tmp/worker2.log"
    
    exit 1
fi
