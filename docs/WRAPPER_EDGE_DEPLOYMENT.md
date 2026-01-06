# Edge Deployment Guide

## Overview

This guide explains how to deploy the FFmpeg-RTMP workload wrapper on **production edge nodes** that are **already receiving signals/streams from clients**.

**Critical principle:** The wrapper integrates **seamlessly** without disrupting existing workloads.

---

## Prerequisites

### System Requirements

- **Linux** with kernel 4.15+ (for cgroups v2 support)
- **systemd** 243+ (for cgroup delegation)
- **Go 1.24+** (for building from source, or use prebuilt binary)
- **FFmpeg** or **GStreamer** (for transcoding workloads)

### Privilege Requirements

The wrapper can run in three modes:

| Mode | Requirements | Capabilities |
|------|--------------|--------------|
| **Unprivileged** | Normal user | ✅ Process observation<br>✅ Nice priority<br>❌ No cgroups |
| **Delegated** | systemd delegation | ✅ Process observation<br>✅ Cgroups (user slice)<br>✅ Nice priority |
| **Privileged** | root or CAP_SYS_ADMIN | ✅ Full cgroup support<br>✅ Negative nice<br>✅ OOM score adjustment |

**Recommended:** Use **delegated mode** for production.

---

## Installation

### Option 1: Install from Binary

```bash
# Download binary (replace with actual release URL)
wget https://github.com/psantana5/ffmpeg-rtmp/releases/latest/download/ffrtmp-linux-amd64
chmod +x ffrtmp-linux-amd64
sudo mv ffrtmp-linux-amd64 /usr/local/bin/ffrtmp

# Verify installation
ffrtmp --version
```

### Option 2: Build from Source

```bash
# Clone repository
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Build wrapper binary
make build-cli
# or: go build -o bin/ffrtmp ./cmd/ffrtmp

# Install binary
sudo cp bin/ffrtmp /usr/local/bin/
```

---

## Configuration

### Enable Cgroup Delegation (Recommended)

This allows unprivileged users to manage cgroups:

```bash
# Create systemd override directory
sudo mkdir -p /etc/systemd/system/user@.service.d/

# Create delegation config
sudo tee /etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
Delegate=yes
EOF

# Reload systemd
sudo systemctl daemon-reload
```

**Verify:**
```bash
# Check if user has cgroup delegation
systemctl --user show-environment | grep DBUS
```

---

## Deployment Scenarios

### Scenario 1: New Edge Node (Fresh Deployment)

Deploy the wrapper on a new edge node:

```bash
# 1. Install wrapper
sudo cp bin/ffrtmp /usr/local/bin/

# 2. Enable cgroup delegation
sudo tee /etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
Delegate=yes
EOF
sudo systemctl daemon-reload

# 3. Test wrapper
ffrtmp run --job-id test-001 -- echo "Wrapper works!"

# 4. Deploy worker agent with wrapper enabled
./bin/agent \
  --master https://master:8080 \
  --register \
  --use-wrapper
```

---

### Scenario 2: Existing Edge Node (Zero Downtime)

**CRITICAL:** Edge nodes are already receiving streams. Use **attach mode** for zero-downtime adoption.

#### Step 1: Identify Existing Workloads

```bash
# Find running FFmpeg processes
ps aux | grep ffmpeg
# Example output:
# user  5678  ... ffmpeg -i rtmp://source/stream -c:v h264_nvenc output.mp4
```

#### Step 2: Attach Wrapper (No Restart!)

```bash
# Attach to existing process
ffrtmp attach \
  --pid 5678 \
  --job-id existing-stream-001 \
  --cpu-weight 150 \
  --memory-max $((4*1024*1024*1024))
```

**Result:**
- ✅ Stream continues uninterrupted
- ✅ No dropped frames
- ✅ Governance applied retroactively
- ✅ Wrapper can be stopped/restarted without affecting stream

#### Step 3: Verify Stream Health

```bash
# Check if process still running
ps aux | grep 5678

# Check if frames are being processed
tail -f /var/log/ffmpeg.log
```

