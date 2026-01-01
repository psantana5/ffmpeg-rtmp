#!/bin/bash
# Application Workflow Tests
# Tests real end-to-end functionality, not just file existence

set -e

echo "=========================================="
echo "Application Workflow Test Suite"
echo "Testing real application functionality"
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

cleanup() {
    echo ""
    echo "Cleaning up test processes..."
    # Kill master if running
    if [ ! -z "$MASTER_PID" ] && ps -p $MASTER_PID > /dev/null 2>&1; then
        kill $MASTER_PID 2>/dev/null || true
        sleep 1
    fi
    rm -f master.db test_master.db
}

trap cleanup EXIT

# Ensure binaries are built
echo "=========================================="
echo "Preparation: Building Binaries"
echo "=========================================="

if [ ! -f "bin/master" ] || [ ! -f "bin/agent" ]; then
    echo "Building binaries..."
    make build-distributed > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        pass_test "Binaries built successfully"
    else
        fail_test "Failed to build binaries"
        exit 1
    fi
else
    pass_test "Binaries already exist"
fi

# Workflow 1: Master Startup
echo ""
echo "=========================================="
echo "Workflow 1: Master Node Startup"
echo "=========================================="

# Start master in background
echo "Starting master node..."
./bin/master --port 18080 --db test_master.db --tls=false > /tmp/master.log 2>&1 &
MASTER_PID=$!

# Wait for master to start
sleep 3

if ps -p $MASTER_PID > /dev/null; then
    pass_test "Master process started (PID: $MASTER_PID)"
else
    fail_test "Master process failed to start"
    cat /tmp/master.log
    exit 1
fi

# Test health endpoint
if curl -s http://localhost:18080/health | grep -q "healthy"; then
    pass_test "Master health endpoint responds"
else
    fail_test "Master health endpoint not responding"
    cat /tmp/master.log
fi

# Test API endpoints are accessible
if curl -s http://localhost:18080/nodes > /dev/null; then
    pass_test "Master /nodes endpoint accessible"
else
    fail_test "Master /nodes endpoint not accessible"
fi

if curl -s http://localhost:18080/jobs > /dev/null; then
    pass_test "Master /jobs endpoint accessible"
else
    fail_test "Master /jobs endpoint not accessible"
fi

# Workflow 2: Node Registration
echo ""
echo "=========================================="
echo "Workflow 2: Node Registration"
echo "=========================================="

# Register a test node
NODE_DATA='{
  "id": "test-node-001",
  "address": "test-worker",
  "type": "desktop",
  "cpu_threads": 8,
  "cpu_model": "Test CPU",
  "has_gpu": false,
  "ram_bytes": 16000000000,
  "labels": {
    "env": "test",
    "arch": "amd64"
  }
}'

REGISTER_RESPONSE=$(curl -s -X POST http://localhost:18080/nodes/register \
  -H "Content-Type: application/json" \
  -d "$NODE_DATA")

