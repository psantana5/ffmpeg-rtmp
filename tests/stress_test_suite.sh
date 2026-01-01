#!/bin/bash
# Comprehensive stress test suite for FFmpeg RTMP project
# Tests various scenarios to identify potential issues

set -e

MASTER_URL="https://localhost:8080"
LOG_DIR="./test_results/stress_tests_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$LOG_DIR"

echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   FFmpeg RTMP Comprehensive Stress Test Suite            ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""
echo "Test Results Directory: $LOG_DIR"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0
TOTAL=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    local expected_duration="$3"
    
    TOTAL=$((TOTAL + 1))
    echo ""
    echo "═══════════════════════════════════════════════════════════"
    echo "TEST $TOTAL: $test_name"
    echo "═══════════════════════════════════════════════════════════"
    echo "Command: $test_cmd"
    echo "Expected Duration: ~${expected_duration}s"
    echo ""
    
    local log_file="$LOG_DIR/test_${TOTAL}_$(echo $test_name | tr ' ' '_' | tr '[:upper:]' '[:lower:]').log"
    local start_time=$(date +%s)
    
    if eval "$test_cmd" > "$log_file" 2>&1; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        echo -e "${GREEN}✓ PASSED${NC} (${duration}s)"
        PASSED=$((PASSED + 1))
        echo "PASSED: ${duration}s" >> "$log_file"
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        echo -e "${RED}✗ FAILED${NC} (${duration}s)"
        FAILED=$((FAILED + 1))
        echo "FAILED: ${duration}s" >> "$log_file"
        echo "Error log: $log_file"
        tail -20 "$log_file"
    fi
}

# Function to check system resources
check_resources() {
    echo ""
    echo "═══════════════════════════════════════════════════════════"
    echo "System Resources Check"
    echo "═══════════════════════════════════════════════════════════"
    
    # CPU usage
    local cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)
    echo "CPU Usage: ${cpu_usage}%"
    
    # Memory usage
    local mem_info=$(free -h | grep Mem)
    echo "Memory: $mem_info"
    
    # Disk space
    local disk_usage=$(df -h . | tail -1 | awk '{print $5}')
    echo "Disk Usage: $disk_usage"
    
    # Process count
    local master_procs=$(pgrep -f "bin/master" | wc -l)
    local agent_procs=$(pgrep -f "bin/agent" | wc -l)
    echo "Master Processes: $master_procs"
    echo "Agent Processes: $agent_procs"
}

# Initial resource check
check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 1: Basic Functionality Tests                     ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Test 1: Basic FFmpeg job (short)
run_test "FFmpeg Basic Short Job (10s)" \
    "./bin/ffrtmp jobs submit --scenario test-ffmpeg-10s --duration 10 --bitrate 2000k --engine ffmpeg --master $MASTER_URL" \
    15

sleep 2

# Test 2: Basic GStreamer job (short)
run_test "GStreamer Basic Short Job (10s)" \
    "./bin/ffrtmp jobs submit --scenario test-gstreamer-10s --duration 10 --bitrate 2000k --engine gstreamer --master $MASTER_URL" \
    15

sleep 2

# Test 3: Input generation test
run_test "Input Generation Test" \
    "./bin/ffrtmp jobs submit --scenario test-input-gen --duration 5 --resolution_width 1280 --resolution_height 720 --frame_rate 30 --master $MASTER_URL" \
    12

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 2: Duration Variance Tests                       ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Test 4: Very short duration (edge case)
run_test "Very Short Duration FFmpeg (2s)" \
    "./bin/ffrtmp jobs submit --scenario test-short-2s --duration 2 --bitrate 1000k --master $MASTER_URL" \
    8

sleep 2

# Test 5: Medium duration
run_test "Medium Duration FFmpeg (30s)" \
    "./bin/ffrtmp jobs submit --scenario test-medium-30s --duration 30 --bitrate 3000k --master $MASTER_URL" \
    40

sleep 2

# Test 6: Long duration
run_test "Long Duration FFmpeg (60s)" \
    "./bin/ffrtmp jobs submit --scenario test-long-60s --duration 60 --bitrate 4000k --master $MASTER_URL" \
    75

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 3: Resolution Variance Tests                     ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Test 7: 480p
run_test "480p Resolution (10s)" \
    "./bin/ffrtmp jobs submit --scenario test-480p --duration 10 --resolution_width 854 --resolution_height 480 --bitrate 1500k --master $MASTER_URL" \
    15

sleep 2

# Test 8: 720p
run_test "720p Resolution (10s)" \
    "./bin/ffrtmp jobs submit --scenario test-720p --duration 10 --resolution_width 1280 --resolution_height 720 --bitrate 2500k --master $MASTER_URL" \
    18

