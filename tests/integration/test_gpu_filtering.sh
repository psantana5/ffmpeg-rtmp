#!/bin/bash
# Test GPU Filtering and Hardware Awareness
# Tests: GPU job assignment filtering, hardware capabilities

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}================================================${NC}"
echo -e "${YELLOW}Testing GPU Filtering & Hardware Awareness${NC}"
echo -e "${YELLOW}================================================${NC}"

# Configuration
MASTER_URL="${MASTER_URL:-http://localhost:${PORT:-8080}}"
API_KEY="${MASTER_API_KEY:-test-api-key}"
TEST_DB="/tmp/test_gpu_filtering.db"

# Cleanup
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ -n "$MASTER_PID" ]; then
        kill $MASTER_PID 2>/dev/null || true
    fi
    rm -f "$TEST_DB" "$TEST_DB-shm" "$TEST_DB-wal"
}
trap cleanup EXIT

# Start master
echo -e "\n${YELLOW}1. Starting master node...${NC}"
cd "$PROJECT_ROOT"
rm -f "$TEST_DB" "$TEST_DB-shm" "$TEST_DB-wal"

go build -o /tmp/test-master ./master/cmd/master
MASTER_API_KEY="$API_KEY" /tmp/test-master \
    --db "$TEST_DB" \
    --port ${PORT:-8080} \
    --tls=false \
    --metrics=false &
MASTER_PID=$!
sleep 2

if ! kill -0 $MASTER_PID 2>/dev/null; then
    echo -e "${RED}✗ Master failed to start${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Master started${NC}"

# Register CPU-only worker
echo -e "\n${YELLOW}2. Registering CPU-only worker...${NC}"
CPU_NODE_RESP=$(curl -s -X POST "$MASTER_URL/nodes/register" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "address": "cpu-worker",
        "type": "server",
        "cpu_threads": 16,
        "cpu_model": "Intel Xeon",
        "has_gpu": false,
        "ram_total_bytes": 34359738368
    }')

CPU_NODE_ID=$(echo "$CPU_NODE_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
if [ -n "$CPU_NODE_ID" ]; then
    echo -e "${GREEN}✓ Registered CPU-only worker (ID: $CPU_NODE_ID)${NC}"
else
    echo -e "${RED}✗ Failed to register CPU worker${NC}"
    exit 1
fi

# Register GPU worker
echo -e "\n${YELLOW}3. Registering GPU-enabled worker...${NC}"
GPU_NODE_RESP=$(curl -s -X POST "$MASTER_URL/nodes/register" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "address": "gpu-worker",
        "type": "server",
        "cpu_threads": 8,
        "cpu_model": "Intel i7",
        "has_gpu": true,
        "gpu_type": "NVIDIA RTX 3090",
        "gpu_capabilities": ["nvenc_h264", "nvenc_h265", "nvenc_av1"],
        "ram_total_bytes": 17179869184
    }')

GPU_NODE_ID=$(echo "$GPU_NODE_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
if [ -n "$GPU_NODE_ID" ]; then
    echo -e "${GREEN}✓ Registered GPU worker (ID: $GPU_NODE_ID)${NC}"
else
    echo -e "${RED}✗ Failed to register GPU worker${NC}"
    exit 1
fi

# Verify node details endpoint
echo -e "\n${YELLOW}4. Testing GET /nodes/{id} endpoint...${NC}"
GPU_NODE_DETAIL=$(curl -s "$MASTER_URL/nodes/$GPU_NODE_ID" -H "Authorization: Bearer $API_KEY")

# Check GPU capabilities are returned
if echo "$GPU_NODE_DETAIL" | grep -q "gpu_capabilities"; then
    echo -e "${GREEN}✓ Node detail includes GPU capabilities${NC}"
    echo "   GPU capabilities: $(echo "$GPU_NODE_DETAIL" | grep -o '"gpu_capabilities":\[[^]]*\]')"
else
    echo -e "${RED}✗ GPU capabilities not found in node detail${NC}"
    exit 1
fi

# Check hardware info is present
if echo "$GPU_NODE_DETAIL" | grep -q "ram_total_bytes"; then
    RAM_TOTAL=$(echo "$GPU_NODE_DETAIL" | grep -o '"ram_total_bytes":[0-9]*' | cut -d':' -f2)
    RAM_GB=$((RAM_TOTAL / 1073741824))
    echo -e "${GREEN}✓ Hardware info present: ${RAM_GB}GB RAM${NC}"
else
    echo -e "${RED}✗ Hardware info incomplete${NC}"
    exit 1
fi

# Test 1: Software encoder job (should work on any node)
echo -e "\n${YELLOW}5. Testing software encoder job assignment...${NC}"

echo "   Creating software encoder job (libx264)..."
SW_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "scenario": "1080p-software",
        "queue": "default",
        "priority": "medium",
        "parameters": {
            "codec": "libx264",
            "bitrate": "5M",
            "preset": "medium"
        }
    }' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# CPU worker should be able to get it
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$CPU_NODE_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$SW_JOB" ]; then
    echo -e "${GREEN}✓ Software encoder job assigned to CPU-only worker${NC}"
else
    echo -e "${RED}✗ Job assignment failed${NC}"
    exit 1
fi

# Complete the job to free the worker
curl -s -X POST "$MASTER_URL/results" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
        \"job_id\": \"$SW_JOB\",
        \"node_id\": \"$CPU_NODE_ID\",
        \"status\": \"completed\",
        \"completed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
    }" > /dev/null

