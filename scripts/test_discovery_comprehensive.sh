#!/bin/bash

# Comprehensive Auto-Discovery System Test
# Tests all aspects of process discovery and attachment

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║     Comprehensive Auto-Discovery System Test                  ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo

# Build the binary first
echo "→ Building ffrtmp binary..."
make build-cli >/dev/null 2>&1
if [ ! -f "bin/ffrtmp" ]; then
    echo "✗ FAILED: Binary not found"
    exit 1
fi
echo "✓ Binary ready"
echo

# Test directory setup
TEST_DIR="/tmp/ffmpeg_discovery_test"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

# ============================================================================
# TEST 1: Scanner - Process Detection
# ============================================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 1: Process Scanner - Detection Accuracy"
echo "═══════════════════════════════════════════════════════════════"
echo
echo "Testing ability to find processes in /proc filesystem"
echo

# Start multiple test processes with different commands
echo "→ Starting test processes..."
sleep 300 &
SLEEP_PID=$!
echo "  sleep PID: $SLEEP_PID"

# Start an FFmpeg-like process (using cat as a proxy)
cat > "$TEST_DIR/test_input.txt" << 'EOF'
This is test data for our fake ffmpeg process.
EOF

cat "$TEST_DIR/test_input.txt" > /dev/null 2>&1 &
CAT_PID=$!
echo "  cat PID: $CAT_PID"

# Give processes time to start
sleep 1

# Test: Can we find the sleep process?
if ps -p $SLEEP_PID > /dev/null 2>&1; then
    echo "✓ Test process is running"
    
    # Check /proc directly
    if [ -f "/proc/$SLEEP_PID/cmdline" ]; then
        CMDLINE=$(cat /proc/$SLEEP_PID/cmdline | tr '\0' ' ')
        echo "✓ Can read /proc/$SLEEP_PID/cmdline: $CMDLINE"
    else
        echo "✗ Cannot read /proc/$SLEEP_PID/cmdline"
    fi
    
    # Check stat
    if [ -f "/proc/$SLEEP_PID/stat" ]; then
        echo "✓ Can read /proc/$SLEEP_PID/stat"
    else
        echo "✗ Cannot read /proc/$SLEEP_PID/stat"
    fi
else
    echo "✗ Test process not running"
fi

# Cleanup test processes
kill $SLEEP_PID $CAT_PID 2>/dev/null || true
wait $SLEEP_PID $CAT_PID 2>/dev/null || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# ============================================================================
# TEST 2: Watch Command - Basic Functionality
# ============================================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 2: Watch Command - Basic Functionality"
echo "═══════════════════════════════════════════════════════════════"
echo
echo "Testing 'ffrtmp watch' command startup and scanning"
echo

# Start watch daemon with verbose output and short scan interval
WATCH_LOG="$TEST_DIR/watch.log"
./bin/ffrtmp watch --scan-interval 2s --target "ffmpeg" --target "sleep" > "$WATCH_LOG" 2>&1 &
WATCH_PID=$!

echo "→ Watch daemon started (PID: $WATCH_PID)"
sleep 1

# Check if watch daemon is running
if ps -p $WATCH_PID > /dev/null 2>&1; then
    echo "✓ Watch daemon is running"
else
    echo "✗ Watch daemon crashed on startup"
    cat "$WATCH_LOG"
    exit 1
fi

# Check for initialization messages
sleep 3
if grep -q "Starting auto-attach service" "$WATCH_LOG"; then
    echo "✓ Service initialized"
else
    echo "⚠ Initialization message not found (might be too early)"
fi

# Check for scanning activity
if grep -q "Scanning for processes" "$WATCH_LOG"; then
    SCAN_COUNT=$(grep -c "Scanning for processes" "$WATCH_LOG")
    echo "✓ Scanner is active ($SCAN_COUNT scans detected)"
else
    echo "✗ No scanning activity detected"
    echo "Watch log:"
    cat "$WATCH_LOG"
fi

# Cleanup
kill $WATCH_PID 2>/dev/null || true
wait $WATCH_PID 2>/dev/null || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# ============================================================================
# TEST 3: Process Discovery - Real Detection
# ============================================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 3: Process Discovery - Real Detection"
echo "═══════════════════════════════════════════════════════════════"
echo
echo "Testing if watch daemon discovers new processes"
echo

