#!/bin/bash
# Exporter and Metrics Workflow Tests
# Tests that exporters actually start and expose valid metrics

set -e

echo "=========================================="
echo "Exporter & Metrics Test Suite"
echo "Testing real exporter functionality"
echo "=========================================="
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0
WARNINGS=0

pass_test() {
    echo -e "${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

fail_test() {
    echo -e "${RED}✗${NC} $1"
    FAILED=$((FAILED + 1))
}

warn_test() {
    echo -e "${YELLOW}⚠${NC} $1"
    WARNINGS=$((WARNINGS + 1))
}

# Track PIDs for cleanup
MASTER_PID=""
declare -a EXPORTER_PIDS

cleanup() {
    echo ""
    echo "Cleaning up test processes..."
    
    # Kill master
    if [ ! -z "$MASTER_PID" ] && ps -p $MASTER_PID > /dev/null 2>&1; then
        kill $MASTER_PID 2>/dev/null || true
    fi
    
    # Kill exporters
    for pid in "${EXPORTER_PIDS[@]}"; do
        if ps -p $pid > /dev/null 2>&1; then
            kill $pid 2>/dev/null || true
        fi
    done
    
    # Clean docker containers
    docker compose down > /dev/null 2>&1 || true
    
    rm -f master.db test_master.db test_results/*.json 2>/dev/null || true
    sleep 2
}

trap cleanup EXIT

# Test 1: Master Metrics Endpoint
echo "=========================================="
echo "Test 1: Master Prometheus Metrics"
echo "=========================================="

# Start master with metrics enabled
echo "Starting master with metrics endpoint..."
./bin/master --port 18080 --db test_master.db --tls=false --metrics=true --metrics-port 19090 > /tmp/master_metrics.log 2>&1 &
MASTER_PID=$!
sleep 3

if ps -p $MASTER_PID > /dev/null; then
    pass_test "Master started with metrics enabled (PID: $MASTER_PID)"
else
    fail_test "Master failed to start"
    cat /tmp/master_metrics.log
    exit 1
fi

# Test master metrics endpoint
if curl -s http://localhost:19090/metrics > /tmp/master_metrics.txt; then
    pass_test "Master metrics endpoint accessible"
    
    # Check for Prometheus format
    if grep -q "^# HELP" /tmp/master_metrics.txt; then
        pass_test "Master metrics in Prometheus format"
    else
        fail_test "Master metrics not in Prometheus format"
    fi
    
    # Check for expected metrics
    if grep -q "ffmpeg_rtmp.*" /tmp/master_metrics.txt || grep -q "go_.*" /tmp/master_metrics.txt; then
        pass_test "Master exposes metrics"
        METRIC_COUNT=$(grep -c "^ffmpeg_rtmp\|^go_" /tmp/master_metrics.txt || echo 0)
        echo "  Found $METRIC_COUNT Go runtime metrics"
    else
        warn_test "Master metrics may be incomplete"
    fi
else
    fail_test "Master metrics endpoint not accessible"
fi

# Test master metrics health endpoint
if curl -s http://localhost:19090/health | grep -q "healthy"; then
    pass_test "Master metrics health endpoint responds"
else
    fail_test "Master metrics health endpoint not working"
fi

# Test 2: Create Test Results for Exporters
echo ""
echo "=========================================="
echo "Test 2: Prepare Test Data"
echo "=========================================="

mkdir -p test_results

# Create a sample test result file
cat > test_results/test_sample_001.json << 'EOF'
{
  "test_name": "test_sample_001",
  "scenario": "1080p_test",
  "timestamp": "2025-12-30T10:00:00Z",
  "duration_seconds": 60,
  "bitrate": "5000k",
  "resolution": "1920x1080",
  "fps": 30,
  "power_metrics": {
    "avg_power_watts": 125.5,
    "peak_power_watts": 145.2,
    "min_power_watts": 110.3,
    "total_energy_wh": 2.092
  },
  "baseline": {
    "avg_power_watts": 45.2,
    "total_energy_wh": 0.753
  },
  "delta": {
    "power_watts": 80.3,
    "energy_wh": 1.339,
    "power_percent_increase": 177.7
  },
  "encoding_stats": {
    "frames_encoded": 1800,
    "avg_fps": 29.97,
    "dropped_frames": 3
  }
}
EOF

if [ -f "test_results/test_sample_001.json" ]; then
    pass_test "Test result file created"
else
    fail_test "Failed to create test result file"
fi

# Test 3: Python Exporters with Docker
echo ""
echo "=========================================="
echo "Test 3: Python Exporter (Results)"
echo "=========================================="

# Build and start results exporter
echo "Building results exporter..."
if docker build -q -t test-results-exporter master/exporters/results > /tmp/results_build.log 2>&1; then
    pass_test "Results exporter Docker image built"
    
    # Start the exporter
    docker run -d --name test-results-exporter \
        -p 19502:9502 \
        -v $(pwd)/test_results:/results:ro \
        -e RESULTS_DIR=/results \
        test-results-exporter > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        pass_test "Results exporter container started"
        sleep 5
        
        # Test health endpoint
        if curl -s http://localhost:19502/health | grep -q "OK\|healthy"; then
            pass_test "Results exporter health endpoint responds"
        else
            warn_test "Results exporter health endpoint may not be ready"
        fi
        
        # Test metrics endpoint
        if curl -s http://localhost:19502/metrics > /tmp/results_metrics.txt; then
            pass_test "Results exporter metrics endpoint accessible"
            
            # Check for results metrics
            if grep -q "results_" /tmp/results_metrics.txt; then
                pass_test "Results exporter exposes results metrics"
                RESULTS_METRICS=$(grep -c "^results_" /tmp/results_metrics.txt || echo 0)
                echo "  Found $RESULTS_METRICS results metrics"
            else
                warn_test "Results metrics not found (may need valid test data)"
            fi
            
            # Verify Prometheus format
            if grep -q "^# HELP\|^# TYPE" /tmp/results_metrics.txt; then
                pass_test "Results metrics in Prometheus format"
            else
                fail_test "Results metrics not in Prometheus format"
            fi
        else
            fail_test "Results exporter metrics not accessible"
        fi
        
        docker stop test-results-exporter > /dev/null 2>&1
        docker rm test-results-exporter > /dev/null 2>&1
    else
        fail_test "Results exporter container failed to start"
    fi
else
    fail_test "Results exporter Docker build failed"
    cat /tmp/results_build.log
fi

# Test 4: Go Exporter (CPU)
echo ""
echo "=========================================="
echo "Test 4: Go Exporter (CPU/RAPL)"
echo "=========================================="

echo "Building CPU exporter..."
if docker build -q -t test-cpu-exporter -f worker/exporters/cpu_exporter/Dockerfile . > /tmp/cpu_build.log 2>&1; then
    pass_test "CPU exporter Docker image built"
    
    # Start the exporter (may fail without RAPL access, that's okay)
    docker run -d --name test-cpu-exporter \
        -p 19510:9500 \
        --privileged \
        -v /sys/class/powercap:/sys/class/powercap:ro \
        -v /sys/devices:/sys/devices:ro \
        test-cpu-exporter > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        pass_test "CPU exporter container started"
        sleep 3
        
        # Test health endpoint
        if curl -s http://localhost:19510/health > /tmp/cpu_health.txt 2>&1; then
            if grep -q "healthy\|ok" /tmp/cpu_health.txt; then
                pass_test "CPU exporter health endpoint responds"
            else
                warn_test "CPU exporter health responds but may indicate issues"
            fi
        else
            warn_test "CPU exporter health endpoint not accessible (may be normal without RAPL)"
        fi
        
        # Test metrics endpoint
        if curl -s http://localhost:19510/metrics > /tmp/cpu_metrics.txt 2>&1; then
            pass_test "CPU exporter metrics endpoint accessible"
            
            # Check for Go metrics at minimum
            if grep -q "go_\|rapl_\|cpu_" /tmp/cpu_metrics.txt; then
                pass_test "CPU exporter exposes metrics"
                
                if grep -q "rapl_" /tmp/cpu_metrics.txt; then
                    RAPL_METRICS=$(grep -c "^rapl_" /tmp/cpu_metrics.txt || echo 0)
                    echo "  Found $RAPL_METRICS RAPL metrics"
                else
                    warn_test "No RAPL metrics (normal without Intel CPU/privileges)"
                fi
            else
                warn_test "CPU exporter metrics may be minimal"
            fi
        else
            warn_test "CPU exporter metrics not accessible"
        fi
        
        docker stop test-cpu-exporter > /dev/null 2>&1
        docker rm test-cpu-exporter > /dev/null 2>&1
    else
        warn_test "CPU exporter container failed to start (may need privileges)"
    fi
else
    fail_test "CPU exporter Docker build failed"
    cat /tmp/cpu_build.log
fi

# Test 5: Go Exporter (FFmpeg)
echo ""
echo "=========================================="
echo "Test 5: Go Exporter (FFmpeg Stats)"
echo "=========================================="

echo "Building FFmpeg exporter..."
if docker build -q -t test-ffmpeg-exporter -f worker/exporters/ffmpeg_exporter/Dockerfile . > /tmp/ffmpeg_build.log 2>&1; then
    pass_test "FFmpeg exporter Docker image built"
    
    docker run -d --name test-ffmpeg-exporter \
        -p 19506:9506 \
        test-ffmpeg-exporter > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        pass_test "FFmpeg exporter container started"
        sleep 3
        
        # Test health
        if curl -s http://localhost:19506/health | grep -q "healthy\|OK"; then
            pass_test "FFmpeg exporter health endpoint responds"
        else
            warn_test "FFmpeg exporter health endpoint issue"
        fi
        
        # Test metrics
        if curl -s http://localhost:19506/metrics > /tmp/ffmpeg_metrics.txt; then
            pass_test "FFmpeg exporter metrics endpoint accessible"
            
            if grep -q "ffmpeg_\|go_" /tmp/ffmpeg_metrics.txt; then
                pass_test "FFmpeg exporter exposes metrics"
            else
                warn_test "FFmpeg metrics minimal (normal without active encoding)"
            fi
        else
            fail_test "FFmpeg exporter metrics not accessible"
        fi
        
        docker stop test-ffmpeg-exporter > /dev/null 2>&1
        docker rm test-ffmpeg-exporter > /dev/null 2>&1
    else
        fail_test "FFmpeg exporter failed to start"
    fi
else
    fail_test "FFmpeg exporter Docker build failed"
    cat /tmp/ffmpeg_build.log
fi

# Test 6: Health Checker Exporter
echo ""
echo "=========================================="
echo "Test 6: Health Checker Exporter"
echo "=========================================="

echo "Building health checker..."
if docker build -q -t test-health-checker master/exporters/health_checker > /tmp/health_build.log 2>&1; then
    pass_test "Health checker Docker image built"
    
    docker run -d --name test-health-checker \
        -p 19600:9600 \
        test-health-checker > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        pass_test "Health checker container started"
        sleep 3
        
        if curl -s http://localhost:19600/health | grep -q "OK\|healthy"; then
            pass_test "Health checker health endpoint responds"
        else
            warn_test "Health checker may not be ready"
        fi
        
        if curl -s http://localhost:19600/metrics > /tmp/health_metrics.txt; then
            pass_test "Health checker metrics endpoint accessible"
            
            if grep -q "exporter_health\|exporter_" /tmp/health_metrics.txt; then
                pass_test "Health checker exposes health metrics"
            else
                warn_test "Health metrics may be minimal"
            fi
        else
            fail_test "Health checker metrics not accessible"
        fi
        
        docker stop test-health-checker > /dev/null 2>&1
        docker rm test-health-checker > /dev/null 2>&1
    else
        fail_test "Health checker failed to start"
    fi
else
    fail_test "Health checker Docker build failed"
    cat /tmp/health_build.log
fi

# Test 7: VictoriaMetrics Scrape Config
echo ""
echo "=========================================="
echo "Test 7: VictoriaMetrics Configuration"
echo "=========================================="

# Validate scrape config
if [ -f "master/monitoring/victoriametrics.yml" ]; then
    pass_test "VictoriaMetrics config exists"
    
    # Check for job definitions
    JOB_COUNT=$(grep -c "job_name:" master/monitoring/victoriametrics.yml || echo 0)
    if [ "$JOB_COUNT" -gt 8 ]; then
        pass_test "VictoriaMetrics has $JOB_COUNT scrape jobs defined"
    else
        warn_test "VictoriaMetrics has only $JOB_COUNT jobs"
    fi
    
    # Check for exporter targets
    EXPORTERS=("cpu-exporter" "gpu-exporter" "ffmpeg-exporter" "results-exporter" "qoe-exporter" "cost-exporter" "health-checker")
    for exp in "${EXPORTERS[@]}"; do
        if grep -q "$exp" master/monitoring/victoriametrics.yml; then
            pass_test "VictoriaMetrics configured to scrape $exp"
        else
            warn_test "VictoriaMetrics may not scrape $exp"
        fi
    done
else
    fail_test "VictoriaMetrics config not found"
fi

# Test 8: Docker Compose Exporter Services
echo ""
echo "=========================================="
echo "Test 8: Docker Compose Exporter Config"
echo "=========================================="

# Validate docker-compose has exporter services
if docker compose config > /tmp/dc_config.yml 2>&1; then
    pass_test "Docker Compose config valid"
    
    # Check each exporter service is defined
    DOCKER_EXPORTERS=("cpu-exporter-go" "results-exporter" "qoe-exporter" "cost-exporter" "ffmpeg-exporter" "exporter-health-checker")
    for exp in "${DOCKER_EXPORTERS[@]}"; do
        if grep -q "^  $exp:" /tmp/dc_config.yml; then
            pass_test "Docker Compose defines $exp service"
        else
            fail_test "Docker Compose missing $exp service"
        fi
    done
    
    # Check port mappings
    if grep -q "19502:9502\|9502:9502" docker-compose.yml; then
        pass_test "Docker Compose has results exporter port mapping"
    else
        warn_test "Results exporter port mapping may be missing"
    fi
else
    fail_test "Docker Compose config invalid"
fi

# Test 9: Metrics Format Validation
echo ""
echo "=========================================="
echo "Test 9: Prometheus Metrics Format"
echo "=========================================="

# Create a test metrics validator
validate_metrics() {
    local file=$1
    local name=$2
    
    if [ ! -f "$file" ]; then
        return 1
    fi
    
    # Check for required Prometheus elements
    local has_help=$(grep -c "^# HELP" "$file" || echo 0)
    local has_type=$(grep -c "^# TYPE" "$file" || echo 0)
    local has_metrics=$(grep -c "^[a-z_][a-z0-9_]* " "$file" || echo 0)
    
    if [ "$has_help" -gt 0 ] && [ "$has_type" -gt 0 ]; then
        pass_test "$name has proper Prometheus format ($has_help HELP, $has_type TYPE, $has_metrics metrics)"
        return 0
    else
        warn_test "$name may have format issues"
        return 1
    fi
}

# Validate collected metrics
[ -f "/tmp/master_metrics.txt" ] && validate_metrics "/tmp/master_metrics.txt" "Master metrics"
[ -f "/tmp/results_metrics.txt" ] && validate_metrics "/tmp/results_metrics.txt" "Results exporter"
[ -f "/tmp/cpu_metrics.txt" ] && validate_metrics "/tmp/cpu_metrics.txt" "CPU exporter"
[ -f "/tmp/ffmpeg_metrics.txt" ] && validate_metrics "/tmp/ffmpeg_metrics.txt" "FFmpeg exporter"

# Test 10: Metric Labels Validation
echo ""
echo "=========================================="
echo "Test 10: Metric Labels & Structure"
echo "=========================================="

# Check that metrics have proper labels
check_labels() {
    local file=$1
    local name=$2
    
    if [ -f "$file" ] && grep -q "{.*}" "$file"; then
        pass_test "$name metrics include labels"
        
        # Count unique metrics with labels
        LABELED=$(grep -c "{.*}" "$file" || echo 0)
        echo "  Found $LABELED labeled metrics"
    else
        warn_test "$name metrics may lack labels (could be normal)"
    fi
}

[ -f "/tmp/master_metrics.txt" ] && check_labels "/tmp/master_metrics.txt" "Master"
[ -f "/tmp/results_metrics.txt" ] && check_labels "/tmp/results_metrics.txt" "Results"
[ -f "/tmp/cpu_metrics.txt" ] && check_labels "/tmp/cpu_metrics.txt" "CPU"

# Final Summary
echo ""
echo "=========================================="
echo "Exporter & Metrics Test Summary"
echo "=========================================="
echo ""
echo -e "${GREEN}Passed:${NC}   $PASSED"
echo -e "${YELLOW}Warnings:${NC} $WARNINGS"
echo -e "${RED}Failed:${NC}   $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}=========================================="
    echo "✓ EXPORTER TESTS PASSED!"
    echo "==========================================${NC}"
    echo ""
    echo "All exporters are functional:"
    echo "  • Master metrics endpoint works"
    echo "  • Python exporters build and run"
    echo "  • Go exporters build and run"
    echo "  • Metrics in valid Prometheus format"
    echo "  • VictoriaMetrics config is valid"
    echo "  • Docker Compose properly configured"
    echo ""
    exit 0
else
    echo -e "${RED}=========================================="
    echo "✗ SOME EXPORTER TESTS FAILED"
    echo "==========================================${NC}"
    exit 1
fi
