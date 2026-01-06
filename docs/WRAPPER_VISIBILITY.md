# Wrapper Visibility: Boring and Correct

## First Principle: Visibility is Derived, Not Driving

The wrapper **never** makes decisions based on dashboards, metrics, or logs.

```
✅ Correct flow:
Workload → OS → Wrapper observes → Metrics/logs emitted → Humans look

❌ Wrong flow:
Metrics → Wrapper reacts → Workload behavior changes
```

**The wrapper must be completely non-reactive.**

## Three Visibility Layers (Minimal)

### 🟢 Layer 1: Job-Level Truth (Immutable)

This is the **source of truth**. Every job produces a final, frozen result:

```json
{
  "job_id": "transcode-001",
  "intent": "production",
  "platform_sla_compliant": true,
  "platform_sla_reason": "completed_within_limits",
  "processing_seconds": 87.5,
  "exit_code": 0,
  "start_time": "2026-01-06T10:00:00Z",
  "end_time": "2026-01-06T10:01:27Z",
  "pid": 12345,
  "mode": "run"
}
```

**Rules:**
- Written **once**
- Never updated
- Never inferred later
- Never recomputed

This is implemented in `internal/report/result.go`:

```go
type Result struct {
	// Identity
	JobID string
	PID   int
	Mode  string // "run" or "attach"

	// Timing (immutable)
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	// Outcome (immutable)
	ExitCode int

	// Platform SLA (immutable, set once)
	PlatformSLA       bool
	PlatformSLAReason string

	// Intent (optional)
	Intent string // "production", "test", etc.
}
```

**Every metric and log must be explainable by looking at a single Result.**

### 🟡 Layer 2: Counters & Ratios (Prometheus-Friendly)

Expose **boring counters only**. No histograms, no percentiles (yet).

```prometheus
# Platform SLA (most important)
ffrtmp_platform_sla_total{compliant="true"} 9847
ffrtmp_platform_sla_total{compliant="false"} 23

# Job lifecycle
ffrtmp_jobs_total{state="started"} 9870
ffrtmp_jobs_total{state="completed"} 9870

# Execution mode
ffrtmp_jobs_by_mode_total{mode="run"} 9500
ffrtmp_jobs_by_mode_total{mode="attach"} 370

# Exit codes
ffrtmp_jobs_by_exit_total{exit="0"} 9850
ffrtmp_jobs_by_exit_total{exit="non_zero"} 20
```

**Derived metrics (optional):**

```prometheus
# Platform SLA rate (0-1)
ffrtmp_platform_sla_rate 0.997668
```

**Implementation:** All counters are updated from a single `Result`:

```go
// internal/report/metrics.go
func (m *Metrics) RecordResult(r *Result) {
	m.JobsCompleted.Add(1)
	
	if r.PlatformSLA {
		m.JobsPlatformCompliant.Add(1)
	} else {
		m.JobsPlatformViolation.Add(1)
	}
	
	// ... all other counters updated from this single result
}
```

**Why this matters:** Counters are hard to lie with. No interpretation, just facts.

### 🔵 Layer 3: Human-Readable Logs (Ops Trust)

Every job logs **one clear summary line**:

```
JOB transcode-001 | intent=production | sla=COMPLIANT | reason=completed_within_limits | runtime=87s | exit=0 | pid=12345
```

Or for violations:

```
JOB transcode-002 | intent=production | sla=VIOLATION | reason=platform_resource_error | runtime=12s | exit=1 | pid=12346
```

**Why this matters:**
- Ops can `grep` at 03:00
- No dashboards needed
- No PromQL archaeology
- **If logs make sense, dashboards become optional**

**Implementation:**

```go
// internal/report/result.go
func (r *Result) LogSummary() {
	slaStatus := "COMPLIANT"
	if !r.PlatformSLA {
		slaStatus = "VIOLATION"
	}
	
	log.Printf("JOB %s | intent=%s | sla=%s | reason=%s | runtime=%.0fs | exit=%d | pid=%d",
		r.JobID, r.Intent, slaStatus, r.PlatformSLAReason,
		r.Duration.Seconds(), r.ExitCode, r.PID)
}
```

## One Killer Feature (Cheap, High Value)

### SLA Violation Sampling

When `platform_sla_compliant=false`, store last N violations (e.g. 50):

