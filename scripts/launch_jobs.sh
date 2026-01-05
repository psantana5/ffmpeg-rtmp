#!/bin/bash
#
# Production-grade job launcher for ffmpeg-rtmp distributed system
# Submits jobs with configurable parameters, monitoring, and error handling
#
# Usage: ./scripts/launch_jobs.sh [OPTIONS]
#
# Options:
#   --count N          Number of jobs to submit (default: 1000)
#   --master URL       Master server URL (default: http://localhost:8080)
#   --scenario NAME    Job scenario (default: random)
#   --batch-size N     Submit jobs in batches of N (default: 50)
#   --delay MS         Delay between batches in milliseconds (default: 100)
#   --priority LEVEL   Priority: high, medium, low (default: mixed)
#   --queue TYPE       Queue: live, default, batch (default: mixed)
#   --engine ENGINE    Engine: auto, ffmpeg, gstreamer (default: auto)
#   --output FILE      Output results to file (default: job_launch_results.json)
#   --dry-run          Print what would be done without submitting
#   --verbose          Enable verbose logging
#   --help             Show this help message
#

set -euo pipefail

# Default configuration
JOB_COUNT=1000
MASTER_URL="${MASTER_URL:-https://localhost:8080}"
SCENARIO="random"
BATCH_SIZE=50
BATCH_DELAY=100  # milliseconds
PRIORITY="mixed"
QUEUE="mixed"
ENGINE="auto"
OUTPUT_FILE="job_launch_results.json"
DRY_RUN=false
VERBOSE=false

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

# Show help
show_help() {
    sed -n '2,20p' "$0" | sed 's/^# //'
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --count)
            JOB_COUNT="$2"
            shift 2
            ;;
        --master)
            MASTER_URL="$2"
            shift 2
            ;;
        --scenario)
            SCENARIO="$2"
            shift 2
            ;;
        --batch-size)
            BATCH_SIZE="$2"
            shift 2
            ;;
        --delay)
            BATCH_DELAY="$2"
            shift 2
            ;;
        --priority)
            PRIORITY="$2"
            shift 2
            ;;
        --queue)
            QUEUE="$2"
            shift 2
            ;;
        --engine)
            ENGINE="$2"
            shift 2
            ;;
        --output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            show_help
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Validate inputs
if ! [[ "$JOB_COUNT" =~ ^[0-9]+$ ]] || [[ "$JOB_COUNT" -lt 1 ]]; then
    log_error "Invalid job count: $JOB_COUNT (must be positive integer)"
    exit 1
fi

if ! [[ "$BATCH_SIZE" =~ ^[0-9]+$ ]] || [[ "$BATCH_SIZE" -lt 1 ]]; then
    log_error "Invalid batch size: $BATCH_SIZE (must be positive integer)"
    exit 1
fi

# Scenario definitions (CPU-friendly mix including h265, VP9, AV1)
SCENARIOS=(
    "4K60-h264"
    "4K60-h265"
    "4K30-h264"
    "4K30-h265"
    "1080p60-h264"
    "1080p60-h265"
    "1080p30-h264"
    "1080p30-h265"
    "720p60-h264"
    "720p60-h265"
    "720p30-h264"
    "720p30-h265"
    "480p30-h264"
    "480p30-h265"
    "480p60-h264"
    "1080p60-vp9"
    "720p30-vp9"
    "480p30-av1"
)

PRIORITIES=("high" "medium" "low")
QUEUES=("live" "default" "batch")

# Function to get random element from array
get_random() {
    local arr=("$@")
    echo "${arr[$((RANDOM % ${#arr[@]}))]}"
}

# Function to generate job payload
generate_job_payload() {
    local scenario="$1"
    local priority="$2"
    local queue="$3"
    local engine="$4"
    
    # Random duration between 30 and 300 seconds
    local duration=$((30 + RANDOM % 271))
    
    # Random bitrate based on scenario
    local bitrate
    if [[ "$scenario" =~ 4K ]]; then
        bitrate="$((10 + RANDOM % 15))M"
    elif [[ "$scenario" =~ 1080p ]]; then
        bitrate="$((4 + RANDOM % 6))M"
    elif [[ "$scenario" =~ 720p ]]; then
        bitrate="$((2 + RANDOM % 3))M"
    else
        bitrate="$((1 + RANDOM % 2))M"
    fi
    
    cat <<EOF
{
  "scenario": "$scenario",
  "confidence": "auto",
  "engine": "$engine",
  "queue": "$queue",
  "priority": "$priority",
  "parameters": {
    "duration": $duration,
    "bitrate": "$bitrate"
  }
}
EOF
}

# Function to submit a single job
submit_job() {
    local payload="$1"
    local job_num="$2"
    
    log_verbose "Job #$job_num payload: $payload"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "{\"id\":\"dry-run-$job_num\",\"sequence_number\":$job_num,\"status\":\"queued\"}"
        return 0
    fi
    
    local response
    local http_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$MASTER_URL/jobs" 2>&1)
    
    http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [[ "$http_code" == "201" ]] || [[ "$http_code" == "200" ]]; then
        echo "$body"
        return 0
    else
        log_error "Job #$job_num failed with HTTP $http_code: $body"
        echo "{\"error\":\"HTTP $http_code\",\"job_number\":$job_num}"
        return 1
    fi
}

