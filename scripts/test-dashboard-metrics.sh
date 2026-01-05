#!/bin/bash
#
# Test script to populate Grafana dashboard metrics
# This script runs test jobs through the system to generate metrics
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Dashboard Metrics Test - Populate Grafana Panels"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Check if master is running
if ! pgrep -f "bin/master" > /dev/null; then
    echo -e "${RED}✗ Master not running${NC}"
    echo "  Start with: ./bin/master --port 8080 --db master.db --metrics"
    exit 1
fi
echo -e "${GREEN}✓${NC} Master running"

# Check if worker is running (on metrics port)
if ! curl -s http://localhost:9091/metrics > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠${NC} Worker metrics not accessible on :9091"
    echo "  Worker may not be running or not exporting metrics"
else
    echo -e "${GREEN}✓${NC} Worker metrics accessible"
fi

# Check VictoriaMetrics
if ! curl -s http://localhost:8428/api/v1/query?query=up > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠${NC} VictoriaMetrics not accessible"
    echo "  Start with: docker compose up -d victoriametrics"
else
    echo -e "${GREEN}✓${NC} VictoriaMetrics collecting metrics"
fi

echo
echo "Submitting test jobs to generate metrics..."
echo

# Submit 10 test jobs
JOB_IDS=()
for i in {1..10}; do
    RESPONSE=$(curl -k -s -X POST https://localhost:8080/jobs \
      -H "Content-Type: application/json" \
      -d "{
        \"scenario\": \"1080p30-h264\",
        \"confidence\": \"high\",
        \"engine\": \"ffmpeg\",
        \"parameters\": {
          \"input\": \"test_${i}.mp4\",
          \"output\": \"output_${i}.mp4\",
          \"duration\": \"3\"
        }
      }" 2>&1)
    
    JOB_ID=$(echo "$RESPONSE" | python3 -c "import json, sys; d=json.load(sys.stdin); print(d.get('id', ''))" 2>/dev/null)
    
    if [ -n "$JOB_ID" ]; then
        echo -e "  ${GREEN}✓${NC} Job $i submitted: ${JOB_ID:0:8}..."
        JOB_IDS+=("$JOB_ID")
    else
        echo -e "  ${RED}✗${NC} Job $i failed"
    fi
    
    sleep 0.5
done

echo
echo "Waiting for jobs to be processed (30 seconds)..."
sleep 30

echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Metrics Status"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Check master metrics
echo "Master Metrics (http://localhost:9090/metrics):"
JOBS_TOTAL=$(curl -s http://localhost:9090/metrics | grep "^ffrtmp_jobs_total" | wc -l)
ACTIVE=$(curl -s http://localhost:9090/metrics | grep "^ffrtmp_active_jobs" | head -1 | awk '{print $2}')
QUEUE=$(curl -s http://localhost:9090/metrics | grep "^ffrtmp_queue_length" | head -1 | awk '{print $2}')

if [ "$JOBS_TOTAL" -gt 0 ]; then
    echo -e "  ${GREEN}✓${NC} Job state metrics: $JOBS_TOTAL states tracked"
else
    echo -e "  ${YELLOW}⚠${NC} Job state metrics: Not yet available"
fi

echo "  Active jobs: ${ACTIVE:-0}"
echo "  Queue length: ${QUEUE:-0}"

echo
echo "Worker Metrics (http://localhost:9091/metrics):"
if curl -s http://localhost:9091/metrics > /dev/null 2>&1; then
    CPU=$(curl -s http://localhost:9091/metrics | grep "^ffrtmp_worker_cpu_usage" | head -1 | awk '{print $2}')
    MEM=$(curl -s http://localhost:9091/metrics | grep "^ffrtmp_worker_memory_bytes" | head -1 | awk '{print $2}')
    ACTIVE_WORKER=$(curl -s http://localhost:9091/metrics | grep "^ffrtmp_worker_active_jobs" | head -1 | awk '{print $2}')
    
    echo -e "  ${GREEN}✓${NC} CPU usage: ${CPU:-N/A}%"
    echo -e "  ${GREEN}✓${NC} Memory: ${MEM:-N/A} bytes"
    echo "  Active jobs on worker: ${ACTIVE_WORKER:-0}"
else
    echo -e "  ${RED}✗${NC} Worker metrics not accessible"
fi

echo
echo "VictoriaMetrics Status:"
VM_METRICS=$(curl -s "http://localhost:8428/api/v1/query?query=up" | python3 -c "import json, sys; d=json.load(sys.stdin); print(len(d['data']['result']))" 2>/dev/null)
echo "  Total targets scraped: ${VM_METRICS:-0}"

echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Dashboard URLs"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo
echo "Grafana: http://localhost:3000 (admin/admin)"
echo
echo "Dashboards:"
echo "  • Production Monitoring:    /d/ffmpeg-rtmp-prod/"
echo "  • Job Scheduler:            /d/job-scheduler/"
echo "  • Worker Monitoring:        /d/worker-monitoring/"
echo "  • Quality Metrics:          /d/quality-metrics/"
echo "  • Cost Analysis:            /d/cost-analysis/"
echo "  • ML Predictions:           /d/ml-predictions/"
echo
echo "VictoriaMetrics UI: http://localhost:8428/vmui"
echo

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Next Steps"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo
echo "1. Open Grafana: http://localhost:3000"
echo "2. Go to Production Monitoring dashboard"
echo "3. Wait 10-30 seconds for metrics to populate"
echo "4. Refresh dashboard if needed"
echo
echo "To generate more metrics:"
echo "  • Run more jobs: curl -X POST https://localhost:8080/jobs ..."
echo "  • Use load testing: ./scripts/load_test.sh quick"
echo "  • Check metrics directly: curl http://localhost:9090/metrics"
echo
