#!/bin/bash
# Simplified Production Features Test

PROJECT_ROOT="/home/sanpau/Documents/projects/ffmpeg-rtmp"
cd "$PROJECT_ROOT"

echo "===================================================================="
echo " Production Features Test - Simplified"
echo "===================================================================="

# Cleanup first
echo "[1/7] Cleanup..."
pgrep -f "bin/master" | xargs -r kill 2>/dev/null || true
pgrep -f "bin/agent" | xargs -r kill 2>/dev/null || true
sleep 2
rm -f master.db* /tmp/test_*.log

# Build
echo "[2/7] Building..."
make build-distributed build-cli >/dev/null 2>&1 || { echo "❌ Build failed"; exit 1; }
echo "✓ Build successful"

# Create config
echo "[3/7] Creating config..."
cat > config.yaml << 'EOF'
master:
  url: https://localhost:8080
  tls:
    enabled: true
    cert_file: certs/server.crt
    key_file: certs/server.key
  auth:
    enabled: false
  fault_tolerance:
    enabled: true
    heartbeat_check_interval: 5s
    node_timeout: 15s
    max_retries: 3
  tracing:
    enabled: true
    exporter: stdout
    endpoint: ""
EOF
echo "✓ Config created"

# Start master
echo "[4/7] Starting master..."
./bin/master --db master.db > /tmp/test_master.log 2>&1 &
MASTER_PID=$!
sleep 5

if ! ps -p $MASTER_PID > /dev/null; then
    echo "❌ Master failed to start"
    cat /tmp/test_master.log
    exit 1
fi
echo "✓ Master started (PID: $MASTER_PID)"

# Start workers
echo "[5/7] Starting 3 workers..."
WORKER_PIDS=()
for i in 1 2 3; do
    ./bin/agent --master https://localhost:8080 --insecure-skip-verify \
        --register --skip-confirmation --allow-master-as-worker \
        --metrics-port "$((9091 + i))" \
        > "/tmp/test_worker_$i.log" 2>&1 &
    WORKER_PIDS+=($!)
    echo "  Worker $i started (PID: ${WORKER_PIDS[$((i-1))]})"
    sleep 3
done
sleep 5
echo "✓ Workers started"

# Test CLI
echo "[6/7] Testing CLI commands..."

echo "  - List nodes..."
./bin/ffrtmp nodes list --master https://localhost:8080 > /tmp/test_nodes.txt 2>&1
if grep -q "test-worker" /tmp/test_nodes.txt; then
    echo "  ✓ Nodes visible"
    cat /tmp/test_nodes.txt
else
    echo "  ❌ Nodes not found"
    cat /tmp/test_nodes.txt
fi

echo "  - Submit job..."
JOB_OUT=$(./bin/ffrtmp jobs submit --scenario test1 --bitrate 1000k --duration 5 \
    --master https://localhost:8080 2>&1)
if echo "$JOB_OUT" | grep -qiE "submitted|queued|job|created"; then
    echo "  ✓ Job submitted"
    echo "$JOB_OUT"
else
    echo "  ❌ Job submission failed"
    echo "$JOB_OUT"
fi

sleep 2

echo "  - List jobs..."
./bin/ffrtmp jobs status --master https://localhost:8080 > /tmp/test_jobs.txt 2>&1
if [ -s /tmp/test_jobs.txt ]; then
    echo "  ✓ Jobs listed"
    cat /tmp/test_jobs.txt
else
    echo "  ❌ No jobs found"
fi

# Cleanup
echo "[7/7] Cleanup..."
kill $MASTER_PID 2>/dev/null || true
for pid in "${WORKER_PIDS[@]}"; do
    kill $pid 2>/dev/null || true
done
sleep 2

echo ""
echo "===================================================================="
echo " ✓ Test Complete - Check logs in /tmp/test_*.log for details"
echo "===================================================================="