# Function to check master health
check_master_health() {
    log_info "Checking master server health at $MASTER_URL..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_success "Dry run mode - skipping health check"
        return 0
    fi
    
    local health_url="$MASTER_URL/health"
    if curl -sf "$health_url" > /dev/null 2>&1; then
        log_success "Master server is healthy"
        return 0
    else
        log_error "Master server health check failed at $health_url"
        return 1
    fi
}

# Function to display progress bar
show_progress() {
    local current=$1
    local total=$2
    local width=50
    local percentage=$((current * 100 / total))
    local filled=$((width * current / total))
    local empty=$((width - filled))
    
    printf "\r[INFO] Progress: ["
    printf "%${filled}s" | tr ' ' '='
    printf "%${empty}s" | tr ' ' ' '
    printf "] %3d%% (%d/%d)" "$percentage" "$current" "$total"
}

# Main execution
main() {
    local start_time=$(date +%s)
    
    echo "=================================================="
    echo "  FFmpeg-RTMP Production Job Launcher"
    echo "=================================================="
    echo ""
    log_info "Configuration:"
    echo "  Jobs to submit: $JOB_COUNT"
    echo "  Master URL: $MASTER_URL"
    echo "  Scenario: $SCENARIO"
    echo "  Batch size: $BATCH_SIZE"
    echo "  Batch delay: ${BATCH_DELAY}ms"
    echo "  Priority: $PRIORITY"
    echo "  Queue: $QUEUE"
    echo "  Engine: $ENGINE"
    echo "  Output file: $OUTPUT_FILE"
    echo "  Dry run: $DRY_RUN"
    echo ""
    
    # Health check
    if ! check_master_health; then
        log_error "Aborting due to health check failure"
        exit 1
    fi
    
    # Initialize results
    local results_file="/tmp/job_launch_$$.json"
    echo "[" > "$results_file"
    
    local submitted=0
    local failed=0
    local batch_num=0
    
    log_info "Starting job submission..."
    
    # Submit jobs in batches
    for ((i=1; i<=JOB_COUNT; i++)); do
        # Determine job parameters
        local job_scenario="$SCENARIO"
        local job_priority="$PRIORITY"
        local job_queue="$QUEUE"
        local job_engine="$ENGINE"
        
        # Handle "random" or "mixed" values
        if [[ "$job_scenario" == "random" ]]; then
            job_scenario=$(get_random "${SCENARIOS[@]}")
        fi
        
        if [[ "$job_priority" == "mixed" ]]; then
            job_priority=$(get_random "${PRIORITIES[@]}")
        fi
        
        if [[ "$job_queue" == "mixed" ]]; then
            job_queue=$(get_random "${QUEUES[@]}")
        fi
        
        # Generate and submit job
        local payload=$(generate_job_payload "$job_scenario" "$job_priority" "$job_queue" "$job_engine")
        local result=$(submit_job "$payload" "$i")
        
        # Track results
        if [[ "$result" =~ \"error\" ]]; then
            ((failed++)) || true
        else
            ((submitted++)) || true
        fi
        
        # Write result to file
        if [[ $i -gt 1 ]]; then
            echo "," >> "$results_file"
        fi
        echo "$result" >> "$results_file"
        
        # Show progress
        if [[ $((i % 10)) -eq 0 ]] || [[ $i -eq "$JOB_COUNT" ]]; then
            show_progress "$i" "$JOB_COUNT"
        fi
        
        # Batch delay
        if [[ $((i % BATCH_SIZE)) -eq 0 ]] && [[ $i -lt "$JOB_COUNT" ]]; then
            ((batch_num++)) || true
            log_verbose "Completed batch $batch_num, sleeping ${BATCH_DELAY}ms..."
            sleep "0.$(printf "%03d" "$BATCH_DELAY")"
        fi
    done
    
    echo ""  # New line after progress bar
    echo "]" >> "$results_file"
    
    # Save final results
    mv "$results_file" "$OUTPUT_FILE"
    
    # Calculate metrics
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    local rate=$(awk "BEGIN {printf \"%.2f\", $submitted/$duration}")
    
    # Summary
    echo ""
    echo "=================================================="
    echo "  Job Submission Summary"
    echo "=================================================="
    log_success "Total submitted: $submitted"
    if [[ $failed -gt 0 ]]; then
        log_warn "Total failed: $failed"
    fi
    echo "  Duration: ${duration}s"
    echo "  Submission rate: ${rate} jobs/sec"
    echo "  Results saved to: $OUTPUT_FILE"
    echo ""
    
    # Extract job IDs for easy access
    if command -v jq &> /dev/null; then
        local job_ids=$(jq -r '.[] | select(.id != null) | .id' "$OUTPUT_FILE" 2>/dev/null | wc -l)
        log_info "Successfully created $job_ids jobs (use jq to parse $OUTPUT_FILE)"
    fi
    
    if [[ $failed -gt 0 ]]; then
        exit 1
    fi
}

# Handle Ctrl+C gracefully
trap 'echo ""; log_warn "Interrupted by user"; exit 130' INT

# Run main function
main
