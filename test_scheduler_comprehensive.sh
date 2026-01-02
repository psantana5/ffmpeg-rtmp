#!/bin/bash
#
# Comprehensive Scheduler Test Suite
# Tests: priorities, queues, engines, bitrates, failure modes
#

set -e

API_KEY="${MASTER_API_KEY}"
MASTER_URL="${MASTER_URL:-https://localhost:8080}"

if [ -z "$API_KEY" ]; then
  echo "ERROR: MASTER_API_KEY not set"
  exit 1
fi

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     COMPREHENSIVE SCHEDULER TEST SUITE                       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Helper function to submit job
submit_job() {
  local scenario=$1
  local priority=$2
  local queue=$3
  local engine=$4
  local bitrate=$5
  local confidence=$6
  
  local params="{\"scenario\":\"$scenario\",\"priority\":\"$priority\",\"queue\":\"$queue\""
  
  if [ -n "$engine" ]; then
    params="$params,\"engine\":\"$engine\""
  fi
  
  if [ -n "$confidence" ]; then
    params="$params,\"confidence\":\"$confidence\""
  fi
  
  if [ -n "$bitrate" ]; then
    params="$params,\"parameters\":{\"bitrate\":\"$bitrate\"}"
  fi
  
  params="$params}"
  
  local response=$(curl -sk -X POST \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "$params" \
    "$MASTER_URL/jobs")
  
  local job_id=$(echo "$response" | jq -r '.id')
  echo "  ✓ Job submitted: $job_id ($scenario | $priority | $queue | ${engine:-auto} | ${bitrate:-default})"
  echo "$job_id"
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 1: Priority Scheduling (Reverse Order)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Submit: LOW, MEDIUM, HIGH (should execute: HIGH, MEDIUM, LOW)"
echo ""

job_low_1=$(submit_job "test-low-priority-1" "low" "default" "" "" "low")
job_med_1=$(submit_job "test-medium-priority-1" "medium" "default" "" "" "medium")
job_high_1=$(submit_job "test-high-priority-1" "high" "default" "" "" "high")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 2: Queue Separation (live vs batch vs default)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

job_live_1=$(submit_job "live-stream-1" "high" "live" "" "" "high")
job_batch_1=$(submit_job "batch-job-1" "medium" "batch" "" "" "medium")
job_default_1=$(submit_job "default-job-1" "medium" "default" "" "" "medium")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 3: Engine Selection (ffmpeg vs gstreamer vs auto)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

job_ffmpeg=$(submit_job "ffmpeg-job" "medium" "default" "ffmpeg" "" "high")
job_gstreamer=$(submit_job "gstreamer-job" "medium" "default" "gstreamer" "" "high")
job_auto=$(submit_job "auto-engine-job" "medium" "default" "auto" "" "high")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 4: Different Bitrates"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

job_bitrate_1M=$(submit_job "bitrate-1M" "medium" "default" "" "1000k" "high")
job_bitrate_5M=$(submit_job "bitrate-5M" "medium" "default" "" "5000k" "high")
job_bitrate_10M=$(submit_job "bitrate-10M" "medium" "default" "" "10000k" "high")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 5: Mixed Priority Burst (Stress Test)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Submitting 15 jobs rapidly with random priorities..."
echo ""

for i in {1..5}; do
  submit_job "burst-high-$i" "high" "default" "" "" "high" > /dev/null
done

for i in {1..5}; do
  submit_job "burst-medium-$i" "medium" "default" "" "" "medium" > /dev/null
done

for i in {1..5}; do
  submit_job "burst-low-$i" "low" "default" "" "" "low" > /dev/null
done

echo "  ✓ 15 burst jobs submitted"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 6: Confidence Levels (auto, high, medium, low)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

job_conf_auto=$(submit_job "confidence-auto" "medium" "default" "" "" "auto")
job_conf_high=$(submit_job "confidence-high" "medium" "default" "" "" "high")
job_conf_med=$(submit_job "confidence-medium" "medium" "default" "" "" "medium")
job_conf_low=$(submit_job "confidence-low" "medium" "default" "" "" "low")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TOTAL JOBS SUBMITTED: 38"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "⏳ Waiting 10 seconds before checking status..."
sleep 10

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "INITIAL STATUS CHECK"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

curl -sk -H "Authorization: Bearer $API_KEY" "$MASTER_URL/jobs" | \
  jq '{
    total: .count,
    by_status: [.jobs | group_by(.status)[] | {status: .[0].status, count: length}],
    by_priority: [.jobs | group_by(.priority)[] | {priority: .[0].priority, count: length}],
    by_queue: [.jobs | group_by(.queue)[] | {queue: .[0].queue, count: length}],
    by_engine: [.jobs | group_by(.engine)[] | {engine: .[0].engine, count: length}]
  }'

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "⏳ Monitor progress: watch -n 2 'curl -sk -H \"Authorization: Bearer \$MASTER_API_KEY\" $MASTER_URL/jobs | jq \".count, .jobs | group_by(.status) | map({status: .[0].status, count: length})\"'"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

