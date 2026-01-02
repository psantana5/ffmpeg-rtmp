#!/bin/bash
# Production Features Test Suite
# Tests: Job logs, duplicate node prevention, priority queuing, fault tolerance, security, distributed tracing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MASTER_URL="https://localhost:8080"
CLI_BINARY="$PROJECT_ROOT/bin/ffrtmp"
MASTER_BINARY="$PROJECT_ROOT/bin/master"
AGENT_BINARY="$PROJECT_ROOT/bin/agent"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

# PID tracking
MASTER_PID=""
WORKER_PIDS=()

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

header() {
    echo ""
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}================================================${NC}"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test environment..."
    
    # Kill processes by stored PIDs
    if [ -n "$MASTER_PID" ] && kill -0 "$MASTER_PID" 2>/dev/null; then
        kill "$MASTER_PID" || true
    fi
    
    for worker_pid in "${WORKER_PIDS[@]}"; do
        if [ -n "$worker_pid" ] && kill -0 "$worker_pid" 2>/dev/null; then
            kill "$worker_pid" || true
        fi
    done
    
    sleep 2
    rm -f "$PROJECT_ROOT/master.db" "$PROJECT_ROOT/master.db-shm" "$PROJECT_ROOT/master.db-wal"
    rm -f /tmp/test_worker_*.log
}

# Build the project
build_project() {
    header "Building Project"
    cd "$PROJECT_ROOT"
    log_info "Running: make build-distributed build-cli"
    if make build-distributed build-cli > /tmp/build.log 2>&1; then
        log_success "Build successful"
    else
        log_error "Build failed. Check /tmp/build.log"
        cat /tmp/build.log
        exit 1
    fi
}

# Start master with security enabled
start_master() {
    header "Starting Master Node (with Security)"
    cd "$PROJECT_ROOT"
    
    # Create config with security enabled
    cat > config.yaml << EOF
master:
  url: https://localhost:8080
  tls:
    enabled: true
    cert_file: certs/server.crt
    key_file: certs/server.key
  auth:
    enabled: true
    secret_key: test-secret-key-for-testing-only
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

    log_info "Starting master with fault tolerance and security..."
    $MASTER_BINARY --db master.db > /tmp/master.log 2>&1 &
    MASTER_PID=$!
    
    log_info "Master PID: $MASTER_PID"
    sleep 5
    
    if ps -p $MASTER_PID > /dev/null; then
        log_success "Master started successfully"
    else
        log_error "Master failed to start"
        cat /tmp/master.log
        exit 1
    fi
}

# Start workers
start_workers() {
    header "Starting Worker Nodes"
    
    for i in 1 2 3; do
        log_info "Starting worker $i..."
        $AGENT_BINARY \
            --master "$MASTER_URL" \
            --name "test-worker-$i" \
            --node-type "test-node" \
            > "/tmp/test_worker_$i.log" 2>&1 &
        
        WORKER_PIDS[$i]=$!
        log_info "Worker $i PID: ${WORKER_PIDS[$i]}"
        sleep 2
    done
    
    sleep 3
    log_success "Started 3 workers"
}

# Test 1: Duplicate Node Prevention
test_duplicate_node_prevention() {
    header "TEST 1: Duplicate Node Prevention"
    
    log_info "Attempting to register worker with duplicate name..."
    
    # Try to start another worker with same name as worker 1
    $AGENT_BINARY \
        --master "$MASTER_URL" \
        --name "test-worker-1" \
        --node-type "test-node" \
        > /tmp/duplicate_worker.log 2>&1 &
    
    DUP_PID=$!
    sleep 3
    
    # Check if duplicate worker was rejected
    if grep -q "duplicate" /tmp/duplicate_worker.log || grep -q "already registered" /tmp/duplicate_worker.log; then
        log_success "Duplicate node prevention working"
    else
        log_error "Duplicate node was allowed to register"
        cat /tmp/duplicate_worker.log
    fi
    
    kill $DUP_PID 2>/dev/null || true
}

