#!/bin/bash
# Quick start script for PostgreSQL deployment

set -e

echo "=== FFmpeg RTMP - PostgreSQL Deployment ==="
echo ""

# Check if master binary exists
if [ ! -f "./bin/master" ]; then
    echo "❌ Master binary not found. Building..."
    make build-master || { echo "❌ Build failed"; exit 1; }
    echo "✅ Master built successfully"
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

# Start master
./bin/master --port 8080 --tls=false "$@"
