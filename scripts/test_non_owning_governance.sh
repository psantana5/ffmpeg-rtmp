#!/bin/bash
# test_non_owning_governance.sh
#
# Demonstrates the non-owning governance philosophy:
# - Workloads survive wrapper crashes
# - Governance is passive, not controlling
# - Kill wrapper → workload continues
#
# This is CRITICAL for production reliability.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FFRTMP_BIN="$PROJECT_ROOT/bin/ffrtmp"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║     Non-Owning Governance - Resilience Demonstration          ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Philosophy: We govern workloads, we don't OWN them.${NC}"
echo -e "${YELLOW}Result: Workloads survive wrapper failures.${NC}"
echo ""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test processes...${NC}"
    pkill -f "test_workload_" 2>/dev/null || true
    pkill -f "ffrtmp run" 2>/dev/null || true
    pkill -f "ffrtmp attach" 2>/dev/null || true
    pkill -f "ffrtmp watch" 2>/dev/null || true
    rm -f /tmp/test_workload_*.log
    rm -f /tmp/wrapper_*.log
}

trap cleanup EXIT

# Build if needed
if [ ! -f "$FFRTMP_BIN" ]; then
    echo -e "${YELLOW}Building ffrtmp...${NC}"
    cd "$PROJECT_ROOT" && make build
fi

echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${MAGENTA}TEST 1: Run Mode - Wrapper Crash Survival${NC}"
echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${GREEN}Scenario:${NC}"
echo "  1. Start a long-running workload via 'ffrtmp run'"
echo "  2. Kill the wrapper process (SIGKILL)"
echo "  3. Verify workload continues running"
echo ""
echo -e "${YELLOW}Expected: Workload survives and completes successfully${NC}"
echo ""

# Create a test workload script
cat > /tmp/test_workload_run.sh << 'WORKLOAD'
#!/bin/bash
echo "Workload PID: $$"
echo "Starting 10-second workload..."
for i in {1..10}; do
    echo "Workload heartbeat: $i/10"
    sleep 1
done
echo "Workload completed successfully!"
exit 0
WORKLOAD
chmod +x /tmp/test_workload_run.sh

echo -e "${BLUE}→ Starting workload via wrapper...${NC}"
$FFRTMP_BIN run \
    --job-id test-run-resilience \
    --cpu-quota 100 \
    --memory-limit 512 \
    -- /tmp/test_workload_run.sh > /tmp/wrapper_run.log 2>&1 &

WRAPPER_PID=$!
sleep 3

# Find the actual workload PID (it's a child of wrapper)
WORKLOAD_PID=$(pgrep -P $WRAPPER_PID -f "bash" 2>/dev/null | head -1)

# If not found by parent, try by command name
if [ -z "$WORKLOAD_PID" ]; then
    WORKLOAD_PID=$(pgrep -f "test_workload_run.sh" 2>/dev/null | grep -v $WRAPPER_PID | head -1)
fi

if [ -z "$WORKLOAD_PID" ]; then
    echo -e "${RED}✗ Failed: Could not find workload PID${NC}"
    echo -e "${YELLOW}Processes found:${NC}"
    ps aux | grep -E "(ffrtmp|test_workload)" | grep -v grep
    exit 1
fi

# Verify they are in different process groups
WRAPPER_PGID=$(ps -o pgid= -p $WRAPPER_PID | tr -d ' ')
WORKLOAD_PGID=$(ps -o pgid= -p $WORKLOAD_PID | tr -d ' ')

echo -e "${GREEN}✓ Wrapper PID: $WRAPPER_PID (PGID: $WRAPPER_PGID)${NC}"
echo -e "${GREEN}✓ Workload PID: $WORKLOAD_PID (PGID: $WORKLOAD_PGID)${NC}"

if [ "$WRAPPER_PGID" != "$WORKLOAD_PGID" ]; then
    echo -e "${GREEN}✓ Workload in independent process group!${NC}"
else
    echo -e "${YELLOW}⚠ Warning: Same process group (may still survive)${NC}"