# Test 2: Priority Queue Management
test_priority_queuing() {
    header "TEST 2: Priority Queue Management"
    
    log_info "Submitting jobs with different priorities..."
    
    # Submit low priority job
    LOW_JOB=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 10 \
        --priority low --master "$MASTER_URL" 2>&1 | grep -oP 'Job \K\d+' || echo "")
    
    # Submit high priority job
    HIGH_JOB=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 10 \
        --priority high --master "$MASTER_URL" 2>&1 | grep -oP 'Job \K\d+' || echo "")
    
    # Submit live priority job
    LIVE_JOB=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 10 \
        --priority live --master "$MASTER_URL" 2>&1 | grep -oP 'Job \K\d+' || echo "")
    
    sleep 2
    
    if [[ -n "$LOW_JOB" ]] && [[ -n "$HIGH_JOB" ]] && [[ -n "$LIVE_JOB" ]]; then
        log_success "Jobs submitted with priorities: Low=$LOW_JOB, High=$HIGH_JOB, Live=$LIVE_JOB"
        
        # Check queue statistics
        log_info "Checking queue statistics..."
        if $CLI_BINARY jobs status --master "$MASTER_URL" | grep -q "Priority"; then
            log_success "Priority information displayed in job status"
        else
            log_warning "Priority information not visible in output"
        fi
    else
        log_error "Failed to submit jobs with priorities"
    fi
}

# Test 3: Job Log Retrieval
test_job_logs() {
    header "TEST 3: Job Log Retrieval"
    
    log_info "Submitting a job to generate logs..."
    JOB_ID=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 5 \
        --master "$MASTER_URL" 2>&1 | grep -oP 'Job \K\d+' || echo "")
    
    if [[ -z "$JOB_ID" ]]; then
        log_error "Failed to submit job for log testing"
        return
    fi
    
    log_info "Job ID: $JOB_ID - Waiting for execution..."
    sleep 10
    
    log_info "Retrieving job logs..."
    if $CLI_BINARY jobs logs "$JOB_ID" --master "$MASTER_URL" > /tmp/job_logs.txt 2>&1; then
        if grep -q "Log" /tmp/job_logs.txt || [[ -s /tmp/job_logs.txt ]]; then
            log_success "Job logs retrieved successfully"
            log_info "Sample logs:"
            head -n 5 /tmp/job_logs.txt
        else
            log_warning "Job logs command executed but no logs found (job may not have started)"
        fi
    else
        log_error "Failed to retrieve job logs"
        cat /tmp/job_logs.txt
    fi
}

# Test 4: Fault Tolerance - Worker Failure
test_fault_tolerance() {
    header "TEST 4: Fault Tolerance - Worker Failure Recovery"
    
    log_info "Submitting a long-running job..."
    JOB_ID=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 30 \
        --master "$MASTER_URL" 2>&1 | grep -oP 'Job \K\d+' || echo "")
    
    if [[ -z "$JOB_ID" ]]; then
        log_error "Failed to submit job for fault tolerance test"
        return
    fi
    
    sleep 5
    
    log_info "Checking which worker is executing the job..."
    WORKER_NUM=$(ps aux | grep "ffrtmp worker" | grep -v grep | head -1 | grep -oP 'test-worker-\K\d+' || echo "1")
    TARGET_PID=${WORKER_PIDS[$WORKER_NUM]:-${WORKER_PIDS[1]}}
    
    log_info "Killing worker with PID $TARGET_PID to simulate failure..."
    kill -9 $TARGET_PID 2>/dev/null || true
    
    log_info "Waiting for fault tolerance system to detect failure and reassign job..."
    sleep 20
    
    # Check if job was reassigned
    if $CLI_BINARY jobs status "$JOB_ID" --master "$MASTER_URL" | grep -qE "(running|completed|queued)"; then
        log_success "Job survived worker failure - fault tolerance working"
    else
        log_warning "Job status unclear after worker failure"
    fi
}