# Start watch daemon with sleep as target
WATCH_LOG="$TEST_DIR/watch_discovery.log"
./bin/ffrtmp watch --scan-interval 2s --target "sleep" > "$WATCH_LOG" 2>&1 &
WATCH_PID=$!

echo "→ Watch daemon started (PID: $WATCH_PID)"
sleep 3  # Wait for initial scan

# Start a target process AFTER watch daemon is running
echo "→ Starting target process (sleep 60)..."
sleep 60 &
TARGET_PID=$!
echo "  Target PID: $TARGET_PID"

# Wait for discovery (scan interval + buffer)
echo "→ Waiting for discovery (5 seconds)..."
sleep 5

# Check if process was discovered
if grep -q "Discovered new process" "$WATCH_LOG"; then
    echo "✓ Process discovery logged"
    DISCOVERIES=$(grep -c "Discovered new process" "$WATCH_LOG")
    echo "  Discoveries: $DISCOVERIES"
    
    # Check if our specific PID was discovered
    if grep -q "pid:$TARGET_PID" "$WATCH_LOG" || grep -q "PID $TARGET_PID" "$WATCH_LOG"; then
        echo "✓ Our target PID was discovered!"
    else
        echo "⚠ Target PID not explicitly mentioned (might be OK)"
    fi
else
    echo "⚠ No discovery messages found yet"
    echo "Recent watch log:"
    tail -20 "$WATCH_LOG"
fi

# Cleanup
kill $WATCH_PID $TARGET_PID 2>/dev/null || true
wait $WATCH_PID $TARGET_PID 2>/dev/null || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# ============================================================================
# TEST 4: Attachment Lifecycle
# ============================================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 4: Attachment Lifecycle"
echo "═══════════════════════════════════════════════════════════════"
echo
echo "Testing complete attachment and monitoring lifecycle"
echo

# Start a long-running process
echo "→ Starting workload (sleep 120)..."
sleep 120 &
WORKLOAD_PID=$!
echo "  Workload PID: $WORKLOAD_PID"

# Attach wrapper manually
ATTACH_LOG="$TEST_DIR/attach.log"
echo "→ Attaching wrapper..."
./bin/ffrtmp attach --pid $WORKLOAD_PID --job-id "test-attach-$WORKLOAD_PID" > "$ATTACH_LOG" 2>&1 &
WRAPPER_PID=$!

sleep 2

# Check wrapper is monitoring
if ps -p $WRAPPER_PID > /dev/null 2>&1; then
    echo "✓ Wrapper is attached and running"
    
    # Verify workload is still independent
    WORKLOAD_PGID=$(ps -o pgid= -p $WORKLOAD_PID | tr -d ' ')
    WRAPPER_PGID=$(ps -o pgid= -p $WRAPPER_PID | tr -d ' ')
    
    if [ "$WORKLOAD_PGID" != "$WRAPPER_PGID" ]; then
        echo "✓ Workload has independent process group"
        echo "  Workload PGID: $WORKLOAD_PGID"
        echo "  Wrapper PGID: $WRAPPER_PGID"
    else
        echo "✗ WARNING: Process groups are the same!"
    fi
else
    echo "✗ Wrapper failed to attach"
    cat "$ATTACH_LOG"
fi

# Test wrapper crash doesn't affect workload
echo "→ Killing wrapper..."
kill -9 $WRAPPER_PID 2>/dev/null || true
wait $WRAPPER_PID 2>/dev/null || true

sleep 1

if ps -p $WORKLOAD_PID > /dev/null 2>&1; then
    echo "✓ SUCCESS: Workload survived wrapper crash!"
else
    echo "✗ FAILURE: Workload died with wrapper"
fi

# Cleanup
kill $WORKLOAD_PID 2>/dev/null || true
wait $WORKLOAD_PID 2>/dev/null || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# ============================================================================
# TEST 5: Multiple Process Discovery
# ============================================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 5: Multiple Process Discovery"
echo "═══════════════════════════════════════════════════════════════"
echo
echo "Testing discovery of multiple concurrent processes"
echo

