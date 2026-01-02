# PostgreSQL Migration Guide

## Overview

This guide shows you how to migrate from SQLite to PostgreSQL for production deployments. PostgreSQL provides:

- **Better scalability**: Handle 100K+ jobs
- **Better concurrency**: No locking issues with multiple masters
- **Better performance**: Optimized for high-throughput workloads
- **HA ready**: Required for multi-master deployments

---

## Quick Start (Docker)

The fastest way to test PostgreSQL:

```bash
# Start PostgreSQL + Master with Docker Compose
docker compose -f docker-compose.postgres.yml up -d

# Check logs
docker logs ffmpeg-master

# Verify connection
docker exec ffmpeg-postgres psql -U ffmpeg -d ffmpeg_rtmp -c '\dt'
```

---

## Production Deployment

### Step 1: Set Up PostgreSQL

#### Option A: Docker
```bash
docker run -d \
  --name postgres \
  -e POSTGRES_DB=ffmpeg_rtmp \
  -e POSTGRES_USER=ffmpeg \
  -e POSTGRES_PASSWORD=your_secure_password \
  -p 5432:5432 \
  -v postgres_data:/var/lib/postgresql/data \
  postgres:15-alpine
```

#### Option B: Native Installation
```bash
# Ubuntu/Debian
sudo apt install postgresql-15

# Create database
sudo -u postgres psql << EOF
CREATE DATABASE ffmpeg_rtmp;
CREATE USER ffmpeg WITH PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE ffmpeg_rtmp TO ffmpeg;
\q
EOF
```

#### Option C: Managed Service (Recommended for Production)
- AWS RDS PostgreSQL
- Google Cloud SQL
- Azure Database for PostgreSQL
- DigitalOcean Managed Databases

### Step 2: Configure Master

Create `config-postgres.yaml`:

```yaml
database:
  type: postgres
  dsn: "postgresql://ffmpeg:password@localhost:5432/ffmpeg_rtmp?sslmode=require"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  conn_max_idle_time: 1m

api:
  port: 8080
  tls_enabled: true

scheduler:
  enabled: true
  scheduling_interval: 5s
  cleanup_interval: 30s
```

### Step 3: Run Master with PostgreSQL

```bash
# Set environment variables (alternative to config file)
export DATABASE_TYPE=postgres
export DATABASE_DSN="postgresql://ffmpeg:password@localhost:5432/ffmpeg_rtmp?sslmode=require"
export MASTER_API_KEY=$(openssl rand -base64 32)

# Start master
./bin/master --config config-postgres.yaml
```

Or use environment variables directly:

```bash
DATABASE_TYPE=postgres \
DATABASE_DSN="postgresql://ffmpeg:password@localhost:5432/ffmpeg_rtmp?sslmode=require" \
./bin/master
```

---

## Migration from SQLite

### Option 1: Fresh Start (Recommended)

If you don't need to preserve existing jobs:

```bash
# Backup SQLite (just in case)
cp master.db master.db.backup

# Start fresh with PostgreSQL
rm master.db  # Remove SQLite database
DATABASE_TYPE=postgres ./bin/master
```

### Option 2: Migrate Existing Data

Coming soon: Migration tool to copy data from SQLite to PostgreSQL.

For now, if you need to preserve data:

```bash
# Export from SQLite
sqlite3 master.db << EOF
.mode insert jobs
.output jobs.sql
SELECT * FROM jobs WHERE status != 'completed';
.quit
EOF

# Import to PostgreSQL (manual adaptation needed)
psql -U ffmpeg -d ffmpeg_rtmp < jobs_adapted.sql
```

---

## Connection String Format

PostgreSQL connection strings (DSN) follow this format:

```
postgresql://[user[:password]@][host][:port][/dbname][?param1=value1&...]
```

### Examples:

```bash
# Local development (no SSL)
postgresql://ffmpeg:password@localhost:5432/ffmpeg_rtmp?sslmode=disable

# Production (with SSL)
postgresql://ffmpeg:password@db.example.com:5432/ffmpeg_rtmp?sslmode=require

# AWS RDS
postgresql://ffmpeg:password@mydb.abc123.us-east-1.rds.amazonaws.com:5432/ffmpeg_rtmp?sslmode=require

# Connection pooling parameters
postgresql://ffmpeg:password@localhost/ffmpeg_rtmp?pool_max_conns=25&pool_min_conns=5
```

---

## Performance Tuning

### Connection Pool Settings

Tune based on your workload:

```yaml
database:
  max_open_conns: 25      # Max concurrent connections
  max_idle_conns: 5       # Idle connections to keep
  conn_max_lifetime: 5m   # Max connection lifetime
  conn_max_idle_time: 1m  # Max idle time before closing
```

**Guidelines:**
- **Small workload** (<10 workers): `max_open_conns: 10`
- **Medium workload** (10-50 workers): `max_open_conns: 25`
- **Large workload** (50-100 workers): `max_open_conns: 50`
- **Very large** (100+ workers): `max_open_conns: 100`

### PostgreSQL Configuration

Edit `postgresql.conf`:

```ini
# Memory settings
shared_buffers = 256MB
effective_cache_size = 1GB
maintenance_work_mem = 64MB
work_mem = 16MB

# Connections
max_connections = 100

# WAL settings (for high write workloads)
wal_buffers = 16MB
checkpoint_completion_target = 0.9

# Query optimization
random_page_cost = 1.1  # For SSD
effective_io_concurrency = 200
```

### Indexes

The store automatically creates these indexes:

```sql
CREATE INDEX idx_jobs_sequence ON jobs(sequence_number);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_queue_priority ON jobs(queue, priority, created_at);
CREATE INDEX idx_nodes_status ON nodes(status);
CREATE INDEX idx_nodes_address ON nodes(address);
```

For large deployments, add:

```sql
-- For faster job queries
CREATE INDEX idx_jobs_created_at ON jobs(created_at) WHERE status IN ('queued', 'retrying');

-- For faster worker queries  
CREATE INDEX idx_jobs_node_id ON jobs(node_id) WHERE status IN ('assigned', 'running');

-- For analytics
CREATE INDEX idx_jobs_completed_at ON jobs(completed_at) WHERE status = 'completed';
```

---

## Monitoring

### Health Check

```bash
# Check database connectivity
curl http://localhost:8080/health

# Check PostgreSQL directly
psql -U ffmpeg -d ffmpeg_rtmp -c "SELECT COUNT(*) FROM jobs;"
```

### Metrics

The master exposes PostgreSQL connection pool metrics at `/metrics`:

```
# Database connections
db_connections_open 15
db_connections_in_use 3
db_connections_idle 12

# Query performance
db_query_duration_seconds_bucket{query="GetJob"} 0.001
db_query_duration_seconds_bucket{query="CreateJob"} 0.002
```

### Query Performance

Monitor slow queries:

```sql
-- Enable slow query logging
ALTER DATABASE ffmpeg_rtmp SET log_min_duration_statement = 1000;  -- Log queries > 1s

-- View active queries
SELECT pid, query, state, query_start
FROM pg_stat_activity
WHERE datname = 'ffmpeg_rtmp'
  AND state != 'idle'
ORDER BY query_start;
```

---

## Troubleshooting

### Connection Errors

**Error**: `connection refused`

```bash
# Check PostgreSQL is running
systemctl status postgresql

# Check firewall
sudo ufw allow 5432/tcp

# Check pg_hba.conf allows connections
sudo nano /etc/postgresql/15/main/pg_hba.conf
# Add: host all all 0.0.0.0/0 md5
```

**Error**: `password authentication failed`

```bash
# Reset password
sudo -u postgres psql
ALTER USER ffmpeg WITH PASSWORD 'new_password';
```

**Error**: `too many connections`

```sql
-- Check current connections
SELECT COUNT(*) FROM pg_stat_activity WHERE datname = 'ffmpeg_rtmp';

-- Increase max_connections in postgresql.conf
max_connections = 200
```

### Performance Issues

**Slow job queries**:

```sql
-- Analyze tables
ANALYZE jobs;
ANALYZE nodes;

-- Reindex if needed
REINDEX TABLE jobs;
```

**Connection pool exhaustion**:

```yaml
# Increase pool size
database:
  max_open_conns: 50  # Was 25
```

---

## High Availability Setup

For production HA, use one of these approaches:

### Option 1: Managed Database (Easiest)
Use AWS RDS, Google Cloud SQL, or Azure Database with automatic failover.

### Option 2: PostgreSQL Replication
Set up primary-replica with automatic failover:

```bash
# Primary (write)
postgresql://ffmpeg:pass@primary.example.com:5432/ffmpeg_rtmp

# Replica (read, automatic failover)
postgresql://ffmpeg:pass@replica.example.com:5432/ffmpeg_rtmp
```

### Option 3: Connection Pooler (PgBouncer)
Add PgBouncer for connection management:

```bash
# Install PgBouncer
sudo apt install pgbouncer

# Configure /etc/pgbouncer/pgbouncer.ini
[databases]
ffmpeg_rtmp = host=localhost port=5432 dbname=ffmpeg_rtmp

[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25

# Connect through PgBouncer
postgresql://ffmpeg:pass@localhost:6432/ffmpeg_rtmp
```

---

## Backup & Recovery

### Automated Backups

```bash
# Daily backup script
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
pg_dump -U ffmpeg ffmpeg_rtmp | gzip > backup_$DATE.sql.gz

# Keep last 7 days
find /backups -name "backup_*.sql.gz" -mtime +7 -delete
```

### Restore

```bash
# Restore from backup
gunzip < backup_20260102_120000.sql.gz | psql -U ffmpeg ffmpeg_rtmp
```

### Point-in-Time Recovery

For production, enable WAL archiving in `postgresql.conf`:

```ini
wal_level = replica
archive_mode = on
archive_command = 'cp %p /archive/%f'
```

---

## Comparison: SQLite vs PostgreSQL

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| **Max jobs/sec** | ~100 | ~10,000+ |
| **Max workers** | ~10 | ~1000+ |
| **Concurrent writes** | Poor | Excellent |
| **Backup** | Copy file | pg_dump |
| **Replication** | No | Yes |
| **HA** | No | Yes |
| **Best for** | Development, single master | Production, multiple masters |

---

## Next Steps

After PostgreSQL is working:

1. ✅ Phase 1 Complete: PostgreSQL implemented
2. 🔄 Phase 2: Add Multi-Tenancy (see `IMPLEMENTATION_PLAN.md`)
3. 🔄 Phase 3: Add RBAC
4. 🔄 Phase 4: Deploy on Kubernetes for HA

See `IMPLEMENTATION_PLAN.md` for the full roadmap.

---

## Support

For issues:
1. Check logs: `docker logs ffmpeg-master` or `journalctl -u ffmpeg-master`
2. Test connection: `psql -U ffmpeg -h localhost -d ffmpeg_rtmp`
3. Check metrics: `curl http://localhost:9090/metrics | grep db_`
4. Review this guide's troubleshooting section

**PostgreSQL support is now PRODUCTION-READY!** 🎉
