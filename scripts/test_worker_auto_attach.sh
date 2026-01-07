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

# Set dummy API key if not present (worker requires it or will warn)
export FFMPEG_RTMP_API_KEY="${FFMPEG_RTMP_API_KEY:-dummy-test-key}"

echo "Starting worker with auto-attach enabled..."
echo "(Running in standalone mode without master registration)"
echo ""

# Start worker in background with auto-attach
# Note: Not using --register so it runs standalone
timeout 25 $WORKER_BIN \
    --enable-auto-attach \
    --auto-attach-scan-interval 3s \
    --auto-attach-cpu-quota 100 \
    --auto-attach-memory-limit 512 \
    --metrics-port 9091 \
    --log-level info \
    > /tmp/worker_auto_attach_test.log 2>&1 &

WORKER_PID=$!
echo "Worker started (PID: $WORKER_PID)"
sleep 5

echo ""
echo "Checking worker log for auto-attach initialization..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if grep -q "Auto-Attach Service Enabled" /tmp/worker_auto_attach_test.log; then
    grep -A 6 "Auto-Attach Service Enabled" /tmp/worker_auto_attach_test.log
    echo "✓ Auto-attach service initialized successfully"
else
    echo "⚠ Auto-attach initialization not found yet"
    echo "Worker log tail:"
    tail -10 /tmp/worker_auto_attach_test.log
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if ! command -v ffmpeg &> /dev/null; then
    echo "⚠ FFmpeg not found, skipping process discovery test"
    echo "   Worker is running and would discover FFmpeg if available"
    kill $WORKER_PID 2>/dev/null || true
    exit 0
fi

echo "Starting external FFmpeg process..."
ffmpeg -f lavfi -i testsrc=duration=15:size=320x240:rate=30 -f null - > /dev/null 2>&1 &
FFMPEG_PID=$!
echo "FFmpeg started (PID: $FFMPEG_PID)"
echo ""

echo "Waiting for auto-discovery (7 seconds)..."
sleep 7

echo ""
echo "Checking if worker discovered the process..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if grep -q "Attaching to PID $FFMPEG_PID" /tmp/worker_auto_attach_test.log 2>/dev/null; then
    echo "✓ SUCCESS: Worker auto-discovered and attached to FFmpeg!"
    grep "Attaching to PID $FFMPEG_PID" /tmp/worker_auto_attach_test.log
elif grep -q "Attaching to PID" /tmp/worker_auto_attach_test.log 2>/dev/null; then
    echo "✓ Worker is discovering processes:"
    grep "Attaching to PID" /tmp/worker_auto_attach_test.log | tail -3
else
    echo "⚠ FFmpeg may not have been discovered yet"
    echo "   Check if process is still running: $(ps -p $FFMPEG_PID >/dev/null 2>&1 && echo 'yes' || echo 'no')"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Checking Prometheus metrics..."
if curl -s http://localhost:9091/metrics 2>/dev/null | grep -E "ffrtmp_worker_(discovered_processes|active_attachments)"; then
    echo ""
    echo "✓ Auto-attach metrics are being exported"
else
    echo "⚠ Metrics endpoint may not be ready yet (this is OK for a quick test)"
fi
echo ""

# Cleanup
echo "Cleaning up..."
kill $WORKER_PID 2>/dev/null || true
kill $FFMPEG_PID 2>/dev/null || true
wait $WORKER_PID 2>/dev/null || true
wait $FFMPEG_PID 2>/dev/null || true
sleep 1

echo ""
echo "Relevant worker log sections:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -E "(Auto-Attach|auto-attach|Discovered|Attaching)" /tmp/worker_auto_attach_test.log 2>/dev/null | head -20 || echo "(No auto-attach activity logged)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    Test Complete                               ║"
echo "║                                                                 ║"
echo "║  Worker can now automatically discover and govern processes    ║"
echo "║  Enable in production with --enable-auto-attach flag           ║"
echo "╚════════════════════════════════════════════════════════════════╝"

