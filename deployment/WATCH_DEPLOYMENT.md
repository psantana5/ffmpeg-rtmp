# Watch Daemon Production Deployment Guide

This guide covers deploying the auto-discovery watch daemon in production environments.

## Overview

The watch daemon automatically discovers and governs FFmpeg/transcoding processes that start outside the wrapper's control. This is **critical for production edge nodes** where:

- Clients initiate streams directly
- External triggers spawn FFmpeg processes
- Legacy systems bypass the job queue
- Processes need governance without code changes

## Architecture

```
┌─────────────────────┐
│   Watch Daemon      │
│  (systemd service)  │
└──────────┬──────────┘
           │
           ├─ Scans /proc every 10s
           ├─ Discovers FFmpeg processes
           ├─ Applies resource limits (cgroups)
           ├─ Tracks health & retries
           └─ Persists state to disk
```

## Installation

### Prerequisites

- Linux with cgroup v2 support (kernel 4.15+)
- Go 1.24+ (for building)
- Root access for installation
- `ffrtmp` user and group

### Quick Install

```bash
# Clone repository
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Build binaries
make build-cli

# Run installation script (as root)
sudo ./deployment/install-edge.sh
```

This installs:
- ✅ `ffrtmp` wrapper binary → `/usr/local/bin/ffrtmp`
- ✅ Systemd service → `/etc/systemd/system/ffrtmp-watch.service`
- ✅ Configuration files → `/etc/ffrtmp/`
- ✅ State directory → `/var/lib/ffrtmp/`
- ✅ Cgroup delegation enabled

## Configuration

### 1. Main Configuration File

Edit `/etc/ffrtmp/watch-config.yaml`:

```yaml
scan_interval: "10s"

target_commands:
  - ffmpeg
  - gst-launch-1.0

default_limits:
  cpu_quota: 200       # 2 cores
  cpu_weight: 100
  memory_limit: 4096   # 4GB

filters:
  min_runtime: "5s"    # Ignore short test commands
  blocked_dirs:
    - /tmp

commands:
  ffmpeg:
    limits:
      cpu_quota: 300   # FFmpeg gets 3 cores
      memory_limit: 8192
    filters:
      min_runtime: "10s"
```

**Key Settings**:
- `scan_interval`: How often to check for new processes (default: 10s)
- `target_commands`: Process names to discover (e.g., `ffmpeg`)
- `default_limits`: Resource limits for discovered processes
- `filters`: Criteria for selective discovery
- `commands`: Per-command overrides

### 2. Environment Variables

Edit `/etc/ffrtmp/watch.env`:

```bash
# Scan interval
SCAN_INTERVAL=10s

# Target commands (comma-separated)
TARGET_COMMANDS=ffmpeg

# State persistence
STATE_FLUSH_INTERVAL=30s

# Retry configuration
MAX_RETRY_ATTEMPTS=5
```

### 3. Recommended Production Settings

**Light workload** (dev/test):
```yaml
scan_interval: "30s"
default_limits:
  cpu_quota: 100
  memory_limit: 2048
```

**Medium workload** (production):
```yaml
scan_interval: "10s"
default_limits:
  cpu_quota: 200
  memory_limit: 4096
```

**Heavy workload** (high-throughput):
```yaml
scan_interval: "5s"
default_limits:
  cpu_quota: 400
  memory_limit: 8192
```

## Service Management

### Start Watch Daemon

```bash
# Enable automatic start on boot
sudo systemctl enable ffrtmp-watch

# Start service
sudo systemctl start ffrtmp-watch

# Check status
sudo systemctl status ffrtmp-watch

# View logs
sudo journalctl -u ffrtmp-watch -f
```

### Stop Watch Daemon

```bash
# Stop service (graceful)
sudo systemctl stop ffrtmp-watch

# Processes remain running (non-owning governance)
# Only monitoring stops
```

### Restart Watch Daemon

