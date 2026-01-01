#!/bin/bash
# Startup script for FFmpeg-RTMP Distributed System
# Starts master and worker nodes with optimal configuration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Color codes for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Default configuration
MASTER_PORT="${MASTER_PORT:-8080}"
DB_PATH="${DB_PATH:-master.db}"
MAX_RETRIES="${MAX_RETRIES:-3}"
SCHEDULER_INTERVAL="${SCHEDULER_INTERVAL:-5s}"
ENABLE_METRICS="${ENABLE_METRICS:-true}"
METRICS_PORT="${METRICS_PORT:-9090}"
AGENT_METRICS_PORT="${AGENT_METRICS_PORT:-9091}"
ENABLE_TRACING="${ENABLE_TRACING:-false}"
HEARTBEAT_INTERVAL="${HEARTBEAT_INTERVAL:-30s}"
POLL_INTERVAL="${POLL_INTERVAL:-10s}"

# API key setup
if [ -z "$MASTER_API_KEY" ]; then
    export MASTER_API_KEY="dev-key-$(date +%s)"
    echo -e "${YELLOW}⚠ No MASTER_API_KEY set, using temporary key: $MASTER_API_KEY${NC}"
fi

# Function to check if port is in use
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to wait for service
wait_for_service() {
    local url=$1
    local max_attempts=30
    local attempt=0
    
    echo -n "Waiting for service at $url..."
    while [ $attempt -lt $max_attempts ]; do
        if curl -k -s "$url/health" >/dev/null 2>&1; then
            echo -e " ${GREEN}✓${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo -e " ${RED}✗${NC}"
    return 1
}

# Check if binaries exist
if [ ! -f "bin/master" ]; then
    echo -e "${YELLOW}Master binary not found, building...${NC}"
    make build-master
fi

if [ ! -f "bin/agent" ]; then
    echo -e "${YELLOW}Agent binary not found, building...${NC}"
    make build-agent
fi

if [ ! -f "bin/ffrtmp" ]; then
    echo -e "${YELLOW}CLI binary not found, building...${NC}"
    make build-cli
fi

# Stop existing processes
echo -e "${YELLOW}Checking for existing processes...${NC}"
if check_port $MASTER_PORT; then
    echo -e "${YELLOW}Master port $MASTER_PORT in use, stopping existing process...${NC}"
    pkill -f "bin/master" || true
    sleep 2
fi

if pgrep -f "bin/agent" >/dev/null 2>&1; then
    echo -e "${YELLOW}Agent running, stopping existing process...${NC}"
    pkill -f "bin/agent" || true
    sleep 2
fi

# Ensure required directories exist
mkdir -p logs certs test_results streams

# Generate certificates if needed
if [ ! -f "certs/master.crt" ] || [ ! -f "certs/master.key" ]; then
    echo -e "${YELLOW}Generating master certificates...${NC}"
    ./bin/master --generate-cert --cert certs/master.crt --key certs/master.key \
        --cert-hosts "localhost,$(hostname)" \
        --cert-ips "127.0.0.1,$(hostname -I | awk '{print $1}')" >/dev/null 2>&1
    echo -e "${GREEN}✓ Master certificates generated${NC}"
fi

if [ ! -f "certs/agent.crt" ] || [ ! -f "certs/agent.key" ]; then
    echo -e "${YELLOW}Generating agent certificates...${NC}"
    # Use master to generate agent certs
    ./bin/master --generate-cert --cert certs/agent.crt --key certs/agent.key \
        --cert-hosts "localhost,$(hostname)" \
        --cert-ips "127.0.0.1,$(hostname -I | awk '{print $1}')" >/dev/null 2>&1
    echo -e "${GREEN}✓ Agent certificates generated${NC}"
fi

# Start Master Node
echo -e "${GREEN}Starting Master Node...${NC}"
echo "  Port: $MASTER_PORT"
echo "  Database: $DB_PATH"
echo "  Max Retries: $MAX_RETRIES"
echo "  Metrics: $ENABLE_METRICS (port $METRICS_PORT)"
echo "  Tracing: $ENABLE_TRACING"

