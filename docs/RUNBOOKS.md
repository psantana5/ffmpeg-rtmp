# FFmpeg-RTMP Production Runbooks

Operational procedures and troubleshooting guides for production incidents.

## Table of Contents

1. [Quick Reference](#quick-reference)
2. [Common Issues](#common-issues)
3. [Alert Response](#alert-response)
4. [Performance Troubleshooting](#performance-troubleshooting)
5. [Database Issues](#database-issues)
6. [Worker Problems](#worker-problems)
7. [Network Issues](#network-issues)
8. [Capacity Management](#capacity-management)

---

## Quick Reference

### Service Status Check

```bash
# Check all services
systemctl status ffmpeg-rtmp-master
systemctl status ffmpeg-rtmp-worker@*

# Check logs
journalctl -u ffmpeg-rtmp-master -f
journalctl -u ffmpeg-rtmp-worker@1 -f

# Check metrics
curl http://localhost:9090/metrics  # Master
curl http://worker-ip:9091/metrics   # Worker
```

### Emergency Commands

```bash
# Stop all services
systemctl stop ffmpeg-rtmp-master
systemctl stop 'ffmpeg-rtmp-worker@*'

# Restart master only
systemctl restart ffmpeg-rtmp-master

# Kill stuck worker processes
pkill -9 -f "ffmpeg|gst-launch"

# Clear job queue (emergency only!)
sqlite3 master.db "UPDATE jobs SET status='failed' WHERE status='running'"
```

---

## Common Issues

### Issue: Master Not Responding

**Symptoms:**
- HTTP requests timing out
- Workers can't register
- CLI commands fail

**Diagnosis:**
```bash
# Check if process is running
ps aux | grep master

# Check port binding
netstat -tlnp | grep 8080

# Check logs
journalctl -u ffmpeg-rtmp-master -n 100
```

**Solutions:**

1. **Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080
# Kill it or change master port
systemctl restart ffmpeg-rtmp-master
```

2. **Database locked:**
```bash
# Check for lock files
ls -la master.db*
# Remove stale locks
rm -f master.db-wal master.db-shm
systemctl restart ffmpeg-rtmp-master
```

3. **Out of memory:**
```bash
# Check memory
free -h
# Check master process
ps aux | grep master | awk '{print $6}'
# Restart with more memory or add swap
```

### Issue: Worker Not Registering

**Symptoms:**
- Worker starts but doesn't appear in node list
- "Failed to register" errors in logs

**Diagnosis:**
```bash
# Test connectivity
curl -v http://master-ip:8080/health

# Check worker logs
journalctl -u ffmpeg-rtmp-worker@1 -n 50

# Verify API key
grep API_KEY /etc/systemd/system/ffmpeg-rtmp-worker@.service
```

**Solutions:**

1. **Network connectivity:**
```bash
# Test connection
telnet master-ip 8080
# Check firewall
sudo ufw status
sudo ufw allow 8080/tcp
```

2. **API key mismatch:**
```bash
# Verify master API key
grep api_key config.yaml
# Update worker systemd service
systemctl edit ffmpeg-rtmp-worker@1
# Add: Environment="FFMPEG_RTMP_API_KEY=your-key-here"
systemctl daemon-reload
systemctl restart ffmpeg-rtmp-worker@1
```

3. **Certificate issues (TLS):**
```bash
# Test TLS connection
openssl s_client -connect master-ip:8080
# Check certificate validity
openssl x509 -in /path/to/cert.pem -text -noout
# Use -insecure-skip-verify for testing (NOT production!)
```

### Issue: Jobs Stuck in "Running" State

**Symptoms:**
- Jobs remain in running status indefinitely
- Worker shows no active jobs
- Master shows jobs assigned to workers

**Diagnosis:**
```bash
# List stuck jobs
curl http://master:8080/api/v1/jobs?status=running

# Check if worker process actually running
ps aux | grep ffmpeg

# Check worker logs
journalctl -u ffmpeg-rtmp-worker@1 | grep "EXECUTING JOB"
```

**Solutions:**

1. **Worker crashed during execution:**
```bash
# Cancel stuck jobs manually
for job_id in $(curl -s http://master:8080/api/v1/jobs?status=running | jq -r '.jobs[].id'); do
  curl -X DELETE http://master:8080/api/v1/jobs/$job_id
done

# Or reset via database
sqlite3 master.db "UPDATE jobs SET status='failed', error='Worker crashed' WHERE status='running'"
```

2. **Worker disconnected:**
```bash
# Check worker heartbeat
curl http://master:8080/api/v1/nodes

# Restart worker
systemctl restart ffmpeg-rtmp-worker@1
```

3. **Process timeout not working:**
```bash
# Check resource limits configured
grep -A5 "resource_limits" /path/to/job.json

# Manually kill process
pkill -9 -f "ffmpeg.*job-id"

# Verify timeout_sec is set in job parameters
```

### Issue: High Failure Rate

**Symptoms:**
- Many jobs failing
- Alert: `HighJobFailureRate` firing

**Diagnosis:**
```bash
# Check failure rate
curl -s http://prometheus:9090/api/v1/query?query='rate(ffrtmp_worker_jobs_failed_total[5m])'

# Get recent failed jobs
curl http://master:8080/api/v1/jobs?status=failed | jq '.jobs[:10]'

# Check error patterns
sqlite3 master.db "SELECT error, COUNT(*) FROM jobs WHERE status='failed' GROUP BY error ORDER BY COUNT(*) DESC LIMIT 10"
```

**Solutions:**

1. **Input file issues:**
```bash
# Verify input files exist
ls -lh /path/to/inputs/

# Check permissions
ls -la /path/to/inputs/

# Test input file manually
ffmpeg -i /path/to/input.mp4 -t 1 -f null -
```

2. **Encoder not available:**
```bash
# Check encoder availability on worker
ffmpeg -encoders | grep h264
# NVENC: h264_nvenc
# QSV: h264_qsv
# VAAPI: h264_vaapi

# Test specific encoder
ffmpeg -f lavfi -i testsrc=duration=1:size=1280x720:rate=30 -c:v h264_nvenc -f null -
```

3. **Resource exhaustion:**
```bash
# Check disk space
df -h /tmp

# Check memory
free -h

# Check CPU load
uptime

# Increase resource limits in job parameters
```

### Issue: SLA Compliance Below Target

**Symptoms:**
- Alert: `SLAComplianceBelowTarget` firing
- Metric: `ffrtmp_worker_sla_compliance_rate < 95`

**Diagnosis:**
```bash
# Check current SLA compliance
curl -s http://prometheus:9090/api/v1/query?query='avg(ffrtmp_worker_sla_compliance_rate)'

# Get P95 job duration
curl -s http://prometheus:9090/api/v1/query?query='histogram_quantile(0.95,rate(job_duration_seconds_bucket[1h]))'

# Check queue depth
curl http://master:8080/api/v1/jobs?status=queued | jq '.jobs | length'
```

**Solutions:**

1. **Queue backlog:**
```bash
# Add more workers
systemctl start ffmpeg-rtmp-worker@{2..5}

# Increase max_concurrent_jobs per worker
# Edit: /etc/systemd/system/ffmpeg-rtmp-worker@.service
# Change: --max-concurrent-jobs=4
systemctl daemon-reload
systemctl restart 'ffmpeg-rtmp-worker@*'
```

2. **Jobs taking too long:**
```bash
# Use faster preset
# In job parameters:
"codec_params": {
  "preset": "faster"  // or "veryfast", "ultrafast"
}

# Use hardware encoding
"codec": "h264_nvenc"  // Instead of libx264

# Reduce quality/bitrate
"bitrate": "2M"  // Instead of "5M"
```

3. **Worker performance issues:**
```bash
# Check CPU usage
top -bn1 | grep ffmpeg

# Check for CPU throttling
dmesg | grep -i throttl

# Check disk I/O
iostat -x 1 5

# Optimize with SSD or faster storage
```

---

## Alert Response

### Critical Alerts

#### MasterNodeDown
**Priority:** P1  
**Impact:** System offline, no jobs can be submitted

**Response:**
1. Check if master process is running:
   ```bash
   systemctl status ffmpeg-rtmp-master
   ```
2. Check logs for errors:
   ```bash
   journalctl -u ffmpeg-rtmp-master -n 100
   ```
3. Restart master:
   ```bash
   systemctl restart ffmpeg-rtmp-master
   ```
4. If database issues, check locks:
   ```bash
   ls -la master.db*
   rm -f master.db-shm master.db-wal
   ```
5. Escalate if doesn't recover in 5 minutes

#### AllWorkersDown
**Priority:** P1  
**Impact:** No job processing capacity

**Response:**
1. Check if any workers running:
   ```bash
   systemctl status 'ffmpeg-rtmp-worker@*'
   ```
2. Check network connectivity:
   ```bash
   ping -c 3 worker-1
   ```
3. Restart all workers:
   ```bash
   systemctl restart 'ffmpeg-rtmp-worker@*'
   ```
4. Check for mass failure cause (network outage, DNS issue, etc.)
5. Escalate if pattern unclear

#### CriticalFailureRate
**Priority:** P1  
**Impact:** >10% of jobs failing

**Response:**
1. Check error patterns:
   ```bash
   sqlite3 master.db "SELECT error, COUNT(*) FROM jobs WHERE status='failed' AND created_at > datetime('now', '-1 hour') GROUP BY error"
   ```
2. Check recent changes (deployments, config updates)
3. If single error type, apply specific fix
4. If varied errors, check infrastructure (disk, network, etc.)
5. Document root cause in incident report

### Warning Alerts

#### HighJobLatency
**Priority:** P2  
**Impact:** Jobs taking longer than expected

**Response:**
1. Check queue depth and add workers if needed
2. Review resource utilization (CPU, memory, disk)
3. Consider increasing `max_concurrent_jobs`
4. Monitor for 30 minutes, escalate if worsening

#### WorkerCapacityHigh
**Priority:** P2  
**Impact:** Workers approaching max capacity

**Response:**
1. Plan to add more workers
2. Review job distribution (check if balanced)
3. Consider autoscaling if available
4. Monitor trends to predict when capacity will be exhausted

---

## Performance Troubleshooting

### Slow Job Execution

**Diagnosis:**
```bash
# Check job duration distribution
curl -s http://prometheus:9090/api/v1/query?query='histogram_quantile(0.95,rate(job_duration_seconds_bucket[1h]))'

# Check CPU usage
top -bn1 | head -20

# Check disk I/O
iostat -x 1 5

# Check network bandwidth
iftop
```

**Optimization:**
1. **Use hardware encoding**: `h264_nvenc` instead of `libx264`
2. **Faster presets**: `preset=faster` or `preset=veryfast`
3. **Reduce quality**: Lower bitrate or use CRF
4. **More workers**: Distribute load
5. **SSD storage**: Use for `/tmp` and output

### High CPU Usage

**Diagnosis:**
```bash
# Find CPU-intensive processes
top -b -n 1 -o %CPU | head -20

# Check per-core usage
mpstat -P ALL 1 5

# Check CPU frequency
cat /proc/cpuinfo | grep MHz
```

**Solutions:**
1. Reduce `max_concurrent_jobs` per worker
2. Use hardware encoding (offload to GPU)
3. Add more CPU cores or workers
4. Use faster CPU preset (less quality but faster)

### Memory Issues

**Diagnosis:**
```bash
# Check memory usage
free -h
vmstat 1 5

# Check for OOM kills
dmesg | grep -i oom

# Check swap usage
swapon --show
```

**Solutions:**
1. Increase memory limits in resource_limits
2. Add more RAM
3. Enable/increase swap
4. Reduce `max_concurrent_jobs`
5. Check for memory leaks (restart workers periodically)

---

## Database Issues

### Database Lock Errors

**Symptoms:**
- "database is locked" errors in logs
- Slow database operations

**Solutions:**
```bash
# Switch to PostgreSQL for production
# SQLite not recommended for high concurrency

# Or increase timeout
# config.yaml:
database:
  sqlite_busy_timeout: 30000  # 30 seconds
```

### Database Growing Too Large

**Diagnosis:**
```bash
# Check database size
du -h master.db

# Count records
sqlite3 master.db "SELECT COUNT(*) FROM jobs"
```

**Solutions:**
```bash
# Implement job retention policy
sqlite3 master.db "DELETE FROM jobs WHERE status IN ('completed', 'failed') AND completed_at < datetime('now', '-30 days')"

# Vacuum to reclaim space
sqlite3 master.db "VACUUM"

# Set up automatic cleanup (cron job)
```

### Query Performance Degradation

**Diagnosis:**
```bash
# Check query execution time
sqlite3 master.db ".timer on" "SELECT * FROM jobs LIMIT 1000"

# Check indexes
sqlite3 master.db ".indexes"
```

**Solutions:**
```bash
# Add missing indexes
sqlite3 master.db "CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status)"
sqlite3 master.db "CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at)"

# Analyze database
sqlite3 master.db "ANALYZE"
```

---

## Worker Problems

### Worker Crashing Frequently

**Diagnosis:**
```bash
# Check crash logs
journalctl -u ffmpeg-rtmp-worker@1 | grep -i "panic\|fatal\|crash"

# Check system logs
dmesg | tail -100

# Check for segfaults
grep -i segfault /var/log/syslog
```

**Solutions:**
1. Update FFmpeg/GStreamer to latest stable version
2. Check for hardware issues (RAM test, disk SMART)
3. Reduce `max_concurrent_jobs`
4. Enable core dumps for debugging:
   ```bash
   ulimit -c unlimited
   echo "/tmp/core.%e.%p" > /proc/sys/kernel/core_pattern
   ```

### Worker Not Picking Up Jobs

**Diagnosis:**
```bash
# Check worker status
curl http://master:8080/api/v1/nodes

# Check poll interval
ps aux | grep agent | grep poll-interval

# Check API key
env | grep FFMPEG_RTMP_API_KEY
```

**Solutions:**
1. Restart worker
2. Check network connectivity to master
3. Verify API key matches
4. Check worker logs for registration errors

---

## Network Issues

### High Network Latency

**Diagnosis:**
```bash
# Check latency
ping -c 100 master-ip | tail -5

# Check packet loss
mtr master-ip

# Check bandwidth
iperf3 -c master-ip
```

**Solutions:**
1. Use same data center/region for master and workers
2. Check for network congestion
3. Use dedicated network (not shared)
4. Consider CDN for input files

### Bandwidth Saturation

**Diagnosis:**
```bash
# Check current bandwidth
iftop -i eth0

# Check Prometheus metric
curl -s http://prometheus:9090/api/v1/query?query='sum(rate(ffrtmp_job_input_bytes_total[5m]))'
```

**Solutions:**
1. Upgrade network interface (10 Gbps recommended)
2. Use local storage for inputs (avoid network FS)
3. Implement bandwidth limits per job
4. Add more workers to distribute load

---

## Capacity Management

### Predicting Capacity Needs

**Formulas:**

**Jobs per hour:**
```
jobs_per_hour = workers * max_concurrent_jobs * 3600 / avg_job_duration
```

**Storage required:**
```
storage_gb = jobs_per_day * avg_file_size_gb * retention_days
```

**Bandwidth required:**
```
bandwidth_mbps = workers * max_concurrent_jobs * avg_job_bandwidth
```

### Scaling Guidelines

| Metric | Threshold | Action |
|--------|-----------|--------|
| CPU Usage | > 80% sustained | Add workers or CPU cores |
| Memory Usage | > 85% | Add RAM or reduce concurrent jobs |
| Disk Usage | > 90% | Add storage or reduce retention |
| Queue Depth | > 100 jobs | Add workers |
| SLA Compliance | < 95% | Add capacity or optimize |

---

## Related Documentation

- [Production Operations Guide](PRODUCTION_OPERATIONS.md)
- [Resource Limits Guide](RESOURCE_LIMITS.md)
- [Alerting Guide](ALERTING.md)
- [SLA Tracking Guide](SLA_TRACKING.md)

## Emergency Contacts

- On-call Engineer: [PagerDuty rotation]
- Infrastructure Team: infrastructure@example.com
- Database Team: database@example.com
