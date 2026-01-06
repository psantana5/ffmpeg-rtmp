package report

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import "time"

// Result is immutable. Set once, never change.
type Result struct {
	JobID              string        `json:"job_id"`
	PID                int           `json:"pid"`
	ExitCode           int           `json:"exit_code"`
	Duration           time.Duration `json:"duration_sec"`
	PlatformSLA        bool          `json:"platform_sla_compliant"`
	PlatformSLAReason  string        `json:"platform_sla_reason"`
	Mode               string        `json:"mode"` // "run" or "attach"
}

// NewResult creates an immutable result
func NewResult(jobID string, pid int, exitCode int, duration time.Duration, mode string) *Result {
	return &Result{
		JobID:    jobID,
		PID:      pid,
		ExitCode: exitCode,
		Duration: duration,
		Mode:     mode,
	}
}

// SetPlatformSLA sets SLA status. Call this ONCE at completion.
func (r *Result) SetPlatformSLA(compliant bool, reason string) {
	r.PlatformSLA = compliant
	r.PlatformSLAReason = reason
}
