# Production Readiness Features

**Status**:  Complete  
**Date**: 2026-01-06  
**Architecture**: Swedish principles - boring, correct, non-reactive

---

## Overview

This document describes the production-grade features added to the FFmpeg RTMP distributed transcoding system. All features follow strict architectural principles:

- **Retries apply to messages, not work** (transport only, never workload)
- **Graceful shutdown** (let jobs finish, no killing)
- **Minimal, correct visibility** (derived, not driving)
- **Centralized logging** (/var/log/ffrtmp structure)

---

## 1. Retry Logic (Transport Only)

### Implementation

**Files Modified:**
- `shared/pkg/agent/client.go` (added retry.Config and retry.Do wrapping)

**Scope:**
-  `SendHeartbeat()` - Retry heartbeat delivery to master
-  `GetNextJob()` - Retry job polling from master
-  `SendResults()` - Retry result delivery to master
-  **NEVER** retry job execution
-  **NEVER** retry wrapper actions
-  **NEVER** retry FFmpeg workloads

### Configuration

```go
retry.Config{
    MaxRetries:      3,
    InitialBackoff:  1 * time.Second,
    MaxBackoff:      30 * time.Second,
    Multiplier:      2.0,
}
```

### Retryable Errors

- Connection refused
- Connection timeout
- HTTP 502, 503, 504 (transient server errors)
- EOF, broken pipe
- Context cancellation stops retries immediately

### Rule

> "Retries apply to messages, not work"

If you can phrase an operation as "sending a message" or "checking for messages", retries are allowed. If it's "doing work" or "executing a task", NO retries.

---

## 2. Graceful Shutdown

### Implementation

**Files Modified:**
- `shared/pkg/shutdown/shutdown.go` (enhanced with Done() channel)
- `master/cmd/master/main.go` (LIFO shutdown order)
- `worker/cmd/agent/main.go` (wait for jobs, bounded timeout)

### Behavior

#### Worker Agent Shutdown:
1. Receive SIGTERM/SIGINT
2. Close `Done()` channel
3. **Stop accepting new jobs** (break out of polling loop)
4. Wait for active jobs to complete (30-second timeout)
5. Stop heartbeat loop
6. Execute shutdown handlers (metrics server → logger)
7. Exit cleanly

#### Master Server Shutdown:
1. Receive SIGTERM/SIGINT
2. Close `Done()` channel
3. Execute shutdown handlers in LIFO order:
   - Close logger
   - Stop HTTP server (30s graceful)
   - Stop metrics server
   - Stop scheduler
   - Stop cleanup manager
   - Close database connection
4. Exit cleanly

### Critical Rules

** Allowed:**
- Stop accepting new jobs
- Let running jobs finish
- Bounded wait (30 seconds)
- Emit final JobResult

** NEVER:**
- Kill workloads to speed up shutdown
- Change workload behavior
- Force-terminate FFmpeg processes
- Interrupt wrapper execution

Workloads are **owned by the OS**, not by the wrapper or agent. We govern exit, not execution.

---

## 3. Enhanced Readiness Checks

### Implementation

**Files Modified:**
- `worker/cmd/agent/main.go` (enhanced `/ready` endpoint)

### Checks

The `/ready` endpoint returns HTTP 200 if all checks pass, 503 otherwise:

#### 1. FFmpeg Availability
```go
exec.LookPath("ffmpeg")
```
Returns "available" or "not_found"

#### 2. Disk Space
```go
resources.CheckDiskSpace("/tmp")
```
- Requires at least 10% free space
- Returns: "ok: X% used, Y MB available"
- Or: "low: X% used" (fails if >90% used)

#### 3. Master Reachability
```go
client.SendHeartbeat() // with 5-second timeout
```
- Tests connectivity to master
- Returns "reachable", "unreachable", or "timeout"
- Gracefully degrades if not registered yet ("not_registered")

### Usage

**Kubernetes Probes:**
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9091
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /ready
    port: 9091
  initialDelaySeconds: 5
  periodSeconds: 10
