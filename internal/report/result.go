package report

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"fmt"
	"log"
	"time"
)

// Result is immutable job-level truth. Set once, never change.
// This is Layer 1 visibility: the source of truth for all metrics and logs.
type Result struct {
	// Identity
	JobID string `json:"job_id"`
	PID   int    `json:"pid"`
	Mode  string `json:"mode"` // "run" or "attach"

	// Timing (immutable)
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"processing_seconds"`

	// Outcome (immutable)
	ExitCode int `json:"exit_code"`

	// Platform SLA (immutable, set once)
	PlatformSLA       bool   `json:"platform_sla_compliant"`
	PlatformSLAReason string `json:"platform_sla_reason"`

	// Intent (optional, for filtering production vs test)
	Intent string `json:"intent,omitempty"` // "production", "test", etc.
}

// NewResult creates an immutable result
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

// SetPlatformSLA sets SLA status. Call this ONCE at completion.
// Never update, never recompute.
func (r *Result) SetPlatformSLA(compliant bool, reason string) {
	r.PlatformSLA = compliant
	r.PlatformSLAReason = reason
}

// SetIntent sets the job intent (production, test, etc.)
// Optional field for filtering in metrics.
func (r *Result) SetIntent(intent string) {
	r.Intent = intent
}

// LogSummary emits Layer 3 visibility: human-readable one-line summary
// This is what ops grep for at 03:00
func (r *Result) LogSummary() {
	slaStatus := "COMPLIANT"
	if !r.PlatformSLA {
		slaStatus = "VIOLATION"
	}

	intentStr := ""
	if r.Intent != "" {
		intentStr = fmt.Sprintf("intent=%s | ", r.Intent)
	}

	log.Printf("JOB %s | %ssla=%s | reason=%s | runtime=%.0fs | exit=%d | pid=%d",
		r.JobID,
		intentStr,
		slaStatus,
		r.PlatformSLAReason,
		r.Duration.Seconds(),
		r.ExitCode,
		r.PID,
	)
}
