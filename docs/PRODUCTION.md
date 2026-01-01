# Production Deployment Guide

This guide covers deploying ffmpeg-rtmp in production environments with high availability, security, and fault tolerance.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Security Configuration](#security-configuration)
3. [High Availability](#high-availability)
4. [Monitoring & Observability](#monitoring--observability)
5. [Performance Tuning](#performance-tuning)
6. [Fault Tolerance](#fault-tolerance)
7. [Backup & Recovery](#backup--recovery)

---

## Architecture Overview

### Components

```
┌─────────────────────────────────────────────────────────────┐
│                      Load Balancer                          │
│                    (HAProxy/Nginx)                          │
└──────────────────┬────────────────────────────────────────┬─┘
                   │                                        │
       ┌───────────▼─────────┐                 ┌──────────▼──────────┐
       │   Master Node 1     │◄────────────────│   Master Node 2     │
       │  (Active/Standby)   │   Replication   │   (Active/Standby)  │
       └───────────┬─────────┘                 └──────────┬──────────┘
                   │                                        │
                   └────────────┬───────────────────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              │                 │                 │
    ┌─────────▼────────┐ ┌─────▼───────┐ ┌──────▼─────────┐
    │  Worker Node 1   │ │ Worker N... │ │ Worker Node N  │
    │   (GPU/CPU)      │ │             │ │   (GPU/CPU)    │
    └──────────────────┘ └─────────────┘ └────────────────┘
```

### Infrastructure Requirements

**Master Node(s):**
- CPU: 4+ cores
- RAM: 8GB+ 
- Storage: 100GB+ SSD (for database)
- Network: Gigabit ethernet minimum

**Worker Nodes:**
- CPU: 8+ cores (or GPU with hardware encoding)
- RAM: 16GB+
- Storage: 50GB+ (for temporary files)
- Network: Gigabit ethernet minimum

---

## Security Configuration

### 1. TLS/mTLS Setup

**Generate CA Certificate:**

```bash
# Generate CA private key
openssl genrsa -out ca.key 4096

# Generate CA certificate
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=FFRTMP-CA"
```

**Generate Master Certificate:**

```bash
# Generate master private key
openssl genrsa -out master.key 4096

# Create certificate signing request
openssl req -new -key master.key -out master.csr \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=master.example.com"

# Sign with CA
openssl x509 -req -in master.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out master.crt -days 365 \
    -extfile <(printf "subjectAltName=DNS:master.example.com,IP:10.0.0.1")
```

**Generate Worker Certificates:**

```bash
# For each worker
openssl genrsa -out worker1.key 4096
openssl req -new -key worker1.key -out worker1.csr \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=worker1"
openssl x509 -req -in worker1.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out worker1.crt -days 365
```

**Start Master with mTLS:**

```bash
./bin/ffrtmp-master \
    --port 8080 \
    --tls \
    --cert certs/master.crt \
    --key certs/master.key \
    --ca certs/ca.crt \
    --mtls \
    --api-key $(openssl rand -base64 32) \
    --db /var/lib/ffrtmp/master.db
```

**Start Worker with mTLS:**

```bash
export FFMPEG_RTMP_API_KEY="your-api-key-here"

./bin/ffrtmp-worker \
    --register \
    --master https://master.example.com:8080 \
    --cert certs/worker1.crt \
    --key certs/worker1.key \
    --ca certs/ca.crt
```

### 2. API Key Management

**Generate Secure API Key:**

```bash
openssl rand -base64 32
```

**Set Environment Variable:**

```bash
export MASTER_API_KEY="your-secure-api-key"
```

**Rotate API Keys:**

1. Generate new key
2. Update master configuration
3. Rolling update workers with new key
4. Revoke old key after migration

### 3. Rate Limiting

Rate limiting is built-in and configured per-IP:

```bash
# Default: 100 requests/second, burst of 200
# Configure in master startup if needed
```

### 4. Network Security

**Firewall Rules:**

```bash
# Master node
ufw allow 8080/tcp    # API endpoint
ufw allow 9090/tcp    # Prometheus metrics

# Worker nodes  
ufw allow 9091/tcp    # Prometheus metrics
ufw deny incoming     # Deny all other incoming
```

**VPC/Network Isolation:**
- Place master and workers in private subnet
- Expose only load balancer to public internet
- Use security groups to restrict traffic between components

---

## High Availability

### Master HA Setup (Coming Soon)

Currently, the master is a single point of failure. High availability features planned:

1. **Leader Election** (Raft/etcd)
2. **State Replication** 
3. **Automatic Failover**
4. **Load Balancer Integration**

**Current Workaround:**

Use database backup/restore for disaster recovery:

```bash
# Backup
sqlite3 master.db ".backup master-backup.db"

# Restore on new master
cp master-backup.db master.db
./bin/ffrtmp-master --db master.db ...
```

### Worker HA

Workers are stateless and automatically re-register:

```bash
# Use systemd for auto-restart
cat > /etc/systemd/system/ffrtmp-worker.service <<EOF
[Unit]
Description=FFRTMP Worker Node
After=network.target

[Service]
Type=simple
User=ffrtmp
WorkingDirectory=/opt/ffrtmp
Environment="FFMPEG_RTMP_API_KEY=your-key"
ExecStart=/opt/ffrtmp/bin/ffrtmp-worker --register --master https://master:8080
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl enable ffrtmp-worker
systemctl start ffrtmp-worker
```

---

## Monitoring & Observability

### Prometheus Metrics

**Master Metrics** (`:9090/metrics`):

- `ffrtmp_master_jobs_total{status}` - Total jobs by status
- `ffrtmp_master_nodes_total{status}` - Total nodes by status
- `ffrtmp_master_job_duration_seconds` - Job duration histogram
- `ffrtmp_master_api_requests_total` - API request counter
- `ffrtmp_master_db_size_bytes` - Database size

**Worker Metrics** (`:9091/metrics`):

- `ffrtmp_worker_job_duration_seconds` - Job duration
- `ffrtmp_worker_cpu_usage_percent` - CPU utilization
- `ffrtmp_worker_memory_usage_bytes` - Memory usage
- `ffrtmp_worker_gpu_usage_percent` - GPU utilization
- `ffrtmp_worker_encoder_available{encoder}` - Encoder availability

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'ffrtmp-master'
    static_configs:
      - targets: ['master:9090']
    scrape_interval: 15s

  - job_name: 'ffrtmp-workers'
    static_configs:
      - targets:
        - 'worker1:9091'
        - 'worker2:9091'
        - 'worker3:9091'
    scrape_interval: 15s
```

### Grafana Dashboards

Import pre-built dashboards from `docs/grafana/`:

1. Master Overview Dashboard
2. Worker Performance Dashboard
3. Job Queue Dashboard
4. System Health Dashboard

### Distributed Tracing (OpenTelemetry)

**Enable Tracing:**

```bash
# Start Jaeger (all-in-one)
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Start master with tracing
./bin/ffrtmp-master \
    --tracing-enabled \
    --tracing-endpoint localhost:4318 \
    --service-name ffrtmp-master \
    ...
```

**View Traces:**

Open `http://localhost:16686` for Jaeger UI.

### Alerting Rules

```yaml
# alerts.yml
groups:
  - name: ffrtmp
    rules:
      - alert: MasterDown
        expr: up{job="ffrtmp-master"} == 0
        for: 1m
        annotations:
          summary: "Master node is down"
          
      - alert: HighJobFailureRate
        expr: rate(ffrtmp_master_jobs_total{status="failed"}[5m]) > 0.1
        for: 5m
        annotations:
          summary: "High job failure rate detected"
          
      - alert: WorkerDisconnected
        expr: ffrtmp_master_nodes_total{status="offline"} > 0
        for: 2m
        annotations:
          summary: "Worker node(s) disconnected"
          
      - alert: HighCPUUsage
        expr: ffrtmp_worker_cpu_usage_percent > 90
        for: 10m
        annotations:
          summary: "Worker CPU usage above 90%"
```

---

## Performance Tuning

### Database Optimization

**SQLite Tuning:**

```sql
-- Enable WAL mode for better concurrency
PRAGMA journal_mode=WAL;

-- Increase cache size (in pages, -2000 = 2MB)
PRAGMA cache_size=-2000;

-- Enable memory-mapped I/O (64MB)
PRAGMA mmap_size=67108864;

-- Synchronous mode (NORMAL for production)
PRAGMA synchronous=NORMAL;
```

Apply on startup:

```bash
sqlite3 master.db <<EOF
PRAGMA journal_mode=WAL;
PRAGMA cache_size=-2000;
PRAGMA mmap_size=67108864;
PRAGMA synchronous=NORMAL;
EOF
```

### Worker Pool Sizing

**Optimal Worker Configuration:**

```
Workers per GPU node: 1-2 (GPU encoding)
Workers per CPU node: 1 (CPU encoding is intensive)
```

### Network Optimization

**TCP Tuning (Linux):**

```bash
# Increase TCP buffer sizes
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 87380 67108864"
sysctl -w net.ipv4.tcp_wmem="4096 65536 67108864"

# Enable TCP window scaling
sysctl -w net.ipv4.tcp_window_scaling=1
```

---

## Fault Tolerance

### Automatic Job Recovery

**Built-in Recovery Features:**

1. **Node Failure Detection**
   - Heartbeat timeout: 2 minutes (default)
   - Automatic job reassignment from dead nodes

2. **Transient Failure Retry**
   - Auto-retry on connection errors, timeouts, network issues
   - Max retries: 3 (configurable)
   - Exponential backoff between retries

3. **Stale Job Detection**
   - Batch jobs: 30 minute timeout
   - Live jobs: 5 minute inactivity timeout

**Configure Recovery:**

```bash
./bin/ffrtmp-master \
    --max-retries 5 \
    --scheduler-interval 10s \
    ...
```

### Job Priority & Queuing

**Priority Levels:**

- `live` queue: Highest priority (real-time streams)
- `high` priority: Important batch jobs
- `medium` priority: Standard batch jobs  
- `low` priority: Background processing
- `batch` queue: Lowest priority (bulk processing)

**Submit Prioritized Job:**

```bash
./bin/ffrtmp jobs submit \
    --scenario 4k60 \
    --queue live \
    --priority high \
    ...
```

### Checkpointing & Resume (Planned)

Future feature: Save job progress and resume from checkpoint on failure.

---

## Backup & Recovery

### Database Backup

**Automated Backup Script:**

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/var/backups/ffrtmp"
DB_PATH="/var/lib/ffrtmp/master.db"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

# Create backup
sqlite3 "$DB_PATH" ".backup $BACKUP_DIR/master_$TIMESTAMP.db"

# Compress
gzip "$BACKUP_DIR/master_$TIMESTAMP.db"

# Keep only last 30 days
find "$BACKUP_DIR" -name "master_*.db.gz" -mtime +30 -delete

echo "Backup completed: master_$TIMESTAMP.db.gz"
```

**Schedule with cron:**

```bash
# Run every 6 hours
0 */6 * * * /opt/ffrtmp/scripts/backup.sh
```

### Disaster Recovery

**Recovery Procedure:**

1. **Stop master service:**
   ```bash
   systemctl stop ffrtmp-master
   ```

2. **Restore database:**
   ```bash
   gunzip -c /var/backups/ffrtmp/master_YYYYMMDD_HHMMSS.db.gz > /var/lib/ffrtmp/master.db
   ```

3. **Start master service:**
   ```bash
   systemctl start ffrtmp-master
   ```

4. **Workers auto-reconnect:**
   Workers will automatically re-register on next heartbeat.

### Data Retention

**Configure cleanup:**

```bash
# Remove completed jobs older than 7 days
sqlite3 master.db <<EOF
DELETE FROM jobs 
WHERE status = 'completed' 
AND completed_at < datetime('now', '-7 days');
EOF
```

**Automate with cron:**

```bash
# Daily at 2 AM
0 2 * * * /opt/ffrtmp/scripts/cleanup.sh
```

---

## Production Checklist

- [ ] TLS/mTLS configured with valid certificates
- [ ] API key authentication enabled
- [ ] Firewall rules configured
- [ ] Prometheus monitoring deployed
- [ ] Alerting rules configured
- [ ] Database backups automated
- [ ] systemd services configured for auto-restart
- [ ] Log rotation configured
- [ ] Resource limits set (ulimit, cgroups)
- [ ] Network tuning applied
- [ ] Load testing completed
- [ ] Disaster recovery tested
- [ ] Documentation updated with deployment details

---

## Support & Troubleshooting

### Common Issues

**1. Workers not connecting:**
- Check TLS certificates
- Verify API key
- Check firewall rules
- Verify network connectivity

**2. High job failure rate:**
- Check worker logs
- Verify input file accessibility
- Check hardware encoder availability
- Monitor system resources

**3. Database locked errors:**
- Enable WAL mode
- Reduce concurrent writes
- Increase timeout values

**4. Memory issues:**
- Limit concurrent jobs per worker
- Increase swap space
- Monitor for memory leaks

### Getting Help

- GitHub Issues: https://github.com/psantana5/ffmpeg-rtmp/issues
- Documentation: https://github.com/psantana5/ffmpeg-rtmp/docs
- Community Discord: [Coming Soon]

---

## Performance Benchmarks

See `production_benchmarks.json` for expected performance metrics:

- **4K60 Encoding:** ~30-60 FPS on modern GPUs
- **1080p60 Encoding:** ~100-200 FPS on modern GPUs
- **CPU Encoding:** ~10-30 FPS depending on preset
- **Job Latency:** <1s for job assignment
- **API Latency:** <100ms for most endpoints

---

*Last Updated: January 2026*