```

**Response Format:**
```json
{
  "status": "ready",
  "checks": {
    "ffmpeg": "available",
    "disk_space": "ok: 45.2% used, 25600 MB available",
    "master": "reachable"
  },
  "timestamp": "2026-01-06T12:00:00Z"
}
```

---

## 4. Centralized Logging

### Implementation

**Files Modified:**
- `master/cmd/master/main.go` (migrated 56 log calls)
- `worker/cmd/agent/main.go` (already using logger)
- `shared/pkg/logging/logger.go` (NewFileLogger)

### Structure

```
/var/log/ffrtmp/
├── master/
│   └── master.log          # Master server logs
├── worker/
│   └── agent.log           # Worker agent logs
└── wrapper/
    └── wrapper.log         # Wrapper execution logs

# Fallback if /var/log not writable:
./logs/
├── master/master.log
├── worker/agent.log
└── wrapper/wrapper.log
```

### Features

- **Multi-writer**: Logs to both file AND stdout
- **Stdout captured by systemd**: journald integration
- **Auto-rotation**: Manual rotation with `RotateIfNeeded()`
- **Logrotate configs**: 14-day retention, daily rotation
- **Log levels**: debug, info, warn, error, fatal

### Migration Complete

- Master:  56/56 log calls migrated to logger.Info/Error/Fatal
- Worker:  Already using logger
- Wrapper:  Uses report.LogSummary (correct - not reactive)

---

## 5. Metrics & Observability

### Endpoints

#### Master (port 9090):
- `GET /metrics` - Prometheus format
- `GET /health` - Liveness probe

#### Worker (port 9091):
- `GET /metrics` - Worker Prometheus metrics
- `GET /health` - Liveness probe
- `GET /ready` - Readiness probe (enhanced)
- `GET /wrapper/metrics` - Wrapper-specific Prometheus metrics
- `GET /violations` - SLA violations JSON (last 50)

### Wrapper Visibility (3 Layers)

**Layer 1: Immutable Job-Level Truth**
```go
type Result struct {
    StartTime      time.Time
    EndTime        time.Time
    ExitCode       int
    PlatformSLA    bool  // wrapper met its obligations
    Intent         string
}
```
Written ONCE per job, never updated.

**Layer 2: Boring Counters Only**
```go
type Metrics struct {
    JobsTotal         atomic.Uint64
    JobsSuccess       atomic.Uint64
    JobsFailed        atomic.Uint64
    PlatformSLAMet    atomic.Uint64
    PlatformSLAFailed atomic.Uint64
}
```
No histograms, no clever interpretation.

**Layer 3: Human-Readable Logs**
```go
func (r *Result) LogSummary() string {
    // For ops to grep at 03:00
}
```

**Killer Feature: Violation Sampling**
- Ring buffer of last 50 SLA violations
- Accessed via `/violations` endpoint
- Newest first, for debugging

---

## Testing

### Validation Scripts

**1. Production Readiness Test**
```bash
scripts/test_production_readiness.sh
```
Validates:
- Retry logic integration (transport only)
- Graceful shutdown (master + worker)
- Enhanced readiness checks
- Logging migration
- No retries on workload execution

**2. Metrics Endpoints Test**
```bash
scripts/test_metrics_endpoints.sh
```
Tests:
- Master /metrics, /health
- Worker /metrics, /health, /ready
- Wrapper /wrapper/metrics, /violations

**3. End-to-End Test Suite**
```bash
scripts/test_all_end_to_end.sh
```
Comprehensive 34-test suite covering all wrapper phases.

### Manual Testing

**Test Graceful Shutdown:**
```bash
# Start master
./bin/master --tls=false --port=8080 --db=""

# In another terminal, send SIGTERM
kill -TERM $(pgrep master)

# Verify logs show clean shutdown
tail -f logs/master/master.log
```

**Test Readiness Checks:**
```bash
# Start worker
./bin/agent --metrics-port=9091

# Check readiness
curl http://localhost:9091/ready | jq
```

---

## Architecture Principles

### Swedish Principles Applied

1. **Boring on Purpose**
   - No clever retries
   - No fancy backoff algorithms beyond exponential
   - No heroics

2. **Correctness Over Features**
   - Retries only where safe (messages)
   - Shutdown doesn't kill workloads
   - Logging is simple file writes

3. **Non-Reactive Visibility**
   - Metrics derive from immutable truth
   - Wrapper never reacts to metrics
   - Flow: Workload → OS → Wrapper observes → Metrics → Humans look

4. **Governance, Not Management**
   - We decide when to start/stop accepting work
   - OS decides when workloads run
   - Wrapper records what happened

### Error Handling Philosophy

**Critical Scope Limits:**

```
 Retry:
  - HTTP requests to master
  - Heartbeat delivery
  - Job polling
  - Result reporting

 NO Retry:
  - Job execution
  - Wrapper run/attach
  - FFmpeg execution
  - Workload failures