```bash
# Restart service
sudo systemctl restart ffrtmp-watch

# State is preserved (loads from /var/lib/ffrtmp/watch-state.json)
# Processes remain governed
```

## Monitoring

### Check Daemon Status

```bash
# Service status
systemctl status ffrtmp-watch

# Recent logs
journalctl -u ffrtmp-watch -n 50

# Follow logs
journalctl -u ffrtmp-watch -f
```

### Key Log Messages

**Successful startup**:
```
[watch] Auto-attach service started
[watch] Retry worker started
[watch] State loaded from /var/lib/ffrtmp/watch-state.json
```

**Process discovery**:
```
[watch] Discovered 3 new process(es)
[watch] Scan complete: new=3 tracked=5 duration=15.2ms
[watch] Attaching to PID 12345 (ffmpeg) as job auto-ffmpeg-12345
```

**Health status**:
```
[watch] Health status: healthy
[watch] Health status: degraded (scan failures: 3, attach failures: 0)
```

### Check State File

```bash
# View current state
sudo cat /var/lib/ffrtmp/watch-state.json | jq

# Example output:
{
  "processes": {
    "12345": {
      "pid": 12345,
      "job_id": "auto-ffmpeg-12345",
      "command": "ffmpeg",
      "discovered_at": "2026-01-07T10:30:00Z",
      "attached_at": "2026-01-07T10:30:01Z"
    }
  },
  "statistics": {
    "total_scans": 1440,
    "total_discovered": 45,
    "total_attachments": 43
  }
}
```

## Validation

### Test Discovery

```bash
# Start a test FFmpeg process (in another terminal)
ffmpeg -f lavfi -i testsrc -f null - &

# Watch daemon should discover it within scan_interval
sudo journalctl -u ffrtmp-watch -f

# Expected output:
# [watch] Discovered 1 new process(es)
# [watch] Attaching to PID <pid> (ffmpeg) as job auto-ffmpeg-<pid>
```

### Test State Persistence

```bash
# Start FFmpeg process
ffmpeg -f lavfi -i testsrc -f null - &
sleep 15  # Wait for discovery

# Check state file
sudo cat /var/lib/ffrtmp/watch-state.json

# Restart daemon
sudo systemctl restart ffrtmp-watch

# Verify process still tracked
sudo journalctl -u ffrtmp-watch | grep "State loaded"
```

### Test Health Monitoring

```bash
# Watch logs for health status
sudo journalctl -u ffrtmp-watch -f | grep "Health"

# Healthy output:
# (no health messages = healthy)

# Degraded output:
# [watch] Health status: degraded (scan failures: 3)
```

## Troubleshooting

### Issue: Daemon Won't Start

```bash
# Check for errors
sudo journalctl -u ffrtmp-watch -n 50

# Common causes:
# 1. Binary not found → make build-cli
# 2. Permission denied → check /usr/local/bin/ffrtmp ownership
# 3. Config syntax error → validate YAML
```

### Issue: Processes Not Discovered

```bash
# Check scan logs
sudo journalctl -u ffrtmp-watch | grep "Scan complete"

# Verify target commands match
ps aux | grep ffmpeg

# Check filters aren't blocking
# Edit /etc/ffrtmp/watch-config.yaml
# Comment out all filters temporarily
```

### Issue: Permission Denied on Cgroups

```bash
# Verify cgroup delegation
systemctl show -p Delegate ffrtmp-watch
# Should show: Delegate=yes

# Check cgroup v2 mounted
mount | grep cgroup2
# Should show: cgroup2 on /sys/fs/cgroup

# Reinstall delegate config
sudo cp deployment/systemd/user@.service.d-delegate.conf \
    /etc/systemd/system/user@.service.d/delegate.conf
sudo systemctl daemon-reload
```

### Issue: State File Corruption

```bash
# Backup and reset state
sudo mv /var/lib/ffrtmp/watch-state.json \
    /var/lib/ffrtmp/watch-state.json.backup

# Restart daemon (creates fresh state)
sudo systemctl restart ffrtmp-watch
```