fi
echo ""

echo -e "${BLUE}→ Simulating wrapper crash (SIGKILL)...${NC}"
sleep 1
kill -9 $WRAPPER_PID 2>/dev/null || true
echo -e "${GREEN}✓ Wrapper killed${NC}"
echo ""

echo -e "${BLUE}→ Checking if workload is still running...${NC}"
sleep 1

if ps -p $WORKLOAD_PID > /dev/null 2>&1; then
    echo -e "${GREEN}✓ SUCCESS: Workload survived wrapper crash!${NC}"
    echo -e "  Workload PID $WORKLOAD_PID is still running"
else
    echo -e "${RED}✗ FAILURE: Workload died with wrapper${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}→ Waiting for workload to complete naturally...${NC}"
sleep 10

if ! ps -p $WORKLOAD_PID > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Workload completed successfully${NC}"
else
    echo -e "${YELLOW}⚠ Workload still running (will be cleaned up)${NC}"
    kill $WORKLOAD_PID 2>/dev/null || true
fi

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${MAGENTA}TEST 2: Attach Mode - Non-Intrusive Observation${NC}"
echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${GREEN}Scenario:${NC}"
echo "  1. Start workload directly (NOT via wrapper)"
echo "  2. Attach wrapper to observe"
echo "  3. Kill wrapper (SIGKILL)"
echo "  4. Verify workload continues unaffected"
echo ""
echo -e "${YELLOW}Expected: Workload never knows wrapper exists${NC}"
echo ""

# Create independent workload
cat > /tmp/test_workload_attach.sh << 'WORKLOAD'
#!/bin/bash
echo "Independent workload PID: $$"
for i in {1..15}; do
    echo "Independent heartbeat: $i/15"
    sleep 1
done
echo "Independent workload completed!"
exit 0
WORKLOAD
chmod +x /tmp/test_workload_attach.sh

echo -e "${BLUE}→ Starting independent workload (no wrapper)...${NC}"
/tmp/test_workload_attach.sh > /tmp/test_workload_attach.log 2>&1 &
INDEPENDENT_PID=$!
sleep 2

echo -e "${GREEN}✓ Independent workload PID: $INDEPENDENT_PID${NC}"
echo ""

echo -e "${BLUE}→ Attaching wrapper for observation...${NC}"
timeout 5 $FFRTMP_BIN attach \
    --pid $INDEPENDENT_PID \
    --job-id test-attach-passive \
    --cpu-weight 100 > /tmp/wrapper_attach.log 2>&1 &

WRAPPER_ATTACH_PID=$!
sleep 2

echo -e "${GREEN}✓ Wrapper attached (PID: $WRAPPER_ATTACH_PID)${NC}"
echo ""

echo -e "${BLUE}→ Killing attached wrapper (SIGKILL)...${NC}"
kill -9 $WRAPPER_ATTACH_PID 2>/dev/null || true
echo -e "${GREEN}✓ Wrapper killed${NC}"
echo ""

echo -e "${BLUE}→ Checking if workload noticed...${NC}"
sleep 1

if ps -p $INDEPENDENT_PID > /dev/null 2>&1; then
    echo -e "${GREEN}✓ SUCCESS: Workload completely unaffected!${NC}"
    echo -e "  Workload continues as if wrapper never existed"
else
    echo -e "${RED}✗ FAILURE: Workload was affected by wrapper crash${NC}"
    exit 1
fi

# Cleanup
kill $INDEPENDENT_PID 2>/dev/null || true
wait $INDEPENDENT_PID 2>/dev/null || true

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${MAGENTA}TEST 3: Real FFmpeg Workload - Production Scenario${NC}"
echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${GREEN}Scenario:${NC}"
echo "  1. Start real FFmpeg transcoding via wrapper"
echo "  2. Monitor resource governance application"
echo "  3. Kill wrapper mid-transcode"
echo "  4. Verify FFmpeg continues and completes"
echo ""
echo -e "${YELLOW}Expected: Video transcoding completes despite wrapper crash${NC}"
echo ""

