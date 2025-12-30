#!/bin/bash
set -e

# Integration test for distributed compute system

echo "=================================="
echo "Distributed Compute Integration Test"
echo "=================================="
echo ""

# Check if master is running
echo "1. Checking master health..."
HEALTH=$(curl -s http://localhost:8080/health)
if echo "$HEALTH" | grep -q "healthy"; then
    echo "   ✓ Master is healthy"
else
    echo "   ✗ Master is not running"
    exit 1
fi
echo ""

# Test node registration via curl (simulating agent)
echo "2. Registering test node..."
NODE_RESPONSE=$(curl -s -X POST http://localhost:8080/nodes/register \
    -H "Content-Type: application/json" \
    -d '{
        "address": "test-node-1",
        "type": "desktop",
        "cpu_threads": 8,
        "cpu_model": "Test CPU",
        "has_gpu": false,
        "ram_bytes": 16000000000,
        "labels": {
            "test": "true"
        }
    }')

NODE_ID=$(echo "$NODE_RESPONSE" | jq -r '.id')
if [ -z "$NODE_ID" ] || [ "$NODE_ID" = "null" ]; then
    echo "   ✗ Failed to register node"
    echo "   Response: $NODE_RESPONSE"
    exit 1
fi
echo "   ✓ Node registered with ID: $NODE_ID"
echo ""

# List nodes
echo "3. Listing registered nodes..."
NODES=$(curl -s http://localhost:8080/nodes | jq -r '.count')
if [ "$NODES" -eq 1 ]; then
    echo "   ✓ Found $NODES registered node"
else
    echo "   ✗ Expected 1 node, found $NODES"
    exit 1
fi
echo ""

# Create a test job
echo "4. Creating test job..."
JOB_RESPONSE=$(curl -s -X POST http://localhost:8080/jobs \
    -H "Content-Type: application/json" \
    -d '{
        "scenario": "test-1080p",
        "confidence": "auto",
        "parameters": {
            "duration": 60,
            "bitrate": "5000k"
        }
    }')

JOB_ID=$(echo "$JOB_RESPONSE" | jq -r '.id')
if [ -z "$JOB_ID" ] || [ "$JOB_ID" = "null" ]; then
    echo "   ✗ Failed to create job"
    echo "   Response: $JOB_RESPONSE"
    exit 1
fi
echo "   ✓ Job created with ID: $JOB_ID"
echo ""

# Get next job (simulating agent polling)
echo "5. Getting next job for node..."
NEXT_JOB=$(curl -s "http://localhost:8080/jobs/next?node_id=$NODE_ID")
ASSIGNED_JOB_ID=$(echo "$NEXT_JOB" | jq -r '.id')
if [ "$ASSIGNED_JOB_ID" != "$JOB_ID" ]; then
    echo "   ✗ Job assignment failed"
    echo "   Response: $NEXT_JOB"
    exit 1
fi
echo "   ✓ Job $JOB_ID assigned to node $NODE_ID"
echo ""

# Send job results
echo "6. Sending job results..."
RESULT_RESPONSE=$(curl -s -X POST http://localhost:8080/results \
    -H "Content-Type: application/json" \
    -d "{
        \"job_id\": \"$JOB_ID\",
        \"node_id\": \"$NODE_ID\",
        \"status\": \"completed\",
        \"metrics\": {
            \"cpu_usage\": 75.5,
            \"memory_usage\": 2048,
            \"duration\": 5.0
        },
        \"analyzer_output\": {
            \"scenario\": \"test-1080p\",
            \"recommendation\": \"optimal\"
        },
        \"completed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
    }")

if echo "$RESULT_RESPONSE" | grep -q "success"; then
    echo "   ✓ Results sent successfully"
else
    echo "   ✗ Failed to send results"
    echo "   Response: $RESULT_RESPONSE"
    exit 1
fi
echo ""

# Try to get another job (should be none)
echo "7. Checking for additional jobs..."
NEXT_JOB=$(curl -s "http://localhost:8080/jobs/next?node_id=$NODE_ID")
if echo "$NEXT_JOB" | grep -q '"job":null'; then
    echo "   ✓ No more jobs available (as expected)"
else
    echo "   ⚠ Unexpected response: $NEXT_JOB"
fi
echo ""

echo "=================================="
echo "✓ All integration tests passed!"
echo "=================================="
echo ""
echo "Summary:"
echo "  • Master node: running"
echo "  • Node registration: working"
echo "  • Job creation: working"
echo "  • Job dispatch: working"
echo "  • Results submission: working"
echo ""
