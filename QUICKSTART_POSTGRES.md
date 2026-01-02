# PostgreSQL Quick Start Guide

## TL;DR - The Fastest Way

```bash
# 1. Start PostgreSQL + Master in one command
./start_postgres.sh

# That's it! Master is running with PostgreSQL.
```

---

## Step-by-Step (Manual)

### Option 1: Using the Start Script (Recommended)

```bash
# Clone and build
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-master

# Start everything
./start_postgres.sh
```

The script will:
1. ✅ Start PostgreSQL in Docker
2. ✅ Wait for it to be ready
3. ✅ Start the master node
4. ✅ Show you the API key

### Option 2: Manual Steps

#### Step 1: Start PostgreSQL
```bash
docker compose -f docker-compose.postgres.yml up -d
```

#### Step 2: Wait for PostgreSQL
```bash
# Wait ~10 seconds for PostgreSQL to start
sleep 10

# Verify it's ready
docker exec ffmpeg-postgres pg_isready -U ffmpeg
# Should output: /var/run/postgresql:5432 - accepting connections
```

#### Step 3: Build Master (if not already built)
```bash
make build-master
```

#### Step 4: Start Master with PostgreSQL
```bash
DATABASE_TYPE=postgres \
DATABASE_DSN="postgresql://ffmpeg:password@localhost:5432/ffmpeg_rtmp?sslmode=disable" \
MASTER_API_KEY=$(openssl rand -base64 32) \
./bin/master --port 8080 --tls=false
```

You should see:
```
2026/01/02 12:54:56 Using PostgreSQL database
2026/01/02 12:54:56 ✓ PostgreSQL connected successfully
2026/01/02 12:54:56 Master node listening on :8080
```

---

## Verify It's Working

### Check PostgreSQL
```bash
docker ps
# Should show: ffmpeg-postgres (healthy)
```

### Check Master Health
```bash
curl http://localhost:8080/health
# Should return: {"status":"ok"}
```

### Check Database
```bash
docker exec ffmpeg-postgres psql -U ffmpeg -d ffmpeg_rtmp -c '\dt'
# Should show: jobs and nodes tables
```

---

## Test with a Job

```bash
# Save your API key
export MASTER_API_KEY="your-key-from-startup"

# Submit a test job
curl -X POST http://localhost:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-postgres",
    "confidence": "auto",
    "parameters": {"duration": 30}
  }'

# Check jobs
curl http://localhost:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY"
```

---

## Common Issues

### Issue: "Connection refused"
**Problem**: PostgreSQL not started or not ready yet.

**Solution**:
```bash
# Check if running
docker ps | grep postgres

# If not running, start it
docker compose -f docker-compose.postgres.yml up -d

# Wait for it to be ready
docker exec ffmpeg-postgres pg_isready -U ffmpeg
```

### Issue: "Failed to create PostgreSQL store"
**Problem**: Wrong DSN or PostgreSQL not accessible.

**Solution**:
```bash
# Test connection manually
docker exec ffmpeg-postgres psql -U ffmpeg -d ffmpeg_rtmp -c 'SELECT 1'

# If fails, check logs
docker logs ffmpeg-postgres

# Restart if needed
docker restart ffmpeg-postgres
```

### Issue: "Master already running"
**Problem**: Port 8080 already in use.

**Solution**:
```bash
# Use a different port
./bin/master --port 8081 --tls=false

# Or kill the old one
pkill -f "bin/master"
```

---

## Stop Everything

```bash
# Stop master (Ctrl+C if running in foreground)

# Stop PostgreSQL
docker compose -f docker-compose.postgres.yml down

# Or keep data and just stop
docker stop ffmpeg-postgres
```

---

## Production Deployment

For production, see [POSTGRES_MIGRATION.md](./POSTGRES_MIGRATION.md) for:
- Using managed PostgreSQL (AWS RDS, Google Cloud SQL, etc.)
- SSL/TLS configuration
- Connection pooling tuning
- Backup strategies
- High availability setup

---

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│   Master    │────▶│  PostgreSQL  │◀────│  Worker(s) │
│   :8080     │     │   :5432      │     │            │
└─────────────┘     └──────────────┘     └────────────┘
      │                     │
      └─────────────────────┘
       Schema auto-created
       on first connection
```

- **Master**: Stateless Go application
- **PostgreSQL**: Persistent data store
- **Workers**: Register and execute jobs

---

## What's Next?

1. ✅ PostgreSQL is running
2. ✅ Master is connected
3. 🔄 Register workers: See [worker README](./worker/README.md)
4. 🔄 Submit jobs: Use the API or CLI
5. 🔄 Monitor: Prometheus metrics on :9090

**PostgreSQL deployment is WORKING!** 🎉

For multi-tenancy and enterprise features, continue to Phase 2 in [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md).
