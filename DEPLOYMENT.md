# Production Deployment Guide

## Quick Start

```bash
# Build binaries
make build

# Start the entire stack (master + 3 workers)
./deploy_production.sh start

# Check status
./deploy_production.sh status

# Watch scheduler in action
tail -f logs/master.log | grep -E '\[Scheduler\]|\[FSM\]|\[Health\]|\[Cleanup\]'

# Submit test jobs
./deploy_production.sh demo

# Stop everything
./deploy_production.sh stop
```

## Available Commands

```bash
./deploy_production.sh start    # Start master + workers
./deploy_production.sh stop     # Stop all services
./deploy_production.sh restart  # Restart everything
./deploy_production.sh status   # Show service status
./deploy_production.sh logs     # View recent logs
./deploy_production.sh health   # Health check
./deploy_production.sh demo     # Submit test jobs
```

## Configuration

Set environment variables before running:

```bash
# Start 5 workers instead of 3
NUM_WORKERS=5 ./deploy_production.sh start

# Use custom ports
MASTER_PORT=9090 WORKER_BASE_PORT=10000 ./deploy_production.sh start

# Custom log directory
LOG_DIR=/var/log/ffmpeg-rtmp ./deploy_production.sh start

# Custom database
DB_PATH=/data/ffmpeg-rtmp.db ./deploy_production.sh start
```

## What Gets Started

### Master Node
- **Port:** 8080 (configurable)
- **Database:** `./master.db` (SQLite)
- **Logs:** `./logs/master.log`
- **Features:**
  - Production scheduler with FSM
  - Heartbeat monitoring (5s interval)
  - Orphan job recovery (10s interval)
  - Automatic retries with backoff
  - Priority scheduling

### Worker Nodes
- **Ports:** 9000, 9001, 9002, ... (configurable)
- **Logs:** `./logs/worker-1.log`, `./logs/worker-2.log`, ...
- **Features:**
  - Auto-registration with master
  - Periodic heartbeats
  - FFmpeg/GStreamer support
  - GPU detection

## Monitoring

### Real-time Scheduler Activity
```bash
# Watch scheduler decisions
tail -f logs/master.log | grep '\[Scheduler\]'

# Watch state transitions
tail -f logs/master.log | grep '\[FSM\]'

# Watch health checks
tail -f logs/master.log | grep '\[Health\]'

# Watch orphan recovery
tail -f logs/master.log | grep '\[Cleanup\]'
```

### API Endpoints
```bash
# Health check
curl http://localhost:8080/health

# List jobs
curl http://localhost:8080/jobs | jq

# List workers
curl http://localhost:8080/nodes | jq

# Submit job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p30-h264",
    "priority": "high",
    "queue": "default"
  }' | jq
```

### Status Dashboard
```bash
./deploy_production.sh status
```

Output:
```
========================================
📊 Production Stack Status
========================================
Master:    RUNNING (PID: 12345, Port: 8080)
  └─ Health: ✓ OK
  └─ Jobs: 5 | Workers: 3

Workers:
  worker-1: RUNNING (PID: 12346, Port: 9000)
  worker-2: RUNNING (PID: 12347, Port: 9001)
  worker-3: RUNNING (PID: 12348, Port: 9002)

Summary: 3/3 workers running

Recent Scheduler Activity:
  [Scheduler] Scheduling: 2 queued jobs, 3 available workers
  [FSM] Job job-123: QUEUED → ASSIGNED (reason: Assigned to worker-1)
  [Health] All workers healthy
```

## Production Setup

### 1. Systemd Services (Recommended)

Create `/etc/systemd/system/ffmpeg-master.service`:
```ini
[Unit]
Description=FFmpeg-RTMP Master Node
After=network.target

[Service]
Type=simple
User=ffmpeg
WorkingDirectory=/opt/ffmpeg-rtmp
ExecStart=/opt/ffmpeg-rtmp/bin/master
Environment="PORT=8080"
Environment="DB_PATH=/var/lib/ffmpeg-rtmp/master.db"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Create `/etc/systemd/system/ffmpeg-worker@.service`:
```ini
[Unit]
Description=FFmpeg-RTMP Worker %i
After=network.target ffmpeg-master.service

[Service]
Type=simple
User=ffmpeg
WorkingDirectory=/opt/ffmpeg-rtmp
ExecStart=/opt/ffmpeg-rtmp/bin/worker
Environment="PORT=900%i"
Environment="MASTER_URL=http://localhost:8080"
Environment="WORKER_NAME=worker-%i"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable ffmpeg-master
sudo systemctl enable ffmpeg-worker@{1..3}
sudo systemctl start ffmpeg-master
sudo systemctl start ffmpeg-worker@{1..3}
```

### 2. Docker Compose

Create `docker-compose.production.yml`:
```yaml
version: '3.8'

