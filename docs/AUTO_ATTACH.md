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

### 3. Automatic Process Discovery

**Watch daemon** continuously scans for and automatically attaches to running FFmpeg processes.

**Command-line usage:**

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

**Configuration file usage (NEW in Phase 2):**

```bash
# Use YAML configuration file for declarative policies
ffrtmp watch --watch-config /etc/ffrtmp/watch-config.yaml
```

Example configuration file:

```yaml
scan_interval: "10s"
target_commands:
  - ffmpeg
  - gst-launch-1.0

default_limits:
  cpu_quota: 200
  memory_limit: 4096

# Advanced filtering (Phase 2)
filters:
  allowed_users: [ffmpeg, video]
  min_runtime: "5s"
  blocked_dirs: [/tmp, /home/test]

# Per-command overrides
commands:
  ffmpeg:
    limits:
      cpu_quota: 300
      memory_limit: 8192
    filters:
      min_runtime: "10s"
```

**Reliability features (NEW in Phase 3):**

```bash
# Enable state persistence and retry
ffrtmp watch \
  --enable-state \
  --state-path /var/lib/ffrtmp/watch.json \
  --state-flush-interval 30s \
  --enable-retry \
  --max-retry-attempts 3
```

See `examples/watch-config.yaml` for full configuration options.

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
- `--watch-config STRING` - Path to YAML configuration file
- `--scan-interval DURATION` - Scan interval (default: 10s)
- `--target STRING` - Target command names (can specify multiple, default: ffmpeg, gst-launch-1.0)
- `--cpu-quota INT` - Default CPU quota for discovered processes
- `--cpu-weight INT` - Default CPU weight (default: 100)
- `--memory-limit INT` - Default memory limit in MB

**Examples:**

```bash
# Run with default settings
ffrtmp watch

# Use configuration file (recommended for production)
ffrtmp watch --watch-config /etc/ffrtmp/watch-config.yaml

# Command-line mode with custom limits
ffrtmp watch \
  --scan-interval 5s \
  --cpu-quota 100 \
  --memory-limit 1024

# Monitor only FFmpeg
ffrtmp watch --target ffmpeg --scan-interval 3s
```

**Configuration File Format:**

See `examples/watch-config.yaml` for a complete example. Key sections:

- `scan_interval`: How often to scan for new processes
- `target_commands`: Which process names to discover
- `default_limits`: Resource limits for discovered processes
- `filters`: Advanced filtering rules (user, runtime, directory, parent PID)
- `commands`: Per-command overrides for limits and filters

## Architecture

## Architecture

### Process Discovery Scanner

The scanner (`internal/discover/scanner.go`) reads `/proc` to discover running processes:

1. Scans `/proc/[pid]/cmdline` for process command lines
2. Matches against target commands (ffmpeg, gst-launch-1.0, etc.)
3. Extracts rich process metadata (Phase 2):
   - User ID and username (from file ownership)
   - Parent process ID (from `/proc/[pid]/stat`)
   - Working directory (from `/proc/[pid]/cwd` symlink)
   - Process age (calculated from start time)
4. Applies filtering rules if configured
5. Tracks monitored PIDs to avoid duplicate attachments
6. Self-filtering: excludes watch daemon's own child processes

**Performance**: Scan duration typically under 25ms for 6 processes.

### Auto-Attach Service

The auto-attach service (`internal/discover/auto_attach.go`) orchestrates automatic discovery:

1. Periodically scans for new processes (configurable interval)
2. Auto-generates job IDs (format: `auto-{command}-{pid}`)
3. Applies default resource limits to discovered processes
4. Monitors process lifecycle until exit
5. Provides callbacks for attach/detach events
6. Tracks statistics: total scans, discoveries, attachments, scan duration

**Statistics Tracking** (Phase 1):
- `TotalScans`: Counter of all scan cycles performed
- `TotalDiscovered`: Counter of all processes found
- `TotalAttachments`: Counter of successful attachments
- `ActiveAttachments`: Current number of monitored processes
- `LastScanDuration`: Duration of most recent scan
- `LastScanTime`: Timestamp of most recent scan

### Advanced Filtering (Phase 2)

The filtering system (`internal/discover/filter.go`) provides six filter types:

1. **User-based filtering**: Whitelist/blacklist by username or UID
2. **Parent PID filtering**: Allow/block based on parent process
3. **Runtime filtering**: Min/max process age constraints
4. **Directory filtering**: Allow/block by working directory
5. **Per-command overrides**: Different rules per tool
6. **Filter composition**: All filters are AND-ed together

Filters are defined in YAML configuration files for declarative policy management.

### Configuration Management (Phase 2)

Configuration file support (`internal/discover/config.go`):

- YAML-based declarative policies
- Per-command resource limit overrides
- Per-command filter rule overrides
- Duration parsing (10s, 1m, 24h formats)
- Validation at load time with helpful error messages
- Backwards compatible with CLI flags

### Process Wrapper

The wrapper (`internal/wrapper/`) provides two modes:

- **Run mode**: Spawns new process in independent process group
- **Attach mode**: Passively observes already-running process

Both modes apply cgroup resource limits and track SLA compliance.

**Non-Owning Governance Philosophy**: The wrapper governs workloads without owning them. Processes survive wrapper crashes. Attachment is non-intrusive - no signals sent, no lifecycle control.

## Use Cases

### Edge Nodes with Client-Initiated Streams

```bash
# Start watch daemon on edge node with configuration file
ffrtmp watch --watch-config /etc/ffrtmp/edge-node-config.yaml
```

When clients connect and trigger FFmpeg processes, they are automatically discovered and governed based on configured policies.

### Multi-Tenant Security

Only discover processes owned by specific service accounts:

```yaml
# /etc/ffrtmp/watch-config.yaml
filters:
  allowed_users: [ffmpeg-prod, video-service]
  blocked_uids: [0]  # Never discover root processes
```

### Development vs Production Isolation

Different policies for different environments:

```yaml
filters:
  allowed_dirs: [/data/production]
  blocked_dirs: [/home/dev, /tmp, /var/test]
```

### Resource-Intensive Jobs Only

Ignore short-lived processes, focus on long-running transcoding:

```yaml
filters:
  min_runtime: "10s"
commands:
  ffmpeg:
    limits:
      cpu_quota: 300
      memory_limit: 8192
    filters:
      min_runtime: "30s"
```

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

### Watch Daemon Output (Phase 1 Enhanced)

```
[watch] 2026/01/07 08:18:13 Starting auto-attach service...
[watch] 2026/01/07 08:18:13 Scan interval: 10s
[watch] 2026/01/07 08:18:13 Target commands: [ffmpeg gst-launch-1.0]
[watch] 2026/01/07 08:18:21 Discovered 1 new process(es)
[watch] 2026/01/07 08:18:21 Scan complete: new=1 tracked=0 duration=9.7ms
[watch] 2026/01/07 08:18:21 Attaching to PID 29651 (ffmpeg) as job auto-ffmpeg-29651
[watch] 2026/01/07 08:18:21 ✓ Attached to PID 29651 (job: auto-ffmpeg-29651)
[watch] 2026/01/07 08:18:22 Process 29651 exited (job auto-ffmpeg-29651, duration: 1.00s)
[watch] 2026/01/07 08:18:22 ⊗ Detached from PID 29651 (job: auto-ffmpeg-29651)
```

Enhanced logging shows:
- Number of new processes discovered
- Currently tracked process count
- Scan duration for performance monitoring

### SLA Tracking

All attached processes report SLA metrics:

```
JOB auto-ffmpeg-29651 | sla=COMPLIANT | reason=observed_to_completion | runtime=1s | exit=-1 | pid=29651
```

### Statistics API (Phase 1)

Access runtime statistics programmatically:

```go
stats := service.GetStats()
// stats.TotalScans - Total scan cycles performed
// stats.TotalDiscovered - Total processes found
// stats.TotalAttachments - Total successful attachments
// stats.ActiveAttachments - Currently monitored processes
// stats.LastScanDuration - Most recent scan duration
// stats.LastScanTime - Most recent scan timestamp
```

## Implementation Details

### Non-Owning Governance Philosophy

The wrapper follows a **governance, not execution** model:

- Workloads run in independent process groups
- If wrapper crashes, workloads continue
- Resource limits are best-effort (cgroups may fail gracefully)
- Attach mode never sends signals to processes
- No lifecycle control - only observation and resource governance

### Process Independence

```go
// Run mode sets Setpgid for process independence
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true, // New process group
}
```

This ensures wrapper PGID != workload PGID, so signals to wrapper don't propagate to workload.

### Passive Observation in Attach Mode

```go
// Attach checks process exists but doesn't control it
err = process.Signal(syscall.Signal(0)) // Check only, no control signals
```

### Self-Filtering (Phase 1)

Watch daemon excludes its own child processes:

```go
// Filter out own PID
if pid == s.ownPID {
    continue
}

// Filter out children of watch daemon
ppid := s.getParentPID(statPath)
if s.excludePPIDs[ppid] {
    continue
}
```

This prevents spurious self-discoveries and keeps logs clean.

### Process Metadata Extraction (Phase 2)

Rich metadata extracted from /proc filesystem:

```go
// User ID from file ownership
stat := os.Stat("/proc/[pid]")
uid := stat.Sys().(*syscall.Stat_t).Uid

// Username from /etc/passwd lookup
username := lookupUsername(uid)

// Parent PID from stat file field 4
ppid := extractParentPID("/proc/[pid]/stat")

// Working directory from symlink
workingDir := os.Readlink("/proc/[pid]/cwd")

// Process age from start time
processAge := time.Since(startTime)
```

### Filter Evaluation (Phase 2)

Filters are composable (all must pass):

```go
func (f *FilterConfig) ShouldDiscover(proc *Process) bool {
    if !f.checkUserFilter(proc) { return false }
    if !f.checkUIDFilter(proc) { return false }
    if !f.checkParentFilter(proc) { return false }
    if !f.checkRuntimeFilter(proc) { return false }
    if !f.checkDirFilter(proc) { return false }
    return true
}
```

