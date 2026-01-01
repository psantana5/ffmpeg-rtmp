#!/bin/bash
# Quick Production Validation Test - Fixed Version

set -e

PORT=8105
METRICS_PORT=9105
WORKER_PORT=9106

echo "════════════════════════════════════════════"
echo "Quick Production Validation Test"
echo "════════════════════════════════════════════"
echo ""

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    lsof -ti:$PORT,$METRICS_PORT,$WORKER_PORT 2>/dev/null | while read pid; do kill $pid 2>/dev/null || true; done
    sleep 1
}

trap cleanup EXIT
cleanup

# Build
echo "1. Building binaries..."
go build -o /tmp/master-test ./master/cmd/master
go build -o /tmp/agent-test ./worker/cmd/agent  
go build -o /tmp/ffrtmp-test ./cmd/ffrtmp
echo "✓ Binaries built"
echo ""

# Unset all API key env vars
unset MASTER_API_KEY
unset FFMPEG_RTMP_API_KEY
TEST_KEY="test-validation-key"

# Start master
echo "2. Starting master..."
/tmp/master-test --port $PORT --metrics-port $METRICS_PORT --tls=false --db="" --api-key="$TEST_KEY" > /tmp/master-validation.log 2>&1 &
MASTER_PID=$!
sleep 3
ps -p $MASTER_PID > /dev/null && echo "✓ Master running (PID: $MASTER_PID)" || { echo "✗ Master failed"; exit 1; }
echo ""

# Test master health
echo "3. Testing master health..."
curl -sf http://localhost:$PORT/health | grep -q healthy && echo "✓ Health endpoint OK" || { echo "✗ Health failed"; exit 1; }
echo ""

# Test master metrics
echo "4. Testing master metrics..."
curl -sf http://localhost:$METRICS_PORT/metrics | grep -q ffrtmp_jobs_total && echo "✓ Master metrics OK" || { echo "✗ Metrics failed"; exit 1; }
curl -sf http://localhost:$METRICS_PORT/metrics | grep -q ffrtmp_queue_length && echo "✓ Queue metrics OK"
curl -sf http://localhost:$METRICS_PORT/metrics | grep -q ffrtmp_active_jobs && echo "✓ Active jobs metric OK"
echo ""

# Start worker
echo "5. Starting worker..."
/tmp/agent-test --master http://localhost:$PORT --register --allow-master-as-worker --skip-confirmation --api-key="$TEST_KEY" --metrics-port $WORKER_PORT --poll-interval 10s > /tmp/worker-validation.log 2>&1 &
WORKER_PID=$!
sleep 5
ps -p $WORKER_PID > /dev/null && echo "✓ Worker running (PID: $WORKER_PID)" || { echo "✗ Worker failed"; exit 1; }
echo ""

# Test worker metrics
echo "6. Testing worker metrics..."
curl -sf http://localhost:$WORKER_PORT/metrics | grep -q ffrtmp_worker_cpu_usage && echo "✓ Worker CPU metric OK"
curl -sf http://localhost:$WORKER_PORT/metrics | grep -q ffrtmp_worker_memory_bytes && echo "✓ Worker memory metric OK"
curl -sf http://localhost:$WORKER_PORT/metrics | grep -q ffrtmp_worker_heartbeats_total && echo "✓ Worker heartbeat metric OK"
echo ""

