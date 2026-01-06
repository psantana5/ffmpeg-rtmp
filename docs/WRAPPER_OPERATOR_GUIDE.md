# Edge Workload Wrapper - Operator Guide

## Quick Reference Card

### Installation

```bash
# Build wrapper
cd ffmpeg-rtmp
make build-cli

# Verify installation
./bin/ffrtmp run --help
./bin/ffrtmp attach --help
```

### Basic Usage

```bash
# Run mode - spawn new process
ffrtmp run [flags] -- <command> [args...]

# Attach mode - govern existing process
ffrtmp attach --pid <PID> [flags]
```

## Common Operations

### 1. Run FFmpeg with Constraints

```bash
ffrtmp run \
  --job-id job-001 \
  --sla-eligible \
  --cpu-quota 200 \
  --memory-limit 4096 \
  -- ffmpeg -i input.mp4 output.mp4
```

### 2. Attach to Running Process

```bash
# Find process
ps aux | grep ffmpeg

# Attach wrapper
ffrtmp attach --pid <PID> --job-id job-002
```

### 3. Low Priority Background Job

```bash
ffrtmp run \
  --cpu-weight 50 \
  --nice 10 \
  --intent test \
  -- ./batch_job.sh
```

### 4. High Priority Production Job

```bash
ffrtmp run \
  --cpu-weight 200 \
  --nice -5 \
  --memory-limit 8192 \
  --sla-eligible \
  -- ./critical_app
```

## Flag Reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--job-id` | string | "unknown" | Job identifier |
| `--sla-eligible` | bool | false | Mark as SLA-worthy |
| `--intent` | string | "production" | production\|test\|experiment\|soak |
| `--cpu-quota` | int | 0 | CPU % (100=1 core, 0=unlimited) |
| `--cpu-weight` | int | 100 | CPU weight (1-10000) |
| `--nice` | int | 0 | Priority (-20 to 19) |
| `--memory-limit` | int64 | 0 | Memory limit MB (0=unlimited) |
| `--io-weight` | int | 0 | IO weight % (0-100) |
| `--oom-score` | int | 0 | OOM score (-1000 to 1000) |
| `--json` | bool | false | JSON output |
| `--pid` | int | - | PID to attach (attach mode only) |

## Constraint Presets

### Default (Unconstrained)
```bash
ffrtmp run -- my_command
# cpu-quota: 0 (unlimited)
# cpu-weight: 100
# nice: 0
# memory-limit: 0 (unlimited)
```

### Low Priority
```bash
ffrtmp run --cpu-weight 50 --nice 10 -- my_command
```

### High Priority
```bash
ffrtmp run --cpu-weight 200 --nice -5 -- my_command
```

### Memory Constrained
```bash
ffrtmp run --memory-limit 2048 --oom-score 500 -- my_command
```

### Balanced Multi-Core
```bash
ffrtmp run --cpu-quota 400 --memory-limit 4096 -- my_command
```

## Privilege Requirements

### Running as Normal User

✅ **Works:**
- Run mode (spawn processes)
- Attach mode (passive observation)
- Nice priority (positive values 0-19)
- JSON output

⚠️ **Limited:**
- Cgroups may not be available
- Falls back to nice priority

❌ **Requires Root:**
- Full cgroup support
- Negative nice values (-20 to -1)
- Negative OOM scores

### Running with Sudo

```bash
# Full cgroup support
sudo ffrtmp run --cpu-quota 200 --memory-limit 4096 -- my_command

# High priority
sudo ffrtmp run --nice -10 -- my_command
```

### Cgroup Delegation (Recommended)

Enable systemd cgroup delegation for unprivileged users:

```ini
# /etc/systemd/system/user@.service.d/delegate.conf
[Service]
Delegate=yes
```

Then reload:
```bash
sudo systemctl daemon-reload
```

## Troubleshooting

### Issue: "Cannot create cgroup (permission denied)"

**Symptom:**
```
[wrapper] WARNING: Cannot create cgroup (permission denied)
```

**Solutions:**
1. Run with sudo: `sudo ffrtmp run ...`
2. Enable cgroup delegation (see above)
3. Use nice fallback (automatic)

---

### Issue: "Cannot set negative nice (requires root)"

**Symptom:**
```
[wrapper] WARNING: Cannot set negative nice (requires root), using 0
```

**Solution:**
Use positive nice values or run with sudo:
```bash
# Positive nice (works without root)
ffrtmp run --nice 10 -- my_command

# Negative nice (requires root)
sudo ffrtmp run --nice -10 -- my_command
```

---

### Issue: Wrapper exits but workload continues

**This is EXPECTED behavior!**

The wrapper is non-owning. The workload runs in its own process group and survives wrapper crashes.

**Verify:**
```bash
# Start workload
ffrtmp run -- sleep 1000 &
WRAPPER_PID=$!

# Kill wrapper
kill $WRAPPER_PID

# Workload still running ✓
ps aux | grep sleep
```

