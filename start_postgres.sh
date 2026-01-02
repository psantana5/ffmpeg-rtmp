#!/bin/bash
# Quick start script for PostgreSQL deployment with Master and Agent

set -e

echo "=== FFmpeg RTMP - PostgreSQL Deployment ==="
echo ""

# Check if master binary exists
if [ ! -f "./bin/master" ]; then
    echo "❌ Master binary not found. Building..."
    make build-master || { echo "❌ Build failed"; exit 1; }
    echo "✅ Master built successfully"
fi

# Check if agent binary exists
if [ ! -f "./bin/agent" ]; then
    echo "❌ Agent binary not found. Building..."
    make build-agent || { echo "❌ Build failed"; exit 1; }
    echo "✅ Agent built successfully"
fi

# Start PostgreSQL with Docker Compose
echo "🐘 Starting PostgreSQL..."
docker compose -f docker-compose.postgres.yml up -d

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker exec ffmpeg-postgres pg_isready -U ffmpeg >/dev/null 2>&1; then
        echo "✅ PostgreSQL is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ PostgreSQL failed to start"
        exit 1
    fi
    sleep 1
done

# Set environment variables
export DATABASE_TYPE=postgres
export DATABASE_DSN="postgresql://ffmpeg:password@localhost:5432/ffmpeg_rtmp?sslmode=disable"
export MASTER_API_KEY=${MASTER_API_KEY:-$(openssl rand -base64 32)}

echo ""
echo "=== Starting Master Node ==="
echo "Database: PostgreSQL"
echo "DSN: postgresql://ffmpeg:****@localhost:5432/ffmpeg_rtmp"
echo "API Key: ${MASTER_API_KEY:0:10}..."
echo ""

# Start master in background
./bin/master --port 8080 --tls=false &
MASTER_PID=$!

# Wait for master to be ready
echo "⏳ Waiting for master to be ready..."
for i in {1..20}; do
    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        echo "✅ Master is ready!"
        break
    fi
    if [ $i -eq 20 ]; then
        echo "❌ Master failed to start"
        kill $MASTER_PID 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

echo ""
echo "=== Starting Agent (Worker) ==="
echo "Master: https://localhost:8080"
echo "Agent: Starting..."
echo ""

# Start agent in background
./bin/agent \
    --master-url "https://localhost:8080" \
    --api-key "$MASTER_API_KEY" \
    --tls-skip-verify &
AGENT_PID=$!

# Wait a moment for agent to register
sleep 3

echo ""
echo "======================================"
echo "✅ All services started successfully!"
echo "======================================"
echo ""
echo "Services:"
echo "  PostgreSQL:  localhost:5432 (in Docker)"
echo "  Master:      http://localhost:8080"
echo "  Metrics:     http://localhost:9090/metrics"
echo "  Agent:       Running (registered with master)"
echo ""
echo "API Key: $MASTER_API_KEY"
echo ""
echo "Test the deployment:"
echo "  curl http://localhost:8080/health"
echo "  curl http://localhost:8080/nodes -H \"Authorization: Bearer \$MASTER_API_KEY\""
echo ""
echo "Submit a test job:"
echo "  ./bin/ffrtmp jobs submit --master https://localhost:8080 --api-key \$MASTER_API_KEY --scenario test"
echo ""
echo "Stop all services:"
echo "  kill $MASTER_PID $AGENT_PID"
echo "  docker compose -f docker-compose.postgres.yml down"
echo ""
echo "Press Ctrl+C to stop..."
echo ""

# Wait for processes
wait $MASTER_PID $AGENT_PID