```

If the operation changes system state beyond just "sending a message", **NO RETRIES**.

---

## Deployment

### Systemd Integration

**Service Files:**
- `deployment/systemd/ffrtmp-master.service`
- `deployment/systemd/ffrtmp-worker.service`

**Critical Settings:**
```ini
# Worker service MUST have:
Delegate=yes              # For cgroup management
KillMode=process          # Don't kill workloads
```

**Graceful Shutdown:**
```ini
TimeoutStopSec=60         # 60 seconds for clean shutdown
Restart=on-failure        # Auto-restart on crash
```

### Logrotate

**Configs:**
- `deployment/logrotate/ffrtmp-master`
- `deployment/logrotate/ffrtmp-worker`
- `deployment/logrotate/ffrtmp-wrapper`

**Settings:**
- 14-day retention
- Daily rotation
- Compress old logs
- Create new log with correct permissions

---

## Monitoring

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'ffrtmp-master'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s

  - job_name: 'ffrtmp-workers'
    static_configs:
      - targets: ['worker1:9091', 'worker2:9091']
    scrape_interval: 15s
```

### Key Metrics to Alert On

**Master:**
- `ffrtmp_jobs_pending` > 100 (backlog building)
- `ffrtmp_nodes_offline` > 0 (worker failure)

**Worker:**
- `worker_heartbeat_failures` > 3 (connectivity issues)
- `worker_disk_usage_percent` > 90 (disk space low)
- `wrapper_platform_sla_failed` > 10 (wrapper problems)

### SLA Violation Investigation

1. Check `/violations` endpoint for recent failures
2. Look for patterns in violation timestamps
3. Grep logs for job IDs in violations
4. Check if workload failed vs platform failed

---

## Production Checklist

Before deploying to production:

- [ ] Review retry config (max retries, backoff)
- [ ] Test graceful shutdown under load
- [ ] Verify log rotation is working
- [ ] Set up Prometheus scraping
- [ ] Configure alerts for SLA violations
- [ ] Test readiness probes with Kubernetes
- [ ] Verify disk space monitoring
- [ ] Check FFmpeg availability on all workers
- [ ] Test master reachability checks
- [ ] Review shutdown timeout (30s sufficient?)

---

## Summary

All production readiness features are implemented following strict architectural principles:

 **Retry Logic**: Messages only, never work  
 **Graceful Shutdown**: Let jobs finish, no killing  
 **Readiness Checks**: FFmpeg, disk, master connectivity  
 **Centralized Logging**: File + stdout, /var/log/ffrtmp  
 **Metrics Endpoints**: Prometheus + health/ready probes  
 **No Broken Principles**: Retries scoped correctly, shutdown clean  

**Total Changes:**
- 4 files modified (client.go, shutdown.go, master/main.go, worker/main.go)
- 317 insertions, 151 deletions
- 3 commits
- 2 test scripts
- 100% backward compatible

**Architecture Preserved:**
- Wrapper still doesn't react to metrics
- OS still owns workload lifecycle
- Job execution has NO retries
- Visibility remains derived, not driving

---

## References

- [LOGGING.md](LOGGING.md) - Centralized logging architecture
- [WRAPPER_VISIBILITY.md](WRAPPER_VISIBILITY.md) - 3-layer visibility
- [WRAPPER_INTEGRATION.md](WRAPPER_INTEGRATION.md) - Worker integration
- [WRAPPER_REPLICATION_GUIDE.md](WRAPPER_REPLICATION_GUIDE.md) - Complete implementation guide

**Test Coverage:**
- `scripts/test_production_readiness.sh` - Feature validation
- `scripts/test_metrics_endpoints.sh` - Endpoint testing
- `scripts/test_all_end_to_end.sh` - 34 comprehensive tests