```json
GET /violations

[
  {"job_id":"job-123","reason":"platform_resource_error","duration":12.5,"exit_code":1,"pid":12346},
  {"job_id":"job-124","reason":"cgroup_creation_failed","duration":0.1,"exit_code":-1,"pid":12347},
  ...
]
```

**This gives you:**
- Instant root cause clues
- No log diving
- No metrics spelunking

**Far more useful than 20 dashboards.**

Implementation in `internal/report/violations.go`:

```go
type ViolationLog struct {
	samples []ViolationSample
	maxSize int  // Ring buffer size (50)
}

func (v *ViolationLog) Record(r *Result) {
	if r.PlatformSLA {
		return  // Only record violations
	}
	// Add to ring buffer...
}
```

## What NOT to Do (Very Important)

❌ **Do not add:**
- Adaptive behavior
- Auto-mitigation
- Feedback loops
- Retries based on metrics
- "if SLA drops, do X"

**That turns visibility into control. And control breaks trust.**

## Dashboards: Keep Them Boring

If you build dashboards, answer only these questions:

### ✅ "Is the platform behaving?"
- Lifetime Platform SLA
- Platform SLA violations (count, not %)
- Success rate

### ✅ "Is it getting worse?"
- SLA slope (flat/up/down)
- New platform violations over time

### ❌ Not needed
- Per-engine flame graphs
- Predictive curves
- ML insights
- "Smart" alerts

**If a graph makes you feel clever, delete it.**

## Mental Model

**Visibility exists to convince humans, not machines.**

Machines already behave correctly.  
Humans need proof.

Your system already has the hardest part:
- ✅ Deterministic behavior
- ✅ Stable SLA
- ✅ Honest accounting

Visibility just needs to show that plainly.

## One-Sentence Design Rule

> Every metric and log must be explainable by looking at a single job record.

If something violates that → it's too clever.

## Implementation Files

```
internal/report/
├── result.go       # Layer 1: Immutable job truth
├── metrics.go      # Layer 2: Boring counters
├── violations.go   # Killer feature: Violation sampling
└── export.go       # Prometheus + JSON export
```

## Usage Example

```go
// In wrapper Run() or Attach()
startTime := time.Now()
// ... run workload ...
endTime := time.Now()

// Create immutable result (Layer 1)
result := report.NewResult(jobID, pid, exitCode, startTime, endTime, "run")
result.SetPlatformSLA(true, "completed_within_limits")
result.SetIntent("production")

// Update all metrics from this single result (Layer 2)
report.Global().RecordResult(result)

// Record violation if needed (killer feature)
report.GlobalViolations().Record(result)

// Human-readable summary (Layer 3)
result.LogSummary()
```

**Output:**
```
JOB transcode-001 | intent=production | sla=COMPLIANT | reason=completed_within_limits | runtime=87s | exit=0 | pid=12345
```

## Accessing Visibility

### Prometheus Metrics

```go
import "github.com/psantana5/ffmpeg-rtmp/internal/report"

http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; version=0.0.4")
    fmt.Fprint(w, report.PrometheusExport())
})
```

### Violation Sampling

```go
http.HandleFunc("/violations", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprint(w, report.ViolationsJSON())
})
```

### Example Dashboard Queries

```promql
# Platform SLA rate (lifetime)
rate(ffrtmp_platform_sla_total{compliant="true"}[5m]) 
/ 
rate(ffrtmp_jobs_total{state="completed"}[5m])

# New violations per hour
increase(ffrtmp_platform_sla_total{compliant="false"}[1h])

# Success rate (exit code 0)
rate(ffrtmp_jobs_by_exit_total{exit="0"}[5m])
/
rate(ffrtmp_jobs_total{state="completed"}[5m])
```

## Testing

```bash
./scripts/test_wrapper_visibility.sh
```

Verifies:
- ✅ Layer 1: Immutable Result with all fields
- ✅ Layer 2: Counters only (no histograms)
- ✅ Layer 3: LogSummary called
- ✅ Violation sampling implemented
- ✅ No reactive behavior in wrapper
- ✅ All 10 tests passing

## Bottom Line

You don't need more visibility.  
You need the **right** visibility:

1. **Immutable job truth** (Layer 1)
2. **Boring counters** (Layer 2)
3. **Readable logs** (Layer 3)
4. **Zero feedback loops**

That's how you keep the wrapper governance-only and still earn deep trust.
