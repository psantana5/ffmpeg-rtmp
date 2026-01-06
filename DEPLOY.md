# Production Deployment Guide

## Quick Start (5 Minutes)

### Option 1: Simple Deployment (Recommended)

```bash
# 1. Clone and build
git clone <repo-url>
cd ffmpeg-rtmp
make build

# 2. Start everything
./deploy.sh start

# 3. Check status
./deploy.sh status

# 4. Test it
./bin/ffrtmp jobs submit --scenario test
./bin/ffrtmp jobs list
```

**That's it!** The system is running with:
- ✅ Master node on port 8080 (HTTPS)
- ✅ Worker agent registered and polling
- ✅ API authentication enabled
- ✅ Metrics on ports 9090-9091
- ✅ SQLite database (master.db)

---

## Deployment Script Usage

### Basic Commands

```bash
# Start the system
./deploy.sh start

# Stop the system
./deploy.sh stop

# Restart the system
./deploy.sh restart

# Check status
./deploy.sh status
```

### Advanced Configuration

```bash
# Start without TLS (development only)
USE_TLS=false ./deploy.sh start

# Custom ports
MASTER_PORT=9090 AGENT_METRICS_PORT=9095 ./deploy.sh start

# Custom database location
DB_PATH=/data/production.db ./deploy.sh start
```

---

## Environment Variables

Create a `.env` file (auto-generated on first run):

```bash
# Authentication
MASTER_API_KEY=your-secure-key-here

# Ports (optional)
MASTER_PORT=8080
MASTER_METRICS_PORT=9090
AGENT_METRICS_PORT=9091

# Database (optional)
DB_PATH=master.db

# TLS (optional)
USE_TLS=true
```

---

## Production Deployment Options

### 1. Single Server (Simple)

**Use Case**: Development, testing, small deployments (<100 jobs/day)

```bash
# Start on single server
./deploy.sh start

# Logs
tail -f logs/master.log logs/agent.log
```

**Pros**: Simple, easy setup  
**Cons**: Single point of failure

---

### 2. Distributed Setup (Recommended for Production)

**Use Case**: Production workloads, high availability

#### Server 1: Master Node

```bash
# On master server
./bin/master \
    --port 8080 \
    --db master.db \
    --tls \
    --cert certs/master.crt \
    --key certs/master.key \
    --api-key $(cat .env | grep MASTER_API_KEY | cut -d= -f2)
```

#### Server 2+: Worker Nodes

```bash
# On each worker server
export FFMPEG_RTMP_API_KEY="<master-api-key>"

./bin/agent \
    --master https://master-server:8080 \
    --register \
    --ca certs/master.crt
```

**Pros**: Scalable, fault-tolerant workers  
**Cons**: Need multiple servers

---

### 3. Docker Deployment

Coming soon - see `docker-compose.yml` for monitoring stack.

---

## PostgreSQL (Production Database)

For production with >1000 jobs or >10 workers, use PostgreSQL:

```bash
# 1. Setup PostgreSQL
docker run -d \
    --name ffrtmp-postgres \
    -e POSTGRES_DB=ffrtmp \
    -e POSTGRES_USER=ffrtmp_user \
    -e POSTGRES_PASSWORD=secure_password \
    -p 5432:5432 \
    postgres:15

# 2. Run migrations
psql -U ffrtmp_user -d ffrtmp -f shared/pkg/store/migrations/001_initial_schema.sql

# 3. Start master with PostgreSQL
DB_TYPE=postgres \
DB_DSN="postgresql://ffrtmp_user:secure_password@localhost/ffrtmp" \
./bin/master --port 8080
```

See `POSTGRES_MIGRATION.md` for details.

---

## Monitoring

### Built-in Metrics

```bash
# Master metrics
curl http://localhost:9090/metrics

# Agent metrics  
curl http://localhost:9091/metrics
```

### Grafana Dashboard (Optional)

```bash
# Start monitoring stack
docker-compose up -d grafana victoriametrics

# Access Grafana
open http://localhost:3000
# Default credentials: admin/admin

# Dashboards auto-loaded from master/monitoring/grafana/
```

---

## Security Checklist

### Before Production Deployment

