# Wrapper Replication Guide

## Complete step-by-step guide to replicate the wrapper implementation from scratch

This document provides complete replication instructions for implementing the production-grade edge workload wrapper with Swedish architectural principles.

---

## Table of Contents

1. [Environment Prerequisites](#environment-prerequisites)
2. [Phase 1: Core Wrapper Architecture](#phase-1-core-wrapper-architecture)
3. [Phase 2: Worker Agent Integration](#phase-2-worker-agent-integration)
4. [Phase 3: Edge Deployment Infrastructure](#phase-3-edge-deployment-infrastructure)
5. [Phase 4: Minimal Visibility](#phase-4-minimal-visibility)
6. [Validation & Testing](#validation--testing)
7. [Production Deployment](#production-deployment)

---

## Environment Prerequisites

### Required Tools

```bash
# Go 1.21+ (required)
go version

# Git (for version control)
git --version

# Build tools
make --version

# Optional but recommended
docker --version
systemctl --version
```

### System Requirements

- Linux kernel 4.5+ (for cgroup v2 support)
- Root or sudo access (for cgroup delegation)
- FFmpeg or GStreamer (for actual workloads)

### Project Structure

```bash
ffmpeg-rtmp/
├── cmd/
│   └── ffrtmp/
│       ├── cmd/          # CLI commands
│       └── main.go
├── internal/
│   ├── wrapper/          # Core wrapper logic
│   ├── cgroups/          # Resource management
│   ├── observe/          # Process observation
│   └── report/           # Immutable results & metrics
├── worker/
│   └── cmd/agent/        # Worker agent
├── shared/pkg/
│   ├── models/           # Job models
│   └── agent/            # Integration helpers
├── deployment/
│   └── systemd/          # Service files
├── docs/                 # Documentation
└── scripts/              # Test scripts
```

---

## Phase 1: Core Wrapper Architecture

**Goal:** Implement minimalist, crash-safe wrapper (14.5 KB)

### Step 1.1: Create Internal Structure

```bash
mkdir -p internal/{wrapper,cgroups,observe,report}
```

### Step 1.2: Implement Cgroup Manager

**File:** `internal/cgroups/manager.go`

```go
package cgroups

// Golden rules as comments in EVERY file:
// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type Manager struct {
    version int // 1 or 2
}

func New() *Manager {
    // Detect cgroup version
    if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
        return &Manager{version: 2}
    }
    return &Manager{version: 1}
}

func (m *Manager) Create(jobID string) (string, error) {
    if m.version == 2 {
        path := filepath.Join("/sys/fs/cgroup/ffrtmp", jobID)
        return path, os.MkdirAll(path, 0755)
    }
    // v1: create in cpu subsystem
    path := filepath.Join("/sys/fs/cgroup/cpu/ffrtmp", jobID)
    return path, os.MkdirAll(path, 0755)
}

func (m *Manager) Join(cgroupPath string, pid int) error {
    procsFile := filepath.Join(cgroupPath, "cgroup.procs")
    return os.WriteFile(procsFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func (m *Manager) Delete(cgroupPath string) error {
    return os.RemoveAll(cgroupPath)
}
```

**Key principles:**
- Detect cgroup v1/v2 automatically
- Create/Join/Delete ONLY (no policy)
- Graceful degradation if cgroups unavailable

### Step 1.3: Implement Resource Limits

**File:** `internal/cgroups/limits.go`

```go
package cgroups

import (
    "fmt"
    "os"
    "path/filepath"
)

type Limits struct {
    CPUMax    string // "quota period" format
    CPUWeight int    // 1-10000
    MemoryMax int64  // bytes
    IOMax     string // "major:minor rbps=X wbps=Y"
}

func (m *Manager) ApplyLimits(cgroupPath string, limits *Limits) error {
    if limits == nil {
        return nil
    }
    
    if m.version == 2 {
        // v2: cpu.max, cpu.weight, memory.max
        if limits.CPUMax != "" {
            WriteCPUMax(cgroupPath, limits.CPUMax)
        }
        if limits.CPUWeight > 0 {
            WriteCPUWeight(cgroupPath, limits.CPUWeight)
        }
        if limits.MemoryMax > 0 {
            WriteMemoryMax(cgroupPath, limits.MemoryMax)
        }
    } else {
        // v1: cpu.shares, memory.limit_in_bytes
        // Simplified for v1
    }
    
    return nil
}

func WriteCPUMax(cgroupPath, value string) error {
    file := filepath.Join(cgroupPath, "cpu.max")
    return os.WriteFile(file, []byte(value), 0644)
}

// ... similar for WriteCPUWeight, WriteMemoryMax
```

**Key principles:**
- Write ONLY: cpu.max, cpu.weight, memory.max, io.max
- NO kernel params, sysctl, or "smart defaults"
- Best effort (ignore write errors)

### Step 1.4: Implement Process Observer

**File:** `internal/observe/watcher.go`

```go
package observe

import (
    "os"
    "time"
)

type Watcher struct {
    pid int
}

func New(pid int) *Watcher {
    return &Watcher{pid: pid}
}

func (w *Watcher) Wait() {
    // Poll until process exits (passive observation)
    for {
        if !pidExists(w.pid) {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
}

func pidExists(pid int) bool {
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    err = process.Signal(os.Signal(syscall.Signal(0)))
    return err == nil
}
```

**File:** `internal/observe/timing.go`

```go
package observe

import "time"

type Timing struct {
    Start time.Time
    End   time.Time
}

func NewTiming() *Timing {
    return &Timing{Start: time.Now()}
}

func (t *Timing) Complete() {
    t.End = time.Now()
}

func (t *Timing) Duration() time.Duration {
    return t.End.Sub(t.Start)
}
```

**Key principles:**
- Passive observation ONLY
- No signals, no intervention
- PID watching only

### Step 1.5: Implement Immutable Result

**File:** `internal/report/result.go`

```go
package report

import "time"

type Result struct {
    JobID     string
    PID       int
    Mode      string // "run" or "attach"
    
    StartTime time.Time
    EndTime   time.Time
    Duration  time.Duration
    
    ExitCode int
    
    PlatformSLA       bool
    PlatformSLAReason string
    
    Intent string // "production", "test"
}

func NewResult(jobID string, pid int, exitCode int, startTime, endTime time.Time, mode string) *Result {
    return &Result{
        JobID:     jobID,
        PID:       pid,
        ExitCode:  exitCode,
        StartTime: startTime,
        EndTime:   endTime,
        Duration:  endTime.Sub(startTime),
        Mode:      mode,
    }
}

func (r *Result) SetPlatformSLA(compliant bool, reason string) {
    r.PlatformSLA = compliant
    r.PlatformSLAReason = reason
}

func (r *Result) LogSummary() {
    slaStatus := "COMPLIANT"
    if !r.PlatformSLA {
        slaStatus = "VIOLATION"
    }
    
    log.Printf("JOB %s | sla=%s | reason=%s | runtime=%.0fs | exit=%d | pid=%d",
        r.JobID, slaStatus, r.PlatformSLAReason,
        r.Duration.Seconds(), r.ExitCode, r.PID)
}
```

**Key principles:**
- Written ONCE
- Never updated
- Never recomputed
- Source of truth for ALL metrics

### Step 1.6: Implement Run Mode

**File:** `internal/wrapper/run.go`

```go
package wrapper

import (
    "context"
    "os/exec"
    "syscall"
    "time"
    
    "github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
    "github.com/psantana5/ffmpeg-rtmp/internal/observe"
    "github.com/psantana5/ffmpeg-rtmp/internal/report"
)

func Run(ctx context.Context, jobID string, limits *cgroups.Limits, command string, args []string) (*report.Result, error) {
    report.Global().IncrStarted()
    timing := observe.NewTiming()
    
    // Create command
    cmd := exec.CommandContext(ctx, command, args...)
    
    // CRITICAL: Process group isolation (setpgid)
    // If wrapper crashes → workload continues
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
        Pgid:    0,
    }
    
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    // Start process
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start: %w", err)
    }
    
    pid := cmd.Process.Pid
    
    // Apply cgroup limits (best effort)
    cgroupPath := applyLimits(jobID, pid, limits)
    defer func() {
        if cgroupPath != "" {
            mgr := cgroups.New()
            mgr.Delete(cgroupPath)
        }
    }()
    
    // Wait for completion
    startTime := timing.Start
    err := cmd.Wait()
    endTime := time.Now()
    
    exitCode := 0
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        }
    }
    
    // Create immutable result
    result := report.NewResult(jobID, pid, exitCode, startTime, endTime, "run")
    calculatePlatformSLA(result, exitCode)
    
    // Record all visibility layers
    report.Global().RecordResult(result)
    report.GlobalViolations().Record(result)
    result.LogSummary()
    
    return result, nil
}

func calculatePlatformSLA(result *report.Result, exitCode int) {
    // Platform SLA = true if wrapper did its job
    // Workload failure (exitCode != 0) is NOT a platform violation
    if exitCode == 0 {
        result.SetPlatformSLA(true, "success")
    } else {
        result.SetPlatformSLA(true, "workload_failed_platform_ok")
    }
}
```

**Key principles:**
- `Setpgid: true` → workload independence
- Cgroup cleanup on exit (defer)
- Platform SLA tracks wrapper, not workload
- Record all visibility layers

### Step 1.7: Implement Attach Mode

**File:** `internal/wrapper/attach.go`

```go
package wrapper

func Attach(ctx context.Context, jobID string, pid int, limits *cgroups.Limits) (*report.Result, error) {
    report.Global().IncrStarted()
    
    // Validate PID exists
    if !pidExists(pid) {
        return nil, fmt.Errorf("process %d does not exist", pid)
    }
    
    timing := observe.NewTiming()
    
    // Apply limits (best effort)
    cgroupPath := applyLimits(jobID, pid, limits)
    defer func() {
        if cgroupPath != "" {
            mgr := cgroups.New()
            mgr.Delete(cgroupPath)
        }
    }()
    
    // Create watcher
    watcher := observe.New(pid)
    
    // Wait for process to exit (or context cancel)
    done := make(chan struct{})
    startTime := timing.Start
    
    go func() {
        watcher.Wait()
        close(done)
    }()
    
    select {
    case <-ctx.Done():
        // Wrapper told to stop, workload continues
        endTime := time.Now()
        result := report.NewResult(jobID, pid, -1, startTime, endTime, "attach")
        result.SetPlatformSLA(true, "detached_workload_continues")
        
        report.Global().RecordResult(result)
        report.GlobalViolations().Record(result)
        result.LogSummary()
        
        return result, ctx.Err()
        
    case <-done:
        // Process exited naturally
        endTime := time.Now()
        result := report.NewResult(jobID, pid, -1, startTime, endTime, "attach")
        result.SetPlatformSLA(true, "observed_to_completion")
        
        report.Global().RecordResult(result)
        report.GlobalViolations().Record(result)
        result.LogSummary()
        
        return result, nil
    }
}
```

**Key principles:**
- NEVER restart
- NEVER send signals
- Just observe
- Workload continues if wrapper detaches

### Step 1.8: Create CLI Commands

**File:** `cmd/ffrtmp/cmd/wrapper.go`

```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/psantana5/ffmpeg-rtmp/internal/wrapper"
    "github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
)

var runCmd = &cobra.Command{
    Use:   "run [flags] -- <command> [args...]",
    Short: "Run a workload with wrapper governance",
    RunE: func(cmd *cobra.Command, args []string) error {
        jobID, _ := cmd.Flags().GetString("job-id")
        cpuMax, _ := cmd.Flags().GetString("cpu-max")
        cpuWeight, _ := cmd.Flags().GetInt("cpu-weight")
        memoryMax, _ := cmd.Flags().GetInt64("memory-max")
        
        limits := &cgroups.Limits{
            CPUMax:    cpuMax,
            CPUWeight: cpuWeight,
            MemoryMax: memoryMax,
        }
        
        result, err := wrapper.Run(cmd.Context(), jobID, limits, args[0], args[1:])
        // ... handle result
        return err
    },
}

var attachCmd = &cobra.Command{
    Use:   "attach --pid <pid>",
    Short: "Attach to existing process",
    RunE: func(cmd *cobra.Command, args []string) error {
        jobID, _ := cmd.Flags().GetString("job-id")
        pid, _ := cmd.Flags().GetInt("pid")
        
        result, err := wrapper.Attach(cmd.Context(), jobID, pid, nil)
        // ... handle result
        return err
    },
}
```

### Step 1.9: Test Core Wrapper

```bash
# Build
go build -o /tmp/ffrtmp ./cmd/ffrtmp/

# Test run mode
/tmp/ffrtmp run --job-id test-1 -- sleep 1

# Test attach mode
sleep 60 &
BG_PID=$!
/tmp/ffrtmp attach --job-id test-2 --pid $BG_PID

# Test crash safety
/tmp/ffrtmp run --job-id test-3 -- sleep 10 &
WRAPPER_PID=$!
sleep 1
kill -9 $WRAPPER_PID
# Verify sleep process still running: ps aux | grep sleep
```

**Expected output:**
```
JOB test-1 | sla=COMPLIANT | reason=success | runtime=1s | exit=0 | pid=12345
```

---

## Phase 2: Worker Agent Integration

**Goal:** Wire wrapper into worker agent execution flow (opt-in, backward compatible)

### Step 2.1: Extend Job Model

**File:** `shared/pkg/models/job.go`

```go
type WrapperConstraints struct {
    CPUMax      string `json:"cpu_max"`
    CPUWeight   int    `json:"cpu_weight"`
    MemoryMaxMB int    `json:"memory_max_mb"`
    IOMax       string `json:"io_max"`
}

type Job struct {
    ID string `json:"id"`
    // ... existing fields ...
    
    // Wrapper integration (opt-in)
    WrapperEnabled     bool                `json:"wrapper_enabled,omitempty"`
    WrapperConstraints *WrapperConstraints `json:"wrapper_constraints,omitempty"`
    
    // Platform SLA results
    PlatformSLA       bool   `json:"platform_sla,omitempty"`
    PlatformSLAReason string `json:"platform_sla_reason,omitempty"`
}
```

### Step 2.2: Create Integration Helper

**File:** `shared/pkg/agent/wrapper_integration.go`

```go
package agent

import (
    "context"
    "github.com/psantana5/ffmpeg-rtmp/internal/wrapper"
    "github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
    "github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func ExecuteWithWrapper(ctx context.Context, job *models.Job, command string, args []string) (*report.Result, error) {
    log.Printf(" Using workload wrapper for job %s", job.ID)
    
    // Build constraints from job
    limits := buildWrapperLimits(job)
    
    // Execute with wrapper
    result, err := wrapper.Run(ctx, job.ID, limits, command, args)
    return result, err
}

func buildWrapperLimits(job *models.Job) *cgroups.Limits {
    if job.WrapperConstraints != nil {
        return &cgroups.Limits{
            CPUMax:    job.WrapperConstraints.CPUMax,
            CPUWeight: job.WrapperConstraints.CPUWeight,
            MemoryMax: int64(job.WrapperConstraints.MemoryMaxMB) * 1024 * 1024,
            IOMax:     job.WrapperConstraints.IOMax,
        }
    }
    return nil
}
```

### Step 2.3: Modify Worker Agent

**File:** `worker/cmd/agent/main.go`

Find the `executeEngineJob()` function and add wrapper routing:

```go
func executeEngineJob(job *models.Job, ...) (...) {
    // Build command
    args, err := engine.BuildCommand(job, masterURL)
    cmdPath := "/usr/bin/ffmpeg" // or gst-launch-1.0
    
    //  NEW: Check wrapper flag
    if job.WrapperEnabled {
        return executeWithWrapperPath(job, cmdPath, cmdName, args, limits, metricsExporter)
    }
    
    // Legacy path: direct exec.CommandContext
    cmd := exec.CommandContext(ctx, cmdPath, args...)
    // ... existing code ...
}

func executeWithWrapperPath(job *models.Job, cmdPath string, cmdName string, args []string, ...) (...) {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
    defer cancel()
    
    // Execute with wrapper
    result, err := agent.ExecuteWithWrapper(ctx, job, cmdPath, args)
    
    // Convert result to worker agent format
    metrics := map[string]interface{}{
        "exec_duration":   result.Duration.Seconds(),
        "wrapper_enabled": true,
        "platform_sla":    result.PlatformSLA,
        "exit_code":       result.ExitCode,
    }
    
    return metrics, analyzerOutput, logs, nil, err
}
```

**Key principles:**
- Opt-in via `WrapperEnabled` flag (default: false)
- Backward compatible
- Legacy execution preserved

### Step 2.4: Test Integration

```bash
# Build worker agent
go build -o /tmp/ffrtmp-worker ./worker/cmd/agent/

# Verify wrapper routing exists
grep -n "if job.WrapperEnabled" worker/cmd/agent/main.go

# Run integration tests
./scripts/test_wrapper_integration.sh
```

---

## Phase 3: Edge Deployment Infrastructure

**Goal:** Production-ready deployment with systemd + cgroup delegation

### Step 3.1: Create Systemd Service

**File:** `deployment/systemd/ffrtmp-worker.service`

```ini
[Unit]
Description=FFmpeg-RTMP Edge Worker with Wrapper
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ffrtmp
Group=ffrtmp
WorkingDirectory=/opt/ffrtmp

EnvironmentFile=/etc/ffrtmp/worker.env

ExecStart=/opt/ffrtmp/bin/ffrtmp-worker \
    --master-url=${MASTER_URL} \
    --worker-id=${WORKER_ID} \
    --capabilities=${CAPABILITIES}

Restart=on-failure
RestartSec=10s
KillMode=process
KillSignal=SIGTERM
TimeoutStopSec=30s

# Security
NoNewPrivileges=true
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes

# CRITICAL: Cgroup delegation for wrapper
Delegate=yes

[Install]
WantedBy=multi-user.target
```

**Key:** `Delegate=yes` allows wrapper to create sub-cgroups

### Step 3.2: Create Environment Template

**File:** `deployment/systemd/worker.env.example`

```bash
# Master server URL (REQUIRED)
MASTER_URL=https://master.example.com:8443

# Worker identification (REQUIRED)
WORKER_ID=edge-node-01

# Worker capabilities
CAPABILITIES=h264,h265,nvenc

# Maximum concurrent jobs
MAX_JOBS=4

# Wrapper mode (default: enabled)
WRAPPER_ENABLED=true
```

### Step 3.3: Create Cgroup Delegation Config

**File:** `deployment/systemd/user@.service.d-delegate.conf`

```ini
# Cgroup delegation for unprivileged wrapper
[Service]
Delegate=yes
```

**Install:**
```bash
sudo mkdir -p /etc/systemd/system/user@.service.d
sudo cp user@.service.d-delegate.conf /etc/systemd/system/user@.service.d/delegate.conf
sudo systemctl daemon-reload
```

### Step 3.4: Create Installer Script

**File:** `deployment/install-edge.sh`

```bash
#!/bin/bash
set -e

echo "Installing FFmpeg-RTMP edge worker with wrapper..."

# Create user
sudo useradd -r -s /bin/bash ffrtmp || true

# Create directories
sudo mkdir -p /opt/ffrtmp/{bin,streams,logs}
sudo chown -R ffrtmp:ffrtmp /opt/ffrtmp

# Enable cgroup delegation
sudo mkdir -p /etc/systemd/system/user@.service.d
sudo cp deployment/systemd/user@.service.d-delegate.conf \
        /etc/systemd/system/user@.service.d/delegate.conf

# Install service
sudo cp deployment/systemd/ffrtmp-worker.service \
        /etc/systemd/system/ffrtmp-worker.service

# Install config
sudo mkdir -p /etc/ffrtmp
sudo cp deployment/systemd/worker.env.example \
        /etc/ffrtmp/worker.env

# Install binary
go build -o /opt/ffrtmp/bin/ffrtmp-worker ./worker/cmd/agent/
sudo chown ffrtmp:ffrtmp /opt/ffrtmp/bin/ffrtmp-worker

# Reload systemd
sudo systemctl daemon-reload

echo "✓ Installation complete"
echo "Edit /etc/ffrtmp/worker.env and start with:"
echo "  sudo systemctl start ffrtmp-worker"
```

### Step 3.5: Deployment Documentation

**File:** `docs/WRAPPER_EDGE_DEPLOYMENT.md`

Write complete deployment guide covering:
- Fresh deployment scenario
- Existing workloads scenario (attach mode)
- Systemd setup
- Cgroup delegation
- Troubleshooting
- Security hardening

---

## Phase 4: Minimal Visibility

**Goal:** 3-layer visibility (derived, not driving)

### Step 4.1: Enhance Metrics (Layer 2)

**File:** `internal/report/metrics.go`

```go
package report

import "sync/atomic"

type Metrics struct {
    // Lifecycle
    JobsStarted   atomic.Uint64
    JobsCompleted atomic.Uint64
    
    // Platform SLA (source of truth: Result.PlatformSLA)
    JobsPlatformCompliant atomic.Uint64
    JobsPlatformViolation atomic.Uint64
    
    // Mode
    JobsRun    atomic.Uint64
    JobsAttach atomic.Uint64
    
    // Exit codes
    JobsExitZero    atomic.Uint64
    JobsExitNonZero atomic.Uint64
}

// RecordResult updates ALL counters from single immutable Result
func (m *Metrics) RecordResult(r *Result) {
    m.JobsCompleted.Add(1)
    
    if r.PlatformSLA {
        m.JobsPlatformCompliant.Add(1)
    } else {
        m.JobsPlatformViolation.Add(1)
    }
    
    if r.Mode == "run" {
        m.JobsRun.Add(1)
    } else if r.Mode == "attach" {
        m.JobsAttach.Add(1)
    }
    
    if r.ExitCode == 0 {
        m.JobsExitZero.Add(1)
    } else {
        m.JobsExitNonZero.Add(1)
    }
}
```

**Key:** All metrics from ONE immutable Result

### Step 4.2: Add Violation Sampling (Killer Feature)

**File:** `internal/report/violations.go`

```go
package report

import "sync"

type ViolationSample struct {
    JobID    string
    Reason   string
    Duration float64
    ExitCode int
    PID      int
}

type ViolationLog struct {
    samples []ViolationSample
    maxSize int
    mu      sync.RWMutex
}

func NewViolationLog(maxSize int) *ViolationLog {
    return &ViolationLog{
        samples: make([]ViolationSample, 0, maxSize),
        maxSize: maxSize,
    }
}

func (v *ViolationLog) Record(r *Result) {
    if r.PlatformSLA {
        return // Only record violations
    }
    
    sample := ViolationSample{
        JobID:    r.JobID,
        Reason:   r.PlatformSLAReason,
        Duration: r.Duration.Seconds(),
        ExitCode: r.ExitCode,
        PID:      r.PID,
    }
    
    v.mu.Lock()
    defer v.mu.Unlock()
    
    // Ring buffer: if full, drop oldest
    if len(v.samples) >= v.maxSize {
        v.samples = v.samples[1:]
    }
    v.samples = append(v.samples, sample)
}

func (v *ViolationLog) GetRecent(n int) []ViolationSample {
    // Return newest first
    // ... implementation
}
```

### Step 4.3: Add Prometheus Export

**File:** `internal/report/export.go`

```go
package report

func PrometheusExport() string {
    snapshot := Global().Snapshot()
    
    var b strings.Builder
    
    // Platform SLA (most important)
    b.WriteString("# HELP ffrtmp_platform_sla_total Platform SLA compliance\n")
    b.WriteString("# TYPE ffrtmp_platform_sla_total counter\n")
    b.WriteString(fmt.Sprintf("ffrtmp_platform_sla_total{compliant=\"true\"} %d\n", snapshot["jobs_platform_compliant"]))
    b.WriteString(fmt.Sprintf("ffrtmp_platform_sla_total{compliant=\"false\"} %d\n", snapshot["jobs_platform_violation"]))
    
    // ... other metrics
    
    return b.String()
}

func ViolationsJSON() string {
    violations := GlobalViolations().GetRecent(50)
    // Return JSON array
}
```

### Step 4.4: Visibility Documentation

**File:** `docs/WRAPPER_VISIBILITY.md`

Document:
- First principle: Visibility is derived, not driving
- Three layers explained
- Killer feature (violation sampling)
- Usage examples
- What NOT to do (no feedback loops)

---

## Validation & Testing

### Run All Test Suites

```bash
# Make test scripts executable
chmod +x scripts/*.sh

# Run all tests
./scripts/test_all_end_to_end.sh
```

This runs:
1. **Environment checks** (Go version, project structure)
2. **Core functionality** (run/attach mode, crash safety)
3. **Visibility layers** (immutable truth, counters, logs)
4. **Worker integration** (routing, backward compatibility)
5. **Deployment** (systemd files, installer)
6. **Documentation** (all guides present)

Expected output:
```
╔════════════════════════════════════════════════════════════════════════════╗
║                        END-TO-END TEST SUMMARY                             ║
╚════════════════════════════════════════════════════════════════════════════╝

Tests run:    35
Tests passed: 35
Tests failed: 0

✓ ✓ ✓ ALL TESTS PASSED ✓ ✓ ✓
```

### Individual Test Suites

```bash
# Core wrapper stability
./scripts/test_wrapper_stability.sh

# Worker agent integration
./scripts/test_wrapper_integration.sh

# Visibility layers
./scripts/test_wrapper_visibility.sh
```

### Manual Validation

```bash
# 1. Test crash safety (CRITICAL)
ffrtmp run --job-id test-crash -- sleep 30 &
WRAPPER_PID=$!
sleep 2
kill -9 $WRAPPER_PID
ps aux | grep sleep  # Should still be running ✓

# 2. Test visibility layers
ffrtmp run --job-id test-vis -- sleep 1
# Check output for: "JOB test-vis | sla=COMPLIANT | ..."

# 3. Test cgroup limits
ffrtmp run --job-id test-limits \
    --cpu-max "50000 100000" \
    --memory-max 104857600 \
    -- stress --cpu 4 --timeout 5s
# Should apply limits without crashing

# 4. Test attach mode (zero downtime)
sleep 60 &
BG_PID=$!
ffrtmp attach --job-id test-attach --pid $BG_PID
# Should observe without interrupting
```

---

## Production Deployment

### On Edge Nodes

```bash
# 1. Clone repository
git clone https://github.com/yourusername/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# 2. Run installer
sudo ./deployment/install-edge.sh

# 3. Configure
sudo vim /etc/ffrtmp/worker.env
# Set MASTER_URL, WORKER_ID, CAPABILITIES, WRAPPER_ENABLED=true

# 4. Start service
sudo systemctl start ffrtmp-worker
sudo systemctl enable ffrtmp-worker

# 5. Verify
sudo systemctl status ffrtmp-worker
journalctl -u ffrtmp-worker -f

# 6. Check cgroup delegation
cat /sys/fs/cgroup/user.slice/user-$(id -u ffrtmp).slice/cgroup.controllers
# Should show: cpuset cpu io memory pids
```

### Zero-Downtime Adoption (Existing Workloads)

```bash
# If edge node already has live streams:

# 1. Find existing ffmpeg PIDs
ps aux | grep ffmpeg

# 2. Attach wrapper to existing processes
for pid in $(pgrep ffmpeg); do
    ffrtmp attach --job-id "existing-$pid" --pid $pid &
done

# 3. New jobs will use wrapper automatically
# Existing streams continue without interruption ✓
```

### Monitoring

```bash
# View metrics
curl http://localhost:9090/metrics

# View recent violations
curl http://localhost:9090/violations

# Check logs
journalctl -u ffrtmp-worker -f | grep "JOB"
# Look for: "sla=COMPLIANT" vs "sla=VIOLATION"

# Platform SLA calculation
awk '/sla=COMPLIANT/ {c++} /sla=VIOLATION/ {v++} END {print "SLA:", c/(c+v)*100"%"}' logs
```

---

## Summary Checklist

Before marking replication complete, verify:

- [ ]  Core wrapper compiles and runs
- [ ]  Crash safety test passes (kill -9 → workload continues)
- [ ]  Run mode works with cgroup limits
- [ ]  Attach mode observes existing processes
- [ ]  Worker agent routes to wrapper when enabled
- [ ]  Legacy execution path preserved
- [ ]  Systemd service files created
- [ ]  Cgroup delegation configured
- [ ]  Three visibility layers implemented
- [ ]  Violation sampling works
- [ ]  Prometheus export available
- [ ]  No reactive behavior (visibility derived only)
- [ ]  All test suites pass (30+ tests)
- [ ]  Documentation complete (4 guides)
- [ ]  Production deployment tested

If all checkboxes pass → **replication successful** 

---

## Troubleshooting

### Build Errors

```bash
# Missing dependencies
go mod tidy
go mod download

# Module issues
go clean -modcache
go mod init github.com/yourusername/ffmpeg-rtmp
go mod tidy
```

### Cgroup Errors

```bash
# Check cgroup version
mount | grep cgroup

# Enable delegation (cgroupv2)
sudo mkdir -p /etc/systemd/system/user@.service.d
# Add Delegate=yes config

# Verify delegation
cat /sys/fs/cgroup/user.slice/user-$(id -u ffrtmp).slice/cgroup.controllers
```

### Wrapper Crashes

```bash
# Check logs
journalctl -u ffrtmp-worker -n 100

# Verify workload continues
ps aux | grep <workload-command>

# Test in isolation
ffrtmp run --job-id debug-test -- sleep 10
```

### Tests Failing

```bash
# Run with verbose output
./scripts/test_all_end_to_end.sh 2>&1 | tee test-output.log

# Check specific test
./scripts/test_wrapper_stability.sh
./scripts/test_wrapper_integration.sh
./scripts/test_wrapper_visibility.sh

# Verify Go version
go version  # Need 1.21+
```

---

## References

- Architecture: `docs/WRAPPER_MINIMALIST_ARCHITECTURE.md`
- Integration: `docs/WRAPPER_INTEGRATION.md`
- Deployment: `docs/WRAPPER_EDGE_DEPLOYMENT.md`
- Visibility: `docs/WRAPPER_VISIBILITY.md`

---

**Replication time estimate:** 4-6 hours for complete implementation + testing

**Critical success criterion:** `kill -9 wrapper` → workload continues ✓