./bin/master \
    --port "$MASTER_PORT" \
    --db "$DB_PATH" \
    --tls \
    --cert certs/master.crt \
    --key certs/master.key \
    --max-retries "$MAX_RETRIES" \
    --scheduler-interval "$SCHEDULER_INTERVAL" \
    --metrics="$ENABLE_METRICS" \
    --metrics-port "$METRICS_PORT" \
    --tracing="$ENABLE_TRACING" \
    > logs/master.log 2>&1 &

MASTER_PID=$!
echo -e "${GREEN}✓ Master started (PID: $MASTER_PID)${NC}"

# Wait for master to be ready
if ! wait_for_service "https://localhost:$MASTER_PORT"; then
    echo -e "${RED}✗ Master failed to start. Check logs/master.log${NC}"
    exit 1
fi

# Start Worker Agent
echo -e "${GREEN}Starting Worker Agent...${NC}"
echo "  Master: https://localhost:$MASTER_PORT"
echo "  Heartbeat Interval: $HEARTBEAT_INTERVAL"
echo "  Poll Interval: $POLL_INTERVAL"
echo "  Metrics Port: $AGENT_METRICS_PORT"

./bin/agent \
    --master "https://localhost:$MASTER_PORT" \
    --ca certs/master.crt \
    --cert certs/agent.crt \
    --key certs/agent.key \
    --register \
    --heartbeat-interval "$HEARTBEAT_INTERVAL" \
    --poll-interval "$POLL_INTERVAL" \
    --metrics-port "$AGENT_METRICS_PORT" \
    --skip-confirmation \
    > logs/agent.log 2>&1 &

AGENT_PID=$!
echo -e "${GREEN}✓ Agent started (PID: $AGENT_PID)${NC}"

# Wait for agent to be ready (give it a few seconds to register)
sleep 3

# Display status
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  FFmpeg-RTMP Distributed System Started!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo "Master Node:"
echo "  • API: https://localhost:$MASTER_PORT"
echo "  • Health: https://localhost:$MASTER_PORT/health"
if [ "$ENABLE_METRICS" = "true" ]; then
    echo "  • Metrics: http://localhost:$METRICS_PORT/metrics"
fi
echo "  • Logs: logs/master.log"
echo "  • PID: $MASTER_PID"
echo ""
echo "Worker Agent:"
echo "  • Master: https://localhost:$MASTER_PORT"
echo "  • Metrics: http://localhost:$AGENT_METRICS_PORT/metrics"
echo "  • Logs: logs/agent.log"
echo "  • PID: $AGENT_PID"
echo ""
echo "CLI Commands:"
echo "  • List nodes:  ./bin/ffrtmp nodes list"
echo "  • Submit job:  ./bin/ffrtmp jobs submit --scenario test-720p"
echo "  • Job status:  ./bin/ffrtmp jobs status"
echo "  • Job logs:    ./bin/ffrtmp jobs logs <job-id>"
echo ""
echo "Environment:"
echo "  • API Key: $MASTER_API_KEY"
echo "  • Database: $DB_PATH"
echo ""
echo -e "${YELLOW}To stop the system: pkill -f 'bin/(master|agent)'${NC}"
echo -e "${YELLOW}To view logs: tail -f logs/master.log logs/agent.log${NC}"
echo ""

# Verify node registration
echo -n "Verifying node registration..."
sleep 1
NODE_COUNT=$(./bin/ffrtmp nodes list --output json 2>/dev/null | jq -r '.count // 0' 2>/dev/null || echo "0")

if [ "$NODE_COUNT" -gt 0 ]; then
    echo -e " ${GREEN}✓ ($NODE_COUNT node(s) registered)${NC}"
    ./bin/ffrtmp nodes list
else
    echo -e " ${YELLOW}⚠ No nodes registered yet${NC}"
fi

echo ""
echo -e "${GREEN}System ready!${NC}"