#### Step 4: Gradual Migration

```bash
# For new workloads, use run mode
ffrtmp run --job-id new-stream-002 -- ffmpeg -i input.mp4 output.mp4

# Existing workloads continue with attach mode
```

---

## Systemd Service

### Worker Agent with Wrapper

Create `/etc/systemd/system/ffrtmp-worker.service`:

```ini
[Unit]
Description=FFmpeg-RTMP Worker Agent with Wrapper
After=network.target

[Service]
Type=simple
User=ffrtmp
Group=ffrtmp
Delegate=yes

# Environment
Environment="MASTER_URL=https://master:8080"
Environment="MASTER_API_KEY=your-api-key-here"

# Worker agent with wrapper enabled
ExecStart=/usr/local/bin/agent \
  --master ${MASTER_URL} \
  --api-key ${MASTER_API_KEY} \
  --register \
  --use-wrapper \
  --max-concurrent-jobs 4

# Restart policy
Restart=always
RestartSec=10

# Resource limits (for the agent itself, not workloads)
MemoryLimit=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

**Enable and start:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable ffrtmp-worker
sudo systemctl start ffrtmp-worker
sudo systemctl status ffrtmp-worker
```

---

### Standalone Wrapper Service

For edge nodes with custom workload management:

```ini
[Unit]
Description=FFmpeg-RTMP Workload Wrapper
After=network.target

[Service]
Type=oneshot
RemainAfterExit=yes
User=ffrtmp
Delegate=yes

# This service just enables cgroup management
# Actual workloads are managed separately

[Install]
WantedBy=multi-user.target
```

---

## Monitoring

### Check Wrapper Status

```bash
# Check running wrapper processes
ps aux | grep ffrtmp

# Check cgroups created by wrapper
ls -la /sys/fs/cgroup/ffrtmp/

# Check specific job cgroup
cat /sys/fs/cgroup/ffrtmp/job-123/cpu.max
cat /sys/fs/cgroup/ffrtmp/job-123/memory.max
```

### Verify Process Isolation

```bash
# Find workload PID
ps aux | grep ffmpeg | grep -v grep

# Check process cgroup
cat /proc/<PID>/cgroup

# Should show: 0::/ffrtmp/job-123
```

### Test Crash Safety

```bash
# Start wrapper with long-running job
ffrtmp run --job-id test-crash -- sleep 60 &
WRAPPER_PID=$!

# Find workload PID
WORKLOAD_PID=$(pgrep -P $WRAPPER_PID sleep)

# Kill wrapper
kill -9 $WRAPPER_PID

# Verify workload still running
ps aux | grep $WORKLOAD_PID
# Should still be running ✓
```

---

## Troubleshooting

### Issue: "Cannot create cgroup (permission denied)"

**Symptom:**
```
[wrapper] WARNING: Cannot create cgroup (permission denied)
```

**Solutions:**

1. **Enable cgroup delegation:**
   ```bash
   sudo tee /etc/systemd/system/user@.service.d/delegate.conf <<EOF
   [Service]
   Delegate=yes
   EOF
   sudo systemctl daemon-reload
   ```

2. **Run as root (not recommended for production):**
   ```bash
   sudo ffrtmp run -- my_command
   ```

3. **Use nice fallback (automatic):**
   The wrapper automatically falls back to nice priority if cgroups aren't available.

---

### Issue: "Process does not exist"

**Symptom:**
```
Error: process 12345 does not exist
```

**Solution:**
```bash
# Verify PID exists
ps aux | grep 12345

# OR kill -0 to check
kill -0 12345 && echo "exists" || echo "not found"
```

---

### Issue: Workload Dies When Wrapper Detaches

**This should NOT happen!** If it does:

1. **Verify process group isolation:**
   ```bash
   # Check if setpgid is working
   ps -o pid,ppid,pgid,command
   ```

2. **Check systemd KillMode:**
   ```ini
   # In service file, ensure:
   KillMode=process  # NOT control-group
   ```