# Test 2: GPU encoder job (should only go to GPU worker)
echo -e "\n${YELLOW}6. Testing GPU encoder job filtering...${NC}"

echo "   Creating NVENC job (requires GPU)..."
GPU_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "scenario": "4k-nvenc",
        "queue": "live",
        "priority": "high",
        "parameters": {
            "codec": "h264_nvenc",
            "bitrate": "20M",
            "preset": "p4"
        }
    }' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

echo "   Attempting to assign to CPU-only worker..."
# CPU worker should NOT get the GPU job
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$CPU_NODE_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$ASSIGNED_ID" ] || [ "$ASSIGNED_ID" != "$GPU_JOB" ]; then
    echo -e "${GREEN}✓ GPU job NOT assigned to CPU-only worker (correct filtering)${NC}"
else
    echo -e "${RED}✗ GPU job incorrectly assigned to CPU-only worker${NC}"
    exit 1
fi

echo "   Attempting to assign to GPU worker..."
# GPU worker SHOULD get it
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$GPU_NODE_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$GPU_JOB" ]; then
    echo -e "${GREEN}✓ GPU job correctly assigned to GPU-enabled worker${NC}"
else
    echo -e "${RED}✗ GPU job not assigned to GPU worker: $ASSIGNED_ID${NC}"
    exit 1
fi

# Test 3: Hardware acceleration parameter
echo -e "\n${YELLOW}7. Testing hwaccel parameter filtering...${NC}"

# Complete GPU job first
curl -s -X POST "$MASTER_URL/results" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
        \"job_id\": \"$GPU_JOB\",
        \"node_id\": \"$GPU_NODE_ID\",
        \"status\": \"completed\",
        \"completed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
    }" > /dev/null

sleep 1

echo "   Creating job with hwaccel=cuda..."
HWACCEL_JOB=$(curl -s -X POST "$MASTER_URL/jobs" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "scenario": "4k-hwaccel",
        "queue": "default",
        "priority": "medium",
        "parameters": {
            "codec": "libx264",
            "hwaccel": "cuda",
            "bitrate": "15M"
        }
    }' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

# CPU worker should NOT get it
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$CPU_NODE_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$ASSIGNED_ID" ] || [ "$ASSIGNED_ID" != "$HWACCEL_JOB" ]; then
    echo -e "${GREEN}✓ hwaccel job NOT assigned to CPU-only worker${NC}"
else
    echo -e "${RED}✗ hwaccel job incorrectly assigned to CPU worker${NC}"
    exit 1
fi

# GPU worker SHOULD get it
NEXT=$(curl -s "$MASTER_URL/jobs/next?node_id=$GPU_NODE_ID" -H "Authorization: Bearer $API_KEY")
ASSIGNED_ID=$(echo "$NEXT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$ASSIGNED_ID" = "$HWACCEL_JOB" ]; then
    echo -e "${GREEN}✓ hwaccel job correctly assigned to GPU worker${NC}"
else
    echo -e "${RED}✗ hwaccel job not assigned properly${NC}"
    exit 1
fi

# Summary
echo -e "\n${YELLOW}================================================${NC}"
echo -e "${GREEN}✓ All GPU Filtering Tests Passed!${NC}"
echo -e "${YELLOW}================================================${NC}"
echo -e "Tested:"
echo -e "  • Node registration with GPU capabilities"
echo -e "  • GET /nodes/{id} returns hardware details"
echo -e "  • Software encoder jobs work on any node"
echo -e "  • NVENC jobs filtered to GPU nodes only"
echo -e "  • hwaccel parameter triggers GPU filtering"
echo -e "  • CPU-only workers don't receive GPU jobs"
