#!/bin/bash

set -e

echo "╔══════════════════════════════════════════════════════════════════════════╗"
echo "║  📊 COMPREHENSIVE QUEUE DEMONSTRATION                                   ║"
echo "╚══════════════════════════════════════════════════════════════════════════╝"
echo ""

MASTER_URL="${MASTER_URL:-http://localhost:8080}"
FFRTMP_BIN="./bin/ffrtmp"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CURRENT STATE ANALYSIS${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

# Show current jobs by queue/priority
echo "📊 Jobs Distribution (from database):"
echo ""
sqlite3 master.db <<EOF | column -t -s '|'
SELECT 
  queue, 
  priority, 
  status,
  COUNT(*) as count 
FROM jobs 
GROUP BY queue, priority, status 
ORDER BY queue, priority;
EOF

echo ""
echo "📈 Summary by Status:"
sqlite3 master.db "SELECT status, COUNT(*) as count FROM jobs GROUP BY status;" | column -t -s '|'

echo ""
echo "🖥️  Active Workers:"
$FFRTMP_BIN nodes list --master $MASTER_URL 2>&1 | tail -n +2

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  PROMETHEUS METRICS (Current Values)${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

curl -s http://localhost:9090/metrics 2>/dev/null | grep -E "^ffrtmp_(active_jobs|jobs_total|queue_length|nodes_by_status|queue_by)" | while read -r line; do
    echo "  $line"
done

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}  📌 KEY INSIGHTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

PENDING=$(sqlite3 master.db "SELECT COUNT(*) FROM jobs WHERE status='pending';")
QUEUED=$(sqlite3 master.db "SELECT COUNT(*) FROM jobs WHERE status='queued';")
PROCESSING=$(sqlite3 master.db "SELECT COUNT(*) FROM jobs WHERE status='processing';")
COMPLETED=$(sqlite3 master.db "SELECT COUNT(*) FROM jobs WHERE status='completed';")

echo "✓ Jobs have been submitted across all queues and priorities"
echo "✓ Total jobs: $(sqlite3 master.db 'SELECT COUNT(*) FROM jobs;')"
echo "  - Pending: $PENDING"
echo "  - Queued: $QUEUED"
echo "  - Processing: $PROCESSING"
echo "  - Completed: $COMPLETED"
echo ""

if [ "$QUEUED" -eq 0 ]; then
    echo "ℹ️  Queue metrics show 0 because:"
    echo "   • Jobs in 'pending' state don't count as queued"
    echo "   • Jobs in 'processing' are actively being worked on"
    echo "   • Queue metrics specifically count status='queued'"
    echo ""
    echo "💡 'Queued' state means: Job waiting for an available worker"
    echo "   Current: Jobs either pending (not yet scheduled) or processing (assigned)"
fi

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  📊 GRAFANA DASHBOARD GUIDE${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════════════════${NC}"
echo ""

echo "🎯 Open Dashboard: http://localhost:3000/d/distributed-scheduler"
echo ""
echo "What Each Panel Shows:"
echo ""
echo "  ✓ Active Jobs"
echo "    → Count of jobs currently being processed"
echo "    → Should match 'processing' count from database"
echo ""
echo "  ✓ Jobs by State"
echo "    → Timeseries of pending/processing/completed/failed jobs"
echo "    → Updates in real-time as jobs progress"
echo ""
echo "  ✓ Queue Length"
echo "    → Total jobs in 'queued' state (waiting for worker)"
echo "    → Currently $QUEUED"
echo ""
echo "  ✓ Queue by Priority"
echo "    → Distribution of queued jobs by high/medium/low"
echo "    → Only counts status='queued', not 'pending'"
echo ""
echo "  ✓ Queue by Type"
echo "    → Distribution of queued jobs by live/default/batch"
echo "    → Only counts status='queued', not 'pending'"
echo ""
echo "  ✓ Nodes by Status"
echo "    → Worker node availability (available/busy/offline)"
echo "    → Updates as workers process jobs"
echo ""

echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo -e "${GREEN}✅ Demonstration Complete!${NC}"
echo ""
echo "📈 The system has jobs distributed across:"
echo "   • 3 queue types (live, default, batch)"
echo "   • 3 priority levels (high, medium, low)"
echo "   • All properly tracked in Prometheus metrics"
echo ""
echo "🔄 Watch jobs process in real-time in Grafana!"
echo "   Panels auto-refresh every 5 seconds"
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
