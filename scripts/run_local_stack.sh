#!/usr/bin/env bash
#
# run_local_stack.sh - Compile, run, and verify FFmpeg RTMP stack locally
# Runs both master and agent on the same host for testing
#
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${PROJECT_ROOT}/bin"
MASTER_PORT="${MASTER_PORT:-8080}"
AGENT_PORT="${AGENT_PORT:-8081}"
MASTER_API_KEY="${MASTER_API_KEY:-$(openssl rand -base64 32)}"
MASTER_PID=""
AGENT_PID=""
CLEANUP_DONE=false

# Log file
LOG_DIR="${PROJECT_ROOT}/logs"
mkdir -p "${LOG_DIR}"
MASTER_LOG="${LOG_DIR}/master.log"
AGENT_LOG="${LOG_DIR}/agent.log"

# Print functions
print_header() {
    echo -e "${BLUE}=====================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}=====================================${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

# Cleanup function
cleanup() {
    if [ "$CLEANUP_DONE" = true ]; then
        return
    fi
    CLEANUP_DONE=true
    
    echo ""
    print_header "Cleaning up..."
    
    if [ -n "$AGENT_PID" ] && kill -0 "$AGENT_PID" 2>/dev/null; then
        print_info "Stopping agent (PID: $AGENT_PID)..."
        kill "$AGENT_PID" 2>/dev/null || true
        wait "$AGENT_PID" 2>/dev/null || true
    fi
    
    if [ -n "$MASTER_PID" ] && kill -0 "$MASTER_PID" 2>/dev/null; then
        print_info "Stopping master (PID: $MASTER_PID)..."
        kill "$MASTER_PID" 2>/dev/null || true
        wait "$MASTER_PID" 2>/dev/null || true
    fi
    
    print_success "Cleanup complete"
}

trap cleanup EXIT INT TERM

# Check prerequisites
check_prerequisites() {
    print_header "Checking Prerequisites"
    
    local missing=0
    
    if ! command -v go &> /dev/null; then
        print_error "Go not found"
        missing=1
    else
        print_success "Go $(go version | awk '{print $3}')"
    fi
    
    if ! command -v python3 &> /dev/null; then
        print_error "Python3 not found"
        missing=1
    else
        print_success "Python $(python3 --version | awk '{print $2}')"
    fi
    
    if ! command -v ffmpeg &> /dev/null; then
        print_error "FFmpeg not found"
        missing=1
    else
        print_success "FFmpeg $(ffmpeg -version | head -n1 | awk '{print $3}')"
    fi
    
    if ! command -v curl &> /dev/null; then
        print_error "curl not found"
        missing=1
    else
        print_success "curl available"
    fi
    
    if [ $missing -eq 1 ]; then
        print_error "Missing prerequisites. Please install the required tools."
        exit 1
    fi
}

# Build binaries
build_binaries() {
    print_header "Building Binaries"
    
    cd "$PROJECT_ROOT"
    
    print_info "Building master..."
    if make build-master > /dev/null 2>&1; then
        print_success "Master binary built: ${BIN_DIR}/master"
    else
        print_error "Failed to build master"
        exit 1
    fi
    
    print_info "Building agent..."
    if make build-agent > /dev/null 2>&1; then
        print_success "Agent binary built: ${BIN_DIR}/agent"
    else
        print_error "Failed to build agent"
        exit 1
    fi
    
    print_info "Building CLI..."
    if make build-cli > /dev/null 2>&1; then
        print_success "CLI binary built: ${BIN_DIR}/ffrtmp"
    else
        print_error "Failed to build CLI"
        exit 1
    fi
}

# Start master
start_master() {
    print_header "Starting Master Node"
    
    cd "$PROJECT_ROOT"
    
    export MASTER_API_KEY
    print_info "API Key: ${MASTER_API_KEY}"
    print_info "Port: ${MASTER_PORT}"
    print_info "Log file: ${MASTER_LOG}"
    
    # Start master in background
    "${BIN_DIR}/master" --port "${MASTER_PORT}" > "${MASTER_LOG}" 2>&1 &
    MASTER_PID=$!
    
    print_info "Master PID: ${MASTER_PID}"
    
    # Wait for master to be ready
    print_info "Waiting for master to be ready..."
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s -k "https://localhost:${MASTER_PORT}/health" > /dev/null 2>&1; then
            print_success "Master is ready"
            return 0
        fi
        
        if ! kill -0 "$MASTER_PID" 2>/dev/null; then
            print_error "Master process died"
            cat "${MASTER_LOG}"
            exit 1
        fi
        
        sleep 1
        attempt=$((attempt + 1))
    done
    
    print_error "Master failed to start within 30 seconds"
    cat "${MASTER_LOG}"
    exit 1
}

# Start agent
start_agent() {
    print_header "Starting Compute Agent"
    
    cd "$PROJECT_ROOT"
    
    export MASTER_API_KEY
    print_info "Registering with master at https://localhost:${MASTER_PORT}"
    print_info "Log file: ${AGENT_LOG}"
    print_info "Using --allow-master-as-worker flag for local testing"
    
    # Start agent in background
    "${BIN_DIR}/agent" --register --master "https://localhost:${MASTER_PORT}" --api-key "${MASTER_API_KEY}" --allow-master-as-worker --skip-confirmation > "${AGENT_LOG}" 2>&1 &
    AGENT_PID=$!
    
    print_info "Agent PID: ${AGENT_PID}"
    
    # Wait for agent to register
    print_info "Waiting for agent to register..."
    sleep 5
    
    if ! kill -0 "$AGENT_PID" 2>/dev/null; then
        print_error "Agent process died"
        cat "${AGENT_LOG}"
        exit 1
    fi
    
    print_success "Agent started successfully"
}

# Verify stack
verify_stack() {
    print_header "Verifying Stack"
    
    # Check master health
    print_info "Checking master health..."
    if curl -s -k "https://localhost:${MASTER_PORT}/health" | grep -q "healthy"; then
        print_success "Master health check passed"
    else
        print_error "Master health check failed"
        return 1
    fi
    
    # Check registered nodes
    print_info "Checking registered nodes..."
    local nodes=$(curl -s -k -H "Authorization: Bearer ${MASTER_API_KEY}" "https://localhost:${MASTER_PORT}/nodes")
    
    if echo "$nodes" | python3 -c "import sys, json; data=json.load(sys.stdin); sys.exit(0 if data.get('count', 0) > 0 else 1)" 2>/dev/null; then
        print_success "Agent registered successfully"
        echo "$nodes" | python3 -m json.tool 2>/dev/null || echo "$nodes"
    else
        print_error "Agent not registered"
        echo "$nodes"
        return 1
    fi
    
    # Check Prometheus metrics
    print_info "Checking Prometheus metrics..."
    if curl -s "http://localhost:9090/metrics" | grep -q "go_"; then
        print_success "Prometheus metrics available"
    else
        print_error "Prometheus metrics not available (this is OK if no jobs have run yet)"
    fi
}

# Submit test job
submit_test_job() {
    print_header "Submitting Test Job"
    
    print_info "Creating test job..."
    local job_response=$(curl -s -k -X POST "https://localhost:${MASTER_PORT}/jobs" \
        -H "Authorization: Bearer ${MASTER_API_KEY}" \
        -H "Content-Type: application/json" \
        -d '{
            "scenario": "test-720p",
            "confidence": "auto",
            "parameters": {
                "duration": 30,
                "bitrate": "2000k",
                "resolution": "1280x720",
                "fps": 30
            }
        }')
    
    if echo "$job_response" | grep -q "job_id"; then
        print_success "Test job submitted"
        echo "$job_response" | python3 -m json.tool 2>/dev/null || echo "$job_response"
        
        local job_id=$(echo "$job_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['job_id'])" 2>/dev/null || echo "")
        
        if [ -n "$job_id" ]; then
            print_info "Job ID: ${job_id}"
            print_info "Monitoring job execution (30 seconds)..."
            
            # Wait and check job status
            sleep 5
            local job_status=$(curl -s -k -H "Authorization: Bearer ${MASTER_API_KEY}" "https://localhost:${MASTER_PORT}/jobs/${job_id}")
            print_info "Job status:"
            echo "$job_status" | python3 -m json.tool 2>/dev/null || echo "$job_status"
        fi
    else
        print_error "Failed to submit test job"
        echo "$job_response"
        return 1
    fi
}

# Display summary
display_summary() {
    print_header "Stack Running Successfully"
    
    echo ""
    echo -e "${GREEN}Master Node:${NC}"
    echo "  - URL: https://localhost:${MASTER_PORT}"
    echo "  - Health: https://localhost:${MASTER_PORT}/health"
    echo "  - Nodes: https://localhost:${MASTER_PORT}/nodes"
    echo "  - Jobs: https://localhost:${MASTER_PORT}/jobs"
    echo "  - PID: ${MASTER_PID}"
    echo "  - Log: ${MASTER_LOG}"
    echo ""
    echo -e "${GREEN}Agent Node:${NC}"
    echo "  - PID: ${AGENT_PID}"
    echo "  - Log: ${AGENT_LOG}"
    echo ""
    echo -e "${GREEN}Monitoring:${NC}"
    echo "  - Prometheus: http://localhost:9090/metrics"
    echo ""
    echo -e "${YELLOW}API Key:${NC} ${MASTER_API_KEY}"
    echo ""
    echo -e "${BLUE}Commands:${NC}"
    echo "  # List nodes"
    echo "  curl -s -k -H \"Authorization: Bearer ${MASTER_API_KEY}\" https://localhost:${MASTER_PORT}/nodes | python3 -m json.tool"
    echo ""
    echo "  # List jobs"
    echo "  curl -s -k -H \"Authorization: Bearer ${MASTER_API_KEY}\" https://localhost:${MASTER_PORT}/jobs | python3 -m json.tool"
    echo ""
    echo "  # Submit job"
    echo "  curl -X POST -k -H \"Authorization: Bearer ${MASTER_API_KEY}\" -H \"Content-Type: application/json\" \\"
    echo "    https://localhost:${MASTER_PORT}/jobs -d '{\"scenario\":\"test\",\"confidence\":\"auto\"}'"
    echo ""
    echo -e "${YELLOW}Press Ctrl+C to stop the stack${NC}"
    echo ""
}

# Main execution
main() {
    print_header "FFmpeg RTMP Local Stack Launcher"
    echo ""
    
    check_prerequisites
    build_binaries
    start_master
    start_agent
    
    sleep 3
    
    if verify_stack; then
        print_success "All checks passed!"
        
        # Optionally submit test job
        if [ "${SKIP_TEST_JOB:-false}" != "true" ]; then
            submit_test_job || true
        fi
        
        display_summary
        
        # Keep running
        while true; do
            if ! kill -0 "$MASTER_PID" 2>/dev/null; then
                print_error "Master process died unexpectedly"
                cat "${MASTER_LOG}"
                exit 1
            fi
            
            if ! kill -0 "$AGENT_PID" 2>/dev/null; then
                print_error "Agent process died unexpectedly"
                cat "${AGENT_LOG}"
                exit 1
            fi
            
            sleep 5
        done
    else
        print_error "Stack verification failed"
        echo ""
        echo "Master log:"
        cat "${MASTER_LOG}"
        echo ""
        echo "Agent log:"
        cat "${AGENT_LOG}"
        exit 1
    fi
}

main "$@"