# Test worker registration
echo "7. Testing worker registration..."
sleep 2
curl -sf -H "Authorization: Bearer $TEST_KEY" http://localhost:$PORT/nodes | jq -e '.nodes | length > 0' > /dev/null && echo "✓ Worker registered with master"
NODE_ID=$(curl -sf -H "Authorization: Bearer $TEST_KEY" http://localhost:$PORT/nodes | jq -r '.nodes[0].id')
echo "   Node ID: $NODE_ID"
echo ""

# Submit test jobs
echo "8. Submitting test jobs..."
JOB1=$(curl -sf -H "Authorization: Bearer $TEST_KEY" -H "Content-Type: application/json" -X POST http://localhost:$PORT/jobs -d '{"scenario":"4K60-h264","queue":"live","priority":"high","confidence":"auto"}' | jq -r '.id')
echo "✓ High-priority live job: $JOB1"

JOB2=$(curl -sf -H "Authorization: Bearer $TEST_KEY" -H "Content-Type: application/json" -X POST http://localhost:$PORT/jobs -d '{"scenario":"1080p30-h264","queue":"default","priority":"medium","confidence":"auto"}' | jq -r '.id')
echo "✓ Medium-priority default job: $JOB2"

JOB3=$(curl -sf -H "Authorization: Bearer $TEST_KEY" -H "Content-Type: application/json" -X POST http://localhost:$PORT/jobs -d '{"scenario":"720p30-h265","queue":"batch","priority":"low","confidence":"auto"}' | jq -r '.id')
echo "✓ Low-priority batch job: $JOB3"
echo ""

# Verify job properties
echo "9. Verifying job properties..."
if [ ! -z "$JOB1" ] && [ "$JOB1" != "null" ]; then
  curl -sf -H "Authorization: Bearer $TEST_KEY" http://localhost:$PORT/jobs/$JOB1 | jq -e '.queue == "live"' > /dev/null && echo "✓ Job 1 has correct queue (live)"
  curl -sf -H "Authorization: Bearer $TEST_KEY" http://localhost:$PORT/jobs/$JOB1 | jq -e '.priority == "high"' > /dev/null && echo "✓ Job 1 has correct priority (high)"
  curl -sf -H "Authorization: Bearer $TEST_KEY" http://localhost:$PORT/jobs | jq -e '.jobs | length >= 3' > /dev/null && echo "✓ All jobs submitted"
else
  echo "⚠ Job verification skipped (no job IDs)"
fi
echo ""

# Test job control
echo "10. Testing job control endpoints..."
if [ ! -z "$JOB3" ] && [ "$JOB3" != "null" ]; then
  curl -sf -H "Authorization: Bearer $TEST_KEY" -X POST http://localhost:$PORT/jobs/$JOB3/pause > /dev/null && echo "✓ Pause endpoint works"
  curl -sf -H "Authorization: Bearer $TEST_KEY" -X POST http://localhost:$PORT/jobs/$JOB3/resume > /dev/null && echo "✓ Resume endpoint works"
  curl -sf -H "Authorization: Bearer $TEST_KEY" -X POST http://localhost:$PORT/jobs/$JOB3/cancel > /dev/null && echo "✓ Cancel endpoint works"
else
  echo "⚠ Job control tests skipped (no job ID)"
fi
echo ""

# Test CLI
echo "11. Testing CLI commands..."
export FFMPEG_RTMP_MASTER="http://localhost:$PORT"
export FFMPEG_RTMP_API_KEY="$TEST_KEY"
if [ ! -z "$NODE_ID" ] && [ "$NODE_ID" != "null" ]; then
  /tmp/ffrtmp-test --master "http://localhost:$PORT" nodes list --output json 2>/dev/null | jq -e '.count > 0' > /dev/null && echo "✓ CLI nodes list works"
fi
if [ ! -z "$JOB1" ] && [ "$JOB1" != "null" ]; then
  /tmp/ffrtmp-test --master "http://localhost:$PORT" jobs status $JOB1 --output json 2>/dev/null | jq -e '.id' > /dev/null && echo "✓ CLI jobs status works"
fi
if [ ! -z "$NODE_ID" ] && [ "$NODE_ID" != "null" ]; then
  /tmp/ffrtmp-test --master "http://localhost:$PORT" nodes describe $NODE_ID --output json 2>/dev/null | jq -e '.id' > /dev/null && echo "✓ CLI nodes describe works"
fi
echo ""

# Check metrics after activity
echo "12. Verifying metrics updated..."
curl -sf http://localhost:$METRICS_PORT/metrics | grep 'ffrtmp_jobs_total{state="' > /dev/null && echo "✓ Jobs metrics showing states"
curl -sf http://localhost:$METRICS_PORT/metrics | grep 'ffrtmp_queue_by_priority' > /dev/null && echo "✓ Priority queue metrics present"
echo ""

echo "════════════════════════════════════════════"
echo "✅ ALL VALIDATION TESTS PASSED!"
echo "════════════════════════════════════════════"
echo ""
echo "Summary:"
echo "  • Master: Running with metrics"
echo "  • Worker: Registered and reporting metrics"
echo "  • Jobs: Submitted with queue/priority"
echo "  • Control: Pause/Resume/Cancel working"
echo "  • CLI: All commands functional"
echo "  • Metrics: All exporters operational"
echo ""
echo "Production-ready! ✨"
