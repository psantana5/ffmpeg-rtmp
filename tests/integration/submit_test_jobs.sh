#!/bin/bash
# Submit test jobs to different queues and priorities for Grafana visualization

MASTER_URL="${MASTER_URL:-http://localhost:8080}"
API_KEY="${MASTER_API_KEY:-}"
VIDEO_FILE="/home/sanpau/Documents/projects/ffmpeg-rtmp/examples/test1.mp4"

if [ -z "$API_KEY" ]; then
    echo "⚠️  MASTER_API_KEY not set. Please export it first."
    echo "   export MASTER_API_KEY=your-api-key"
    exit 1
fi

echo "🚀 Submitting test jobs to demonstrate queue and priority system..."
echo "Master URL: $MASTER_URL"
echo ""

# Check if video file exists
if [ ! -f "$VIDEO_FILE" ]; then
    echo "⚠️  Test video not found at $VIDEO_FILE"
    echo "Using dummy input instead"
    VIDEO_FILE="testsrc=duration=60:size=1280x720:rate=30"
    INPUT_ARGS="-f lavfi -i $VIDEO_FILE"
else
    INPUT_ARGS="-i $VIDEO_FILE"
fi

# Function to submit a job
submit_job() {
    local name=$1
    local queue=$2
    local priority=$3
    local bitrate=$4
    
    echo "📤 Submitting: $name (queue=$queue, priority=$priority, bitrate=$bitrate)"
    
    curl -s -X POST "$MASTER_URL/jobs" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -d "{
            \"scenario\": \"${name}\",
            \"parameters\": {
                \"input\": \"$VIDEO_FILE\",
                \"output\": \"rtmp://localhost/live/${name}\",
                \"bitrate\": \"$bitrate\",
                \"codec\": \"libx264\"
            },
            \"queue\": \"$queue\",
            \"priority\": \"$priority\"
        }" | jq -r '.id // .job_id // "error"'
    
    sleep 0.5
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1️⃣  Submitting HIGH PRIORITY jobs to LIVE queue"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
submit_job "live-hq-1" "live" "high" "5000k"
submit_job "live-hq-2" "live" "high" "4000k"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2️⃣  Submitting MEDIUM PRIORITY jobs to DEFAULT queue"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
submit_job "vod-med-1" "default" "medium" "3000k"
submit_job "vod-med-2" "default" "medium" "2500k"
submit_job "vod-med-3" "default" "medium" "2000k"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3️⃣  Submitting LOW PRIORITY jobs to BATCH queue"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
submit_job "batch-low-1" "batch" "low" "1500k"
submit_job "batch-low-2" "batch" "low" "1000k"
submit_job "batch-low-3" "batch" "low" "800k"
submit_job "batch-low-4" "batch" "low" "500k"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4️⃣  Mixed priority jobs to test scheduling"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
submit_job "urgent-live" "live" "high" "6000k"
submit_job "normal-vod" "default" "medium" "2000k"
submit_job "archive-batch" "batch" "low" "500k"

echo ""
echo "✅ All jobs submitted!"
echo ""
echo "📊 Check Grafana dashboards:"
echo "   • Distributed Job Scheduler: http://localhost:3000/d/distributed-scheduler"
echo "   • Worker Monitoring: http://localhost:3000/d/worker-monitoring"
echo ""
echo "🔍 Monitor metrics directly:"
echo "   curl http://localhost:9090/metrics | grep ffrtmp_queue"
echo ""
echo "📈 Expected behavior:"
echo "   • Jobs will be processed in order: live/high > default/medium > batch/low"
echo "   • Within same priority: FIFO order"
echo "   • Queue metrics should show distribution across queues and priorities"
