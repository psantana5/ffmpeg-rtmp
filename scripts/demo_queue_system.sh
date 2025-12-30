#!/bin/bash
#
# Demo script showing the queue system in action
#

set -e

MASTER_URL="http://localhost:8080"
METRICS_URL="http://localhost:9090"

echo "=================================="
echo "Queue System Demo"
echo "=================================="
echo ""

echo "1. Current System Status:"
echo "-------------------------"
curl -s "$METRICS_URL/metrics" | grep -E "(ffrtmp_queue_length|ffrtmp_jobs_total|ffrtmp_nodes_by_status)" | grep -v "^#"
echo ""

echo "2. Jobs by State:"
echo "-------------------------"
curl -s "$METRICS_URL/metrics" | grep "ffrtmp_jobs_total{state=" | awk -F'"' '{print $2 ": " $0}' | awk '{print $1 " " $NF}'
echo ""

echo "3. Queue Breakdown by Priority:"
echo "-------------------------"
curl -s "$METRICS_URL/metrics" | grep "ffrtmp_queue_by_priority{priority=" | awk -F'"' '{print $2 ": " $0}' | awk '{print $1 " " $NF}'
echo ""

echo "4. Queue Breakdown by Type:"
echo "-------------------------"
curl -s "$METRICS_URL/metrics" | grep "ffrtmp_queue_by_type{type=" | awk -F'"' '{print $2 ": " $0}' | awk '{print $1 " " $NF}'
echo ""

echo "5. Worker Nodes Status:"
echo "-------------------------"
curl -s "$METRICS_URL/metrics" | grep "ffrtmp_nodes_by_status{status=" | awk -F'"' '{print $2 ": " $0}' | awk '{print $1 " " $NF}'
echo ""

echo "=================================="
echo "✅ Demo Complete!"
echo "=================================="
echo ""
echo "Open Grafana to see real-time visualization:"
echo "  URL: http://localhost:3000"
echo "  Dashboard: 'Distributed Job Scheduler'"
echo ""
