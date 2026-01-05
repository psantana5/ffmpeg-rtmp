#!/bin/bash
#
# Load Testing Script for FFmpeg-RTMP Distributed System
# 
# Usage:
#   ./scripts/load_test.sh --jobs 1000 --workers 1 --concurrency 4
#   ./scripts/load_test.sh --quick-test
#   ./scripts/load_test.sh --stress-test
#
# Features:
# - Submits N jobs rapidly to test throughput
# - Monitors system resources during test
# - Tracks job completion rates
# - Generates comprehensive performance report
# - Exports results in JSON and Markdown formats

set -euo pipefail

# Configuration
MASTER_URL="${MASTER_URL:-https://localhost:8080}"
MASTER_API_KEY="${MASTER_API_KEY:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$PROJECT_ROOT/test_results/load_tests"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_NAME="load_test_${TIMESTAMP}"
RESULTS_FILE="$RESULTS_DIR/${TEST_NAME}.json"
REPORT_FILE="$RESULTS_DIR/${TEST_NAME}_report.md"

# Default test parameters
NUM_JOBS=100
NUM_WORKERS=1
CONCURRENT_JOBS=4
SUBMIT_RATE=10  # jobs per second
MONITOR_INTERVAL=5  # seconds
TEST_TYPE="standard"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# Show usage
usage() {
    cat << EOF
FFmpeg-RTMP Load Testing Tool

Usage: $0 [OPTIONS]

Options:
    --jobs N                Number of jobs to submit (default: 100)
    --workers N             Expected number of workers (default: 1)
    --concurrency N         Expected concurrent jobs per worker (default: 4)
    --submit-rate N         Job submission rate (jobs/sec) (default: 10)
    --master URL            Master URL (default: https://localhost:8080)
    --api-key KEY           API key for authentication
    
Preset Tests:
    --quick-test            Quick validation (10 jobs)
    --standard-test         Standard test (100 jobs, 1 worker)
    --stress-test           Stress test (1000 jobs, high rate)
    --scale-test            Multi-worker test (500 jobs, 3 workers)
    
Other Options:
    --skip-monitoring       Don't collect system metrics
    --help                  Show this help message

Examples:
    # Quick validation
    $0 --quick-test
    
    # Standard load test
    $0 --jobs 100 --workers 1 --concurrency 4
    
    # Stress test with high submission rate
    $0 --stress-test
    
    # Multi-worker scale test
    $0 --scale-test --workers 3

EOF
    exit 0
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --jobs)
                NUM_JOBS="$2"
                shift 2
                ;;
            --workers)
                NUM_WORKERS="$2"
                shift 2
                ;;
            --concurrency)
                CONCURRENT_JOBS="$2"
                shift 2
                ;;
            --submit-rate)
                SUBMIT_RATE="$2"
                shift 2
                ;;
            --master)
                MASTER_URL="$2"
                shift 2
                ;;
            --api-key)
                MASTER_API_KEY="$2"
                shift 2
                ;;
            --quick-test)
                TEST_TYPE="quick"
                NUM_JOBS=10
                SUBMIT_RATE=5
                shift
                ;;
            --standard-test)
                TEST_TYPE="standard"
                NUM_JOBS=100
                SUBMIT_RATE=10
                shift
                ;;
            --stress-test)
                TEST_TYPE="stress"
                NUM_JOBS=1000
                SUBMIT_RATE=50
                shift
                ;;
            --scale-test)
                TEST_TYPE="scale"
                NUM_JOBS=500
                NUM_WORKERS=3
                SUBMIT_RATE=20
                shift
                ;;
            --skip-monitoring)
                SKIP_MONITORING=true
                shift
                ;;
            --help)
                usage
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed. Install with: sudo apt install jq"
        exit 1
    fi
    
    # Check if master is reachable
    if ! curl -sk "$MASTER_URL/health" &> /dev/null; then
        log_error "Cannot reach master at $MASTER_URL"
        log_error "Make sure master is running and URL is correct"
        exit 1
    fi
    
    # Create results directory
    mkdir -p "$RESULTS_DIR"
    
    log_success "Prerequisites check passed"
}

