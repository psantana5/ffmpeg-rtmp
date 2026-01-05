# Resource Limits Documentation

## Overview

The FFmpeg-RTMP distributed transcoding system now supports comprehensive resource limits per job to prevent resource exhaustion and ensure system stability.

## Features

### 1. CPU Limits (cgroup-based)
- **Purpose**: Prevent jobs from monopolizing CPU resources
- **Implementation**: Linux cgroups (v1 and v2 supported)
- **Fallback**: Process nice priority when cgroups unavailable

### 2. Memory Limits (cgroup-based)  
- **Purpose**: Prevent OOM conditions
- **Implementation**: Linux cgroups memory controller
- **Behavior**: Process killed if exceeding limit

### 3. Disk Space Monitoring
- **Purpose**: Prevent disk full conditions
- **Check**: Before job starts
- **Alert**: Warning at 90% usage, reject at 95%

### 4. Timeout Enforcement
- **Purpose**: Kill runaway jobs
- **Implementation**: Context-based with fallback monitoring
- **Cleanup**: Process group termination (SIGTERM → SIGKILL)

### 5. Process Priority Management
- **Purpose**: Lower priority for transcoding jobs
- **Implementation**: Linux nice value (default: 10)
- **Benefit**: System remains responsive during heavy load

## API Usage

### Job Submission with Resource Limits

```json
{
  "scenario": "1080p-h264",
  "parameters": {
    "bitrate": "4M",
    "duration": 300
  },
  "resource_limits": {
    "max_cpu_percent": 200,      // 200% = 2 cores
    "max_memory_mb": 2048,        // 2GB memory limit
    "max_disk_mb": 5000,          // 5GB temp space required
    "timeout_sec": 600            // 10 minute timeout
  }
}
```

### Default Limits

If no `resource_limits` specified, defaults are:

```go
{
    MaxCPUPercent: numCPU * 100,  // All available CPUs
    MaxMemoryMB:   2048,           // 2GB
    MaxDiskMB:     5000,           // 5GB
    TimeoutSec:    3600,           // 1 hour
}
```

## Resource Limit Parameters

### max_cpu_percent
- **Type**: Integer
- **Unit**: Percentage (100 = 1 core)
- **Examples**:
  - `100` = 1 CPU core
  - `200` = 2 CPU cores
  - `400` = 4 CPU cores
- **Note**: Requires cgroups (root/sudo), otherwise uses nice priority

### max_memory_mb
- **Type**: Integer
- **Unit**: Megabytes
- **Typical Values**:
  - 720p: 512-1024 MB
  - 1080p: 1024-2048 MB
  - 4K: 2048-4096 MB
- **Note**: Requires cgroups (root/sudo)

### max_disk_mb
- **Type**: Integer  
- **Unit**: Megabytes
- **Purpose**: Check available disk space before starting
- **Note**: Always enforced, no special permissions needed

### timeout_sec
- **Type**: Integer
- **Unit**: Seconds
- **Purpose**: Maximum job execution time
- **Note**: Always enforced via context timeout

## System Architecture

### Worker Initialization
```
1. Detect cgroup version (v1 or v2)
2. Check cgroup permissions
3. Initialize cgroup manager
4. Set up disk monitoring
```

### Job Execution Flow
```
1. Check disk space (reject if < 5% available)
2. Parse resource limits from job parameters
3. Generate input video if needed
4. Start transcoding process
5. Set process priority (nice value)
6. Create cgroup (if permissions available)
7. Add process to cgroup
8. Monitor process (timeout, memory)
9. Wait for completion
10. Cleanup cgroup
11. Return results
```

### Process Monitoring
- **Interval**: Every 5 seconds
- **Checks**:
  - Process exists
  - Timeout exceeded
  - Memory usage (logged if over limit)
- **Action**: Kill process group if timeout exceeded

## Cgroup Support

### Cgroup v2 (Unified Hierarchy)
- **Path**: `/sys/fs/cgroup/ffmpeg-job-{jobID}/`
- **Files**:
  - `cpu.max`: CPU quota/period
  - `memory.max`: Memory limit in bytes
  - `cgroup.procs`: Process list

### Cgroup v1 (Separate Hierarchies)
- **CPU Path**: `/sys/fs/cgroup/cpu/ffmpeg-job-{jobID}/`
- **Memory Path**: `/sys/fs/cgroup/memory/ffmpeg-job-{jobID}/`
- **Files**:
  - CPU: `cpu.cfs_quota_us`, `cpu.cfs_period_us`
  - Memory: `memory.limit_in_bytes`

### Permission Requirements

**With Root/Sudo** (Full cgroup support):
```bash
sudo ./bin/agent -master https://localhost:8080 ...
```

**Without Root** (Fallback mode):
```bash
./bin/agent -master https://localhost:8080 ...
# Still enforces: disk limits, timeouts, nice priority
# Cannot enforce: CPU/memory cgroups
```

## Monitoring & Logs