3. **Report bug:** This violates the core design principle.

---

### Issue: High Overhead

**Check wrapper overhead:**
```bash
# Wrapper should use < 0.1% CPU
top -p $(pgrep ffrtmp)

# Wrapper should use < 10 MB memory
ps aux | grep ffrtmp
```

If overhead is high, this is a bug.

---

## Security

### Principle of Least Privilege

```bash
# Create dedicated user
sudo useradd -r -s /bin/false ffrtmp

# Give only necessary capabilities
sudo setcap cap_sys_admin+ep /usr/local/bin/ffrtmp
```

### Cgroup Isolation

The wrapper ensures:
- ✅ Processes cannot escape their cgroup
- ✅ Resource limits are enforced by kernel
- ✅ No privilege escalation possible

### Audit Logging

```bash
# Enable audit logging for wrapper
sudo auditctl -w /usr/local/bin/ffrtmp -p x -k ffrtmp_exec
```

---

## Performance Tuning

### CPU Constraints

```bash
# 1 core (100%)
ffrtmp run --cpu-max "100000 100000" -- my_app

# 2 cores (200%)
ffrtmp run --cpu-max "200000 100000" -- my_app

# Proportional share (2x normal)
ffrtmp run --cpu-weight 200 -- my_app
```

### Memory Constraints

```bash
# 4GB limit
ffrtmp run --memory-max $((4*1024*1024*1024)) -- my_app

# 8GB limit
ffrtmp run --memory-max $((8*1024*1024*1024)) -- my_app
```

### IO Constraints (cgroup v2 only)

```bash
# Limit IO to 100 MB/s
ffrtmp run --io-max "8:0 rbps=104857600 wbps=104857600" -- my_app
```

---

## Production Checklist

Before deploying to production:

- [ ] Binary installed: `/usr/local/bin/ffrtmp`
- [ ] Cgroup delegation enabled
- [ ] Systemd service created and tested
- [ ] Crash safety verified (kill -9 wrapper → workload continues)
- [ ] Attach mode tested on existing workload
- [ ] Monitoring configured
- [ ] Alert rules defined
- [ ] Backup plan documented
- [ ] Rollback procedure tested
- [ ] Team trained on attach mode

---

## Rollback Procedure

If issues occur:

```bash
# 1. Stop wrapper
sudo systemctl stop ffrtmp-worker

# 2. Workloads continue running (by design)

# 3. Verify workloads healthy
ps aux | grep ffmpeg

# 4. Remove wrapper binary (optional)
sudo rm /usr/local/bin/ffrtmp

# 5. Worker agent falls back to direct execution
```

**Critical:** Workloads are never owned by the wrapper, so stopping it is safe.

---

## Support

### Get Help

- **Architecture:** `docs/WRAPPER_MINIMALIST_ARCHITECTURE.md`
- **Examples:** `docs/WRAPPER_EXAMPLES.md`
- **Operator Guide:** `docs/WRAPPER_OPERATOR_GUIDE.md`
- **Test Suite:** `scripts/test_wrapper_stability.sh`

### Report Issues

Include:
- OS version: `uname -a`
- Cgroup version: `ls -d /sys/fs/cgroup/cgroup.controllers && echo "v2" || echo "v1"`
- Wrapper version: `ffrtmp --version`
- Error logs
- Steps to reproduce

---

## Summary

The edge workload wrapper is designed for **seamless integration** with existing production workloads.

**Key principles:**
- ✅ **Non-owning:** Workloads survive wrapper crashes
- ✅ **Attach mode:** Zero-downtime adoption
- ✅ **Graceful degradation:** Works without cgroups
- ✅ **Minimal overhead:** < 0.1% CPU, < 10 MB RAM
- ✅ **Production-ready:** Comprehensive testing, stable

**For existing edge nodes:** Use **attach mode** to avoid disrupting live streams.

**For new deployments:** Use **run mode** from the start.

**Remember:** If you're not sure, do less. The wrapper is boring on purpose. 😄
