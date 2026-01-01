#!/bin/bash

set -e

# Populate all queues with jobs to demonstrate the distributed scheduler
# This script creates jobs across all queue types and priorities

echo "╔══════════════════════════════════════════════════════════════════════════╗"
echo "║  📊 QUEUE POPULATION SCRIPT - Distributed Job Scheduler Test           ║"
echo "╚══════════════════════════════════════════════════════════════════════════╝"
echo ""

FFRTMP_BIN="${FFRTMP_BIN:-./bin/ffrtmp}"
MASTER_URL="${MASTER_URL:-http://localhost:8080}"

# Check if master is running
if ! curl -sf "$MASTER_URL/health" > /dev/null 2>&1; then
    echo "❌ Error: Master not running at $MASTER_URL"
    echo "Start with: ./bin/master --port 8080 --metrics-port 9090 --tls=false"
    exit 1
fi

echo "✓ Master is running at $MASTER_URL"
echo ""

# Function to submit job
submit_job() {
    local queue=$1
    local priority=$2
    local scenario=$3
    local duration=$4
    
    echo -n "  Submitting: queue=$queue, priority=$priority, scenario=$scenario ... "
    
    JOB_ID=$($FFRTMP_BIN jobs submit \
        --scenario "$scenario" \
        --queue "$queue" \
        --priority "$priority" \
        --duration "$duration" \
        --master "$MASTER_URL" \
        2>&1 | grep "Job ID:" | awk '{print $NF}' || echo "FAILED")
    
    if [ "$JOB_ID" != "FAILED" ] && [ -n "$JOB_ID" ]; then
        echo "✓ $JOB_ID"
        return 0
    else
        echo "❌ Failed"
        return 1
    fi
}

echo "═══════════════════════════════════════════════════════════════════════════"
echo "  PHASE 1: LIVE Queue (Real-time processing)"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo "Live queue jobs are meant for real-time streaming with tight latency requirements."
echo ""

submit_job "live" "high"   "live-stream-4k"     60
submit_job "live" "high"   "live-stream-1080p"  60
submit_job "live" "medium" "live-stream-720p"   60
submit_job "live" "medium" "live-webinar"       60
submit_job "live" "low"    "live-backup-feed"   60

echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "  PHASE 2: DEFAULT Queue (Standard processing)"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo "Default queue for typical transcoding jobs with balanced priority."
echo ""

submit_job "default" "high"   "urgent-vod-1080p"      120
submit_job "default" "high"   "urgent-vod-720p"       120
submit_job "default" "medium" "standard-vod-1080p"    120
submit_job "default" "medium" "standard-vod-720p"     120
submit_job "default" "medium" "standard-vod-480p"     120
submit_job "default" "low"    "reprocess-old-video"   120
submit_job "default" "low"    "preview-generation"    120

echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "  PHASE 3: BATCH Queue (Bulk processing)"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo "Batch queue for large-scale, non-urgent transcoding operations."
echo ""

submit_job "batch" "high"   "batch-library-migration" 180
submit_job "batch" "medium" "batch-thumbnail-gen"     180
submit_job "batch" "medium" "batch-quality-check"     180
submit_job "batch" "medium" "batch-archive-convert"   180
submit_job "batch" "low"    "batch-backup-encode"     180
submit_job "batch" "low"    "batch-test-quality"      180

echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "  SUMMARY"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""

# Wait a moment for metrics to update
sleep 3

# Get current queue state from master metrics
echo "Current Queue State (from Prometheus metrics):"
echo ""
curl -s http://localhost:9090/metrics 2>/dev/null | grep -E "ffrtmp_queue_" | while read -r line; do
    if [[ $line =~ ffrtmp_queue_by_priority\{priority=\"([^\"]+)\"\}[[:space:]]+([0-9]+) ]]; then
        printf "  Priority %-8s: %s jobs\n" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
    elif [[ $line =~ ffrtmp_queue_by_type\{type=\"([^\"]+)\"\}[[:space:]]+([0-9]+) ]]; then
        printf "  Queue %-11s: %s jobs\n" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
    fi
done

echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo "✅ Queue population complete!"
echo ""
echo "📊 View in Grafana:"
echo "   Dashboard: http://localhost:3000/d/distributed-scheduler"
echo "   Panel: 'Queue by Priority' should show distribution across high/medium/low"
echo "   Panel: 'Queue by Type' should show distribution across live/default/batch"
echo ""
echo "🔍 View jobs:"
echo "   CLI: $FFRTMP_BIN jobs list --master $MASTER_URL"
echo "   API: curl $MASTER_URL/jobs"
echo ""
echo "⚠️  NOTE: Jobs are processing and metrics update in real-time!"
echo "   Refresh Grafana to see live changes (auto-refresh: 5s)"
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
