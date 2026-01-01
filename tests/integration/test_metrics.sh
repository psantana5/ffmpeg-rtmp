#!/bin/bash
# Test Prometheus Metrics Endpoints

set -e

PORT=8097
METRICS_PORT=9097
WORKER_METRICS_PORT=9098

echo "================================="
echo "Testing Prometheus Metrics"
echo "================================="
echo ""

# Start master
echo "Starting master on port $PORT with metrics on $METRICS_PORT..."
./bin/master --port $PORT --metrics-port $METRICS_PORT --tls=false --db="" &
MASTER_PID=$!
sleep 3

# Test master metrics endpoint
echo "Testing master metrics endpoint..."
MASTER_METRICS=$(curl -s http://localhost:$METRICS_PORT/metrics)

if echo "$MASTER_METRICS" | grep -q "ffrtmp_jobs_total"; then
    echo "✓ Master metrics: ffrtmp_jobs_total found"
else
    echo "✗ Master metrics: ffrtmp_jobs_total NOT found"
fi

if echo "$MASTER_METRICS" | grep -q "ffrtmp_active_jobs"; then
    echo "✓ Master metrics: ffrtmp_active_jobs found"
else
    echo "✗ Master metrics: ffrtmp_active_jobs NOT found"
fi

if echo "$MASTER_METRICS" | grep -q "ffrtmp_queue_length"; then
    echo "✓ Master metrics: ffrtmp_queue_length found"
else
    echo "✗ Master metrics: ffrtmp_queue_length NOT found"
fi

if echo "$MASTER_METRICS" | grep -q "ffrtmp_schedule_attempts_total"; then
    echo "✓ Master metrics: ffrtmp_schedule_attempts_total found"
else
    echo "✗ Master metrics: ffrtmp_schedule_attempts_total NOT found"
fi

echo ""
echo "Master metrics sample:"
echo "$MASTER_METRICS" | head -20

# Cleanup
echo ""
echo "Cleaning up..."
kill $MASTER_PID 2>/dev/null || true
sleep 1

echo ""
echo "✅ Metrics test complete!"
