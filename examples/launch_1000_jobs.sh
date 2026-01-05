#!/bin/bash
#
# Example: Launch 1000 jobs to populate the system for testing
#
# Prerequisites:
# 1. Master server must be running (docker compose up -d or ./bin/master)
# 2. At least one worker should be registered
#
# This script demonstrates best practices for submitting large job batches

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=================================================="
echo "  1000 Job Launch Example"
echo "=================================================="
echo ""

# Step 1: Verify master is running
echo "Step 1: Checking master server status..."
MASTER_URL="${MASTER_URL:-http://localhost:8080}"

if ! curl -sf "$MASTER_URL/health" > /dev/null 2>&1; then
    echo "ERROR: Master server is not responding at $MASTER_URL"
    echo ""
    echo "Please start the master server first:"
    echo "  docker compose up -d"
    echo "  OR"
    echo "  ./bin/master -config config.yaml"
    exit 1
fi

echo "✓ Master server is healthy"
echo ""

# Step 2: Check for workers
echo "Step 2: Checking for registered workers..."
WORKER_COUNT=$(curl -sf "$MASTER_URL/nodes" | jq 'if type=="array" then length elif type=="object" and has("nodes") then .nodes | length else 0 end' 2>/dev/null || echo "0")

if [ "$WORKER_COUNT" -eq 0 ]; then
    echo "WARNING: No workers registered. Jobs will queue until a worker connects."
    echo ""
    echo "To register a worker:"
    echo "  docker compose up -d worker"
    echo "  OR"
    echo "  ./bin/worker -master-url $MASTER_URL"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "✓ Found $WORKER_COUNT worker(s) registered"
fi
echo ""

# Step 3: Launch jobs
echo "Step 3: Submitting 1000 jobs..."
echo ""

"$PROJECT_ROOT/scripts/launch_jobs.sh" \
    --count 1000 \
    --master "$MASTER_URL" \
    --scenario random \
    --priority mixed \
    --queue mixed \
    --batch-size 50 \
    --delay 100 \
    --output "job_launch_1000_$(date +%Y%m%d_%H%M%S).json" \
    --verbose

echo ""
echo "=================================================="
echo "  Launch Complete!"
echo "=================================================="
echo ""
echo "Next steps:"
echo "  1. Monitor job execution:"
echo "     watch -n 2 'curl -s $MASTER_URL/jobs | jq \".jobs | group_by(.status) | map({status: .[0].status, count: length})\"'"
echo ""
echo "  2. View master metrics:"
echo "     curl -s $MASTER_URL/metrics"
echo ""
echo "  3. Check Grafana dashboard:"
echo "     http://localhost:3000 (if using docker-compose)"
echo ""
echo "  4. View VictoriaMetrics:"
echo "     http://localhost:8428 (if using docker-compose)"
echo ""