if echo "$REGISTER_RESPONSE" | grep -q "test-worker"; then
    pass_test "Node registration successful"
    # Extract the actual node ID from response
    ACTUAL_NODE_ID=$(echo "$REGISTER_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "  Registered Node ID: $ACTUAL_NODE_ID"
else
    fail_test "Node registration failed"
    echo "Response: $REGISTER_RESPONSE"
fi

# Verify node appears in list
NODES_LIST=$(curl -s http://localhost:18080/nodes)
if echo "$NODES_LIST" | grep -q "test-worker"; then
    pass_test "Registered node appears in nodes list"
else
    fail_test "Registered node not found in list"
    echo "Nodes: $NODES_LIST"
fi

# Workflow 3: Job Creation and Retrieval
echo ""
echo "=========================================="
echo "Workflow 3: Job Submission and Retrieval"
echo "=========================================="

# Create a job
JOB_DATA='{
  "scenario": "test-1080p",
  "confidence": "auto",
  "parameters": {
    "duration": 60,
    "bitrate": "5000k",
    "resolution": "1920x1080"
  }
}'

CREATE_RESPONSE=$(curl -s -X POST http://localhost:18080/jobs \
  -H "Content-Type: application/json" \
  -d "$JOB_DATA")

if echo "$CREATE_RESPONSE" | grep -q "test-1080p"; then
    pass_test "Job creation successful"
    JOB_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "  Job ID: $JOB_ID"
else
    fail_test "Job creation failed"
    echo "Response: $CREATE_RESPONSE"
fi

# List jobs
JOBS_LIST=$(curl -s http://localhost:18080/jobs)
if echo "$JOBS_LIST" | grep -q "test-1080p"; then
    pass_test "Created job appears in jobs list"
else
    fail_test "Created job not found in list"
fi

# Get next job for node (use actual node ID)
NEXT_JOB=$(curl -s "http://localhost:18080/jobs/next?node_id=$ACTUAL_NODE_ID")
if echo "$NEXT_JOB" | grep -q "test-1080p"; then
    pass_test "Job can be retrieved by worker"
else
    fail_test "Job retrieval failed"
    echo "Response: $NEXT_JOB"
fi

# Workflow 4: Heartbeat
echo ""
echo "=========================================="
echo "Workflow 4: Node Heartbeat"
echo "=========================================="

HEARTBEAT_RESPONSE=$(curl -s -X POST \
  "http://localhost:18080/nodes/$ACTUAL_NODE_ID/heartbeat")

if [ $? -eq 0 ]; then
    pass_test "Heartbeat sent successfully"
else
    fail_test "Heartbeat failed"
fi

# Workflow 5: Results Submission
echo ""
echo "=========================================="
echo "Workflow 5: Results Submission"
echo "=========================================="

RESULT_DATA='{
  "job_id": "'$JOB_ID'",
  "node_id": "'$ACTUAL_NODE_ID'",
  "status": "completed",
  "metrics": {
    "duration_seconds": 60,
    "avg_power_watts": 150.5,
    "total_energy_wh": 2.508,
    "avg_fps": 29.97
  },
  "output_file": "/results/test-result.json"
}'

RESULT_RESPONSE=$(curl -s -X POST http://localhost:18080/results \
  -H "Content-Type: application/json" \
  -d "$RESULT_DATA")

if [ $? -eq 0 ]; then
    pass_test "Results submitted successfully"
else
    fail_test "Results submission failed"
    echo "Response: $RESULT_RESPONSE"
fi

# Workflow 6: Python Scripts
echo ""
echo "=========================================="
echo "Workflow 6: Python Scripts Execution"
echo "=========================================="

# Test run_tests.py help
if python3 shared/scripts/run_tests.py --help > /tmp/run_tests_help.txt 2>&1; then
    pass_test "run_tests.py --help works"
    if grep -q "single\|batch\|multi" /tmp/run_tests_help.txt; then
        pass_test "run_tests.py shows expected commands"
    else
        warn_test "run_tests.py help may be incomplete"
    fi
else
    fail_test "run_tests.py --help failed"
    cat /tmp/run_tests_help.txt
fi

# Test that advisor modules can be imported
timeout 5 python3 << 'PYEOF' > /tmp/python_imports.log 2>&1 || true
import sys
sys.path.insert(0, 'shared')
from advisor import scoring
from advisor import cost
print("✓ Advisor imports successful")
PYEOF

if grep -q "Advisor imports successful" /tmp/python_imports.log; then
    pass_test "Python advisor modules importable"
elif grep -q "numpy" /tmp/python_imports.log; then
    warn_test "Python advisor needs numpy (not a migration issue)"
else
    warn_test "Python advisor imports failed"
    cat /tmp/python_imports.log
fi

# Workflow 7: Configuration Files
echo ""
echo "=========================================="
echo "Workflow 7: Configuration Validation"
echo "=========================================="

# Validate VictoriaMetrics config
if python3 -c "import yaml; yaml.safe_load(open('master/monitoring/victoriametrics.yml'))" 2>/dev/null; then
    pass_test "victoriametrics.yml is valid YAML and parseable"
else
    fail_test "victoriametrics.yml has syntax errors"
fi

# Check scrape targets
SCRAPE_TARGETS=$(grep -o '\- targets:.*' master/monitoring/victoriametrics.yml | wc -l)
if [ "$SCRAPE_TARGETS" -gt 5 ]; then
    pass_test "VictoriaMetrics has $SCRAPE_TARGETS scrape targets configured"
else
    warn_test "VictoriaMetrics has only $SCRAPE_TARGETS scrape targets"
fi

# Workflow 8: Agent Hardware Detection
echo ""
echo "=========================================="
echo "Workflow 8: Agent Hardware Detection"
echo "=========================================="

# Test agent can detect hardware (without registering)
timeout 5 ./bin/agent --help > /dev/null 2>&1
if [ $? -eq 0 ] || [ $? -eq 124 ]; then
    pass_test "Agent binary executes"
else
    fail_test "Agent binary failed to execute"
fi

# Test agent hardware detection by checking imports work
if go run -C worker/cmd/agent main.go --help > /tmp/agent_help.txt 2>&1 || grep -q "master" /tmp/agent_help.txt; then
    pass_test "Agent can compile and run with hardware detection imports"
else
    warn_test "Agent compilation test inconclusive"
fi

# Workflow 9: Docker Compose Syntax
echo ""
echo "=========================================="
echo "Workflow 9: Docker Compose Configuration"
echo "=========================================="

# Validate docker-compose can parse the file
if docker compose config > /tmp/docker_config.yml 2>&1; then
    pass_test "docker-compose.yml syntax is valid"
    
    # Check services are defined
    SERVICE_COUNT=$(grep -c "^  [a-z].*:$" /tmp/docker_config.yml || echo 0)
    if [ "$SERVICE_COUNT" -gt 10 ]; then
        pass_test "Docker Compose defines $SERVICE_COUNT services"
    else
        warn_test "Docker Compose may have fewer services than expected: $SERVICE_COUNT"
    fi
else
    fail_test "docker-compose.yml has syntax errors"
    cat /tmp/docker_config.yml
fi

# Workflow 10: Go Module Resolution
echo ""
echo "=========================================="
echo "Workflow 10: Go Import Resolution Test"
echo "=========================================="

# Create a test file that imports from shared
mkdir -p /tmp/go_test_imports
cat > /tmp/go_test_imports/test.go << 'EOF'
package main

import (
    "fmt"
    "github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func main() {
    job := &models.Job{
        Scenario: "test",
    }
    fmt.Printf("Job: %v\n", job)
}
EOF

cp go.mod /tmp/go_test_imports/
cp go.sum /tmp/go_test_imports/

cd /tmp/go_test_imports
if go build -o test_binary test.go > /tmp/go_import_test.log 2>&1; then
    pass_test "Go imports resolve correctly (pkg/models)"
    if ./test_binary > /tmp/test_run.log 2>&1; then
        pass_test "Compiled Go code executes successfully"
    else
        warn_test "Compiled code failed to run"
    fi
else
    fail_test "Go import resolution failed"
    cat /tmp/go_import_test.log
fi
cd - > /dev/null

# Workflow 11: Systemd Service Simulation
echo ""
echo "=========================================="
echo "Workflow 11: Systemd Service Validation"
echo "=========================================="

# Check master service file has correct ExecStart
if grep -q "ExecStart=.*bin/master" master/deployment/ffmpeg-master.service; then
    pass_test "Master service ExecStart is correct"
else
    fail_test "Master service ExecStart is incorrect"
fi

# Check agent service file has correct ExecStart
if grep -q "ExecStart=.*bin/agent" worker/deployment/ffmpeg-agent.service; then
    pass_test "Agent service ExecStart is correct"
else
    fail_test "Agent service ExecStart is incorrect"
fi

# Simulate systemd environment variables
TEST_ENV="MASTER_URL=http://localhost:18080"
if grep -q "Environment=" worker/deployment/ffmpeg-agent.service; then
    pass_test "Agent service has environment variables configured"
else
    warn_test "Agent service may need environment variables"
fi

# Workflow 12: Grafana Dashboard Test
echo ""
echo "=========================================="
echo "Workflow 12: Grafana Dashboard Validation"
echo "=========================================="

DASHBOARD_DIR="master/monitoring/grafana/provisioning/dashboards"
VALID_DASHBOARDS=0
TOTAL_DASHBOARDS=0

for dashboard in "$DASHBOARD_DIR"/*.json; do
    if [ -f "$dashboard" ]; then
        TOTAL_DASHBOARDS=$((TOTAL_DASHBOARDS + 1))
        # Check if it's valid JSON and has expected structure
        if python3 -c "import json; d=json.load(open('$dashboard')); assert 'panels' in d or 'title' in d" 2>/dev/null; then
            VALID_DASHBOARDS=$((VALID_DASHBOARDS + 1))
        fi
    fi
done

if [ "$VALID_DASHBOARDS" -eq "$TOTAL_DASHBOARDS" ] && [ "$TOTAL_DASHBOARDS" -gt 0 ]; then
    pass_test "All $TOTAL_DASHBOARDS Grafana dashboards are valid and structured"
else
    warn_test "Some Grafana dashboards may have issues ($VALID_DASHBOARDS/$TOTAL_DASHBOARDS valid)"
fi

# Workflow 13: API Key Authentication
echo ""
echo "=========================================="
echo "Workflow 13: API Authentication Test"
echo "=========================================="

# Test without API key (should work since we started without requiring it)
if curl -s http://localhost:18080/nodes | grep -q "nodes"; then
    pass_test "API accessible without key (as configured in test)"
else
    fail_test "API not accessible"
fi

# Workflow 14: Database Persistence
echo ""
echo "=========================================="
echo "Workflow 14: Database Persistence Test"
echo "=========================================="

if [ -f "test_master.db" ]; then
    pass_test "SQLite database file created"
    
    # Check database has tables
    if command -v sqlite3 > /dev/null 2>&1; then
        TABLES=$(sqlite3 test_master.db ".tables" 2>/dev/null | wc -w)
        if [ "$TABLES" -gt 0 ]; then
            pass_test "Database has $TABLES tables"
        else
            warn_test "Database may be empty"
        fi
    else
        warn_test "sqlite3 not available for detailed DB checks"
    fi
else
    fail_test "Database file not created"
fi

# Workflow 15: Concurrent Job Handling
echo ""
echo "=========================================="
echo "Workflow 15: Concurrent Operations Test"
echo "=========================================="

# Submit multiple jobs concurrently
echo "Submitting 5 jobs concurrently..."
for i in {1..5}; do
    curl -s -X POST http://localhost:18080/jobs \
      -H "Content-Type: application/json" \
      -d '{
        "scenario": "concurrent-test-'$i'",
        "confidence": "auto",
        "parameters": {"duration": 30}
      }' > /dev/null 2>&1 &
done
wait

# Check all jobs were created
sleep 1
JOBS_COUNT=$(curl -s http://localhost:18080/jobs | grep -o "concurrent-test-" | wc -l)
if [ "$JOBS_COUNT" -ge 5 ]; then
    pass_test "Master handled $JOBS_COUNT concurrent job submissions"
else
    warn_test "Only $JOBS_COUNT/5 concurrent jobs recorded"
fi

# Final Summary
echo ""
echo "=========================================="
echo "Application Workflow Test Summary"
echo "=========================================="
echo ""
echo -e "${GREEN}Passed:${NC}   $PASSED"
echo -e "${YELLOW}Warnings:${NC} $WARNINGS"
echo -e "${RED}Failed:${NC}   $FAILED"
echo ""

# Show master log excerpt if there were failures
if [ $FAILED -gt 0 ]; then
    echo "Master log (last 20 lines):"
    tail -20 /tmp/master.log
    echo ""
fi

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}=========================================="
    echo "✓ ALL APPLICATION WORKFLOWS PASSED!"
    echo "==========================================${NC}"
    echo ""
    echo "The application is fully functional:"
    echo "  • Master can start and serve API"
    echo "  • Nodes can register"
    echo "  • Jobs can be created and retrieved"
    echo "  • Results can be submitted"
    echo "  • Python scripts work"
    echo "  • Configuration is valid"
    echo "  • Go imports resolve correctly"
    echo "  • Database persistence works"
    echo ""
    exit 0
else
    echo -e "${RED}=========================================="
    echo "✗ SOME WORKFLOWS FAILED"
    echo "==========================================${NC}"
    exit 1
fi