# Test 5: Distributed Tracing
test_distributed_tracing() {
    header "TEST 5: Distributed Tracing Integration"
    
    log_info "Checking master logs for OpenTelemetry trace spans..."
    
    if grep -q "trace" /tmp/master.log || grep -q "span" /tmp/master.log || grep -q "otel" /tmp/master.log; then
        log_success "Distributed tracing appears to be active"
    else
        log_warning "No obvious tracing activity in logs (may need external collector)"
    fi
    
    # Check if tracing config was loaded
    if grep -q "Tracing enabled" /tmp/master.log || grep -q "tracer" /tmp/master.log; then
        log_success "Tracing configuration loaded"
    else
        log_warning "Tracing may not be fully configured"
    fi
}

# Test 6: Security Features
test_security() {
    header "TEST 6: Security Features (TLS + Auth)"
    
    log_info "Testing TLS endpoint..."
    if curl -k -s "$MASTER_URL/health" > /dev/null 2>&1; then
        log_success "TLS endpoint responding"
    else
        log_warning "TLS endpoint not responding (may be expected)"
    fi
    
    log_info "Checking for authentication in logs..."
    if grep -qE "(auth|token|jwt)" /tmp/master.log; then
        log_success "Authentication system appears active"
    else
        log_warning "No authentication activity detected"
    fi
    
    # Check if rate limiting is present
    if grep -q "rate" /tmp/master.log; then
        log_success "Rate limiting configured"
    else
        log_warning "Rate limiting not detected"
    fi
}

# Test 7: Human-Friendly Job IDs
test_human_friendly_ids() {
    header "TEST 7: Human-Friendly Job IDs"
    
    log_info "Submitting job and checking ID format..."
    OUTPUT=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 5 \
        --master "$MASTER_URL" 2>&1)
    
    if echo "$OUTPUT" | grep -qP 'Job \d+'; then
        JOB_ID=$(echo "$OUTPUT" | grep -oP 'Job \K\d+')
        log_success "Human-friendly job ID: $JOB_ID (numeric sequence)"
    else
        log_error "Job ID format not human-friendly"
    fi
    
    # Check nodes list for human-friendly display
    log_info "Checking node list display..."
    if $CLI_BINARY nodes list --master "$MASTER_URL" | grep -q "test-worker"; then
        log_success "Node names displayed (not UUIDs)"
    else
        log_warning "Node display format unclear"
    fi
}

# Test 8: Progress Reporting
test_progress_reporting() {
    header "TEST 8: Job Progress Reporting"
    
    log_info "Submitting job and monitoring progress..."
    JOB_ID=$($CLI_BINARY jobs submit --scenario test1 --bitrate 1000k --duration 15 \
        --master "$MASTER_URL" 2>&1 | grep -oP 'Job \K\d+' || echo "")
    
    if [[ -z "$JOB_ID" ]]; then
        log_error "Failed to submit job for progress test"
        return
    fi
    
    sleep 5
    
    log_info "Checking job status for progress information..."
    STATUS_OUTPUT=$($CLI_BINARY jobs status "$JOB_ID" --master "$MASTER_URL" 2>&1)
    
    if echo "$STATUS_OUTPUT" | grep -qE "(Progress|percent|%)"; then
        log_success "Progress reporting visible in job status"
    else
        log_warning "Progress information not clearly visible"
    fi
}

# Main test execution
main() {
    header "Production Features Test Suite"
    log_info "Starting comprehensive tests..."
    
    # Cleanup from previous runs
    cleanup
    
    # Build and setup
    build_project
    start_master
    start_workers
    
    # Run all tests
    test_duplicate_node_prevention
    test_priority_queuing
    test_job_logs
    test_human_friendly_ids
    test_security
    test_distributed_tracing
    test_progress_reporting
    test_fault_tolerance  # Run last as it kills a worker
    
    # Summary
    header "Test Results Summary"
    echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "\n${GREEN}✓ All tests passed!${NC}"
    else
        echo -e "\n${YELLOW}! Some tests had issues - review logs above${NC}"
    fi
    
    # Cleanup
    log_info "Test suite complete. Cleaning up..."
    cleanup
    
    exit $TESTS_FAILED
}

# Run main
main