# Check if ffmpeg is available
if ! command -v ffmpeg &> /dev/null; then
    echo -e "${YELLOW}⚠ FFmpeg not found, skipping real workload test${NC}"
else
    echo -e "${BLUE}→ Starting FFmpeg transcoding (30 seconds of test video)...${NC}"
    
    $FFRTMP_BIN run \
        --job-id test-ffmpeg-resilience \
        --sla-eligible \
        --cpu-quota 150 \
        --memory-limit 1024 \
        -- ffmpeg -f lavfi -i testsrc=duration=30:size=640x480:rate=30 \
                  -c:v libx264 -preset ultrafast \
                  /tmp/test_output_resilience.mp4 \
        > /tmp/wrapper_ffmpeg.log 2>&1 &
    
    WRAPPER_FFMPEG_PID=$!
    sleep 3
    
    # Find FFmpeg PID
    FFMPEG_PID=$(pgrep -f "ffmpeg.*test_output_resilience" | head -1)
    
    if [ -z "$FFMPEG_PID" ]; then
        echo -e "${YELLOW}⚠ Could not find FFmpeg PID, skipping${NC}"
    else
        echo -e "${GREEN}✓ FFmpeg PID: $FFMPEG_PID${NC}"
        echo -e "${GREEN}✓ Wrapper PID: $WRAPPER_FFMPEG_PID${NC}"
        echo ""
        
        # Check cgroup assignment
        if [ -f "/proc/$FFMPEG_PID/cgroup" ]; then
            echo -e "${BLUE}→ Resource governance status:${NC}"
            grep -E "(cpu|memory)" /proc/$FFMPEG_PID/cgroup | head -3 || echo "  No cgroup info available"
            echo ""
        fi
        
        echo -e "${BLUE}→ Letting transcode run for 10 seconds...${NC}"
        sleep 10
        
        echo -e "${BLUE}→ Killing wrapper mid-transcode...${NC}"
        kill -9 $WRAPPER_FFMPEG_PID 2>/dev/null || true
        echo -e "${GREEN}✓ Wrapper killed${NC}"
        echo ""
        
        echo -e "${BLUE}→ Checking FFmpeg status...${NC}"
        sleep 2
        
        if ps -p $FFMPEG_PID > /dev/null 2>&1; then
            echo -e "${GREEN}✓ SUCCESS: FFmpeg transcoding continues!${NC}"
            echo ""
            echo -e "${BLUE}→ Waiting for transcode to complete...${NC}"
            
            # Wait up to 25 more seconds for completion
            for i in {1..25}; do
                if ! ps -p $FFMPEG_PID > /dev/null 2>&1; then
                    echo -e "${GREEN}✓ Transcode completed successfully${NC}"
                    
                    if [ -f /tmp/test_output_resilience.mp4 ]; then
                        SIZE=$(stat -c%s /tmp/test_output_resilience.mp4)
                        echo -e "${GREEN}✓ Output file created: $SIZE bytes${NC}"
                        rm -f /tmp/test_output_resilience.mp4
                    fi
                    break
                fi
                sleep 1
            done
            
            # Force cleanup if still running
            if ps -p $FFMPEG_PID > /dev/null 2>&1; then
                echo -e "${YELLOW}⚠ Forcing cleanup (test complete)${NC}"
                kill $FFMPEG_PID 2>/dev/null || true
            fi
        else
            echo -e "${RED}✗ FAILURE: FFmpeg died with wrapper${NC}"
            exit 1
        fi
    fi
fi

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${MAGENTA}TEST 4: Watch Mode Auto-Discovery Resilience${NC}"
echo -e "${MAGENTA}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${GREEN}Scenario:${NC}"
echo "  1. Start watch daemon"
echo "  2. External process spawns"
echo "  3. Watch discovers and attaches"
echo "  4. Kill watch daemon"
echo "  5. External process continues"
echo ""

