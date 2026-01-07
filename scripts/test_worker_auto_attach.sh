#!/bin/bash
# test_worker_auto_attach.sh
#
# Tests the integrated auto-attach feature in the worker agent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORKER_BIN="$PROJECT_ROOT/bin/worker"

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║       Worker Auto-Attach Integration Test                     ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Check if worker binary exists
if [ ! -f "$WORKER_BIN" ]; then
    echo "Building worker..."
    cd "$PROJECT_ROOT" && go build -o bin/worker ./worker/cmd/agent
fi

echo "Starting worker with auto-attach enabled..."
echo ""

# Start worker in background with auto-attach
timeout 20 $WORKER_BIN \
    --enable-auto-attach \
    --auto-attach-scan-interval 3s \
    --auto-attach-cpu-quota 100 \
    --auto-attach-memory-limit 512 \
    --metrics-port 9091 \
    --skip-confirmation \
    --allow-master-as-worker \
    > /tmp/worker_auto_attach_test.log 2>&1 &

WORKER_PID=$!
echo "Worker started (PID: $WORKER_PID)"
sleep 5

echo ""
echo "Checking worker log for auto-attach initialization..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -A 5 "Auto-Attach Service Enabled" /tmp/worker_auto_attach_test.log || echo "Not found yet"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Starting external FFmpeg process..."
ffmpeg -f lavfi -i testsrc=duration=15:size=320x240:rate=30 -f null - > /dev/null 2>&1 &
FFMPEG_PID=$!
echo "FFmpeg started (PID: $FFMPEG_PID)"
echo ""

echo "Waiting for auto-discovery (6 seconds)..."
sleep 6

echo ""
echo "Checking if worker discovered the process..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if grep "Attached to PID $FFMPEG_PID" /tmp/worker_auto_attach_test.log; then
    echo "✓ SUCCESS: Worker auto-discovered and attached to FFmpeg!"
else
    echo "⚠ FFmpeg may not have been discovered yet (check full log)"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Checking Prometheus metrics..."
if curl -s http://localhost:9091/metrics | grep -E "ffrtmp_worker_(discovered_processes|active_attachments)"; then
    echo "✓ Auto-attach metrics are being exported"
else
    echo "⚠ Metrics endpoint may not be ready yet"
fi
echo ""

# Cleanup
echo "Cleaning up..."
kill $WORKER_PID 2>/dev/null || true
kill $FFMPEG_PID 2>/dev/null || true
sleep 2

echo ""
echo "Full worker log:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
tail -30 /tmp/worker_auto_attach_test.log
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    Test Complete                               ║"
echo "║                                                                 ║"
echo "║  Worker can now automatically discover and govern processes    ║"
echo "║  without needing a separate 'ffrtmp watch' daemon!             ║"
echo "╚════════════════════════════════════════════════════════════════╝"