services:
  master:
    build: .
    command: /app/bin/master
    ports:
      - "8080:8080"
    volumes:
      - master-data:/data
    environment:
      PORT: 8080
      DB_PATH: /data/master.db
    restart: unless-stopped

  worker-1:
    build: .
    command: /app/bin/worker
    ports:
      - "9000:9000"
    environment:
      PORT: 9000
      MASTER_URL: http://master:8080
      WORKER_NAME: worker-1
    restart: unless-stopped
    depends_on:
      - master

  worker-2:
    build: .
    command: /app/bin/worker
    ports:
      - "9001:9001"
    environment:
      PORT: 9001
      MASTER_URL: http://master:8080
      WORKER_NAME: worker-2
    restart: unless-stopped
    depends_on:
      - master

  worker-3:
    build: .
    command: /app/bin/worker
    ports:
      - "9002:9002"
    environment:
      PORT: 9002
      MASTER_URL: http://master:8080
      WORKER_NAME: worker-3
    restart: unless-stopped
    depends_on:
      - master

volumes:
  master-data:
```

Start:
```bash
docker-compose -f docker-compose.production.yml up -d
```

### 3. Kubernetes (Advanced)

See `deployment/kubernetes/` for manifests.

## Troubleshooting

### Master won't start
```bash
# Check logs
cat logs/master.log

# Check port availability
lsof -i :8080

# Check database
sqlite3 master.db "SELECT COUNT(*) FROM jobs;"
```

### Workers not registering
```bash
# Check worker logs
cat logs/worker-1.log

# Check master is reachable
curl http://localhost:8080/health

# Check worker port
lsof -i :9000
```

### Jobs stuck in queue
```bash
# Check worker availability
curl http://localhost:8080/nodes | jq '.[] | select(.status=="available")'

# Check scheduler logs
grep '\[Scheduler\]' logs/master.log | tail -20

# Check for errors
grep 'ERROR\|Failed' logs/master.log | tail -20
```

### High orphan rate
```bash
# Check worker heartbeats
grep '\[Health\].*dead' logs/master.log

# Increase worker timeout
# Edit deploy_production.sh or set in code:
# WorkerTimeout: 5 * time.Minute
```

## Performance Tuning

### Scheduler Intervals
Edit `shared/pkg/scheduler/production_scheduler.go`:
```go
config := &SchedulerConfig{
    SchedulingInterval:   1 * time.Second,  // More aggressive
    HealthCheckInterval:  10 * time.Second, // Less frequent
    CleanupInterval:      30 * time.Second, // Less frequent
}
```

### Worker Count
```bash
# Scale up dynamically
NUM_WORKERS=10 ./deploy_production.sh start

# Or add workers while running
PORT=9010 MASTER_URL=http://localhost:8080 ./bin/worker &
```

### Database Optimization
For high load, consider PostgreSQL:
1. Change DB_PATH to postgres connection string
2. Update store initialization in master code
3. Run migrations

## Backup & Recovery

### Backup Database
```bash
# While running
sqlite3 master.db ".backup master_backup.db"

# Or copy file (stop master first)
./deploy_production.sh stop
cp master.db master_backup_$(date +%Y%m%d).db
./deploy_production.sh start
```

### Restore from Backup
```bash
./deploy_production.sh stop
cp master_backup.db master.db
./deploy_production.sh start
```

### Job Recovery After Crash
The production scheduler automatically recovers orphaned jobs on restart.
Check logs for recovery:
```bash
grep '\[Cleanup\].*Recovering orphaned' logs/master.log
```

## Security

### Production Checklist
- [ ] Change default ports
- [ ] Enable TLS/HTTPS
- [ ] Set up authentication
- [ ] Configure firewall rules
- [ ] Run as non-root user
- [ ] Set up log rotation
- [ ] Enable monitoring/alerting
- [ ] Regular database backups
- [ ] Resource limits (ulimit, cgroups)

### Enable TLS
Add to master/worker configs:
```bash
export TLS_CERT=/path/to/cert.pem
export TLS_KEY=/path/to/key.pem
```

## Monitoring & Alerting

### Prometheus (Optional)
Metrics endpoint: `http://localhost:8080/metrics`

### Alert Rules
- Worker offline > 2 minutes
- Queue depth > 100 jobs
- Job failure rate > 10%
- Orphan rate > 5%

## Support

- Documentation: `docs/`
- Issues: GitHub Issues
- Logs: `./logs/`
- Health: `http://localhost:8080/health`

---

**Deployment Status:** 🟢 Production Ready

Built with ❤️ using the production scheduler with FSM, idempotency, and automatic fault recovery.
