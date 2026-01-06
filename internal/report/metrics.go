package report

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import "sync/atomic"

// Metrics are Layer 2 visibility: boring counters only.
// No histograms, no percentiles, no interpretation.
// Every counter must be explainable by looking at a single job record.
type Metrics struct {
	// Job lifecycle
	JobsStarted   atomic.Uint64 // Incremented when Run() or Attach() called
	JobsCompleted atomic.Uint64 // Incremented when job exits (any exit code)

	// Platform SLA (source of truth: Result.PlatformSLA)
	JobsPlatformCompliant atomic.Uint64 // platform_sla_compliant=true
	JobsPlatformViolation atomic.Uint64 // platform_sla_compliant=false

	// Mode (source of truth: Result.Mode)
	JobsRun    atomic.Uint64 // mode=run
	JobsAttach atomic.Uint64 // mode=attach

	// Exit codes (source of truth: Result.ExitCode)
	JobsExitZero    atomic.Uint64 // exit_code=0
	JobsExitNonZero atomic.Uint64 // exit_code!=0
}

var globalMetrics = &Metrics{}

// Global returns global metrics instance
func Global() *Metrics {
	return globalMetrics
}

// RecordResult updates all counters from a single immutable Result.
// This is the ONLY way to update metrics - from frozen job truth.
func (m *Metrics) RecordResult(r *Result) {
	// Lifecycle
	m.JobsCompleted.Add(1)

	// Platform SLA
	if r.PlatformSLA {
		m.JobsPlatformCompliant.Add(1)
	} else {
		m.JobsPlatformViolation.Add(1)
	}

	// Mode
	if r.Mode == "run" {
		m.JobsRun.Add(1)
	} else if r.Mode == "attach" {
		m.JobsAttach.Add(1)
	}

	// Exit codes
	if r.ExitCode == 0 {
		m.JobsExitZero.Add(1)
	} else {
		m.JobsExitNonZero.Add(1)
	}
}

// IncrStarted increments jobs started counter
func (m *Metrics) IncrStarted() {
	m.JobsStarted.Add(1)
}

// Snapshot returns current counter values (for Prometheus export)
// These are just projections of job-level truth.
func (m *Metrics) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"jobs_started":            m.JobsStarted.Load(),
		"jobs_completed":          m.JobsCompleted.Load(),
		"jobs_platform_compliant": m.JobsPlatformCompliant.Load(),
		"jobs_platform_violation": m.JobsPlatformViolation.Load(),
		"jobs_run":                m.JobsRun.Load(),
		"jobs_attach":             m.JobsAttach.Load(),
		"jobs_exit_zero":          m.JobsExitZero.Load(),
		"jobs_exit_non_zero":      m.JobsExitNonZero.Load(),
	}
}
