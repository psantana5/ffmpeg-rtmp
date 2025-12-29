#!/bin/bash
#
# run_benchmarks.sh - Automated Performance Benchmark Script
# Runs 4 standard workload scenarios and pushes results to VictoriaMetrics
#

set -e

# Configuration
RESULTS_DIR="${RESULTS_DIR:-./test_results}"
VM_URL="${VM_URL:-http://localhost:8428}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
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

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if VictoriaMetrics is running
check_victoriametrics() {
    log_info "Checking VictoriaMetrics availability..."
    if curl -sf "${VM_URL}/health" > /dev/null 2>&1; then
        log_success "VictoriaMetrics is accessible at ${VM_URL}"
        return 0
    else
        log_error "VictoriaMetrics is not accessible at ${VM_URL}"
        return 1
    fi
}

# Push metrics to VictoriaMetrics
push_metrics() {
    local benchmark_name="$1"
    local duration="$2"
    local status="$3"
    local timestamp=$(date +%s)000
    
    # Benchmark metadata
    local metrics="benchmark_run_duration_seconds{benchmark=\"${benchmark_name}\",status=\"${status}\"} ${duration} ${timestamp}
benchmark_run_total{benchmark=\"${benchmark_name}\"} 1 ${timestamp}
benchmark_run_status{benchmark=\"${benchmark_name}\",status=\"${status}\"} 1 ${timestamp}"
    
    # Push to VictoriaMetrics
    echo "$metrics" | curl -sf -X POST "${VM_URL}/api/v1/import/prometheus" --data-binary @- > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        log_success "Pushed metrics for ${benchmark_name} to VictoriaMetrics"
    else
        log_warning "Failed to push metrics for ${benchmark_name} to VictoriaMetrics"
    fi
}

# Run a single benchmark workload
run_workload() {
    local workload_name="$1"
    local bitrate="$2"
    local resolution="$3"
    local fps="$4"
    local duration="$5"
    local stabilization="${6:-10}"
    local cooldown="${7:-10}"
    
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "Running benchmark: ${workload_name}"
    log_info "  Bitrate: ${bitrate}"
    log_info "  Resolution: ${resolution}"
    log_info "  FPS: ${fps}"
    log_info "  Duration: ${duration}s"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    local start_time=$(date +%s)
    
    # Run the test
    cd "$PROJECT_ROOT"
    if python3 scripts/run_tests.py --output-dir "${RESULTS_DIR}" single \
        --name "benchmark_${workload_name}" \
        --bitrate "${bitrate}" \
        --resolution "${resolution}" \
        --fps "${fps}" \
        --duration "${duration}" \
        --stabilization "${stabilization}" \
        --cooldown "${cooldown}"; then
        
        local end_time=$(date +%s)
        local elapsed=$((end_time - start_time))
        
        log_success "Benchmark ${workload_name} completed in ${elapsed}s"
        push_metrics "${workload_name}" "${elapsed}" "success"
        return 0
    else
        local end_time=$(date +%s)
        local elapsed=$((end_time - start_time))
        
        log_error "Benchmark ${workload_name} failed after ${elapsed}s"
        push_metrics "${workload_name}" "${elapsed}" "failed"
        return 1
    fi
}

# Main benchmark suite
run_benchmark_suite() {
    log_info "═══════════════════════════════════════════"
    log_info "  FFmpeg RTMP Performance Benchmark Suite  "
    log_info "═══════════════════════════════════════════"
    echo ""
    
    # Check prerequisites
    if ! check_victoriametrics; then
        log_error "VictoriaMetrics is required for benchmark automation"
        log_info "Start it with: make up"
        exit 1
    fi
    
    # Create results directory
    mkdir -p "${RESULTS_DIR}"
    
    local total_benchmarks=4
    local completed=0
    local failed=0
    
    # Workload 1: Laptop - Low power, 720p
    log_info ""
    log_info "Workload 1/4: Laptop Profile"
    if run_workload "laptop" "1000k" "1280x720" "30" "60" "10" "10"; then
        ((completed++))
    else
        ((failed++))
    fi
    
    sleep 5  # Cool down between benchmarks
    
    # Workload 2: Desktop - Medium power, 1080p
    log_info ""
    log_info "Workload 2/4: Desktop Profile"
    if run_workload "desktop" "2500k" "1920x1080" "30" "60" "10" "10"; then
        ((completed++))
    else
        ((failed++))
    fi
    
    sleep 5
    
    # Workload 3: Single-GPU - High power, 1080p 60fps
    log_info ""
    log_info "Workload 3/4: Single-GPU Profile"
    if run_workload "single_gpu" "5000k" "1920x1080" "60" "60" "10" "10"; then
        ((completed++))
    else
        ((failed++))
    fi
    
    sleep 5
    
    # Workload 4: Dual-GPU - Very high power, 4K
    log_info ""
    log_info "Workload 4/4: Dual-GPU Profile"
    if run_workload "dual_gpu" "15000k" "3840x2160" "30" "60" "10" "10"; then
        ((completed++))
    else
        ((failed++))
    fi
    
    # Summary
    echo ""
    log_info "═══════════════════════════════════════════"
    log_info "  Benchmark Suite Complete  "
    log_info "═══════════════════════════════════════════"
    log_info "Total benchmarks: ${total_benchmarks}"
    log_success "Completed: ${completed}"
    if [ $failed -gt 0 ]; then
        log_error "Failed: ${failed}"
    fi
    echo ""
    
    # Push summary metrics
    local timestamp=$(date +%s)000
    local summary_metrics="benchmark_suite_total{} ${total_benchmarks} ${timestamp}
benchmark_suite_completed{} ${completed} ${timestamp}
benchmark_suite_failed{} ${failed} ${timestamp}"
    
    echo "$summary_metrics" | curl -sf -X POST "${VM_URL}/api/v1/import/prometheus" --data-binary @- > /dev/null 2>&1
    
    log_info "Results available in: ${RESULTS_DIR}"
    log_info "View metrics in Grafana: http://localhost:3000"
    log_info "View VictoriaMetrics: ${VM_URL}"
    
    if [ $failed -gt 0 ]; then
        return 1
    fi
    return 0
}

# Parse command line arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [OPTIONS]"
        echo ""
        echo "Automated performance benchmark runner for FFmpeg RTMP streaming"
        echo ""
        echo "Options:"
        echo "  --help, -h           Show this help message"
        echo "  --results-dir DIR    Set results directory (default: ./test_results)"
        echo "  --vm-url URL         Set VictoriaMetrics URL (default: http://localhost:8428)"
        echo ""
        echo "Environment Variables:"
        echo "  RESULTS_DIR          Override results directory"
        echo "  VM_URL               Override VictoriaMetrics URL"
        echo ""
        echo "Workload Profiles:"
        echo "  1. Laptop      - 720p @30fps, 1000kbps  (low power)"
        echo "  2. Desktop     - 1080p @30fps, 2500kbps (medium power)"
        echo "  3. Single-GPU  - 1080p @60fps, 5000kbps (high power)"
        echo "  4. Dual-GPU    - 4K @30fps, 15000kbps   (very high power)"
        echo ""
        exit 0
        ;;
    --results-dir)
        RESULTS_DIR="$2"
        shift 2
        ;;
    --vm-url)
        VM_URL="$2"
        shift 2
        ;;
esac

# Run the benchmark suite
run_benchmark_suite
exit $?