## Best Practices

### 1. Resource Limits

- **Start conservative**: Begin with lower limits, increase based on monitoring
- **Per-command tuning**: Different tools need different resources
- **Leave headroom**: Don't allocate 100% of system resources

### 2. Filtering

- **Use `min_runtime`**: Prevents attaching to test commands
- **Block temp directories**: Avoids noise from `/tmp` scripts
- **User restrictions**: Multi-tenant environments should use `allowed_users`

### 3. State Persistence

- **Regular backups**: Include `/var/lib/ffrtmp/` in backup strategy
- **Monitor file size**: Should be <10KB for normal operation
- **Flush interval**: 30s default is good balance

### 4. Health Monitoring

- **Watch for degraded status**: Indicates transient issues
- **Alert on unhealthy**: Requires investigation
- **Retry attempts**: 5 is recommended for production

## Security Considerations

### Cgroup Isolation

- Watch daemon runs as `ffrtmp` user (non-root)
- Cgroup delegation allows sub-cgroup creation
- Processes are isolated from each other

### File Permissions

```bash
# State file should be owned by ffrtmp user
ls -la /var/lib/ffrtmp/
# Expected: drwxr-xr-x ffrtmp ffrtmp

# Config files readable by ffrtmp
ls -la /etc/ffrtmp/
# Expected: -rw-r--r-- root root
```

### Prevent Discovery of Sensitive Processes

```yaml
filters:
  blocked_uids:
    - 0  # Never discover root processes
  blocked_dirs:
    - /root
    - /etc
    - /usr/sbin
```

## Performance Tuning

### Scan Interval

- **5s**: High-throughput (1000+ processes/day)
- **10s**: Standard production (100-1000 processes/day)
- **30s**: Low-priority (< 100 processes/day)

### State Flush Interval

- **10s**: Frequent restarts expected
- **30s**: Standard (default)
- **60s**: Minimal overhead, infrequent restarts

### Memory Usage

- **Base**: ~10MB (daemon overhead)
- **Per tracked process**: ~2KB
- **State file**: ~500 bytes per process

### CPU Usage

- **Scanning**: <1% CPU (typical)
- **Per process**: <0.1% CPU overhead

## Integration with Worker Agent

The watch daemon complements the worker agent:

```
Worker Agent          Watch Daemon
     │                     │
     ├─ Polls master       ├─ Scans /proc
     ├─ Fetches jobs       ├─ Discovers processes
     ├─ Spawns FFmpeg ────▶ ├─ Applies limits
     └─ Reports status     └─ Tracks health
```

**Together they provide**:
- Orchestrated jobs (agent)
- Ad-hoc discovery (watch)
- Complete governance coverage

## Upgrade Path

### Upgrading from Previous Versions

```bash
# Stop services
sudo systemctl stop ffrtmp-watch

# Backup state and config
sudo cp -r /etc/ffrtmp /etc/ffrtmp.backup
sudo cp -r /var/lib/ffrtmp /var/lib/ffrtmp.backup

# Update binaries
cd ffmpeg-rtmp
git pull
make build-cli
sudo cp bin/ffrtmp /usr/local/bin/

# Update systemd files
sudo cp deployment/systemd/ffrtmp-watch.service /etc/systemd/system/
sudo systemctl daemon-reload

# Start service
sudo systemctl start ffrtmp-watch

# Verify
sudo systemctl status ffrtmp-watch
```

## Next Steps

Once watch daemon is deployed:

1. **Monitor for 24 hours**: Validate discovery patterns
2. **Tune resource limits**: Adjust based on actual workload
3. **Add Prometheus metrics**: See [Prometheus integration guide]
4. **Scale to cluster**: Deploy on all worker nodes

## Support

- Documentation: `docs/AUTO_ATTACH.md`
- Issues: GitHub Issues
- Examples: `examples/watch-config.yaml`