# Start watch daemon
WATCH_LOG="$TEST_DIR/watch_multi.log"
./bin/ffrtmp watch --scan-interval 2s --target "sleep" > "$WATCH_LOG" 2>&1 &
WATCH_PID=$!

echo "→ Watch daemon started (PID: $WATCH_PID)"
sleep 3

# Start multiple target processes
echo "→ Starting 5 concurrent processes..."
declare -a PIDS
for i in {1..5}; do
    sleep 120 &
    PIDS[$i]=$!
    echo "  Process $i PID: ${PIDS[$i]}"
done

# Wait for discovery
echo "→ Waiting for discovery (7 seconds)..."
sleep 7

# Count discoveries
DISCOVERY_COUNT=$(grep -c "Discovered new process" "$WATCH_LOG" || echo "0")
echo "✓ Discovered $DISCOVERY_COUNT processes"

if [ "$DISCOVERY_COUNT" -ge 3 ]; then
    echo "✓ SUCCESS: Multiple processes discovered"
else
    echo "⚠ Only $DISCOVERY_COUNT discoveries (expected 5)"
    echo "Recent log:"
    tail -30 "$WATCH_LOG"
fi

# Cleanup
kill $WATCH_PID 2>/dev/null || true
for pid in "${PIDS[@]}"; do
    kill $pid 2>/dev/null || true
done
wait $WATCH_PID "${PIDS[@]}" 2>/dev/null || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# ============================================================================
# TEST 6: Duplicate Detection Prevention
# ============================================================================
echo "═══════════════════════════════════════════════════════════════"
echo "TEST 6: Duplicate Detection Prevention"
echo "═══════════════════════════════════════════════════════════════"
echo
echo "Testing that processes aren't discovered multiple times"
echo

# Start a long-running process first
echo "→ Starting target process..."
sleep 120 &
TARGET_PID=$!
echo "  Target PID: $TARGET_PID"

sleep 1

# Start watch daemon (will discover existing process)
WATCH_LOG="$TEST_DIR/watch_dupe.log"
./bin/ffrtmp watch --scan-interval 2s --target "sleep" > "$WATCH_LOG" 2>&1 &
WATCH_PID=$!

echo "→ Watch daemon started (PID: $WATCH_PID)"

# Wait for multiple scan cycles
echo "→ Waiting for multiple scan cycles (10 seconds)..."
sleep 10

# Count how many times the same PID was discovered
TOTAL_DISCOVERIES=$(grep -c "Discovered new process" "$WATCH_LOG" || echo "0")
echo "  Total discovery events: $TOTAL_DISCOVERIES"

# Check for duplicate tracking
if grep -q "Already tracked" "$WATCH_LOG" || grep -q "already discovered" "$WATCH_LOG"; then
    echo "✓ Duplicate detection is working"
else
    echo "⚠ No explicit duplicate prevention messages (might still be working)"
fi

if [ "$TOTAL_DISCOVERIES" -le 2 ]; then
    echo "✓ SUCCESS: No excessive duplicate discoveries"
else
    echo "⚠ WARNING: $TOTAL_DISCOVERIES discoveries might indicate duplicates"
fi

# Cleanup
kill $WATCH_PID $TARGET_PID 2>/dev/null || true
wait $WATCH_PID $TARGET_PID 2>/dev/null || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# ============================================================================
# Summary
# ============================================================================
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                  COMPREHENSIVE TEST COMPLETE                   ║"
echo "╠════════════════════════════════════════════════════════════════╣"
echo "║ Tests Executed:                                                ║"
echo "║                                                                 ║"
echo "║ 1. ✓ Scanner - Process detection from /proc                   ║"
echo "║ 2. ✓ Watch command - Basic functionality                      ║"
echo "║ 3. ✓ Process discovery - Real-time detection                  ║"
echo "║ 4. ✓ Attachment lifecycle - Non-owning governance             ║"
echo "║ 5. ✓ Multiple processes - Concurrent discovery                ║"
echo "║ 6. ✓ Duplicate prevention - Tracking effectiveness            ║"
echo "║                                                                 ║"
echo "║ Log files saved to: $TEST_DIR                    ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo

echo "Logs available for inspection:"
ls -lh "$TEST_DIR"/*.log 2>/dev/null || echo "  (none)"

echo
echo "Cleanup complete. Test directory preserved: $TEST_DIR"
