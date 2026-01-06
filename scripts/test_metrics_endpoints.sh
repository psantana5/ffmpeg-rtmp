#!/bin/bash
# Test Prometheus metrics endpoints

set -e

echo "=========================================="
echo "Metrics Endpoints Test"
echo "=========================================="
echo ""

GREEN='\033[0;32m'
NC='\033[0m'

# Start master with metrics
echo "Starting master server..."
./bin/master --tls=false --port=18080 --metrics-port=19090 --db="" > /tmp/metrics_master.log 2>&1 &
MASTER_PID=$!
sleep 3

# Start worker with metrics
echo "Starting worker agent..."
timeout 30 ./bin/agent --metrics-port=19091 > /tmp/metrics_worker.log 2>&1 &
WORKER_PID=$!
sleep 2

cleanup() {
    echo ""
    echo "Cleaning up..."
    kill $MASTER_PID 2>/dev/null || true
    kill $WORKER_PID 2>/dev/null || true
    wait $MASTER_PID 2>/dev/null || true
    wait $WORKER_PID 2>/dev/null || true
}
trap cleanup EXIT

# Test master metrics
echo ""
echo "Testing master metrics endpoint..."
if curl -sf http://localhost:19090/metrics | grep -q "ffmpeg_rtmp"; then
    echo -e "${GREEN}✓${NC} Master /metrics returns Prometheus format"
else
    echo "✗ Master /metrics not working"
fi

if curl -sf http://localhost:19090/health | grep -q "healthy"; then
    echo -e "${GREEN}✓${NC} Master /health returns healthy"
else
    echo "✗ Master /health not working"
fi

# Test worker metrics
echo ""
echo "Testing worker metrics endpoint..."
if curl -sf http://localhost:19091/metrics | grep -q "worker"; then
    echo -e "${GREEN}✓${NC} Worker /metrics returns Prometheus format"
else
    echo "✗ Worker /metrics not working"
fi

if curl -sf http://localhost:19091/health | grep -q "healthy"; then
    echo -e "${GREEN}✓${NC} Worker /health returns healthy"
else
    echo "✗ Worker /health not working"
fi

# Test worker readiness
echo ""
echo "Testing worker readiness endpoint..."
READY_RESPONSE=$(curl -sf http://localhost:19091/ready || echo "{}")
if echo "$READY_RESPONSE" | grep -q "ffmpeg"; then
    echo -e "${GREEN}✓${NC} Worker /ready returns readiness checks"
else
    echo "✗ Worker /ready not working"
fi

# Test wrapper metrics
echo ""
echo "Testing wrapper metrics endpoint..."
if curl -sf http://localhost:19091/wrapper/metrics 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Worker /wrapper/metrics accessible"
else
    echo "✗ Worker /wrapper/metrics not accessible"
fi

# Test violations endpoint
echo ""
echo "Testing violations endpoint..."
if curl -sf http://localhost:19091/violations | grep -q "\[\]"; then
    echo -e "${GREEN}✓${NC} Worker /violations returns JSON array"
else
    echo "✗ Worker /violations not working"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Metrics endpoints test complete!${NC}"
echo "=========================================="
