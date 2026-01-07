# Production Deployment Guide - Worker Nodes

Complete guide for deploying FFmpeg RTMP worker agents in production with full resource management capabilities.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Running as Root (Recommended)](#running-as-root-recommended)
4. [System Setup](#system-setup)
5. [Building and Installation](#building-and-installation)
6. [Resource Management Configuration](#resource-management-configuration)
7. [Systemd Service Configuration](#systemd-service-configuration)
8. [Starting the Service](#starting-the-service)
9. [Verification and Testing](#verification-and-testing)
10. [Security Hardening](#security-hardening)
11. [Troubleshooting](#troubleshooting)
12. [Performance Tuning](#performance-tuning)

---

## Overview

Worker nodes execute transcoding jobs distributed by the master node. For production deployments, workers should run with root privileges to enable full resource management and isolation capabilities.

**Key Features:**
- Automatic job polling and execution
- Hardware detection and optimization
- Resource limits per job (CPU, memory, disk)
- Process isolation via cgroups
- Automatic retry on failures
- Metrics collection and reporting

---

## Prerequisites

### Hardware Requirements

**Minimum:**
- 4 CPU cores
- 8 GB RAM
- 50 GB disk space (/tmp)
- Linux kernel 4.5+ (for cgroup v2)

**Recommended:**
- 8+ CPU cores (or GPU with hardware encoding)
- 16+ GB RAM
- 100+ GB SSD for /tmp
- Linux kernel 5.4+ with cgroup v2
- NVIDIA GPU (optional, for NVENC acceleration)

### Software Requirements

- **Operating System**: Linux with systemd and cgroup support
  - Ubuntu 20.04+ (recommended)
  - Debian 11+
  - CentOS 8+ / RHEL 8+
  - Fedora 31+
- **FFmpeg**: 4.4+ with required codecs
- **GStreamer**: 1.18+ (optional, for low-latency streaming)
- **Go**: 1.24+ (for building the binary)

### System Configuration

**Verify cgroup v2 support:**
```bash
# Check if cgroup v2 is mounted
mount | grep cgroup2
# Should show: cgroup2 on /sys/fs/cgroup type cgroup2

# Check cgroup version
stat -fc %T /sys/fs/cgroup
# Should output: cgroup2fs
```

**If cgroup v2 is not enabled**, add to GRUB:
```bash
sudo nano /etc/default/grub
# Add to GRUB_CMDLINE_LINUX:
systemd.unified_cgroup_hierarchy=1

# Update GRUB
sudo update-grub  # Ubuntu/Debian
sudo grub2-mkconfig -o /boot/grub2/grub.cfg  # CentOS/RHEL

# Reboot
sudo reboot
```

---

## Running as Root (Recommended)

### Why Run as Root?

**Production deployments should run worker agents as root** to enable complete resource management:

| Feature | With Root | Without Root |
|---------|-----------|--------------|
| **CPU Limits** |  Full enforcement via cgroups |  Soft limits (nice only) |
| **Memory Limits** |  Hard caps with OOM protection |  No enforcement |
| **Process Isolation** |  Complete cgroup isolation |  Limited |
| **Disk Monitoring** |  Always enforced |  Always enforced |
| **Timeout Enforcement** |  Always enforced |  Always enforced |
| **Multi-tenant Safety** |  Full isolation |  Shared resources |

### Security Considerations

Running as root is safe when properly configured:

** Security Measures in Place:**
- Process isolation via cgroups (each job in separate cgroup)
- Resource limits prevent DoS attacks
- Automatic cleanup of job processes
- No shell access in job execution
- Input validation on all parameters
- Separate cgroup namespace per job

** Best Practices:**
- Use dedicated server/VM for workers (no other services)
- Keep system and dependencies updated
- Configure firewall (only outbound HTTPS to master)
- Use systemd hardening options (see below)
- Monitor system logs for anomalies
- Rotate logs regularly

**Alternative: Rootless with cgroup delegation (Advanced)**

If you cannot run as root, enable cgroup delegation:
```bash
# Create user
sudo useradd -m -s /bin/bash ffmpeg-worker

# Enable cgroup delegation (systemd 252+)
sudo mkdir -p /etc/systemd/system/user@.service.d/
sudo tee /etc/systemd/system/user@.service.d/delegate.conf << EOF
[Service]
Delegate=yes
EOF

sudo systemctl daemon-reload

# Enable lingering
sudo loginctl enable-linger ffmpeg-worker
```

Note: Rootless mode has limitations and is not recommended for production.

---

## System Setup

### 1. Install Dependencies

**Ubuntu/Debian:**
```bash
# System utilities
sudo apt-get update
sudo apt-get install -y \
  build-essential \
  git \
  curl \
  wget \
  htop

# FFmpeg with full codec support
sudo apt-get install -y \
  ffmpeg \
  libavcodec-extra \
  libavformat-dev \
  libavutil-dev

# GStreamer (optional)
sudo apt-get install -y \
  gstreamer1.0-tools \
  gstreamer1.0-plugins-base \
  gstreamer1.0-plugins-good \
  gstreamer1.0-plugins-bad \
  gstreamer1.0-plugins-ugly \
  gstreamer1.0-libav \
  gstreamer1.0-rtsp

# Development tools
sudo apt-get install -y golang-1.24
```

**CentOS/RHEL:**
```bash
# Enable EPEL
sudo yum install -y epel-release

# System utilities
sudo yum install -y \
  gcc \
  make \
  git \
  curl \
  wget \
  htop

# FFmpeg (from RPM Fusion)
sudo yum install -y https://download1.rpmfusion.org/free/el/rpmfusion-free-release-8.noarch.rpm
sudo yum install -y ffmpeg ffmpeg-devel

# Go
sudo yum install -y golang

# GStreamer (optional)
sudo yum install -y \
  gstreamer1 \
  gstreamer1-plugins-base \
  gstreamer1-plugins-good \
  gstreamer1-plugins-bad-free \
  gstreamer1-plugins-ugly-free
```

### 2. Verify Installation

```bash
# Check FFmpeg
ffmpeg -version
# Should show: ffmpeg version 4.4+ or higher

# Check cgroup support
mount | grep cgroup2
# Should show cgroup2 mounted at /sys/fs/cgroup

# Check disk space
df -h /tmp
# Should have 50GB+ available

# Check kernel version
uname -r
# Should be 4.5+ (5.4+ recommended)
```

### 3. Configure System Limits

```bash
# Increase file descriptors for worker
sudo tee -a /etc/security/limits.conf << EOF
root soft nofile 65536
root hard nofile 65536
* soft nofile 65536
* hard nofile 65536
EOF

# Increase inotify limits (for file monitoring)
sudo tee -a /etc/sysctl.conf << EOF
fs.inotify.max_user_watches=524288
fs.file-max=2097152
EOF

sudo sysctl -p
```

---

## Building and Installation

### 1. Clone Repository

```bash
cd /opt
sudo git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
```

### 2. Build Worker Binary

```bash
# Build for production
cd worker
sudo make build

# Verify binary
./bin/agent --version
```

### 3. Install Binary

```bash
# Install to system path
sudo cp bin/agent /usr/local/bin/ffmpeg-worker
sudo chmod +x /usr/local/bin/ffmpeg-worker

# Verify installation
ffmpeg-worker --version
```

### 4. Create Directories

```bash
# Working directories
sudo mkdir -p /var/lib/ffmpeg-worker
sudo mkdir -p /var/log/ffmpeg-worker
sudo mkdir -p /etc/ffmpeg-worker

# Set permissions (owned by root)
sudo chown -R root:root /var/lib/ffmpeg-worker
sudo chown -R root:root /var/log/ffmpeg-worker
sudo chown -R root:root /etc/ffmpeg-worker
sudo chmod 700 /var/lib/ffmpeg-worker
sudo chmod 755 /var/log/ffmpeg-worker
sudo chmod 700 /etc/ffmpeg-worker
```

---

## Resource Management Configuration

### 1. Create Configuration File

```bash
sudo tee /etc/ffmpeg-worker/config.env << 'EOF'
# Master node connection
MASTER_URL=https://MASTER_IP:8080
MASTER_API_KEY=your-api-key-here

# Worker configuration
MAX_CONCURRENT_JOBS=4
POLL_INTERVAL=3s
HEARTBEAT_INTERVAL=30s

# Resource limits (defaults applied per job)
DEFAULT_CPU_PERCENT=200
DEFAULT_MEMORY_MB=2048
DEFAULT_DISK_MB=5000
DEFAULT_TIMEOUT_SEC=3600

# Features
GENERATE_INPUT=true
INSECURE_SKIP_VERIFY=false

# Metrics
METRICS_PORT=9091
EOF

sudo chmod 600 /etc/ffmpeg-worker/config.env
```

### 2. Set API Key

```bash
# Generate or get API key from master
MASTER_API_KEY=$(openssl rand -base64 32)

# Update config
sudo sed -i "s/your-api-key-here/$MASTER_API_KEY/" /etc/ffmpeg-worker/config.env
```

### 3. Update Master URL

```bash
# Replace with actual master IP
sudo sed -i "s/MASTER_IP/192.168.1.100/" /etc/ffmpeg-worker/config.env
```

---

## Systemd Service Configuration

### 1. Create Service File

```bash
sudo tee /etc/systemd/system/ffmpeg-worker.service << 'EOF'
[Unit]
Description=FFmpeg RTMP Worker Agent
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root

# Environment
EnvironmentFile=/etc/ffmpeg-worker/config.env
WorkingDirectory=/var/lib/ffmpeg-worker

# Binary and arguments
ExecStart=/usr/local/bin/ffmpeg-worker \
  --register \
  --master ${MASTER_URL} \
  --api-key ${MASTER_API_KEY} \
  --max-concurrent-jobs ${MAX_CONCURRENT_JOBS} \
  --poll-interval ${POLL_INTERVAL} \
  --heartbeat-interval ${HEARTBEAT_INTERVAL} \
  --generate-input=${GENERATE_INPUT} \
  --insecure-skip-verify=${INSECURE_SKIP_VERIFY} \
  --metrics-port ${METRICS_PORT} \
  --skip-confirmation \
  --allow-master-as-worker

# Restart policy
Restart=always
RestartSec=10s

# Resource limits for the service itself (not jobs)
MemoryMax=1G
TasksMax=1024

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/ffmpeg-worker /var/log/ffmpeg-worker /tmp
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=no
RestrictRealtime=yes
RestrictNamespaces=no
LockPersonality=yes
SystemCallArchitectures=native

# Logging
StandardOutput=append:/var/log/ffmpeg-worker/worker.log
StandardError=append:/var/log/ffmpeg-worker/worker.log
SyslogIdentifier=ffmpeg-worker

[Install]
WantedBy=multi-user.target
EOF
```

### 2. Enable Logrotate

```bash
sudo tee /etc/logrotate.d/ffmpeg-worker << 'EOF'
/var/log/ffmpeg-worker/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 root root
    sharedscripts
    postrotate
        systemctl reload ffmpeg-worker.service > /dev/null 2>&1 || true
    endscript
}
EOF
```

---

## Starting the Service

### 1. Reload Systemd

```bash
sudo systemctl daemon-reload
```

### 2. Enable Service (Auto-start on Boot)

```bash
sudo systemctl enable ffmpeg-worker.service
```

### 3. Start Service

```bash
sudo systemctl start ffmpeg-worker.service
```

### 4. Check Status

```bash
# Service status
sudo systemctl status ffmpeg-worker.service

# View logs
sudo journalctl -u ffmpeg-worker.service -f

# Check if registered with master
curl -k https://MASTER_IP:8080/nodes
```

---

## Verification and Testing

### 1. Verify Cgroup Setup

```bash
# Check if worker created cgroups
ls /sys/fs/cgroup/ | grep ffmpeg-job

# Monitor resource usage during job
sudo systemd-cgtop
```

### 2. Submit Test Job

```bash
# Simple test job
curl -k -X POST https://MASTER_IP:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "720p30-h264",
    "parameters": {
      "duration": 60,
      "bitrate": "2M"
    },
    "resource_limits": {
      "max_cpu_percent": 200,
      "max_memory_mb": 1024,
      "timeout_sec": 300
    }
  }'
```

### 3. Monitor Job Execution

```bash
# Watch worker logs
sudo tail -f /var/log/ffmpeg-worker/worker.log

# Check system resources
htop

# Monitor cgroup limits
sudo systemd-cgtop
```

### 4. Verify Resource Limits

Look for these log entries:
```
>>> RESOURCE CHECK PHASE <<<
Disk space: XX% used (XXXX MB available)
Resource limits: CPU=200%, Memory=1024MB, Disk=5000MB, Timeout=300s

>>> TRANSCODING EXECUTION PHASE <<<
Process started with PID: XXXXX
Set process priority: nice=10 (lower than normal)
Detected cgroup v2 (root: /sys/fs/cgroup)
Set CPU limit: 200% (quota=200000, period=100000)
Set memory limit: 1024 MB
✓ Process added to cgroup: /sys/fs/cgroup/ffmpeg-job-XXXXX
```

---

## Security Hardening

### 1. Firewall Configuration

```bash
# Only allow outbound HTTPS to master
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow out to MASTER_IP port 8080 proto tcp

# Allow metrics if needed (restrict to monitoring subnet)
sudo ufw allow from MONITORING_SUBNET to any port 9091 proto tcp

sudo ufw enable
```

### 2. SELinux Configuration (CentOS/RHEL)

```bash
# Check SELinux status
sestatus

# Create custom policy if needed
sudo setsebool -P httpd_can_network_connect 1
```

### 3. AppArmor Profile (Ubuntu/Debian)

```bash
# Optional: Create AppArmor profile for additional security
# This is advanced - only needed for highly sensitive environments
```

### 4. Audit Logging

```bash
# Enable auditd
sudo apt-get install -y auditd

# Monitor worker process
sudo auditctl -w /usr/local/bin/ffmpeg-worker -p x -k ffmpeg-worker
sudo auditctl -w /var/lib/ffmpeg-worker -p rwxa -k ffmpeg-worker-data
```

---

## Troubleshooting

### Common Issues

**1. Cgroup Permission Denied**

```bash
# Check cgroup mount
mount | grep cgroup2

# Verify running as root
ps aux | grep ffmpeg-worker

# Check SELinux
sudo ausearch -m avc -ts recent | grep ffmpeg-worker
```

**2. Jobs Failing Immediately**

```bash
# Check worker logs
sudo journalctl -u ffmpeg-worker.service -n 100

# Verify FFmpeg works
ffmpeg -version
ffmpeg -encoders | grep h264

# Test cgroup creation manually
sudo mkdir /sys/fs/cgroup/test-job
echo 100000 | sudo tee /sys/fs/cgroup/test-job/cpu.max
sudo rmdir /sys/fs/cgroup/test-job
```

**3. High Memory Usage**

```bash
# Check job memory limits
sudo cat /sys/fs/cgroup/ffmpeg-job-*/memory.max

# Monitor actual usage
sudo cat /sys/fs/cgroup/ffmpeg-job-*/memory.current

# Adjust job limits in config or per-job
```

**4. Disk Space Issues**

```bash
# Check disk usage
df -h /tmp

# Find large temp files
sudo du -sh /tmp/* | sort -h

# Clean up old files
sudo find /tmp -name "input_*" -mtime +1 -delete
sudo find /tmp -name "job_*_output.*" -mtime +1 -delete
```

### Debug Mode

Enable verbose logging:
```bash
# Edit service file
sudo systemctl edit ffmpeg-worker.service

# Add override
[Service]
Environment="LOG_LEVEL=debug"

# Reload and restart
sudo systemctl daemon-reload
sudo systemctl restart ffmpeg-worker.service
```

### Resource Limit Testing

Test limits work:
```bash
# Submit job with strict limits
curl -k -X POST https://MASTER_IP:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "720p30-h264",
    "parameters": {"duration": 120, "bitrate": "2M"},
    "resource_limits": {
      "max_cpu_percent": 100,
      "max_memory_mb": 512,
      "timeout_sec": 60
    }
  }'

# Monitor with htop (should not exceed 1 core)
htop

# Check cgroup stats
sudo cat /sys/fs/cgroup/ffmpeg-job-*/cpu.stat
sudo cat /sys/fs/cgroup/ffmpeg-job-*/memory.current
```

---

## Performance Tuning

### 1. Optimize Concurrent Jobs

```bash
# Calculate optimal concurrency
# Rule of thumb: 0.75 * CPU_CORES for CPU-bound work
NUM_CORES=$(nproc)
OPTIMAL_JOBS=$((NUM_CORES * 3 / 4))

echo "Recommended max-concurrent-jobs: $OPTIMAL_JOBS"
```

### 2. Tune Kernel Parameters

```bash
sudo tee -a /etc/sysctl.conf << EOF
# TCP tuning for high throughput
net.core.rmem_max=134217728
net.core.wmem_max=134217728
net.ipv4.tcp_rmem=4096 87380 67108864
net.ipv4.tcp_wmem=4096 65536 67108864

# Increase connection backlog
net.core.netdev_max_backlog=5000
net.ipv4.tcp_max_syn_backlog=8192

# Enable BBR congestion control
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF

sudo sysctl -p
```

### 3. Optimize Disk I/O

```bash
# Use faster /tmp if available (ramdisk or SSD)
# Add to /etc/fstab:
tmpfs /tmp tmpfs defaults,size=20G 0 0

# Or mount existing SSD partition
sudo mkdir /mnt/fast-tmp
sudo mount /dev/nvme0n1p1 /mnt/fast-tmp

# Update worker to use fast storage
# Modify ExecStart to add: --work-dir /mnt/fast-tmp
```

### 4. CPU Governor (Performance Mode)

```bash
# Set CPU to performance mode
sudo apt-get install -y cpufrequtils
sudo cpufreq-set -g performance

# Make permanent
echo 'GOVERNOR="performance"' | sudo tee /etc/default/cpufrequtils
sudo systemctl restart cpufrequtils
```

---

## Monitoring and Maintenance

### Health Checks

```bash
# Check service health
sudo systemctl is-active ffmpeg-worker.service

# Check master connectivity
curl -k https://MASTER_IP:8080/nodes | jq '.[] | select(.name=="hostname")'

# Monitor metrics
curl localhost:9091/metrics | grep jobs_
```

### Regular Maintenance

**Daily:**
- Check disk space: `df -h /tmp`
- Review error logs: `sudo journalctl -u ffmpeg-worker.service -p err`

**Weekly:**
- Clean old temp files: `sudo find /tmp -mtime +7 -delete`
- Review resource usage: `sudo systemd-cgtop`
- Check for updates: `git pull && make build`

**Monthly:**
- Rotate logs: `sudo journalctl --vacuum-time=30d`
- Review security updates: `sudo apt-get update && sudo apt-get upgrade`
- Performance review: analyze job completion times

---

## Additional Resources

- **[Resource Limits Documentation](../docs/RESOURCE_LIMITS.md)** - Complete API reference
- **[Production Features Guide](../shared/docs/PRODUCTION_FEATURES.md)** - TLS, authentication, retry
- **[Troubleshooting Guide](../shared/docs/troubleshooting.md)** - Common issues and solutions
- **[Master Deployment Guide](README.md)** - Deploy and configure master node

---

**Version**: 1.0  
**Last Updated**: 2026-01-05  
**Status**: Production Ready
