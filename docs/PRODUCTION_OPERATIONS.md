# Production Operations Guide

Complete operational guide for running FFmpeg RTMP in production environments.

## Table of Contents

1. [Running as Root](#running-as-root)
2. [Resource Management](#resource-management)
3. [Security Best Practices](#security-best-practices)
4. [Monitoring and Alerting](#monitoring-and-alerting)
5. [Performance Optimization](#performance-optimization)
6. [Capacity Planning](#capacity-planning)
7. [Incident Response](#incident-response)
8. [Maintenance Procedures](#maintenance-procedures)

---

## Running as Root

### Why Root Access is Required

**For production deployments, worker agents MUST run as root** to enable full resource management capabilities through Linux cgroups.

### Without Root vs With Root

| Capability | Without Root | With Root |
|------------|--------------|-----------|
| **CPU Limits** | ⚠️ Soft (nice only) | ✅ Hard (cgroup enforcement) |
| **Memory Limits** | ❌ None | ✅ Hard caps + OOM protection |
| **Process Isolation** | ⚠️ Limited | ✅ Complete cgroup isolation |
| **Resource Accounting** | ⚠️ Best-effort | ✅ Accurate per-job metrics |
| **Multi-tenant Safety** | ❌ Not safe | ✅ Full isolation |
| **Production Ready** | ❌ Not recommended | ✅ Yes |

### Security Architecture with Root

The system is designed to run safely as root:

**Defense in Depth:**
1. **Process Isolation**: Each job runs in separate cgroup namespace
2. **Resource Limits**: Hard caps prevent resource exhaustion
3. **No Shell Access**: Jobs execute directly via exec (no shell)
4. **Input Validation**: All parameters validated before execution
5. **Automatic Cleanup**: Process groups killed on timeout/failure
6. **Systemd Hardening**: Additional sandboxing via systemd units
7. **Audit Logging**: All job executions logged

**Attack Surface Minimization:**
- Worker runs on dedicated compute nodes (no other services)
- Only outbound HTTPS connection to master (no inbound)
- No SSH access required for operation (systemd managed)
- File system access restricted to /tmp and /var/lib/ffmpeg-worker
- No privilege escalation possible (NoNewPrivileges=yes)

### Systemd Security Hardening

Worker service includes comprehensive systemd sandboxing:

```ini
[Service]
# Run as root for cgroup access
User=root
Group=root

# Security hardening
NoNewPrivileges=yes                # Prevent privilege escalation
PrivateTmp=yes                     # Private /tmp per service
ProtectSystem=strict               # Read-only /usr, /boot
ProtectHome=yes                    # No access to home directories
ProtectKernelTunables=yes         # Read-only /proc/sys
ProtectKernelModules=yes          # Cannot load kernel modules
ProtectControlGroups=no           # Need cgroup access
RestrictRealtime=yes              # No realtime scheduling
RestrictNamespaces=no             # Allow PID namespace
LockPersonality=yes               # Lock personality syscall
SystemCallArchitectures=native    # No 32-bit on 64-bit system

# Resource limits for service itself
MemoryMax=1G                      # Service overhead limit
TasksMax=1024                     # Max concurrent processes
```

### Alternative: Rootless Deployment (Not Recommended)

If root access is absolutely not possible, workers can run in degraded mode:

**Limitations:**
- ❌ No CPU enforcement (jobs can use all cores)
- ❌ No memory enforcement (OOM risk)
- ⚠️ Shared system resources (one job affects others)
- ⚠️ Cannot safely run multiple jobs concurrently
- ❌ Not suitable for production use

**Configuration:**
```bash
# Run as regular user (NOT RECOMMENDED FOR PRODUCTION)
./bin/agent \
  --register \
  --master https://master:8080 \
  --max-concurrent-jobs 1    # MUST be 1 without root
  --api-key "$API_KEY"

# Note: Disk monitoring and timeouts still work
```

**Use Cases for Rootless:**
- Development/testing only
- Proof-of-concept demonstrations
- Single-tenant dedicated hardware

---

## Resource Management

### Resource Limit Architecture

Every job can specify resource limits to ensure system stability:

```json
{
  "scenario": "1080p-h264",
  "parameters": {
    "duration": 300,
    "bitrate": "4M"
  },
  "resource_limits": {
    "max_cpu_percent": 200,      // 200% = 2 CPU cores
    "max_memory_mb": 2048,        // 2GB hard limit
    "max_disk_mb": 5000,          // 5GB temp space required
    "timeout_sec": 600            // 10 minute timeout
  }
}
```

### Default Resource Limits

If not specified, sensible defaults are applied:

| Resource | Default | Adjustable |
|----------|---------|------------|
| **CPU** | All cores (numCPU × 100%) | Yes, per-job |
| **Memory** | 2048 MB (2GB) | Yes, per-job |
| **Disk** | 5000 MB (5GB) | Yes, per-job |
| **Timeout** | 3600 seconds (1 hour) | Yes, per-job |

### Resource Limit Enforcement

**CPU Limits (cgroup-based):**
```bash
# Set in /sys/fs/cgroup/ffmpeg-job-{id}/cpu.max
# Format: quota period (both in microseconds)
# Example: 200000 100000 = 200% (2 cores)

# Verified enforcement:
$ cat /sys/fs/cgroup/ffmpeg-job-abc/cpu.max
200000 100000

# Monitor actual usage:
$ systemd-cgtop | grep ffmpeg-job
```

**Memory Limits (cgroup-based):**
```bash
# Set in /sys/fs/cgroup/ffmpeg-job-{id}/memory.max
# Hard limit - process killed if exceeded (OOM)

$ cat /sys/fs/cgroup/ffmpeg-job-abc/memory.max
2147483648  # 2GB in bytes

# Monitor current usage:
$ cat /sys/fs/cgroup/ffmpeg-job-abc/memory.current
1048576000  # Currently using ~1GB
```

**Disk Space (pre-job validation):**
```bash
# Checked before job starts
# Rejects if < 5% available
# Warns at 90% usage

>>> RESOURCE CHECK PHASE <<<
Disk space: 89.5% used (25000 MB available)
Resource limits: max_disk_mb=5000
✓ Sufficient disk space available
```

**Timeout (process monitoring):**
```bash
# Enforced via context.WithTimeout + monitoring goroutine
# SIGTERM after timeout, SIGKILL if needed
# Cleans up entire process group

Process 12345 exceeded timeout (600 seconds), killing...
✓ Process group terminated
```

### Best Practices by Workload Type

**Live Streaming (Low Latency):**
```json
"resource_limits": {
  "max_cpu_percent": 150,       // 1.5 cores (realtime constraint)
  "max_memory_mb": 1024,        // 1GB (streaming buffer)
  "timeout_sec": 7200           // 2 hours (long streams)
}
```

**Batch Transcoding (High Quality):**
```json
"resource_limits": {
  "max_cpu_percent": 400,       // 4 cores (quality over speed)
  "max_memory_mb": 2048,        // 2GB (filter chains)
  "timeout_sec": 3600           // 1 hour (long videos)
}
```

**Fast Preview Generation:**
```json
"resource_limits": {
  "max_cpu_percent": 100,       // 1 core (quick turnaround)
  "max_memory_mb": 512,         // 512MB (minimal)
  "timeout_sec": 300            // 5 minutes
}
```

**4K HDR (Heavy Workload):**
```json
"resource_limits": {
  "max_cpu_percent": 800,       // 8 cores (parallel encoding)
  "max_memory_mb": 8192,        // 8GB (large frames)
  "timeout_sec": 7200           // 2 hours
}
```

### Resource Planning

**Calculate Worker Capacity:**

```python
# CPU capacity
cpu_cores = 16
cpu_per_job = 2  # 200%
max_jobs_cpu = cpu_cores / cpu_per_job  # = 8 jobs

# Memory capacity
total_memory_mb = 32768  # 32GB
memory_per_job = 2048    # 2GB
max_jobs_mem = total_memory_mb / memory_per_job  # = 16 jobs

# Actual capacity (limited by smallest)
max_concurrent_jobs = min(max_jobs_cpu, max_jobs_mem)  # = 8 jobs
```

**Disk Capacity Planning:**

```bash
# Estimate temp space per job
bitrate_mbps = 4
duration_sec = 300
temp_space_mb = bitrate_mbps * duration_sec * 2  # 2x for safety
# = 4 * 300 * 2 = 2400 MB per job

# Required disk space
concurrent_jobs = 8
required_disk_mb = concurrent_jobs * temp_space_mb * 1.5  # +50% buffer
# = 8 * 2400 * 1.5 = 28800 MB = ~29 GB

# Recommendation: 50GB+ for /tmp
```

### Monitoring Resource Usage

**Real-time Monitoring:**
```bash
# System-wide view
htop

# Cgroup-specific
sudo systemd-cgtop

# Per-job details
sudo cat /sys/fs/cgroup/ffmpeg-job-*/cpu.stat
sudo cat /sys/fs/cgroup/ffmpeg-job-*/memory.current
```

**Metrics Collection:**
```bash
# Worker exports Prometheus metrics on port 9091
curl localhost:9091/metrics | grep -E "(cpu|memory|disk)"

# Key metrics:
# - jobs_cpu_percent_bucket
# - jobs_memory_mb_bucket  
# - jobs_duration_seconds_bucket
# - disk_space_available_mb
```

---

## Security Best Practices

### Network Security

**Firewall Configuration:**
```bash
# Worker nodes (restrictive)
sudo ufw default deny incoming
sudo ufw default deny outgoing
sudo ufw allow out to <MASTER_IP> port 8080 proto tcp
sudo ufw allow out to <DNS_SERVER> port 53
sudo ufw allow out port 80,443 proto tcp  # For package updates

# Allow metrics if needed
sudo ufw allow from <MONITORING_SUBNET> to any port 9091 proto tcp

sudo ufw enable
```

**TLS Certificate Validation:**
```bash
# Production: Use proper CA-signed certificates
./bin/agent \
  --master https://master.company.com:8080 \
  --ca /etc/ssl/certs/company-ca.crt

# NOT: --insecure-skip-verify (development only)
```

### Authentication

**API Key Management:**
```bash
# Generate strong API key
MASTER_API_KEY=$(openssl rand -base64 32)

# Store securely (never in source control)
echo "$MASTER_API_KEY" | sudo tee /etc/ffmpeg-worker/api-key
sudo chmod 600 /etc/ffmpeg-worker/api-key

# Use in systemd service
EnvironmentFile=/etc/ffmpeg-worker/api-key
```

**Key Rotation:**
```bash
# Generate new key
NEW_KEY=$(openssl rand -base64 32)

# Update master to accept both keys (transition period)
# Update workers one by one
# Remove old key after all workers updated
```

### Audit Logging

**Enable Audit Trail:**
```bash
# Install auditd
sudo apt-get install -y auditd

# Monitor worker binary
sudo auditctl -w /usr/local/bin/ffmpeg-worker -p x -k worker-exec

# Monitor configuration changes
sudo auditctl -w /etc/ffmpeg-worker -p wa -k worker-config

# Monitor cgroup operations
sudo auditctl -w /sys/fs/cgroup -p wa -k cgroup-ops

# Search audit logs
sudo ausearch -k worker-exec -ts recent
```

### File System Security

**Protect Sensitive Data:**
```bash
# Restrict access to config
sudo chmod 600 /etc/ffmpeg-worker/*
sudo chown root:root /etc/ffmpeg-worker/*

# Restrict logs
sudo chmod 640 /var/log/ffmpeg-worker/*
sudo chown root:root /var/log/ffmpeg-worker/*

# Temporary files auto-cleaned
# Set systemd PrivateTmp=yes for isolation
```

---

## Monitoring and Alerting

### Key Metrics to Monitor

**System Health:**
- Worker registration status (heartbeat)
- Queue depth (jobs waiting)
- Active jobs count
- Failed jobs rate
- Job completion latency

**Resource Utilization:**
- CPU usage per worker (should not exceed configured limits)
- Memory usage per worker
- Disk space available (/tmp)
- Network bandwidth

**Job Performance:**
- Average job duration by scenario
- P50, P95, P99 latency
- Success rate by scenario
- Timeout rate
- Retry rate

### Prometheus Queries

**Worker Availability:**
```promql
# Workers online
up{job="ffmpeg-worker"} == 1

# Workers missing heartbeat (alert if > 0)
time() - worker_last_heartbeat_timestamp > 60
```

**Resource Usage:**
```promql
# CPU usage by worker
rate(process_cpu_seconds_total{job="ffmpeg-worker"}[5m]) * 100

# Memory usage by worker
process_resident_memory_bytes{job="ffmpeg-worker"} / 1024 / 1024

# Disk space remaining
disk_space_available_mb < 10000  # Alert if < 10GB
```

**Job Metrics:**
```promql
# Job failure rate (alert if > 10%)
rate(jobs_failed_total[5m]) / rate(jobs_total[5m]) * 100 > 10

# Job duration P95
histogram_quantile(0.95, rate(jobs_duration_seconds_bucket[5m]))

# Queue depth (alert if > 100)
jobs_queued_total > 100
```

### Alerting Rules

```yaml
# /etc/prometheus/alerts/ffmpeg-worker.yml
groups:
- name: ffmpeg_worker_alerts
  interval: 30s
  rules:
  
  # Critical: Worker down
  - alert: WorkerDown
    expr: up{job="ffmpeg-worker"} == 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Worker {{ $labels.instance }} is down"
      description: "Worker has been down for more than 2 minutes"
  
  # Critical: High failure rate
  - alert: HighJobFailureRate
    expr: rate(jobs_failed_total[5m]) / rate(jobs_total[5m]) * 100 > 20
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High job failure rate on {{ $labels.instance }}"
      description: "{{ $value | humanize }}% of jobs failing"
  
  # Warning: Low disk space
  - alert: LowDiskSpace
    expr: disk_space_available_mb < 10000
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Low disk space on {{ $labels.instance }}"
      description: "Only {{ $value }} MB available"
  
  # Warning: High queue depth
  - alert: HighQueueDepth
    expr: jobs_queued_total > 100
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "High queue depth: {{ $value }} jobs"
      description: "Jobs are queuing up, may need more workers"
```

### Grafana Dashboards

**Worker Overview Dashboard:**
- Worker status (up/down)
- Active jobs per worker
- CPU/memory usage
- Jobs completed/failed rate
- Queue depth over time

**Job Performance Dashboard:**
- Job duration histogram
- Success rate by scenario
- Failure reasons breakdown
- Resource usage distribution
- Throughput (jobs/minute)

**Capacity Planning Dashboard:**
- Worker utilization trends
- Peak load analysis
- Resource headroom
- Growth projections

---

## Performance Optimization

### Worker Configuration Tuning

**Optimal Concurrent Jobs:**
```bash
# Calculate based on CPU cores
NUM_CORES=$(nproc)

# For CPU-bound workloads (transcoding)
OPTIMAL_JOBS=$((NUM_CORES * 75 / 100))  # 75% utilization

# For I/O-bound workloads (streaming)
OPTIMAL_JOBS=$((NUM_CORES))  # 100% utilization

# For mixed workloads
OPTIMAL_JOBS=$((NUM_CORES * 85 / 100))  # 85% utilization

echo "Recommended: --max-concurrent-jobs $OPTIMAL_JOBS"
```

### System Tuning

**CPU Performance Mode:**
```bash
# Set CPU governor to performance
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor

# Or install cpufrequtils
sudo apt-get install cpufrequtils
echo 'GOVERNOR="performance"' | sudo tee /etc/default/cpufrequtils
sudo systemctl restart cpufrequtils
```

**Disk I/O Optimization:**
```bash
# Use tmpfs for /tmp (RAM disk)
# Add to /etc/fstab:
tmpfs /tmp tmpfs defaults,size=20G 0 0

# Or use fast SSD with elevator=noop
echo noop | sudo tee /sys/block/nvme0n1/queue/scheduler
```

**Network Tuning:**
```bash
sudo tee -a /etc/sysctl.conf << EOF
# Increase TCP buffer sizes
net.core.rmem_max=134217728
net.core.wmem_max=134217728
net.ipv4.tcp_rmem=4096 87380 67108864
net.ipv4.tcp_wmem=4096 65536 67108864

# Enable BBR congestion control
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF

sudo sysctl -p
```

### Job Optimization

**Use Hardware Acceleration:**
```bash
# NVIDIA GPU
{
  "parameters": {
    "encoder": "h264_nvenc",
    "preset": "p4"  # Faster on GPU
  }
}

# Intel QSV
{
  "parameters": {
    "encoder": "h264_qsv",
    "preset": "veryfast"
  }
}
```

**Optimize Encoding Settings:**
```bash
# Faster preset for time-sensitive jobs
{
  "parameters": {
    "preset": "ultrafast",   # Fastest encoding
    "tune": "zerolatency"    # Low latency
  }
}

# Better quality for batch jobs
{
  "parameters": {
    "preset": "medium",      # Balanced
    "crf": 23                # Quality target
  }
}
```

---

## Capacity Planning

### Worker Sizing

**Small Worker (Development/Testing):**
- 4 CPU cores
- 8GB RAM
- 50GB SSD
- max-concurrent-jobs: 2-3
- **Cost**: ~$50/month (cloud)

**Medium Worker (Production):**
- 8 CPU cores
- 16GB RAM
- 100GB SSD
- max-concurrent-jobs: 4-6
- **Cost**: ~$150/month (cloud)

**Large Worker (High Volume):**
- 16 CPU cores
- 32GB RAM
- 200GB SSD
- max-concurrent-jobs: 8-12
- **Cost**: ~$300/month (cloud)

**GPU Worker (Hardware Acceleration):**
- 8 CPU cores
- 16GB RAM
- NVIDIA T4 or better
- 100GB SSD
- max-concurrent-jobs: 16-32 (GPU limited)
- **Cost**: ~$500/month (cloud)

### Scaling Strategy

**Horizontal Scaling (Add Workers):**
```bash
# When to add workers:
# - Queue depth consistently > 50
# - Job completion latency > 5 minutes
# - Worker CPU utilization > 90%

# Formula:
required_workers = peak_jobs_per_hour / (jobs_per_worker_per_hour)
```

**Vertical Scaling (Upgrade Workers):**
```bash
# When to upgrade:
# - Individual jobs timing out
# - Memory pressure (OOM kills)
# - Cannot increase concurrent jobs

# Upgrade path:
# Small → Medium → Large → GPU
```

### Cost Optimization

**Use Spot/Preemptible Instances:**
```bash
# Workers are stateless - perfect for spot instances
# Save 60-90% on cloud costs
# Configure auto-retry on master for interruptions
```

**Schedule Workers:**
```bash
# Scale down during off-peak hours
# Use cron or cloud auto-scaling
# Keep minimum workers for on-demand jobs
```

**Right-size Resources:**
```bash
# Monitor actual usage vs limits
# Adjust max-concurrent-jobs based on metrics
# Use smaller instances if underutilized
```

---

## Incident Response

### Common Incidents

**1. Worker Offline**
```bash
# Check service status
sudo systemctl status ffmpeg-worker

# Check network connectivity
curl -k https://MASTER_IP:8080/health

# Check logs
sudo journalctl -u ffmpeg-worker -n 100

# Recovery:
sudo systemctl restart ffmpeg-worker
```

**2. Jobs Failing**
```bash
# Check failure reason
curl -k https://MASTER_IP:8080/jobs?status=failed | jq '.[0].error'

# Common causes:
# - Out of disk space: df -h /tmp
# - Out of memory: dmesg | grep -i oom
# - Timeout: Check job duration vs timeout_sec

# Recovery:
# - Clean disk: sudo find /tmp -name "input_*" -delete
# - Adjust limits: Update resource_limits in job
# - Restart worker: sudo systemctl restart ffmpeg-worker
```

**3. High Queue Depth**
```bash
# Check queue status
curl -k https://MASTER_IP:8080/jobs?status=queued | jq 'length'

# Causes:
# - Not enough workers
# - Workers offline
# - Jobs taking too long

# Recovery:
# - Add workers
# - Increase max-concurrent-jobs
# - Optimize job parameters
```

**4. Resource Exhaustion**
```bash
# Check system resources
htop
df -h
free -h

# Check cgroup limits
sudo systemd-cgtop

# Recovery:
# - Clean temp files
# - Lower concurrent jobs
# - Add RAM/disk
# - Adjust job limits
```

### Incident Playbooks

See **[INCIDENT_PLAYBOOKS.md](INCIDENT_PLAYBOOKS.md)** for detailed step-by-step recovery procedures.

---

## Maintenance Procedures

### Daily Maintenance

**Check System Health:**
```bash
#!/bin/bash
# Daily health check script

# Service status
systemctl is-active ffmpeg-worker || echo "ERROR: Worker service down"

# Disk space
DISK_USED=$(df /tmp | tail -1 | awk '{print $5}' | sed 's/%//')
if [ "$DISK_USED" -gt 80 ]; then
  echo "WARNING: Disk usage at ${DISK_USED}%"
fi

# Check for failed jobs
FAILED=$(curl -s -k https://MASTER_IP:8080/jobs?status=failed | jq 'length')
if [ "$FAILED" -gt 10 ]; then
  echo "WARNING: ${FAILED} failed jobs"
fi
```

### Weekly Maintenance

**Clean Temporary Files:**
```bash
# Clean files older than 7 days
sudo find /tmp -name "input_*" -mtime +7 -delete
sudo find /tmp -name "job_*_output.*" -mtime +7 -delete

# Clean orphaned cgroups
for cgroup in /sys/fs/cgroup/ffmpeg-job-*; do
  if ! pgrep -f "$(basename $cgroup)"; then
    sudo rmdir "$cgroup" 2>/dev/null
  fi
done
```

**Review Metrics:**
```bash
# Job success rate
# CPU/memory utilization
# Disk usage trends
# Queue depth patterns
```

### Monthly Maintenance

**System Updates:**
```bash
# Update OS packages
sudo apt-get update
sudo apt-get upgrade -y

# Update FFmpeg
sudo apt-get install --only-upgrade ffmpeg

# Rebuild worker (if updates available)
cd /opt/ffmpeg-rtmp
git pull
cd worker && make build
sudo cp bin/agent /usr/local/bin/ffmpeg-worker
sudo systemctl restart ffmpeg-worker
```

**Log Rotation:**
```bash
# Compress old logs
sudo journalctl --vacuum-time=30d

# Archive if needed
sudo tar czf logs-$(date +%Y%m).tar.gz /var/log/ffmpeg-worker/
```

**Performance Review:**
```bash
# Analyze job durations
# Review resource utilization
# Adjust worker sizing
# Plan capacity changes
```

---

## Additional Resources

- **[Resource Limits Documentation](RESOURCE_LIMITS.md)** - Complete API reference
- **[Worker Deployment Guide](../deployment/WORKER_DEPLOYMENT.md)** - Step-by-step setup
- **[Troubleshooting Guide](../shared/docs/troubleshooting.md)** - Common issues
- **[Production Features](../shared/docs/PRODUCTION_FEATURES.md)** - TLS, auth, retry

---

**Version**: 1.0  
**Last Updated**: 2026-01-05  
**Status**: Production Ready
