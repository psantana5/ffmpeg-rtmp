# FFmpeg Resource Governance - Auto-Attach Feature

## Overview

The FFmpeg RTMP system now supports **automatic process discovery and attachment**, allowing resource governance to be applied to FFmpeg/transcoding processes that start outside the wrapper's control. This is **critical for production edge nodes** where client-initiated streams or external triggers may spawn processes independently.

## Features

### 1. Manual Process Wrapping

Spawn new processes with resource constraints:

```bash
# Run FFmpeg with resource constraints
ffrtmp run \
  --job-id transcode-001 \
  --sla-eligible \
  --cpu-quota 200 \
  --memory-limit 4096 \
  -- ffmpeg -i input.mp4 -c:v h264_nvenc output.mp4
```

### 2. Manual Process Attachment

Attach to already-running processes:

```bash
# Attach to existing process
ffrtmp attach \
  --pid 12345 \
  --job-id existing-job-042 \
  --cpu-weight 150 \
  --nice -5
```

### 3. Automatic Process Discovery (NEW)

**Watch daemon** continuously scans for and automatically attaches to running FFmpeg processes:

```bash
# Start watch daemon with default settings
ffrtmp watch

# Start with custom resource limits
ffrtmp watch \
  --scan-interval 5s \
  --cpu-quota 150 \
  --memory-limit 2048 \
  --target ffmpeg \
  --target gst-launch-1.0
```

## Command Reference

### `ffrtmp run`

Spawn a new workload with governance.

**Flags:**
- `--job-id STRING` - Job identifier
- `--sla-eligible` - Mark job as SLA-eligible
- `--cpu-quota INT` - CPU quota percentage (100=1 core, 200=2 cores)
- `--cpu-max STRING` - Raw cgroup format (e.g., "200000 100000")
- `--cpu-weight INT` - CPU weight (1-10000, default: 100)
- `--memory-limit INT` - Memory limit in MB
- `--memory-max INT` - Memory limit in bytes
- `--io-max STRING` - IO limit (cgroup v2 format)
- `--workdir STRING` - Working directory
- `--json` - JSON output

**Examples:**
```bash
# Simple transcoding job
ffrtmp run --job-id job-001 -- ffmpeg -i input.mp4 output.mp4

# With resource limits
ffrtmp run \
  --job-id transcode-hd \
  --sla-eligible \
  --cpu-quota 200 \
  --memory-limit 4096 \
  -- ffmpeg -i input.mp4 -c:v h264_nvenc -preset fast output.mp4
```

### `ffrtmp attach`

Attach to an already-running process (passive observation).

**Flags:**
- `--pid INT` - PID to attach to (required)
- `--job-id STRING` - Job identifier
- `--cpu-quota INT` - CPU quota percentage
- `--cpu-weight INT` - CPU weight
- `--memory-limit INT` - Memory limit in MB
- `--nice INT` - Process nice value (-20 to 19)
- `--json` - JSON output

**Examples:**
```bash
# Basic attachment
ffrtmp attach --pid 12345 --job-id external-stream

# With priority adjustment
ffrtmp attach \
  --pid 12345 \
  --job-id live-stream \
  --cpu-weight 150 \
  --nice -5
```

### `ffrtmp watch`

Automatically discover and attach to running FFmpeg/transcoding processes.

**Flags:**
- `--scan-interval DURATION` - Scan interval (default: 10s)
- `--target STRING` - Target command names (can specify multiple, default: ffmpeg, gst-launch-1.0)
- `--cpu-quota INT` - Default CPU quota for discovered processes
- `--cpu-weight INT` - Default CPU weight (default: 100)
- `--memory-limit INT` - Default memory limit in MB

**Examples:**
```bash
# Run with default settings
ffrtmp watch

# Aggressive resource limits
ffrtmp watch \
  --scan-interval 5s \
  --cpu-quota 100 \
  --memory-limit 1024

# Monitor only FFmpeg
ffrtmp watch --target ffmpeg --scan-interval 3s
```

## Architecture

### Process Discovery Scanner

The scanner (`internal/discover/scanner.go`) reads `/proc` to discover running processes:

1. Scans `/proc/[pid]/cmdline` for process command lines
2. Matches against target commands (ffmpeg, gst-launch-1.0, etc.)
3. Tracks process start times from `/proc/[pid]/stat`
4. Maintains list of monitored PIDs to avoid duplicate attachments

### Auto-Attach Service

The auto-attach service (`internal/discover/auto_attach.go`) orchestrates automatic discovery:

1. Periodically scans for new processes (configurable interval)
2. Auto-generates job IDs (format: `auto-{command}-{pid}`)
3. Applies default resource limits to discovered processes
4. Monitors process lifecycle until exit
5. Provides callbacks for attach/detach events

### Process Wrapper

The wrapper (`internal/wrapper/`) provides two modes:

- **Run mode**: Spawns new process in independent process group
- **Attach mode**: Passively observes already-running process

Both modes apply cgroup resource limits and track SLA compliance.

## Use Cases

### Edge Nodes with Client-Initiated Streams

```bash
# Start watch daemon on edge node
ffrtmp watch --cpu-quota 150 --memory-limit 2048
```

When clients connect and trigger FFmpeg processes, they are automatically discovered and governed.

### Development/Testing

```bash
# Terminal 1: Start watch daemon
ffrtmp watch --scan-interval 3s

# Terminal 2: Start FFmpeg manually
ffmpeg -f lavfi -i testsrc=duration=10:size=1920x1080:rate=30 output.mp4

# Watch daemon automatically attaches and applies limits
```

### Production Worker Integration

The worker agent can enable auto-attach:

```bash
worker/bin/agent \
  --enable-auto-attach \
  --auto-attach-scan-interval 10s \
  --auto-attach-cpu-quota 150 \
  --auto-attach-memory-limit 2048
```

## Resource Limit Conversion

### CPU Quota

- **User-friendly**: `--cpu-quota 200` (200% = 2 cores)
- **Internal**: Converted to cgroup format: `200000 100000` (200ms per 100ms period)

### Memory Limit

- **User-friendly**: `--memory-limit 4096` (4GB in MB)
- **Internal**: Converted to bytes: `4294967296`

### CPU Weight

- **cgroup v2**: `cpu.weight` (1-10000)
- **cgroup v1**: Converted to `cpu.shares` (weight * 10.24)

## Monitoring

### Watch Daemon Output

```
[watch] 2026/01/07 08:18:13 Starting auto-attach service...
[watch] 2026/01/07 08:18:13 Scan interval: 10s
[watch] 2026/01/07 08:18:13 Target commands: [ffmpeg gst-launch-1.0]
[watch] 2026/01/07 08:18:21 Discovered 1 new process(es)
[watch] 2026/01/07 08:18:21 Attaching to PID 29651 (ffmpeg) as job auto-ffmpeg-29651
[watch] 2026/01/07 08:18:21 ✓ Attached to PID 29651 (job: auto-ffmpeg-29651)
[watch] 2026/01/07 08:18:22 Process 29651 exited (job auto-ffmpeg-29651, duration: 1.00s)
[watch] 2026/01/07 08:18:22 ⊗ Detached from PID 29651 (job: auto-ffmpeg-29651)
```

### SLA Tracking

All attached processes report SLA metrics:

```
JOB auto-ffmpeg-29651 | sla=COMPLIANT | reason=observed_to_completion | runtime=1s | exit=-1 | pid=29651
```

## Implementation Details

### Non-Owning Governance Philosophy

The wrapper follows a **governance, not execution** model:

- Workloads run in independent process groups
- If wrapper crashes, workloads continue
- Resource limits are best-effort (cgroups may fail gracefully)
- Attach mode never sends signals to processes

### Process Independence

```go
// Run mode sets Setpgid for process independence
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true, // New process group
}
```

### Passive Observation in Attach Mode

```go
// Attach checks process exists but doesn't control it
err = process.Signal(syscall.Signal(0)) // Check only
```

## Security Considerations

1. **Process Nice Values**: Require appropriate permissions (CAP_SYS_NICE or root)
2. **Cgroup Access**: Requires write access to cgroup hierarchy
3. **Process Discovery**: Reads from `/proc` (standard on Linux)
4. **PID Tracking**: In-memory only, no persistence

## Limitations

- Linux-only (requires `/proc` filesystem and cgroups)
- Nice value adjustment may fail without elevated privileges (warns but continues)
- Cgroup operations are best-effort (failures logged but don't stop attachment)
- Process start time detection depends on `/proc/stat` accuracy

## Future Enhancements

- [ ] Persistence of tracked PIDs across restarts
- [ ] Integration with master node for centralized governance
- [ ] Network bandwidth limits (TC integration)
- [ ] GPU resource limits (nvidia-docker integration)
- [ ] Process affinity management (CPU pinning)
- [ ] Cgroup v1 full support (currently focuses on v2)
