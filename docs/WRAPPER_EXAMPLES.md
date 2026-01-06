# Workload Wrapper Examples

This document provides comprehensive examples for using the FFmpeg-RTMP edge workload wrapper in production scenarios.

## Table of Contents

1. [Basic Usage](#basic-usage)
2. [Production FFmpeg Transcoding](#production-ffmpeg-transcoding)
3. [Attaching to Existing Workloads](#attaching-to-existing-workloads)
4. [Resource Constraint Scenarios](#resource-constraint-scenarios)
5. [Testing and Development](#testing-and-development)
6. [Integration with Distributed System](#integration-with-distributed-system)
7. [Troubleshooting](#troubleshooting)

---

## Basic Usage

### Run a simple command

```bash
ffrtmp run -- echo "Hello from wrapper"
```

Output:
```
[wrapper] RUN mode: /bin/echo [Hello from wrapper]
[wrapper] Job: unknown, Intent: production, SLA: false
[wrapper] Started PID 12345
Hello from wrapper
[wrapper] Process exited: code=0, reason=success, duration=0.0s
```

### Run with metadata

```bash
ffrtmp run \
  --job-id my-job-001 \
  --intent production \
  --sla-eligible \
  -- python train_model.py
```

### Attach to running process

```bash
# Find PID
ps aux | grep my_app

# Attach wrapper
ffrtmp attach --pid 12345 --job-id my-job-002
```

---

## Production FFmpeg Transcoding

### Example 1: H.264 Transcode with NVENC (GPU-accelerated)

```bash
ffrtmp run \
  --job-id transcode-h264-001 \
  --sla-eligible \
  --intent production \
  --cpu-quota 200 \
  --memory-limit 4096 \
  --nice -5 \
  -- ffmpeg \
    -hwaccel cuda \
    -i /streams/input.mp4 \
    -c:v h264_nvenc \
    -preset fast \
    -b:v 5000k \
    -maxrate 6000k \
    -bufsize 12000k \
    -c:a aac \
    -b:a 192k \
    /streams/output.mp4
```

**Explanation:**
- `--cpu-quota 200`: Allow up to 2 CPU cores (200%)
- `--memory-limit 4096`: Limit to 4GB RAM
- `--nice -5`: Higher priority (requires privilege)
- `--sla-eligible`: Track as SLA-worthy workload

### Example 2: H.265 Transcode (CPU-only, constrained)

```bash
ffrtmp run \
  --job-id transcode-h265-002 \
  --sla-eligible \
  --cpu-quota 400 \
  --memory-limit 8192 \
  --io-weight 75 \
  -- ffmpeg \
    -i /streams/4k_input.mp4 \
    -c:v libx265 \
    -preset medium \
    -crf 23 \
    -c:a copy \
    /streams/4k_output.mp4
```

**Explanation:**
- `--cpu-quota 400`: Allow up to 4 CPU cores (H.265 is CPU-intensive)
- `--memory-limit 8192`: 8GB for 4K video buffering
- `--io-weight 75`: Prioritize IO (heavy disk usage)

### Example 3: Live Streaming (RTMP to HLS)

```bash
ffrtmp run \
  --job-id live-stream-hls-003 \
  --sla-eligible \
  --cpu-weight 200 \
  --memory-limit 2048 \
  -- ffmpeg \
    -i rtmp://live-server/stream \
    -c:v copy \
    -c:a copy \
    -f hls \
    -hls_time 2 \
    -hls_list_size 10 \
    -hls_flags delete_segments \
    /var/www/hls/stream.m3u8
```

**Explanation:**
- `--cpu-weight 200`: Prioritize over other workloads (2x weight)
- `--memory-limit 2048`: 2GB sufficient for live streaming
- No CPU quota → Can burst when needed

### Example 4: Batch Processing (Low Priority)

```bash
ffrtmp run \
  --job-id batch-transcode-004 \
  --intent test \
  --cpu-weight 50 \
  --nice 10 \
  --memory-limit 1024 \
  -- ffmpeg \
    -i /archive/old_video.avi \
    -c:v libx264 \
    -preset slow \
    -crf 22 \
    /archive/new_video.mp4
```

**Explanation:**
- `--intent test`: Not production (not SLA-tracked)
- `--cpu-weight 50`: Half the normal CPU priority
- `--nice 10`: Low scheduler priority
- Runs when system is idle

---

## Attaching to Existing Workloads

### Example 5: Attach to Running OBS Studio

```bash
# OBS is already running (started by user)
# PID: 5678

# Apply governance without restart
ffrtmp attach \
  --pid 5678 \
  --job-id obs-stream-005 \
  --sla-eligible \
  --cpu-quota 300 \
  --memory-limit 4096 \
  --nice -10
```

**Critical features:**
- ✅ No restart required
- ✅ Process continues uninterrupted
- ✅ Constraints applied retroactively
- ✅ Wrapper can detach/crash without affecting OBS

### Example 6: Attach to GStreamer Pipeline

```bash
# Find GStreamer process
GSTREAMER_PID=$(pgrep -f "gst-launch-1.0")

# Attach wrapper
ffrtmp attach \
  --pid $GSTREAMER_PID \
  --job-id gstreamer-006 \
  --cpu-weight 150 \
  --io-weight 80
```

### Example 7: Attach to Legacy FFmpeg (Unknown Origin)

```bash
# Unknown FFmpeg process found running
# PID: 9012

# Add governance for observability
ffrtmp attach \
  --pid 9012 \
  --job-id legacy-ffmpeg-007 \
  --intent experiment \
  --cpu-quota 100 \
  --memory-limit 2048 \
  --json > /tmp/legacy-report.json
```

**Use case:**
- Legacy system with unmanaged processes
- Need visibility without restart
- Gradual migration to governed execution

---

## Resource Constraint Scenarios

### Example 8: High-Priority Production Job

```bash
ffrtmp run \
  --job-id critical-008 \
  --sla-eligible \
  --cpu-quota 0 \
  --cpu-weight 500 \
  --nice -10 \
  --oom-score -500 \
  -- /usr/local/bin/critical_app
```

**Explanation:**
- `--cpu-quota 0`: Unlimited CPU (can use all cores)
- `--cpu-weight 500`: 5x normal priority
- `--nice -10`: High scheduler priority (requires root)
- `--oom-score -500`: Avoid OOM killer

### Example 9: Memory-Constrained Job

```bash
ffrtmp run \
  --job-id memory-limited-009 \
  --memory-limit 512 \
  --oom-score 500 \
  -- python data_processing.py
```

**Explanation:**
- `--memory-limit 512`: Hard limit at 512MB
- `--oom-score 500`: OK to kill if OOM occurs
- If process exceeds 512MB → cgroup OOM killer activates

### Example 10: Balanced Multi-Tenant Setup

```bash
# Tenant A: High priority
ffrtmp run --job-id tenant-a-010 --cpu-weight 200 --memory-limit 4096 -- ./tenant_a_app &

# Tenant B: Normal priority
ffrtmp run --job-id tenant-b-011 --cpu-weight 100 --memory-limit 2048 -- ./tenant_b_app &

# Tenant C: Low priority
ffrtmp run --job-id tenant-c-012 --cpu-weight 50 --memory-limit 1024 -- ./tenant_c_app &
```

**Result:**
- Tenant A gets ~50% CPU under contention (200/(200+100+50))
- Tenant B gets ~29% CPU
- Tenant C gets ~14% CPU
- Fair proportional sharing without hard limits

---

## Testing and Development

### Example 11: Quick Test Run

```bash
ffrtmp run \
  --job-id test-001 \
  --intent test \
  -- /bin/sleep 5
```

### Example 12: Benchmark with Monitoring

```bash
ffrtmp run \
  --job-id benchmark-012 \
  --intent experiment \
  --json \
  -- sysbench cpu --threads=4 run \
  > benchmark-report.json
```

### Example 13: Debug Failed Job

```bash
# Run with verbose output
ffrtmp run \
  --job-id debug-013 \
  --intent test \
  -- /bin/bash -x failing_script.sh
```

**Wrapper captures:**
- Exit code (non-zero)
- Exit reason (error vs signal vs timeout)
- Lifecycle events
- Duration

---

## Integration with Distributed System

### Example 14: Worker Agent Integration

```go
// worker/cmd/agent/main.go (pseudocode)
func executeJobWithWrapper(job *models.Job) error {
    // Build wrapper command
    wrapperCmd := []string{
        "ffrtmp", "run",
        "--job-id", job.ID,
        "--sla-eligible",
        "--intent", "production",
        "--cpu-quota", fmt.Sprintf("%d", job.CPUQuota),
        "--memory-limit", fmt.Sprintf("%d", job.MemoryLimitMB),
        "--json",
        "--",
    }
    
    // Append actual workload command
    wrapperCmd = append(wrapperCmd, job.Command...)
    
    // Execute
    output, err := exec.Command(wrapperCmd[0], wrapperCmd[1:]...).CombinedOutput()
    
    // Parse JSON report
    var report WrapperReport
    json.Unmarshal(output, &report)
    
    // Update job with wrapper metadata
    job.ExitCode = report.ExitCode
    job.ExitReason = report.ExitReason
    job.Duration = report.DurationSec
    
    return err
}
```

### Example 15: Master Scheduling with Constraints

```bash
# Master API: Submit job with constraints
curl -X POST https://master:8080/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario_name": "h264_nvenc_5mbps",
    "encoder": "h264_nvenc",
    "wrapper_constraints": {
      "cpu_quota_percent": 200,
      "memory_limit_mb": 4096,
      "nice_priority": -5
    },
    "sla_eligible": true
  }'
```

**Worker receives:**
```json
{
  "job_id": "job-12345",
  "command": ["ffmpeg", "-i", "input.mp4", "output.mp4"],
  "wrapper_constraints": {
    "cpu_quota_percent": 200,
    "memory_limit_mb": 4096,
    "nice_priority": -5
  }
}
```

**Worker executes:**
```bash
ffrtmp run \
  --job-id job-12345 \
  --sla-eligible \
  --cpu-quota 200 \
  --memory-limit 4096 \
  --nice -5 \
  --json \
  -- ffmpeg -i input.mp4 output.mp4
```

---

## Troubleshooting

### Issue 1: Cgroup Permission Denied

**Symptom:**
```
[wrapper] WARNING: Cannot create cgroup (permission denied)
```

**Solution 1: Run with sudo**
```bash
sudo ffrtmp run -- my_command
```

**Solution 2: Enable cgroup delegation**
```bash
# /etc/systemd/system/edge-workload.service
[Service]
User=edgeuser
Delegate=yes
```

**Solution 3: Use nice fallback**
```bash
# Wrapper automatically falls back to nice priority
ffrtmp run --nice 10 -- my_command
```

### Issue 2: Negative Nice Requires Privilege

**Symptom:**
```
[wrapper] WARNING: Cannot set negative nice (requires root), using 0
```

**Solution:**
```bash
# Either run as root
sudo ffrtmp run --nice -10 -- my_command

# Or use positive nice (lower priority)
ffrtmp run --nice 10 -- my_command
```

### Issue 3: Wrapper Exits but Workload Continues

**This is EXPECTED behavior!**

```bash
# Start long-running job
ffrtmp run -- sleep 1000 &
WRAPPER_PID=$!

# Kill wrapper
kill $WRAPPER_PID

# Workload (sleep 1000) still running ✓
ps aux | grep sleep
```

**Explanation:** The wrapper is non-owning. The workload process group is independent.

### Issue 4: Attach Mode - Process Already Exited

**Symptom:**
```
Error: process 12345 does not exist
```

**Solution:**
```bash
# Verify process exists first
kill -0 12345 && echo "Process alive"

# Then attach
ffrtmp attach --pid 12345
```

### Issue 5: JSON Output Missing

**Symptom:**
```bash
ffrtmp run --json -- my_command
# No JSON output
```

**Cause:** Command printed to stdout, obscuring JSON report.

**Solution:**
```bash
# Redirect command output
ffrtmp run --json -- sh -c "my_command > /dev/null" > report.json
```

---

## Advanced Patterns

### Pattern 1: Wrapper in Wrapper (NOT RECOMMENDED)

```bash
# Don't do this - it's redundant
ffrtmp run -- ffrtmp run -- my_command
```

**Instead:** Apply all constraints in one wrapper.

### Pattern 2: Dynamic Constraint Adjustment

```bash
# Low priority during day
ffrtmp run --cpu-weight 50 --nice 10 -- night_batch.sh

# High priority at night (cron)
0 22 * * * ffrtmp run --cpu-weight 200 --nice -5 -- night_batch.sh
```

### Pattern 3: Constraint Profiles

```bash
# Create profile scripts
cat > run_low_priority.sh << 'EOF'
#!/bin/bash
exec ffrtmp run \
  --cpu-weight 50 \
  --nice 10 \
  --memory-limit 1024 \
  --oom-score 500 \
  -- "$@"
EOF

# Use profile
./run_low_priority.sh python batch_job.py
```

---

## Production Checklist

Before deploying wrapper in production:

- [ ] Test Run mode with sample workloads
- [ ] Test Attach mode with existing processes
- [ ] Verify cgroup permissions (or enable delegation)
- [ ] Test wrapper crash (workload should continue)
- [ ] Validate constraint application (`cat /proc/[pid]/cgroup`)
- [ ] Test graceful degradation (no cgroups)
- [ ] Monitor wrapper overhead (< 0.1% CPU)
- [ ] Integrate with monitoring (JSON output)
- [ ] Document SLA eligibility criteria
- [ ] Train operators on attach mode

---

## Summary

The workload wrapper provides:

✅ **Non-owning governance** - Workloads survive wrapper crashes
✅ **Attach to existing** - Zero-downtime adoption
✅ **OS-level constraints** - CPU, memory, IO, nice, OOM
✅ **Lifecycle tracking** - Exit codes, reasons, duration
✅ **Graceful degradation** - Works even without privileges
✅ **Production-ready** - Minimal overhead, comprehensive testing

For architecture details, see [WRAPPER_ARCHITECTURE.md](WRAPPER_ARCHITECTURE.md).