## Security Considerations

1. **Process Nice Values**: Require appropriate permissions (CAP_SYS_NICE or root)
2. **Cgroup Access**: Requires write access to cgroup hierarchy
3. **Process Discovery**: Reads from `/proc` (standard on Linux)
4. **User Filtering**: Can restrict discovery to specific users/UIDs for multi-tenant security
5. **Directory Filtering**: Can prevent discovery in sensitive directories
6. **PID Tracking**: In-memory only, no persistence (secure by default)

## Limitations

- Linux-only (requires `/proc` filesystem and cgroups)
- Nice value adjustment may fail without elevated privileges (warns but continues)
- Cgroup operations are best-effort (failures logged but don't stop attachment)
- Process start time detection depends on `/proc/stat` accuracy
- Scan-based discovery has max latency equal to scan interval (default 10s)
- No state persistence across daemon restarts

## Testing

Comprehensive test suites validate all functionality:

- `scripts/test_non_owning_governance.sh` - 4 tests for resilience and non-owning governance
- `scripts/test_worker_auto_attach.sh` - Worker integration tests
- `scripts/test_discovery_comprehensive.sh` - 6 tests for Phase 1 enhancements
- `scripts/test_phase2_metadata.sh` - 5 tests for Phase 2 metadata and filtering
- `scripts/demo_watch_discovery.sh` - Interactive demonstration

Run all tests:
```bash
./scripts/test_non_owning_governance.sh
./scripts/test_discovery_comprehensive.sh
./scripts/test_phase2_metadata.sh
```

## Documentation

Related documentation:

- `docs/AUTO_DISCOVERY_TEST_SUMMARY.md` - Phase 1 testing results and analysis
- `docs/AUTO_DISCOVERY_PHASE1_RESULTS.md` - Phase 1 executive summary
- `docs/AUTO_DISCOVERY_PHASE2_SUMMARY.md` - Phase 2 complete guide
- `docs/AUTO_DISCOVERY_ENHANCEMENTS.md` - Enhancement roadmap
- `docs/NON_OWNING_BENEFITS.md` - Benefits analysis
- `docs/QUICKREF_AUTO_ATTACH.md` - Quick reference guide
- `examples/watch-config.yaml` - Example configuration file

## Enhancements Completed

### Phase 1: Visibility and Statistics (2026-01-07)

- Self-process filtering (no spurious self-discoveries)
- Enhanced statistics tracking (scans, discoveries, attachments)
- Performance monitoring (scan duration tracking)
- Detailed logging (scan summaries with counts and timing)

**Performance**: Sub-25ms scan times for 6 processes

### Phase 2: Intelligence and Filtering (2026-01-07)

- Rich process metadata (UserID, Username, ParentPID, WorkingDir, ProcessAge)
- Advanced filtering system (6 filter types)
- YAML configuration file support
- Per-command overrides for limits and filters
- Declarative policy management

**Features**: Config-driven discovery with security and compliance capabilities

### Phase 3: Reliability Features (2026-01-07)

#### Phase 3.1: State Persistence
- **Status**: Complete and production-ready
- JSON-based state files with atomic writes
- Periodic flushing (default: 30s)
- Stale PID cleanup on startup
- Statistics preservation across restarts
- **CLI**: `--enable-state`, `--state-path`, `--state-flush-interval`

#### Phase 3.2-3.4: Error Handling, Retry, Health Checks
- **Status**: Complete and production-ready
- Error classification (5 types: Transient, Permanent, RateLimit, Resource, Unknown)
- Automatic retry queue with exponential backoff (1s → 5min)
- Health check system (Healthy, Degraded, Unhealthy)
- Background retry worker (5-second intervals)
- Health status logging on degradation
- **CLI**: `--enable-retry`, `--max-retry-attempts`

**Full reliability stack example**:
```bash
ffrtmp watch \
  --enable-state --state-path /var/lib/ffrtmp/watch.json \
  --enable-retry --max-retry-attempts 5
```

**Error handling**: Intelligent classification determines retry eligibility
**Backoff strategy**: 1s → 2s → 4s → 8s → ... → 5min (max)
**Health thresholds**: 5 consecutive scan failures = unhealthy, 10 attach failures = degraded

#### Testing
- Phase 1: `scripts/test_discovery_comprehensive.sh` (6 tests)
- Phase 2: `scripts/test_phase2_metadata.sh` (5 tests)
- Phase 3: `scripts/test_phase3_reliability.sh` (6 tests)
- All test suites validated and passing

## Future Enhancements

Potential future improvements:

- inotify-based discovery (instant detection, no polling)
- Network bandwidth limits (TC integration)
- GPU resource limits (nvidia-docker integration)
- Process affinity management (CPU pinning)
- Prometheus metrics HTTP endpoint
- Process tree analysis (discover parent + children)
- Cgroup v1 full support (currently focuses on v2)