### Log Output Example
```
>>> RESOURCE CHECK PHASE <<<
Disk space: 39.9% used (142023 MB available)
Resource limits: CPU=200%, Memory=2048MB, Disk=5000MB, Timeout=600s

>>> TRANSCODING EXECUTION PHASE <<<
Process started with PID: 12345
Set process priority: nice=10 (lower than normal)
Detected cgroup v2 (root: /sys/fs/cgroup)
Set CPU limit: 200% (quota=200000, period=100000)
Set memory limit: 2048 MB
✓ Process added to cgroup: /sys/fs/cgroup/ffmpeg-job-abc123
```

### Warning Messages
```
WARNING: Cannot create cgroup (permission denied), running without cgroup limits
WARNING: Disk usage is high: 92.5% (available: 15000 MB)
WARNING: Process 12345 using 2500 MB (limit: 2048 MB)
```

### Error Conditions
```
❌ insufficient disk space: 96.2% used
Process 12345 exceeded timeout (600 seconds), killing...
```

## Performance Impact

### Overhead
- **Disk check**: <1ms per job
- **Cgroup creation**: 1-5ms per job
- **Process monitoring**: Negligible (5s intervals)
- **Nice priority**: No measurable overhead

### Benefits
- **System stability**: Prevents resource exhaustion
- **Predictable performance**: Jobs stay within limits
- **Multi-tenancy**: Fair resource sharing
- **Responsiveness**: System remains usable under load

## Best Practices

### CPU Limits
```json
// Light encoding (720p, fast preset)
"max_cpu_percent": 100-200

// Medium encoding (1080p, medium preset)
"max_cpu_percent": 200-400

// Heavy encoding (4K, slow preset)
"max_cpu_percent": 400-800
```

### Memory Limits
```json
// Conservative (safety margin)
"max_memory_mb": 2048

// Standard transcoding
"max_memory_mb": 1024

// Lightweight (720p only)
"max_memory_mb": 512
```

### Timeouts
```json
// Fast jobs (< 1 min duration)
"timeout_sec": 300  // 5 minutes

// Standard jobs (1-5 min duration)
"timeout_sec": 900  // 15 minutes

// Long jobs (> 5 min duration)  
"timeout_sec": 3600  // 1 hour
```

### Disk Space
```json
// Always require sufficient space
"max_disk_mb": duration_sec * bitrate_mbps * 2  // 2x for safety
```

## Troubleshooting

### Cgroups Not Working
**Problem**: `WARNING: Cannot create cgroup (permission denied)`

**Solutions**:
1. Run worker with sudo: `sudo ./bin/agent ...`
2. Add user to cgroup-permitted group
3. Accept fallback mode (disk/timeout/nice still work)

### Jobs Timing Out
**Problem**: Jobs consistently timeout before completion

**Solutions**:
1. Increase `timeout_sec` value
2. Check if CPU limit too low (causing slow encoding)
3. Verify hardware capabilities match workload

### High Disk Usage
**Problem**: `WARNING: Disk usage is high`

**Solutions**:
1. Clean up /tmp regularly
2. Increase disk space
3. Reduce concurrent jobs
4. Enable input cleanup (PERSIST_INPUTS=false)

### OOM Kills
**Problem**: Jobs killed by OOM killer

**Solutions**:
1. Increase `max_memory_mb`
2. Reduce concurrent jobs
3. Use lower resolution/bitrate
4. Check for memory leaks in FFmpeg

## Example Scenarios

### Live Streaming (Low Latency)
```json
{
  "scenario": "720p30-h264",
  "parameters": {
    "bitrate": "2M",
    "output_mode": "rtmp"
  },
  "resource_limits": {
    "max_cpu_percent": 200,
    "max_memory_mb": 1024,
    "timeout_sec": 3600
  },
  "queue": "live",
  "priority": "high"
}
```

### Batch Transcoding (Efficiency)
```json
{
  "scenario": "1080p30-h264",
  "parameters": {
    "bitrate": "4M",
    "duration": 300,
    "preset": "medium"
  },
  "resource_limits": {
    "max_cpu_percent": 400,
    "max_memory_mb": 2048,
    "timeout_sec": 1800
  },
  "queue": "batch",
  "priority": "low"
}
```

### 4K Transcoding (High Quality)
```json
{
  "scenario": "4k-h265",
  "parameters": {
    "bitrate": "20M",
    "duration": 600,
    "preset": "slow"
  },
  "resource_limits": {
    "max_cpu_percent": 800,
    "max_memory_mb": 4096,
    "timeout_sec": 7200
  },
  "queue": "batch",
  "priority": "medium"
}
```

## System Requirements

### Minimum
- Linux kernel 3.10+ (cgroup v1)
- /tmp with 10GB+ free space
- 2GB+ RAM per worker

### Recommended
- Linux kernel 4.5+ (cgroup v2)
- /tmp with 50GB+ free space
- 8GB+ RAM per worker
- SSD storage for /tmp

### Optional
- Root/sudo access for full cgroup support
- systemd for cgroup management
- Dedicated /tmp partition

## Future Enhancements

- [ ] GPU resource limits (CUDA/OpenCL)
- [ ] Network bandwidth throttling
- [ ] I/O priority (ionice)
- [ ] Per-user resource quotas
- [ ] Resource reservation/scheduling
- [ ] Cgroup delegation (rootless)

---

**Version**: 1.0  
**Last Updated**: 2026-01-05  
**Status**: Production Ready