- [ ] **Change API key** (don't use generated one)
  ```bash
  export MASTER_API_KEY="$(openssl rand -hex 32)"
  ```

- [ ] **Enable TLS** (already default)
  ```bash
  USE_TLS=true ./deploy.sh start
  ```

- [ ] **Use strong certificates** (for production, use proper CA)
  ```bash
  # Replace self-signed with CA-signed certificates
  cp /path/to/ca-signed.crt certs/master.crt
  cp /path/to/ca-signed.key certs/master.key
  ```

- [ ] **Firewall rules** (restrict ports)
  ```bash
  # Only allow necessary ports
  ufw allow 8080/tcp  # Master API
  ufw allow 9090/tcp  # Master metrics (restrict to monitoring)
  ```

- [ ] **Rate limiting** (already enabled by default)

- [ ] **Database backups** (automated)
  ```bash
  # SQLite backup
  cp master.db master.db.backup-$(date +%Y%m%d)
  
  # PostgreSQL backup
  pg_dump ffrtmp > backup-$(date +%Y%m%d).sql
  ```

---

## System Requirements

### Minimum (Development/Testing)
- CPU: 2 cores
- RAM: 2 GB
- Disk: 5 GB
- OS: Linux (Ubuntu 20.04+, Debian 10+)

### Recommended (Production)
- CPU: 4+ cores
- RAM: 8+ GB
- Disk: 50+ GB (for job results and logs)
- OS: Linux (Ubuntu 22.04 LTS recommended)

### Software Requirements
- Go 1.21+ (for building)
- FFmpeg 4.4+ with libx264
- (Optional) NVIDIA GPU + drivers for NVENC
- (Optional) Docker + Docker Compose for monitoring

---

## Troubleshooting

### Master won't start

```bash
# Check if port is in use
lsof -i:8080

# Check logs
tail -f logs/master.log

# Verify database
file master.db
```

### Agent can't register

```bash
# Check API key matches
echo $FFMPEG_RTMP_API_KEY
grep MASTER_API_KEY .env

# Check TLS certificates
openssl x509 -in certs/master.crt -text -noout

# Test master health
curl -k https://localhost:8080/health
```

### Jobs not processing

```bash
# Check agent is registered
./bin/ffrtmp nodes list

# Check job status
./bin/ffrtmp jobs list

# Check agent logs
tail -f logs/agent.log
```

---

## Maintenance

### Log Rotation

**Production-ready logrotate configs are included and installed:**

```bash
# Verify logrotate configs are installed
ls -la /etc/logrotate.d/ffrtmp-*

# Expected output:
# /etc/logrotate.d/ffrtmp-master
# /etc/logrotate.d/ffrtmp-worker  
# /etc/logrotate.d/ffrtmp-wrapper
```

**Configuration details:**
- **Rotation**: Daily
- **Retention**: 14 days
- **Compression**: Enabled (gzip)
- **Location**: `/var/log/ffrtmp/<component>/*.log`
- **Fallback**: `./logs/` if `/var/log` not writable

**Testing logrotate:**

```bash
# Dry-run test (shows what would happen)
sudo logrotate -d /etc/logrotate.d/ffrtmp-master
sudo logrotate -d /etc/logrotate.d/ffrtmp-worker

# Force rotation (for testing)
sudo logrotate -f /etc/logrotate.d/ffrtmp-master
sudo logrotate -f /etc/logrotate.d/ffrtmp-worker
```

**Manual installation (if needed):**

```bash
# Copy logrotate configs (usually done automatically)
sudo cp deployment/logrotate/ffrtmp-* /etc/logrotate.d/

# Set proper permissions
sudo chmod 644 /etc/logrotate.d/ffrtmp-*
sudo chown root:root /etc/logrotate.d/ffrtmp-*
```

**Customization:**

Edit `/etc/logrotate.d/ffrtmp-master` to adjust:
- `rotate 14` - Number of days to keep logs
- `daily` - Change to `weekly` or `monthly`  
- `compress` - Remove to disable compression

### Database Maintenance

```bash
# SQLite vacuum (compact)
sqlite3 master.db "VACUUM;"

# PostgreSQL vacuum
psql -U ffrtmp_user -d ffrtmp -c "VACUUM ANALYZE;"
```

### Cleanup Old Jobs

```bash
# Delete completed jobs older than 30 days
sqlite3 master.db "DELETE FROM jobs WHERE status='completed' AND completed_at < datetime('now', '-30 days');"
```

---

## Upgrading

```bash
# 1. Stop the system
./deploy.sh stop

# 2. Backup database
cp master.db master.db.backup

# 3. Pull latest code
git pull origin main

# 4. Rebuild binaries
make clean build

# 5. Restart
./deploy.sh start

# 6. Verify
./deploy.sh status
```

---

## Performance Tuning

### For High-Throughput Workloads

```bash
# Increase worker poll frequency
./bin/agent --poll-interval 5s

# Add more workers
NUM_WORKERS=5 ./deploy_production.sh start

# Use PostgreSQL instead of SQLite
DB_TYPE=postgres ./bin/master --db-dsn "postgresql://..."
```

### For Low-Latency Jobs

```bash
# Reduce scheduler interval
./bin/master --scheduler-interval 2s

# Use LIVE queue
curl -X POST http://localhost:8080/jobs \
  -d '{"scenario":"test","queue":"live"}'
```

---

## Support

- **Documentation**: See `docs/` folder
- **Issues**: GitHub Issues
- **Configuration**: See `.env` file
- **Logs**: Check `logs/` directory

---

## File Structure

```
ffmpeg-rtmp/
├── deploy.sh              # ← Simple deployment script (NEW!)
├── deploy_production.sh   # Advanced deployment (use for multi-worker)
├── .env                   # Auto-generated API key
├── master.db             # SQLite database
├── logs/                 # Application logs
│   ├── master.log
│   └── agent.log
├── pids/                 # Process IDs
│   ├── master.pid
│   └── agent.pid
├── certs/                # TLS certificates
│   ├── master.crt
│   └── master.key
├── bin/                  # Binaries
│   ├── master
│   ├── agent
│   └── ffrtmp (CLI)
└── test_results/         # Job output files
```

---

## Next Steps

1. ✅ **Start the system**: `./deploy.sh start`
2. ✅ **Submit a job**: `./bin/ffrtmp jobs submit --scenario test`
3. ✅ **Check status**: `./deploy.sh status`
4. 📚 **Read advanced docs**: See `docs/` folder
5. 🎨 **Setup monitoring**: `docker-compose up -d grafana`

---

**Deployment Script**: `deploy.sh` (Simple, recommended)  
**Advanced Script**: `deploy_production.sh` (Multi-worker deployments)  
**CLI Tool**: `bin/ffrtmp` (Job management)

**Status**: ✅ Production-Ready  
**Version**: 2.3.0+  
**Updated**: 2026-01-05
