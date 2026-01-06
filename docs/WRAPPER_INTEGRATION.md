# Worker Agent Wrapper Integration

## Overview

The worker agent now integrates with the edge workload wrapper, providing production-grade process governance for transcoding jobs. This integration is **opt-in** and **backward compatible** with existing deployments.

## Architecture

```
Job with WrapperEnabled=true
         ↓
Worker Agent (main.go)
         ↓
  executeEngineJob()
         ↓
   [Check WrapperEnabled]
         ↓
  executeWithWrapperPath()
         ↓
  agent.ExecuteWithWrapper()
         ↓
  internal/wrapper/run.go
         ↓
  [Fork + Setpgid + Cgroups]
         ↓
    FFmpeg/GStreamer Process
```

## How It Works

### 1. Job Configuration (Opt-In)

Set `WrapperEnabled: true` on job submission:

```go
job := &models.Job{
    ID:             "job-123",
    WrapperEnabled: true,  // ← Enable wrapper
    WrapperConstraints: &models.WrapperConstraints{
        CPUMax:      "50000 100000",  // 50% CPU
        CPUWeight:   100,
        MemoryMaxMB: 1024,             // 1GB
        IOMax:       "8:0 rbps=10485760 wbps=10485760",
    },
    // ... rest of job config
}
```

### 2. Worker Agent Routing

In `worker/cmd/agent/main.go`:

```go
func executeEngineJob(...) {
    // Build command
    args, err := engine.BuildCommand(job, masterURL)
    cmdPath := "/usr/bin/ffmpeg"
    
    // Check wrapper flag
    if job.WrapperEnabled {
        return executeWithWrapperPath(job, cmdPath, cmdName, args, limits, metricsExporter)
    }
    
    // Legacy path: direct exec.CommandContext
    cmd := exec.CommandContext(ctx, cmdPath, args...)
    // ...
}
```

### 3. Wrapper Execution

The `executeWithWrapperPath()` function:
- Creates context with timeout
- Calls `agent.ExecuteWithWrapper(ctx, job, cmdPath, args)`
- Returns wrapper result with platform SLA metrics

### 4. Platform SLA Tracking

The wrapper automatically tracks platform reliability:

```go
result := &report.Result{
    PID:                12345,
    ExitCode:          0,
    Duration:          45 * time.Second,
    PlatformSLA:       true,   // Platform upheld its SLA
    PlatformSLAReason: "success",
}
```

Platform SLA is `false` only for wrapper failures, not workload failures.

## Migration Path

### Existing Deployments (Zero Downtime)

**Step 1:** Update master to support `WrapperEnabled` field
```bash
# Job model already includes field, just ensure API supports it
```

**Step 2:** Deploy updated worker agent
```bash
# Workers now check WrapperEnabled but default to false
# Existing jobs continue with legacy execution
```

**Step 3:** Enable wrapper per job
```json
{
  "wrapper_enabled": true,
  "wrapper_constraints": {
    "cpu_max": "50000 100000",
    "memory_max_mb": 1024
  }
}
```

### New Deployments

For new edge nodes, wrapper can be enabled by default:
- Set `WRAPPER_ENABLED=true` in worker.env
- Configure default constraints in deployment config

## Backward Compatibility

| Scenario | WrapperEnabled | Behavior |
|----------|----------------|----------|
| Old job, old worker | (field missing) | Legacy exec path ✓ |
| Old job, new worker | false or missing | Legacy exec path ✓ |
| New job, new worker | true | Wrapper path ✓ |
| New job, new worker | false | Legacy exec path ✓ |

**Key:** The integration is **additive only**. No existing behavior changes.

## Benefits

### 1. Crash Safety
- Worker agent crashes → workload continues
- Process group isolation ensures independence
- Verified with `kill -9` wrapper test

### 2. Resource Governance
- Cgroup-based CPU/memory/IO limits
- Per-job constraints via `WrapperConstraints`
- Graceful degradation if cgroups unavailable

### 3. Platform SLA Tracking
- Distinguishes wrapper failures from job failures
- Enables reliability metrics: `platform_sla_uptime = successes / total`
- External errors don't count against platform SLA