# Get system info
get_system_info() {
    log_info "Collecting system information..."
    
    cat > "$RESULTS_DIR/${TEST_NAME}_sysinfo.json" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "hostname": "$(hostname)",
    "os": "$(uname -s)",
    "kernel": "$(uname -r)",
    "cpu_model": "$(lscpu | grep "Model name" | sed 's/Model name: *//' | xargs)",
    "cpu_threads": $(nproc),
    "memory_total_gb": $(free -g | awk '/^Mem:/{print $2}'),
    "master_url": "$MASTER_URL",
    "test_type": "$TEST_TYPE",
    "num_jobs": $NUM_JOBS,
    "num_workers": $NUM_WORKERS,
    "concurrent_jobs": $CONCURRENT_JOBS,
    "submit_rate": $SUBMIT_RATE
}
EOF
}

# Get baseline metrics
get_baseline_metrics() {
    log_info "Collecting baseline metrics..."
    
    # Get current job count
    local jobs_before=0
    if [[ -n "$MASTER_API_KEY" ]]; then
        jobs_before=$(curl -sk "$MASTER_URL/jobs" \
            -H "Authorization: Bearer $MASTER_API_KEY" 2>/dev/null | \
            jq '[.[] | select(.status == "completed")] | length' 2>/dev/null || echo "0")
    fi
    
    # Get worker count and status
    local workers_available=0
    if [[ -n "$MASTER_API_KEY" ]]; then
        workers_available=$(curl -sk "$MASTER_URL/nodes" \
            -H "Authorization: Bearer $MASTER_API_KEY" 2>/dev/null | \
            jq '.nodes | length' 2>/dev/null || echo "0")
    fi
    
    cat > "$RESULTS_DIR/${TEST_NAME}_baseline.json" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "jobs_completed_before": $jobs_before,
    "workers_available": $workers_available,
    "cpu_usage_percent": $(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d% -f1),
    "memory_used_gb": $(free -g | awk '/^Mem:/{print $3}'),
    "load_average": "$(uptime | awk -F'load average:' '{print $2}' | xargs)"
}
EOF
    
    log_info "Baseline: $jobs_before completed jobs, $workers_available workers available"
}