sleep 2

# Test 9: 1080p
run_test "1080p Resolution (10s)" \
    "./bin/ffrtmp jobs submit --scenario test-1080p --duration 10 --resolution_width 1920 --resolution_height 1080 --bitrate 5000k --master $MASTER_URL" \
    22

sleep 2

# Test 10: 4K (stress test)
run_test "4K Resolution (10s)" \
    "./bin/ffrtmp jobs submit --scenario test-4k --duration 10 --resolution_width 3840 --resolution_height 2160 --bitrate 15000k --master $MASTER_URL" \
    30

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 4: Bitrate Variance Tests                        ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Test 11: Low bitrate
run_test "Low Bitrate (500k, 10s)" \
    "./bin/ffrtmp jobs submit --scenario test-lowbitrate --duration 10 --bitrate 500k --master $MASTER_URL" \
    15

sleep 2

# Test 12: High bitrate
run_test "High Bitrate (20000k, 10s)" \
    "./bin/ffrtmp jobs submit --scenario test-highbitrate --duration 10 --bitrate 20000k --master $MASTER_URL" \
    20

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 5: Edge Cases & Error Handling                   ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Test 13: Missing duration (should use default or fail gracefully)
run_test "Missing Duration Parameter" \
    "./bin/ffrtmp jobs submit --scenario test-no-duration --bitrate 2000k --master $MASTER_URL" \
    620  # 10 minute default timeout

sleep 2

# Test 14: Invalid bitrate format
run_test "Invalid Bitrate Format" \
    "./bin/ffrtmp jobs submit --scenario test-invalid-bitrate --duration 5 --bitrate invalid --master $MASTER_URL" \
    10

sleep 2

# Test 15: Zero duration (edge case)
run_test "Zero Duration (should fail or use default)" \
    "./bin/ffrtmp jobs submit --scenario test-zero-duration --duration 0 --bitrate 2000k --master $MASTER_URL" \
    15

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 6: Concurrent Jobs Test                          ║"
echo "╚═══════════════════════════════════════════════════════════╝"

echo "Submitting 3 concurrent jobs..."

# Test 16: Concurrent jobs
./bin/ffrtmp jobs submit --scenario concurrent-1 --duration 15 --bitrate 2000k --master $MASTER_URL > "$LOG_DIR/concurrent_1.log" 2>&1 &
PID1=$!

./bin/ffrtmp jobs submit --scenario concurrent-2 --duration 15 --bitrate 2000k --master $MASTER_URL > "$LOG_DIR/concurrent_2.log" 2>&1 &
PID2=$!

./bin/ffrtmp jobs submit --scenario concurrent-3 --duration 15 --bitrate 2000k --master $MASTER_URL > "$LOG_DIR/concurrent_3.log" 2>&1 &
PID3=$!

echo "Waiting for concurrent jobs to complete..."
wait $PID1 && echo "Job 1: PASSED" || echo "Job 1: FAILED"
wait $PID2 && echo "Job 2: PASSED" || echo "Job 2: FAILED"
wait $PID3 && echo "Job 3: PASSED" || echo "Job 3: FAILED"

TOTAL=$((TOTAL + 1))

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 7: GStreamer Specific Tests                      ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Test 17: GStreamer medium duration
run_test "GStreamer Medium Duration (30s)" \
    "./bin/ffrtmp jobs submit --scenario gst-medium-30s --duration 30 --bitrate 2000k --engine gstreamer --master $MASTER_URL" \
    40

sleep 2

# Test 18: GStreamer high bitrate
run_test "GStreamer High Bitrate (10000k, 10s)" \
    "./bin/ffrtmp jobs submit --scenario gst-highbitrate --duration 10 --bitrate 10000k --engine gstreamer --master $MASTER_URL" \
    18

check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   PHASE 8: Performance Monitoring                        ║"
echo "╚═══════════════════════════════════════════════════════════╝"

# Check for memory leaks
echo "Checking for potential memory leaks..."
ps aux | grep -E "(master|agent)" | grep -v grep > "$LOG_DIR/process_memory.log"
cat "$LOG_DIR/process_memory.log"

# Check log sizes
echo ""
echo "Log file sizes:"
du -h logs/*.log 2>/dev/null || echo "No logs found"

# Final resource check
check_resources

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   Test Summary                                            ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""
echo "Total Tests:  $TOTAL"
echo -e "${GREEN}Passed:       $PASSED${NC}"
echo -e "${RED}Failed:       $FAILED${NC}"
echo ""
echo "Success Rate: $(( PASSED * 100 / TOTAL ))%"
echo ""
echo "Results saved to: $LOG_DIR"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Some tests failed. Check logs for details.${NC}"
    exit 1
else
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
fi
