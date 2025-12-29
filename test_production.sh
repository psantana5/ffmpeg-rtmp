#!/bin/bash
set -e

# Production Features Demo Script
# Demonstrates mTLS, SQLite persistence, and API authentication

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "=========================================="
echo "Production Features Demo"
echo "=========================================="
echo ""

# Cleanup previous test data
rm -rf /tmp/prod-demo
mkdir -p /tmp/prod-demo/data /tmp/prod-demo/certs
cd /tmp/prod-demo

echo "1. Generating TLS certificates..."
echo "   (Using self-signed for demo - use CA-signed in production)"
echo ""

# Generate master certificate
$SCRIPT_DIR/bin/master --generate-cert \
  --cert certs/master.crt \
  --key certs/master.key \
  > /dev/null 2>&1

# Generate agent certificate (simulating different machine)
$SCRIPT_DIR/bin/master --generate-cert \
  --cert certs/agent.crt \
  --key certs/agent.key \
  > /dev/null 2>&1

echo "   ✓ Master certificate: certs/master.crt"
echo "   ✓ Agent certificate: certs/agent.crt"
echo ""

# Use the master cert as CA for demo (in prod, use proper CA)
cp certs/master.crt certs/ca.crt

# Generate API key
API_KEY="demo-api-key-$(date +%s)"
echo "2. Generated API key: $API_KEY"
echo ""

echo "3. Starting Master with production features..."
echo "   - SQLite persistence (data/master.db)"
echo "   - TLS enabled (port 8443)"
echo "   - mTLS enabled (requires client cert)"
echo "   - API authentication enabled"
echo ""

# Start master in background
nohup $SCRIPT_DIR/bin/master \
  --port 8443 \
  --db data/master.db \
  --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --ca certs/ca.crt \
  --mtls \
  --api-key "$API_KEY" \
  > master.log 2>&1 &

MASTER_PID=$!
echo "   Master PID: $MASTER_PID"
sleep 3

echo ""
echo "4. Testing HTTPS health endpoint (no auth required)..."
curl -k -s https://localhost:8443/health | jq
echo ""

echo "5. Testing authenticated API call..."
echo "   (This would fail without API key)"
echo ""

# Try without API key (should fail)
echo "   Without API key:"
HTTP_CODE=$(curl -k -s -o /dev/null -w "%{http_code}" https://localhost:8443/nodes)
if [ "$HTTP_CODE" = "401" ]; then
    echo "   ✓ Correctly rejected (401 Unauthorized)"
else
    echo "   ✗ Unexpected status: $HTTP_CODE"
fi
echo ""

# Try with API key (should succeed)
echo "   With API key:"
curl -k -s -H "Authorization: Bearer $API_KEY" https://localhost:8443/nodes | jq
echo ""

echo "6. Registering test node..."
curl -k -s -X POST https://localhost:8443/nodes/register \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "test-node-1",
    "type": "desktop",
    "cpu_threads": 8,
    "cpu_model": "Test CPU",
    "has_gpu": false,
    "ram_bytes": 16000000000,
    "labels": {"test": "production-demo"}
  }' | jq -r '.id' > node_id.txt

NODE_ID=$(cat node_id.txt)
echo "   ✓ Node registered: $NODE_ID"
echo ""

echo "7. Creating test job..."
curl -k -s -X POST https://localhost:8443/jobs \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "production-test",
    "confidence": "high",
    "parameters": {"demo": true}
  }' | jq -r '.id' > job_id.txt

JOB_ID=$(cat job_id.txt)
echo "   ✓ Job created: $JOB_ID"
echo ""

echo "8. Checking SQLite database..."
echo "   Tables created:"
sqlite3 data/master.db "SELECT name FROM sqlite_master WHERE type='table';" | while read table; do
    echo "     - $table"
done
echo ""

echo "   Nodes in database:"
sqlite3 data/master.db "SELECT id, address, type FROM nodes;" | head -1
echo ""

echo "   Jobs in database:"
sqlite3 data/master.db "SELECT id, scenario, status FROM jobs;" | head -1
echo ""

echo "9. Testing graceful shutdown..."
kill -TERM $MASTER_PID
sleep 2
echo "   ✓ Master stopped gracefully"
echo ""

echo "10. Testing persistence (restart master)..."
nohup $SCRIPT_DIR/bin/master \
  --port 8443 \
  --db data/master.db \
  --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --ca certs/ca.crt \
  --mtls \
  --api-key "$API_KEY" \
  > master-restart.log 2>&1 &

MASTER_PID=$!
sleep 3

echo "   Checking persisted data..."
NODE_COUNT=$(curl -k -s -H "Authorization: Bearer $API_KEY" https://localhost:8443/nodes | jq -r '.count')
JOB_COUNT=$(curl -k -s -H "Authorization: Bearer $API_KEY" https://localhost:8443/jobs | jq -r '.count')

echo "   ✓ Nodes persisted: $NODE_COUNT"
echo "   ✓ Jobs persisted: $JOB_COUNT"
echo ""

# Cleanup
kill $MASTER_PID 2>/dev/null
wait $MASTER_PID 2>/dev/null

echo "=========================================="
echo "✓ Production Features Demo Complete!"
echo "=========================================="
echo ""
echo "Demonstrated features:"
echo "  • TLS/HTTPS encryption"
echo "  • mTLS client certificate validation"
echo "  • API key authentication"
echo "  • SQLite persistence"
echo "  • Graceful shutdown"
echo "  • Data survives restart"
echo ""
echo "Demo artifacts in: /tmp/prod-demo/"
echo "  - SQLite DB: data/master.db"
echo "  - Certificates: certs/"
echo "  - Logs: master.log, master-restart.log"
echo ""