# Submit jobs
submit_jobs() {
    log_info "Submitting $NUM_JOBS jobs at $SUBMIT_RATE jobs/sec..."
    
    local submitted=0
    local failed=0
    local start_time=$(date +%s)
    local job_ids=()
    
    # Calculate delay between submissions (in milliseconds for sleep)
    local delay_sec=$(awk "BEGIN {printf \"%.3f\", 1.0 / $SUBMIT_RATE}")
    
    # Job scenarios (mix of different configurations)
    local scenarios=("1080p-h264" "720p-h264" "720p-vp8" "1080p-vp9" "480p-h264")
    local bitrates=("5M" "3M" "2M" "4M" "1M")
    local durations=(30 60 90 120)
    
    # Create/clear submissions log
    echo -n "" > "$RESULTS_DIR/${TEST_NAME}_submissions.log"
    
    local i
    for ((i=1; i<=NUM_JOBS; i++)); do
        # Select random scenario  
        local idx=$((RANDOM % ${#scenarios[@]}))
        local scenario="${scenarios[$idx]}"
        local bitrate="${bitrates[$idx]}"
        local duration_idx=$((RANDOM % ${#durations[@]}))
        local duration="${durations[$duration_idx]}"
        
        # Submit job
        local submit_start=$(date +%s%N)
        
        # Use temporary file for response
        local temp_resp="/tmp/load_test_resp_$$_$i"
        local http_code
        http_code=$(curl -sk -X POST "$MASTER_URL/jobs" \
            -H "Authorization: Bearer $MASTER_API_KEY" \
            -H "Content-Type: application/json" \
            -w "%{http_code}" \
            -o "$temp_resp" \
            -d "{
                \"scenario\": \"$scenario\",
                \"confidence\": \"auto\",
                \"parameters\": {
                    \"duration\": $duration,
                    \"bitrate\": \"$bitrate\"
                },
                \"priority\": \"medium\",
                \"queue\": \"default\"
            }" 2>/dev/null || echo "000")
        
        local submit_end=$(date +%s%N)
        local submit_latency_ms=$(( (submit_end - submit_start) / 1000000 ))
        
        # Read response
        local response
        if [[ -f "$temp_resp" ]]; then
            response=$(cat "$temp_resp" 2>/dev/null || echo "{}")
            rm -f "$temp_resp"
        else
            response="{}"
        fi
        
        # Check success
        if [[ "$http_code" == "200" ]] || [[ "$http_code" == "201" ]]; then
            local job_id
            job_id=$(echo "$response" | jq -r '.id' 2>/dev/null || echo "")
            if [[ -n "$job_id" ]] && [[ "$job_id" != "null" ]]; then
                job_ids+=("$job_id")
                ((submitted++))
                echo "$(date -Iseconds)|SUCCESS|$job_id|$submit_latency_ms|$scenario" >> "$RESULTS_DIR/${TEST_NAME}_submissions.log"
            else
                ((failed++))
                echo "$(date -Iseconds)|FAILED|N/A|$submit_latency_ms|$scenario|HTTP:$http_code|NoJobID" >> "$RESULTS_DIR/${TEST_NAME}_submissions.log"
            fi
        else
            ((failed++))
            local error_msg=$(echo "$response" | tr '\n' ' ' | cut -c1-100)
            echo "$(date -Iseconds)|FAILED|N/A|$submit_latency_ms|$scenario|HTTP:$http_code|$error_msg" >> "$RESULTS_DIR/${TEST_NAME}_submissions.log"
        fi
        
        # Progress indicator
        if (( i % 10 == 0 )) || (( i == NUM_JOBS )); then
            local elapsed=$(($(date +%s) - start_time))
            local rate=0
            if [[ $elapsed -gt 0 ]]; then
                rate=$(awk "BEGIN {printf \"%.2f\", $submitted / $elapsed}")
            fi
            echo -ne "\rSubmitted: $submitted/$NUM_JOBS (Failed: $failed) | Rate: $rate jobs/sec | Elapsed: ${elapsed}s     "
        fi
        
        # Rate limiting
        sleep "$delay_sec" 2>/dev/null || sleep 0.2
    done
    
    echo ""
    local total_time=$(($(date +%s) - start_time))
    local actual_rate=$(awk "BEGIN {printf \"%.2f\", $submitted / $total_time}")
    
    log_success "Submitted $submitted jobs in ${total_time}s (${actual_rate} jobs/sec)"
    
    if [[ $failed -gt 0 ]]; then
        log_warning "$failed job submissions failed"
    fi
    
    # Save job IDs
    printf '%s\n' "${job_ids[@]}" > "$RESULTS_DIR/${TEST_NAME}_job_ids.txt"
}

# Monitor progress
monitor_progress() {
    local total_jobs=$1
    local start_time=$(date +%s)
    
    log_info "Monitoring job completion (checking every ${MONITOR_INTERVAL}s)..."
    
    # Initial delay to let jobs start
    sleep 5
    
    while true; do
        local completed=0
        local processing=0
        local queued=0
        local failed=0
        
        if [[ -n "$MASTER_API_KEY" ]]; then
            local metrics=$(curl -sk "$MASTER_URL/jobs" \
                -H "Authorization: Bearer $MASTER_API_KEY" 2>/dev/null | \
                jq -r '
                    group_by(.status) | 
                    map({status: .[0].status, count: length}) | 
                    from_entries
                ' 2>/dev/null || echo "{}")
            
            completed=$(echo "$metrics" | jq -r '.completed // 0' 2>/dev/null || echo "0")
            processing=$(echo "$metrics" | jq -r '.processing // 0' 2>/dev/null || echo "0")
            queued=$(echo "$metrics" | jq -r '.queued // 0' 2>/dev/null || echo "0")
            failed=$(echo "$metrics" | jq -r '.failed // 0' 2>/dev/null || echo "0")
        fi
        
        local elapsed=$(($(date +%s) - start_time))
        local completion_rate=0
        if [[ $elapsed -gt 0 && $completed -gt 0 ]]; then
            completion_rate=$(awk "BEGIN {printf \"%.2f\", $completed / $elapsed}")
        fi
        
        echo "$(date -Iseconds)|$completed|$processing|$queued|$failed|$elapsed|$completion_rate" \
            >> "$RESULTS_DIR/${TEST_NAME}_progress.log"
        
        echo -ne "\rCompleted: $completed/$total_jobs | Processing: $processing | Queued: $queued | Failed: $failed | Rate: $completion_rate jobs/sec     "
        
        # Check if all jobs are done - use proper integer comparison
        local total_done=$((completed + failed))
        if [[ $total_done -ge $total_jobs ]]; then
            break
        fi
        
        sleep $MONITOR_INTERVAL
    done
    
    echo ""
    log_success "Monitoring complete"
}

# Collect resource metrics
collect_metrics() {
    log_info "Collecting final metrics..."
    
    # Get final job stats
    local final_stats=$(curl -sk "$MASTER_URL/jobs" \
        -H "Authorization: Bearer $MASTER_API_KEY" 2>/dev/null | \
        jq -r '
            group_by(.status) | 
            map({status: .[0].status, count: length}) | 
            from_entries
        ' 2>/dev/null || echo "{}")
    
    # Get Prometheus metrics if available
    local prometheus_metrics=""
    if curl -s http://localhost:9090/metrics &> /dev/null; then
        prometheus_metrics=$(curl -s http://localhost:9090/metrics | \
            grep -E "ffrtmp_jobs_total|ffrtmp_active_jobs" || echo "")
    fi
    
    cat > "$RESULTS_DIR/${TEST_NAME}_final.json" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "job_stats": $final_stats,
    "system": {
        "cpu_usage_percent": $(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d% -f1),
        "memory_used_gb": $(free -g | awk '/^Mem:/{print $3}'),
        "load_average": "$(uptime | awk -F'load average:' '{print $2}' | xargs)"
    },
    "prometheus_metrics": $(echo "$prometheus_metrics" | jq -Rs '.' || echo '""')
}
EOF
}

# Generate report
generate_report() {
    log_info "Generating performance report..."
    
    # Parse submission data
    local submissions_file="$RESULTS_DIR/${TEST_NAME}_submissions.log"
    local submitted=$(grep -c "SUCCESS" "$submissions_file" || echo "0")
    local failed_submissions=$(grep -c "FAILED" "$submissions_file" || echo "0")
    
    # Calculate submission stats
    local avg_latency=$(awk -F'|' '/SUCCESS/{sum+=$4; count++} END {if(count>0) print sum/count; else print 0}' "$submissions_file")
    local max_latency=$(awk -F'|' '/SUCCESS/{if($4>max) max=$4} END {print max+0}' "$submissions_file")
    
    # Parse progress data
    local progress_file="$RESULTS_DIR/${TEST_NAME}_progress.log"
    local completed=$(tail -1 "$progress_file" | cut -d'|' -f2)
    local failed=$(tail -1 "$progress_file" | cut -d'|' -f5)
    local total_time=$(tail -1 "$progress_file" | cut -d'|' -f6)
    local avg_completion_rate=$(tail -1 "$progress_file" | cut -d'|' -f7)
    
    # Calculate success rate
    local success_rate=0
    if [[ $submitted -gt 0 ]]; then
        success_rate=$(awk "BEGIN {printf \"%.2f\", ($completed / $submitted) * 100}")
    fi
    
    # Read system info
    local sys_info=$(cat "$RESULTS_DIR/${TEST_NAME}_sysinfo.json")
    local cpu_model=$(echo "$sys_info" | jq -r '.cpu_model')
    local cpu_threads=$(echo "$sys_info" | jq -r '.cpu_threads')
    local memory_gb=$(echo "$sys_info" | jq -r '.memory_total_gb')
    
    # Generate markdown report
    cat > "$REPORT_FILE" << EOF
# Load Test Report: $TEST_NAME

## Test Configuration

- **Test Type**: $TEST_TYPE
- **Date**: $(date -Iseconds)
- **Master URL**: $MASTER_URL
- **Jobs Submitted**: $submitted
- **Expected Workers**: $NUM_WORKERS
- **Concurrent Jobs per Worker**: $CONCURRENT_JOBS
- **Target Submission Rate**: $SUBMIT_RATE jobs/sec

## System Configuration

- **Hostname**: $(hostname)
- **CPU**: $cpu_model ($cpu_threads threads)
- **Memory**: ${memory_gb}GB
- **OS**: $(uname -s) $(uname -r)

## Results Summary

### Job Submission
- **Submitted**: $submitted jobs
- **Failed Submissions**: $failed_submissions
- **Average Latency**: ${avg_latency}ms
- **Max Latency**: ${max_latency}ms

### Job Completion
- **Completed**: $completed jobs
- **Failed**: $failed jobs
- **Success Rate**: ${success_rate}%
- **Total Time**: ${total_time}s
- **Average Completion Rate**: ${avg_completion_rate} jobs/sec

## Performance Analysis

### Throughput
- **Submission Throughput**: $(awk "BEGIN {printf \"%.2f\", $submitted / $total_time}") jobs/sec
- **Completion Throughput**: $avg_completion_rate jobs/sec
- **Expected Capacity**: $((NUM_WORKERS * CONCURRENT_JOBS)) concurrent jobs

### Bottlenecks
EOF

    # Analyze bottlenecks
    local completion_rate_int=$(echo "$avg_completion_rate * 100" | bc 2>/dev/null | cut -d. -f1)
    local submit_rate_int=$((SUBMIT_RATE * 100))
    
    if [[ ${completion_rate_int:-0} -lt ${submit_rate_int} ]]; then
        echo "- ⚠️ Completion rate ($avg_completion_rate) is slower than submission rate ($SUBMIT_RATE)" >> "$REPORT_FILE"
        echo "- Workers may be overloaded or jobs are taking longer than expected" >> "$REPORT_FILE"
    else
        echo "- ✅ System is keeping up with load" >> "$REPORT_FILE"
    fi
    
    if [[ $failed -gt 0 ]]; then
        echo "- ⚠️ $failed jobs failed ($(awk "BEGIN {printf \"%.2f\", ($failed / $submitted) * 100}")%)" >> "$REPORT_FILE"
    fi
    
    cat >> "$REPORT_FILE" << EOF

## Detailed Metrics

### Job Progress Over Time

\`\`\`
$(cat "$progress_file" | awk -F'|' 'BEGIN {print "Time | Completed | Processing | Queued | Failed | Elapsed(s) | Rate(jobs/s)"} {printf "%s | %6d | %6d | %6d | %6d | %6d | %8s\n", $1, $2, $3, $4, $5, $6, $7}' | head -20)
...
$(cat "$progress_file" | awk -F'|' '{printf "%s | %6d | %6d | %6d | %6d | %6d | %8s\n", $1, $2, $3, $4, $5, $6, $7}' | tail -10)
\`\`\`

## Recommendations

EOF

    # Generate recommendations
    local success_rate_int=${success_rate%.*}
    if [[ ${success_rate_int:-0} -lt 95 ]]; then
        echo "- ⚠️ Success rate below 95% - investigate job failures" >> "$REPORT_FILE"
    fi
    
    local expected_throughput=$((NUM_WORKERS * CONCURRENT_JOBS))
    local completion_int=${avg_completion_rate%.*}
    if [[ ${completion_int:-0} -lt $expected_throughput ]]; then
        echo "- Consider increasing worker count or concurrent jobs per worker" >> "$REPORT_FILE"
    fi
    
    echo "- Increase worker concurrent jobs if CPU usage is low" >> "$REPORT_FILE"
    echo "- Add more workers if single worker is CPU-saturated" >> "$REPORT_FILE"
    echo "- Consider PostgreSQL if SQLite becomes a bottleneck" >> "$REPORT_FILE"
    
    cat >> "$REPORT_FILE" << EOF

## Files Generated

- System Info: \`${TEST_NAME}_sysinfo.json\`
- Baseline: \`${TEST_NAME}_baseline.json\`
- Submissions Log: \`${TEST_NAME}_submissions.log\`
- Job IDs: \`${TEST_NAME}_job_ids.txt\`
- Progress Log: \`${TEST_NAME}_progress.log\`
- Final Metrics: \`${TEST_NAME}_final.json\`
- This Report: \`${TEST_NAME}_report.md\`

---
Generated by FFmpeg-RTMP Load Testing Tool
EOF

    log_success "Report generated: $REPORT_FILE"
}

# Main execution
main() {
    echo ""
    echo "================================================"
    echo "  FFmpeg-RTMP Load Testing Tool"
    echo "================================================"
    echo ""
    
    parse_args "$@"
    check_prerequisites
    get_system_info
    get_baseline_metrics
    
    # Submit jobs
    submit_jobs
    
    # Get number of submitted jobs from log
    local submitted=$(grep -c "SUCCESS" "$RESULTS_DIR/${TEST_NAME}_submissions.log" 2>/dev/null || echo "0")
    
    # Monitor progress
    if [[ $submitted -gt 0 ]]; then
        monitor_progress "$submitted"
    else
        log_error "No jobs were successfully submitted"
        exit 1
    fi
    
    # Collect final metrics
    collect_metrics
    
    # Generate report
    generate_report
    
    echo ""
    log_success "Load test complete!"
    log_info "Results saved to: $RESULTS_DIR"
    log_info "Report: $REPORT_FILE"
    echo ""
}

# Run main
main "$@"
