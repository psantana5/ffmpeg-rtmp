# Edge Workload Wrapper Architecture

## Overview

The **Edge Workload Wrapper** is a production-grade, thin governance layer that wraps existing workloads on edge nodes. This wrapper operates in the **control plane** (not data plane) and applies OS-level constraints to workloads without owning or managing them.

## Core Design Principle (NON-NEGOTIABLE)

**This system NEVER owns the workload.**

- Workloads already exist on edge nodes
- Workloads may already be running
- Workloads must continue running even if this wrapper crashes
- This wrapper ONLY governs how the OS executes workloads

If this rule is broken → the design is invalid.

## Stack Position

```
┌───────────────────────────────────────────┐
│  Application (ffmpeg/gstreamer/OBS/etc)  │  ← Business logic
├───────────────────────────────────────────┤
│  THIS WRAPPER (control & governance)     │  ← OS constraints
├───────────────────────────────────────────┤
│  Linux kernel (cgroups, scheduler, IO)   │  ← Resource enforcement
└───────────────────────────────────────────┘
```

**The wrapper is:**
- Out of the data plane
- In the control plane
- Below the application
- Above the kernel

## Purpose

Build a thin, node-local wrapper that:

1. Wraps existing workloads on edge nodes
2. Can attach to already-running processes
3. Can optionally spawn a workload, but never owns it
4. Applies OS-level constraints (cgroups, nice, env)
5. Observes lifecycle and exit reasons
6. Reports metrics and metadata

**This is NOT:**
- A new execution system
- A scheduler
- A pipeline
- A job orchestrator

## Execution Modes

### 1️⃣ Run Mode (wrapper-spawned, still non-owning)

```bash
ffrtmp run [flags] -- <command> [args...]
```

**Behavior:**
- Fork + exec the workload
- Apply constraints BEFORE exec
- Track PID
- If wrapper crashes → workload MUST keep running
- Process is started in its own process group (`setpgid`)

**Example:**
```bash
ffrtmp run --job-id job123 --sla-eligible \
  --cpu-quota 200 --memory-limit 4096 \
  -- ffmpeg -i input.mp4 -c:v h264_nvenc output.mp4
```

### 2️⃣ Attach Mode (CRITICAL FOR ADOPTION)

```bash
ffrtmp attach --pid <PID> [flags]
```

**Behavior:**
- Attach to a process that is ALREADY running
- Move the PID into a managed cgroup
- Start passive observation only
- DO NOT restart
- DO NOT modify execution flow
- DO NOT inject code

**Why this mode is required:**
- Edge workloads already exist
- Restarts are unacceptable
- Legacy systems must be supported
- Zero-downtime governance adoption

**Example:**
```bash
# Existing process running (PID 12345)
ffrtmp attach --pid 12345 --job-id job123 --sla-eligible \
  --cpu-weight 50 --nice 10
```

## SLA Semantics

SLA eligibility is decided **before** wrapping:

```json
{
  "job_id": "job123",
  "sla_eligible": true,
  "intent": "production"
}
```

**Intent types:**
- `production` - Production workload (default)
- `test` - Test/development workload
- `experiment` - Experimental workload
- `soak` - Long-running soak test

**Key principle:** SLA is about **platform behavior**, not workload success. A job may fail correctly and still be SLA-compliant.

## Wrapper Responsibilities (ONLY THESE)

### ✅ Allowed:

1. **Create or join one cgroup per workload**
2. **Apply reversible OS constraints:**
   - `cpu.max` (CPU quota)
   - `cpu.weight` (CPU proportional share)
   - `memory.max` (Memory limit)
   - `io.max` (IO limit, cgroup v2 only)
   - `nice` priority (fallback when unprivileged)
   - `oom_score_adj` (OOM killer priority)

3. **Observe:**
   - PID lifecycle
   - Start time
   - Exit code
   - Termination reason (signal, timeout, policy)

4. **Forward stdout/stderr untouched**

### ❌ Forbidden:

- ❌ No scheduling logic
- ❌ No retries or restarts
- ❌ No job ownership
- ❌ No orchestration
- ❌ No pipelines
- ❌ No business logic
- ❌ No kernel tuning
- ❌ No LD_PRELOAD / hooks