---

### Issue: "process does not exist" (attach mode)

**Symptom:**
```
Error: process 12345 does not exist
```

**Solution:**
Verify process exists first:
```bash
# Check if process exists
kill -0 12345 && echo "alive" || echo "dead"

# Find correct PID
ps aux | grep my_app
```

---

## Monitoring & Observability

### Check Process Cgroup

```bash
# After wrapping process with PID 12345
cat /proc/12345/cgroup
```

### View Applied Constraints

```bash
# Find wrapper cgroup
ls /sys/fs/cgroup/ffrtmp-wrapper-*

# View CPU constraint
cat /sys/fs/cgroup/ffrtmp-wrapper-job123/cpu.max

# View memory constraint
cat /sys/fs/cgroup/ffrtmp-wrapper-job123/memory.max
```

### JSON Output for Integration

```bash
# Run with JSON output
ffrtmp run --json --job-id job-123 -- sleep 5 > report.json

# Parse with jq
cat report.json | jq '.exit_code'
cat report.json | jq '.exit_reason'
cat report.json | jq '.duration_sec'
```

## Integration Patterns

### Pattern 1: Distributed Worker Integration

```bash
# Worker agent spawns wrapper instead of raw process
ffrtmp run \
  --job-id $JOB_ID \
  --sla-eligible \
  --cpu-quota $CPU_QUOTA \
  --memory-limit $MEMORY_LIMIT \
  --json \
  -- ffmpeg -i input.mp4 output.mp4
```

### Pattern 2: Legacy System Migration

```bash
# Phase 1: Attach to existing processes (no restart)
for pid in $(pgrep ffmpeg); do
  ffrtmp attach --pid $pid --job-id legacy-$pid
done

# Phase 2: New processes use run mode
ffrtmp run -- ffmpeg ...
```

### Pattern 3: Systemd Integration

```ini
# /etc/systemd/system/wrapped-service.service
[Service]
ExecStart=/usr/local/bin/ffrtmp run \
  --job-id %N \
  --sla-eligible \
  --cpu-quota 200 \
  -- /usr/local/bin/my_service

# Wrapper exits on service stop, workload terminates naturally
KillMode=control-group
```

## Performance Tuning

### CPU-Bound Workloads

```bash
# Allow multiple cores
ffrtmp run --cpu-quota 400 -- cpu_intensive_app
```

### Memory-Bound Workloads

```bash
# Set memory limit
ffrtmp run --memory-limit 8192 -- memory_intensive_app
```

### IO-Bound Workloads

```bash
# Prioritize IO (cgroup v2 only)
ffrtmp run --io-weight 100 -- io_intensive_app
```

### Mixed Workloads

```bash
# Balanced constraints
ffrtmp run \
  --cpu-quota 200 \
  --memory-limit 4096 \
  --io-weight 75 \
  -- mixed_workload
```

## Security Considerations

### Minimal Privilege Principle

```bash
# Run as normal user when possible
ffrtmp run -- my_app

# Only use sudo when cgroups needed
sudo ffrtmp run --cpu-quota 200 -- my_app
```

### Cgroup Escape Prevention

The wrapper uses Linux cgroup mechanisms. Processes cannot escape their cgroup without elevated privileges.

### Resource Exhaustion Prevention

```bash
# Always set limits for untrusted workloads
ffrtmp run \
  --cpu-quota 100 \
  --memory-limit 1024 \
  --oom-score 1000 \
  -- untrusted_app
```

## Best Practices

1. **Always set job-id** for tracking
2. **Use sla-eligible for production** workloads
3. **Start with defaults** then tune constraints
4. **Test attach mode** before production use
5. **Monitor cgroup metrics** in production
6. **Enable cgroup delegation** for unprivileged users
7. **Use JSON output** for automation
8. **Set memory limits** for untrusted workloads
9. **Use attach mode** for zero-downtime adoption
10. **Document intent** (production/test/etc)

## Quick Diagnostics

```bash
# Check cgroup version
ls -d /sys/fs/cgroup/cgroup.controllers && echo "v2" || echo "v1"

# Check cgroup availability
ls -d /sys/fs/cgroup && echo "available" || echo "not available"

# Check if process is in wrapper cgroup
cat /proc/<PID>/cgroup | grep ffrtmp-wrapper

# Check wrapper binary
./bin/ffrtmp run -- echo "test"
```

## Support

For detailed documentation:
- [Wrapper Architecture](WRAPPER_ARCHITECTURE.md)
- [Usage Examples](WRAPPER_EXAMPLES.md)
- [Main README](../README.md)

For issues:
- Check troubleshooting section above
- Review logs for `[wrapper]` prefix
- Verify cgroup availability and permissions
- Test with simple commands first (e.g., `sleep`)
