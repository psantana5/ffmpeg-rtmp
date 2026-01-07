#!/bin/bash
# Health Check and Post-Deployment Verification
# Verifies services are running correctly after deployment
# Usage: ./health-check.sh [--master|--worker] [--url URL] [--api-key KEY]

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
log_step() { echo -e "${CYAN}[STEP]${NC} $1"; }

# Counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_WARNED=0

# Configuration
CHECK_TYPE=""
MASTER_URL=""
API_KEY=""
TIMEOUT=10

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --master) CHECK_TYPE="master"; shift ;;
        --worker) CHECK_TYPE="worker"; shift ;;
        --url) MASTER_URL="$2"; shift 2 ;;
        --api-key) API_KEY="$2"; shift 2 ;;
        --timeout) TIMEOUT="$2"; shift 2 ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

# Detect check type if not specified
if [ -z "$CHECK_TYPE" ]; then
    if systemctl list-unit-files | grep -q "ffrtmp-master.service"; then
        CHECK_TYPE="master"
    elif systemctl list-unit-files | grep -q "ffrtmp-worker.service"; then
        CHECK_TYPE="worker"
    else
        log_error "Cannot determine deployment type. Use --master or --worker"
        exit 1
    fi
fi

log_info "Starting health checks for $CHECK_TYPE node..."
echo ""

# Function to check service status
check_service() {
    local service=$1
    local required=${2:-true}
    
    log_step "Checking service: $service"
    
    if systemctl is-active --quiet "$service"; then
        log_success "$service is running"
        ((TESTS_PASSED++))
        return 0
    else
        if [ "$required" = "true" ]; then
            log_error "$service is not running"
            ((TESTS_FAILED++))
            systemctl status "$service" --no-pager -l || true
        else
            log_warn "$service is not running (optional)"
            ((TESTS_WARNED++))
        fi
        return 1
    fi
}

# Function to check port listening
check_port() {
    local port=$1
    local service=$2
    local required=${3:-true}
    
    log_step "Checking port $port ($service)"
    
    if ss -tlnp | grep -q ":$port "; then
        log_success "Port $port is listening"
        ((TESTS_PASSED++))
        return 0
    else
        if [ "$required" = "true" ]; then
            log_error "Port $port is not listening"
            ((TESTS_FAILED++))
        else
            log_warn "Port $port is not listening (optional)"
            ((TESTS_WARNED++))
        fi
        return 1
    fi
}