## Constraints

The wrapper supports these OS-level constraints:

| Constraint | Type | Range | Default | Description |
|------------|------|-------|---------|-------------|
| `cpu-quota` | int | 0-∞ | 0 (unlimited) | CPU quota % (100=1 core, 200=2 cores) |
| `cpu-weight` | int | 1-10000 | 100 | CPU proportional share weight |
| `nice` | int | -20 to 19 | 0 | Process priority (lower = higher prio) |
| `memory-limit` | int64 | 0-∞ | 0 (unlimited) | Memory limit in MB |
| `io-weight` | int | 0-100 | 0 | IO weight % (cgroup v2 only) |
| `oom-score` | int | -1000 to 1000 | 0 | OOM killer score adjustment |

**Preset constraints:**
- `DefaultConstraints()` - Unlimited, normal priority
- `LowPriorityConstraints()` - Deprioritized (nice=10, cpu_weight=50)
- `HighPriorityConstraints()` - Prioritized (nice=-5, cpu_weight=200)

## Lifecycle Tracking

The wrapper tracks workload lifecycle through these states:

```
unknown → starting → running → {completed, failed, killed}
```

**Exit reasons:**
- `success` - Exit code 0
- `error` - Exit code != 0
- `signal` - Killed by signal (SIGTERM, SIGKILL, etc.)
- `timeout` - Wrapper timeout (if implemented)
- `oom` - Out of memory killed
- `cgroup_limit` - Cgroup limit exceeded
- `policy_violation` - Policy enforcement
- `unknown` - Unable to determine

## Failure Model (EDGE-SAFE)

| Scenario | Behavior |
|----------|----------|
| Wrapper crash | Workload continues |
| Missing permissions | Degrade gracefully (log and continue) |
| No cgroups | Log and continue (use nice fallback) |
| No remote dependency | Required - works standalone |

**Graceful degradation example:**

```
[wrapper] WARNING: Cannot create cgroup (permission denied)
[wrapper] Applying nice priority 10 to PID 12345
[wrapper] Attached to existing PID 12345
```

The wrapper **always tries to do SOMETHING useful**, even without full privileges.

## Implementation Details

### Package Structure

```
shared/pkg/wrapper/
├── metadata.go      # WorkloadMetadata (job_id, sla_eligible, intent)
├── constraints.go   # Constraints (cpu, memory, io limits)
├── lifecycle.go     # LifecycleState, ExitReason tracking
├── cgroup.go        # CgroupManager (v1 and v2 support)
└── wrapper.go       # Wrapper (core Run/Attach logic)
```

### Cgroup Support

The wrapper supports **both cgroup v1 and v2**:

**Cgroup v2 (unified hierarchy):**
- `/sys/fs/cgroup/ffrtmp-wrapper-{workload_id}/`
- `cpu.max`, `cpu.weight`, `memory.max`, `io.weight`

**Cgroup v1 (separate hierarchies):**
- `/sys/fs/cgroup/cpu/ffrtmp-wrapper-{workload_id}/`
- `/sys/fs/cgroup/memory/ffrtmp-wrapper-{workload_id}/`
- `cpu.cfs_quota_us`, `cpu.shares`, `memory.limit_in_bytes`

### Process Group Isolation

In **Run mode**, processes are started in their own process group:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true, // New process group
    Pgid:    0,    // Process becomes its own group leader
}
```

This ensures:
1. Workload survives wrapper crash
2. Signals to wrapper don't affect workload
3. Workload can spawn children without wrapper interference

## Usage Examples

### Example 1: Run FFmpeg with CPU/Memory constraints

```bash
ffrtmp run \
  --job-id transcode-job-001 \
  --sla-eligible \
  --intent production \
  --cpu-quota 200 \
  --memory-limit 4096 \
  -- ffmpeg -i input.mp4 -c:v h264_nvenc -b:v 5M output.mp4
```

### Example 2: Attach to existing GStreamer pipeline

```bash
# Find existing process
ps aux | grep gst-launch

# Attach wrapper
ffrtmp attach \
  --pid 12345 \
  --job-id stream-job-042 \
  --sla-eligible \
  --cpu-weight 150 \
  --nice -5