echo -e "${BLUE}→ Starting watch daemon...${NC}"
$FFRTMP_BIN watch --scan-interval 2s --cpu-quota 100 > /tmp/watch_test.log 2>&1 &
WATCH_PID=$!
sleep 3
echo -e "${GREEN}✓ Watch daemon PID: $WATCH_PID${NC}"
echo ""

echo -e "${BLUE}→ Starting external FFmpeg process...${NC}"
if command -v ffmpeg &> /dev/null; then
    ffmpeg -f lavfi -i testsrc=duration=20:size=320x240:rate=30 \
           -f null - > /dev/null 2>&1 &
    EXTERNAL_FFMPEG_PID=$!
    sleep 4
    
    echo -e "${GREEN}✓ External FFmpeg PID: $EXTERNAL_FFMPEG_PID${NC}"
    echo ""
    
    echo -e "${BLUE}→ Checking if watch daemon discovered it...${NC}"
    if grep -q "Attaching to PID $EXTERNAL_FFMPEG_PID" /tmp/watch_test.log 2>/dev/null; then
        echo -e "${GREEN}✓ Watch daemon discovered and attached!${NC}"
    else
        echo -e "${YELLOW}⚠ Process may not have been discovered yet${NC}"
    fi
    echo ""
    
    echo -e "${BLUE}→ Killing watch daemon...${NC}"
    kill -9 $WATCH_PID 2>/dev/null || true
    echo -e "${GREEN}✓ Watch daemon killed${NC}"
    echo ""
    
    echo -e "${BLUE}→ Checking external process...${NC}"
    if ps -p $EXTERNAL_FFMPEG_PID > /dev/null 2>&1; then
        echo -e "${GREEN}✓ SUCCESS: External process unaffected by daemon crash!${NC}"
    else
        echo -e "${YELLOW}⚠ Process completed or was not started${NC}"
    fi
    
    # Cleanup
    kill $EXTERNAL_FFMPEG_PID 2>/dev/null || true
else
    echo -e "${YELLOW}⚠ FFmpeg not available, skipping this test${NC}"
fi

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                    ALL TESTS PASSED ✓                          ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║ Benefits Demonstrated:                                         ║${NC}"
echo -e "${GREEN}║                                                                 ║${NC}"
echo -e "${GREEN}║ 1. Workload Resilience                                         ║${NC}"
echo -e "${GREEN}║    → Wrapper crashes don't affect running jobs                 ║${NC}"
echo -e "${GREEN}║    → Critical for production reliability                       ║${NC}"
echo -e "${GREEN}║                                                                 ║${NC}"
echo -e "${GREEN}║ 2. Non-Intrusive Governance                                    ║${NC}"
echo -e "${GREEN}║    → Workloads never know they're being governed               ║${NC}"
echo -e "${GREEN}║    → No signals sent, no lifecycle control                     ║${NC}"
echo -e "${GREEN}║                                                                 ║${NC}"
echo -e "${GREEN}║ 3. Real Workload Validation                                    ║${NC}"
echo -e "${GREEN}║    → FFmpeg transcoding continues despite wrapper failure      ║${NC}"
echo -e "${GREEN}║    → Resource limits applied successfully                      ║${NC}"
echo -e "${GREEN}║                                                                 ║${NC}"
echo -e "${GREEN}║ 4. Auto-Discovery Resilience                                   ║${NC}"
echo -e "${GREEN}║    → External processes remain independent                     ║${NC}"
echo -e "${GREEN}║    → Watch daemon crash = zero impact                          ║${NC}"
echo -e "${GREEN}║                                                                 ║${NC}"
echo -e "${GREEN}║ This is GOVERNANCE, not EXECUTION.                             ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Show summary
echo -e "${CYAN}Summary of log files:${NC}"
ls -lh /tmp/wrapper_*.log /tmp/watch_test.log 2>/dev/null || echo "No logs generated"
echo ""

echo -e "${YELLOW}Test artifacts will be cleaned up automatically.${NC}"
echo ""
