#!/bin/bash

set -e

echo "╔══════════════════════════════════════════════════════════════════════════╗"
echo "║  🚀 QUEUE DEMONSTRATION - Real-time Scheduler Testing                   ║"
echo "╚══════════════════════════════════════════════════════════════════════════╝"
echo ""

MASTER_URL="http://localhost:8080"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  STEP 1: Stop all workers to force jobs into queue${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

echo "❌ Stopping all worker agents..."
pkill -f "agent --master" 2>/dev/null || echo "  No agents running"
sleep 2

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}  STEP 2: Submit jobs across all queues and priorities${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

# Create test video
if [ ! -f /tmp/test_input.mp4 ]; then
    echo "📹 Creating test video..."
    ffmpeg -f lavfi -i testsrc=duration=30:size=1280x720:rate=30 \
           -pix_fmt yuv420p -c:v libx264 -t 30 /tmp/test_input.mp4 -y 2>&1 | tail -3
fi

echo ""
echo "📤 Submitting 9 jobs (3 queues × 3 priorities)..."
echo ""

# Live queue jobs
echo "  • Live/High priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_live_high.mp4",
    "format": "mp4",
    "bitrate": "2000k",
    "queue": "live",
    "priority": "high"
  }' | jq -r '.job_id' > /tmp/job_live_high.txt

echo "  • Live/Medium priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_live_med.mp4",
    "format": "mp4",
    "bitrate": "2000k",
    "queue": "live",
    "priority": "medium"
  }' | jq -r '.job_id' > /tmp/job_live_med.txt

echo "  • Live/Low priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_live_low.mp4",
    "format": "mp4",
    "bitrate": "2000k",
    "queue": "live",
    "priority": "low"
  }' | jq -r '.job_id' > /tmp/job_live_low.txt

# Default queue jobs
echo "  • Default/High priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_default_high.mp4",
    "format": "mp4",
    "bitrate": "1500k",
    "queue": "default",
    "priority": "high"
  }' | jq -r '.job_id' > /tmp/job_default_high.txt

echo "  • Default/Medium priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_default_med.mp4",
    "format": "mp4",
    "bitrate": "1500k",
    "queue": "default",
    "priority": "medium"
  }' | jq -r '.job_id' > /tmp/job_default_med.txt

echo "  • Default/Low priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_default_low.mp4",
    "format": "mp4",
    "bitrate": "1500k",
    "queue": "default",
    "priority": "low"
  }' | jq -r '.job_id' > /tmp/job_default_low.txt

# Batch queue jobs
echo "  • Batch/High priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_batch_high.mp4",
    "format": "mp4",
    "bitrate": "1000k",
    "queue": "batch",
    "priority": "high"
  }' | jq -r '.job_id' > /tmp/job_batch_high.txt

echo "  • Batch/Medium priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_batch_med.mp4",
    "format": "mp4",
    "bitrate": "1000k",
    "queue": "batch",
    "priority": "medium"
  }' | jq -r '.job_id' > /tmp/job_batch_med.txt

echo "  • Batch/Low priority..."
curl -s -X POST $MASTER_URL/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/out_batch_low.mp4",
    "format": "mp4",
    "bitrate": "1000k",
    "queue": "batch",
    "priority": "low"
  }' | jq -r '.job_id' > /tmp/job_batch_low.txt

echo ""
echo -e "${GREEN}✅ All 9 jobs submitted${NC}"
sleep 2

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}  STEP 3: Wait for scheduler to detect no workers and queue jobs${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

echo "⏳ Waiting 10 seconds for scheduler to run (interval: 5s)..."
for i in {10..1}; do
    echo -ne "  $i seconds remaining...\r"
    sleep 1
done
echo ""

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  STEP 4: Check queue metrics${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

echo "📊 Database Status:"
sqlite3 master.db <<EOF | column -t -s '|'
SELECT 
  status, 
  queue, 
  priority, 
  COUNT(*) as count 
FROM jobs 
GROUP BY status, queue, priority 
ORDER BY 
  CASE queue WHEN 'live' THEN 1 WHEN 'default' THEN 2 WHEN 'batch' THEN 3 END,
  CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 END;
EOF

echo ""
echo "📈 Prometheus Metrics:"
curl -s http://localhost:9090/metrics 2>/dev/null | grep -E "^ffrtmp_(queue_length|queue_by)" || echo "  No queue metrics yet"

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  ✅ DEMONSTRATION COMPLETE${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

echo "📌 What Happened:"
echo "  1. Submitted 9 jobs while no workers were running"
echo "  2. Jobs started in 'pending' state"
echo "  3. Scheduler detected no available workers"
echo "  4. Jobs transitioned to 'queued' state"
echo ""

echo "🎯 Check Grafana Dashboard:"
echo "  URL: http://localhost:3000/d/distributed-scheduler"
echo "  You should now see:"
echo "    • Queue Length = 9"
echo "    • Queue by Priority showing distribution"
echo "    • Queue by Type showing distribution"
echo ""

echo "🚀 Next Steps:"
echo "  1. Start a worker: ./bin/agent --master http://localhost:8080"
echo "  2. Watch jobs process in priority order (live>default>batch, high>med>low)"
echo "  3. Monitor in real-time via Grafana"
echo ""

echo "═══════════════════════════════════════════════════════════════════════════"