```

### Example 3: Run test workload (not SLA-eligible)

```bash
ffrtmp run \
  --job-id test-benchmark-01 \
  --intent test \
  --cpu-quota 100 \
  --nice 10 \
  -- ./benchmark.sh
```

### Example 4: JSON output for integration

```bash
ffrtmp run --json --job-id job123 -- sleep 5 > report.json
```

Output:
```json
{
  "job_id": "job123",
  "exit_code": 0,
  "exit_reason": "success",
  "duration_sec": 5.002,
  "events": [
    {
      "pid": 12345,
      "state": "starting",
      "timestamp": "2026-01-06T10:00:00Z",
      "message": "Spawning workload process"
    },
    {
      "pid": 12345,
      "state": "running",
      "timestamp": "2026-01-06T10:00:00Z",
      "message": "PID 12345 started"
    },
    {
      "pid": 12345,
      "state": "completed",
      "timestamp": "2026-01-06T10:00:05Z",
      "exit_code": 0,
      "exit_reason": "success",
      "message": "Completed successfully"
    }
  ]
}
```

## Security Considerations

### Privilege Requirements

**Unprivileged (normal user):**
- ✅ Run mode with unconstrained workloads
- ✅ Attach mode with passive observation
- ✅ Nice priority (positive values 0-19)
- ⚠️  Cgroups may not be available (degrades to nice)

**Privileged (root or cgroup delegation):**
- ✅ Full cgroup support (CPU, memory, IO)
- ✅ Nice priority (negative values -20 to -1)
- ✅ OOM score adjustment (negative values)

### Cgroup Delegation (Recommended)

For production edge nodes, use systemd cgroup delegation:

```ini
# /etc/systemd/system/edge-workload.service
[Service]
User=edgeuser
Delegate=yes
```

This allows unprivileged users to manage cgroups under their slice.

## Performance Impact

**Overhead:**
- Wrapper CPU: < 0.1% (passive observation)
- Wrapper memory: < 10 MB
- Workload impact: None (constraints applied at OS level)

**Cgroup overhead:**
- CPU accounting: < 1% overhead
- Memory accounting: Negligible
- IO accounting: < 2% overhead (if enabled)

## Integration with Distributed System

The wrapper is designed to integrate with the existing ffmpeg-rtmp distributed system:

1. **Worker agent** spawns wrapper instead of raw process
2. **Wrapper** applies constraints and tracks lifecycle
3. **Worker agent** collects wrapper reports
4. **Master** receives enhanced metrics (SLA, constraints, exit reasons)

```
Master
  ↓ job assignment
Worker Agent
  ↓ spawn with constraints
Wrapper (ffrtmp run)
  ↓ fork + exec + cgroup
Workload (ffmpeg)
```

## Future Enhancements

**Phase 1 (Current):**
- ✅ Run and Attach modes
- ✅ Cgroup v1/v2 support
- ✅ Basic constraints (CPU, memory, nice)
- ✅ Lifecycle tracking
- ✅ Graceful degradation

**Phase 2 (Planned):**
- [ ] Real-time resource usage tracking
- [ ] Dynamic constraint adjustment
- [ ] Network bandwidth constraints (tc integration)
- [ ] Timeout enforcement
- [ ] Prometheus metrics export

**Phase 3 (Future):**
- [ ] Advanced IO constraints (per-device)
- [ ] NUMA awareness
- [ ] GPU resource constraints (nvidia-smi integration)
- [ ] Power management (cpufreq governor hints)
- [ ] Thermal throttling detection

## Testing

Run integration tests:

```bash
# Test run mode
./bin/ffrtmp run --job-id test-1 --intent test -- sleep 2

# Test attach mode
sleep 10 &
./bin/ffrtmp attach --pid $! --intent test
```

See `docs/WRAPPER_EXAMPLES.md` for more comprehensive test scenarios.

## References

- Cgroup v2: https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html
- Process groups: `man 2 setpgid`
- Nice priority: `man 2 setpriority`
- OOM killer: https://www.kernel.org/doc/gorman/html/understand/understand016.html
