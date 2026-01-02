#!/bin/bash

set -ex

cd /home/sanpau/Documents/projects/ffmpeg-rtmp

echo "Step 1: Build"
make build-distributed build-cli || { echo "Build failed"; exit 1; }

echo "Step 2: Create config"
cat > config.yaml << 'EOF'
master:
  url: https://localhost:8080
  tls:
    enabled: true
    cert_file: certs/server.crt
    key_file: certs/server.key
  auth:
    enabled: true
    secret_key: test-secret-key-for-testing-only
EOF

echo "Step 3: Start master"
./bin/master --db master_test.db > /tmp/master_test.log 2>&1 &
MASTER_PID=$!
echo "Master PID: $MASTER_PID"

echo "Step 4: Wait for master"
sleep 5

echo "Step 5: Check if master is running"
if ps -p $MASTER_PID > /dev/null 2>&1; then
    echo "✓ Master is running"
else
    echo "✗ Master not running"
    cat /tmp/master_test.log
    exit 1
fi

echo "Step 6: Cleanup"
kill $MASTER_PID
rm -f master_test.db*

echo "✓ All steps completed"