### 4. Zero-Downtime Adoption
- Edge nodes with existing streams can use attach mode
- No process restarts required
- Seamless integration with live workloads

## Configuration Examples

### Example 1: CPU-Limited Job

```go
job := &models.Job{
    ID:             "transcode-hd",
    WrapperEnabled: true,
    WrapperConstraints: &models.WrapperConstraints{
        CPUMax:    "75000 100000",  // 75% CPU quota
        CPUWeight: 150,             // Higher priority
    },
}
```

### Example 2: Memory-Limited Job

```go
job := &models.Job{
    ID:             "transcode-4k",
    WrapperEnabled: true,
    WrapperConstraints: &models.WrapperConstraints{
        CPUMax:      "100000 100000",  // 100% CPU
        MemoryMaxMB: 2048,             // 2GB limit
    },
}
```

### Example 3: Legacy Job (No Wrapper)

```go
job := &models.Job{
    ID:             "legacy-transcode",
    WrapperEnabled: false,  // or omit field
    Parameters: map[string]interface{}{
        "resource_limits": map[string]interface{}{
            "max_cpu_percent": 80.0,
            "max_memory_mb":   1024.0,
        },
    },
}
```

The worker agent converts legacy `resource_limits` to wrapper constraints if needed.

## Metrics Integration

Wrapper execution adds metrics to job results:

```json
{
  "metrics": {
    "exec_duration": 45.67,
    "wrapper_enabled": true,
    "platform_sla": true,
    "exit_code": 0,
    "pid": 12345
  },
  "analyzer_output": {
    "status": "success",
    "wrapper": true,
    "platform_sla": true
  }
}
```

These metrics can be exported to Prometheus for monitoring.

## Testing

Run integration tests:

```bash
./scripts/test_wrapper_integration.sh
```

Tests verify:
- Worker agent routes to wrapper when enabled
- Backward compatibility with legacy execution
- Integration helper exists and is callable
- Platform SLA metrics included in results
- All 10 tests must pass

## Edge Deployment

For production edge deployment with wrapper:

1. **Install dependencies:**
   ```bash
   ./deployment/install-edge.sh
   ```

2. **Configure worker:**
   ```bash
   sudo vim /etc/ffrtmp/worker.env
   # Set WRAPPER_ENABLED=true
   ```

3. **Enable cgroup delegation:**
   ```bash
   sudo cp deployment/systemd/user@.service.d-delegate.conf \
           /etc/systemd/system/user@.service.d/delegate.conf
   sudo systemctl daemon-reload
   ```

4. **Start worker:**
   ```bash
   sudo systemctl start ffrtmp-worker
   ```

See `docs/WRAPPER_EDGE_DEPLOYMENT.md` for complete deployment guide.

## Troubleshooting

### Issue: Jobs not using wrapper despite WrapperEnabled=true

**Check:**
```bash
# Verify worker agent has integration
grep -n "job.WrapperEnabled" worker/cmd/agent/main.go

# Check job payload
curl http://master:8080/api/jobs/{job_id} | jq .wrapper_enabled
```

### Issue: Wrapper execution fails

**Check logs:**
```bash
journalctl -u ffrtmp-worker -f
```

Common causes:
- Cgroups not delegated (see deployment docs)
- Permissions issue (worker user can't create cgroups)
- Command not found (ffmpeg not in PATH)

### Issue: Legacy execution still used

This is expected if:
- `WrapperEnabled` is false or missing on job
- Job was submitted before worker update
- Default behavior is legacy for compatibility

## Next Steps

- **Phase 1 Extension:** Add Prometheus `/metrics` endpoint
- **Phase 4:** Add attach mode support for zero-downtime adoption
- **Phase 5:** Add advanced monitoring (resource usage tracking)

## References

- Core wrapper: `internal/wrapper/`
- Integration helper: `shared/pkg/agent/wrapper_integration.go`
- Worker agent: `worker/cmd/agent/main.go` (line 862+)
- Job model: `shared/pkg/models/job.go`
- Deployment guide: `docs/WRAPPER_EDGE_DEPLOYMENT.md`