# Function to check HTTP endpoint
check_http_endpoint() {
    local url=$1
    local expected_code=${2:-200}
    local description=$3
    
    log_step "Checking HTTP endpoint: $description"
    
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT" "$url" 2>/dev/null || echo "000")
    
    if [ "$response_code" = "$expected_code" ]; then
        log_success "$description returned $response_code"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "$description returned $response_code (expected $expected_code)"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to check API endpoint with auth
check_api_endpoint() {
    local url=$1
    local api_key=$2
    local description=$3
    
    log_step "Checking API endpoint: $description"
    
    local response
    response=$(curl -s --max-time "$TIMEOUT" -H "X-API-Key: $api_key" "$url" 2>/dev/null || echo "")
    
    if [ -n "$response" ]; then
        log_success "$description is responding"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "$description is not responding"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to check file/directory exists
check_path() {
    local path=$1
    local type=$2  # file or directory
    local required=${3:-true}
    
    log_step "Checking $type: $path"
    
    if [ "$type" = "file" ] && [ -f "$path" ]; then
        log_success "File exists: $path"
        ((TESTS_PASSED++))
        return 0
    elif [ "$type" = "directory" ] && [ -d "$path" ]; then
        log_success "Directory exists: $path"
        ((TESTS_PASSED++))
        return 0
    else
        if [ "$required" = "true" ]; then
            log_error "$type not found: $path"
            ((TESTS_FAILED++))
        else
            log_warn "$type not found (optional): $path"
            ((TESTS_WARNED++))
        fi
        return 1
    fi
}

# Function to check disk space
check_disk_space() {
    local path=$1
    local min_gb=$2
    
    log_step "Checking disk space for $path (minimum: ${min_gb}GB)"
    
    local available_gb
    available_gb=$(df -BG "$path" | tail -1 | awk '{print $4}' | sed 's/G//')
    
    if [ "$available_gb" -ge "$min_gb" ]; then
        log_success "Sufficient disk space: ${available_gb}GB available"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "Insufficient disk space: ${available_gb}GB available (need ${min_gb}GB)"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to check log files
check_logs() {
    local log_file=$1
    local service=$2
    
    log_step "Checking logs for errors: $service"
    
    if [ ! -f "$log_file" ]; then
        log_warn "Log file not found: $log_file"
        ((TESTS_WARNED++))
        return 1
    fi
    
    local error_count
    error_count=$(grep -i "error\|fatal\|panic" "$log_file" 2>/dev/null | tail -10 | wc -l)
    
    if [ "$error_count" -eq 0 ]; then
        log_success "No recent errors in $service logs"
        ((TESTS_PASSED++))
        return 0
    else
        log_warn "Found $error_count recent errors in $service logs"
        ((TESTS_WARNED++))
        echo "Recent errors:"
        grep -i "error\|fatal\|panic" "$log_file" 2>/dev/null | tail -5
        return 1
    fi
}

# Master node health checks
check_master_health() {
    echo "═══════════════════════════════════════"
    echo "  Master Node Health Checks"
    echo "═══════════════════════════════════════"
    echo ""
    
    # Service checks
    check_service "ffrtmp-master.service"
    
    # Port checks
    check_port 8080 "Master API"
    check_port 1935 "RTMP" false
    
    # File/directory checks
    check_path "/opt/ffrtmp-master/bin/ffrtmp-master" "file"
    check_path "/etc/ffrtmp-master" "directory"
    check_path "/var/lib/ffrtmp-master" "directory"
    check_path "/var/log/ffrtmp" "directory"
    
    # Configuration file check
    check_path "/etc/ffrtmp-master/config.yaml" "file"
    
    # Database check
    if [ -f "/etc/ffrtmp-master/config.yaml" ]; then
        if grep -q "postgres" "/etc/ffrtmp-master/config.yaml"; then
            log_step "PostgreSQL database configured"
            check_service "postgresql" false
        else
            log_step "SQLite database configured"
            check_path "/var/lib/ffrtmp-master/master.db" "file"
        fi
    fi
    
    # HTTP endpoint checks
    check_http_endpoint "http://localhost:8080/health" 200 "Health endpoint"
    check_http_endpoint "http://localhost:8080/api/v1/jobs" 401 "API endpoint (auth required)"
    
    # Disk space
    check_disk_space "/var/lib/ffrtmp-master" 10
    
    # Log checks
    check_logs "/var/log/ffrtmp/master.log" "ffrtmp-master"
    
    # Monitoring (optional)
    check_service "prometheus" false
    check_service "grafana-server" false
}

# Worker node health checks
check_worker_health() {
    echo "═══════════════════════════════════════"
    echo "  Worker Node Health Checks"
    echo "═══════════════════════════════════════"
    echo ""
    
    # Service checks
    check_service "ffrtmp-worker.service"
    check_service "ffrtmp-watch.service" false
    
    # File/directory checks
    check_path "/opt/ffrtmp/bin/agent" "file"
    check_path "/opt/ffrtmp/bin/ffrtmp" "file"
    check_path "/etc/ffrtmp" "directory"
    check_path "/var/lib/ffrtmp" "directory"
    check_path "/opt/ffrtmp/streams" "directory"
    check_path "/opt/ffrtmp/logs" "directory"
    
    # Configuration checks
    check_path "/etc/ffrtmp/worker.env" "file"
    if [ -f "/etc/ffrtmp/watch-config.yaml" ]; then
        check_path "/etc/ffrtmp/watch-config.yaml" "file"
    fi
    
    # Disk space
    check_disk_space "/opt/ffrtmp/streams" 50
    
    # Log checks
    check_logs "/var/log/ffrtmp/worker.log" "ffrtmp-worker"
    if [ -f "/var/log/ffrtmp/watch.log" ]; then
        check_logs "/var/log/ffrtmp/watch.log" "ffrtmp-watch"
    fi
    
    # Master connectivity check
    if [ -n "$MASTER_URL" ]; then
        log_step "Checking connectivity to master"
        check_http_endpoint "${MASTER_URL}/health" 200 "Master health endpoint"
        
        if [ -n "$API_KEY" ]; then
            check_api_endpoint "${MASTER_URL}/api/v1/workers" "$API_KEY" "Worker registration API"
        fi
    else
        log_warn "Master URL not provided, skipping connectivity check"
        ((TESTS_WARNED++))
    fi
    
    # Check FFmpeg
    log_step "Checking FFmpeg installation"
    if command -v ffmpeg >/dev/null 2>&1; then
        local ffmpeg_version
        ffmpeg_version=$(ffmpeg -version 2>/dev/null | head -1 | awk '{print $3}')
        log_success "FFmpeg installed: $ffmpeg_version"
        ((TESTS_PASSED++))
    else
        log_error "FFmpeg not found"
        ((TESTS_FAILED++))
    fi
    
    # Check cgroups v2
    log_step "Checking cgroups v2"
    if [ -f "/sys/fs/cgroup/cgroup.controllers" ]; then
        log_success "Cgroups v2 is enabled"
        ((TESTS_PASSED++))
    else
        log_warn "Cgroups v2 not detected (using v1?)"
        ((TESTS_WARNED++))
    fi
}

# Run checks based on type
case $CHECK_TYPE in
    master)
        check_master_health
        ;;
    worker)
        check_worker_health
        ;;
    *)
        log_error "Invalid check type: $CHECK_TYPE"
        exit 1
        ;;
esac

# Summary
echo ""
echo "═══════════════════════════════════════"
echo "  Health Check Summary"
echo "═══════════════════════════════════════"
echo -e "${GREEN}Passed:${NC}  $TESTS_PASSED"
echo -e "${YELLOW}Warnings:${NC} $TESTS_WARNED"
echo -e "${RED}Failed:${NC}  $TESTS_FAILED"
echo ""

# Exit code
if [ $TESTS_FAILED -gt 0 ]; then
    log_error "Health checks failed!"
    exit 1
elif [ $TESTS_WARNED -gt 0 ]; then
    log_warn "Health checks passed with warnings"
    exit 0
else
    log_success "All health checks passed!"
    exit 0
fi
