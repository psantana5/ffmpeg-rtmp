package report

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import "sync"

// ViolationSample stores recent platform SLA violations for debugging.
// This is cheap, high-value visibility: instant root cause without log diving.
type ViolationSample struct {
	JobID    string  `json:"job_id"`
	Reason   string  `json:"reason"`
	Duration float64 `json:"duration_seconds"`
	ExitCode int     `json:"exit_code"`
	PID      int     `json:"pid"`
}

// ViolationLog maintains a ring buffer of recent violations (last N)
type ViolationLog struct {
	samples []ViolationSample
	maxSize int
	mu      sync.RWMutex
}

var globalViolationLog = NewViolationLog(50) // Keep last 50 violations

// NewViolationLog creates a violation log with fixed size
func NewViolationLog(maxSize int) *ViolationLog {
	return &ViolationLog{
		samples: make([]ViolationSample, 0, maxSize),
		maxSize: maxSize,
	}
}

// GlobalViolations returns the global violation log
func GlobalViolations() *ViolationLog {
	return globalViolationLog
}

// Record adds a violation sample (ring buffer)
func (v *ViolationLog) Record(r *Result) {
	// Only record violations
	if r.PlatformSLA {
		return
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

// GetRecent returns recent violations (newest first)
func (v *ViolationLog) GetRecent(n int) []ViolationSample {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if n <= 0 || n > len(v.samples) {
		n = len(v.samples)
	}

	// Return newest first (reverse order)
	result := make([]ViolationSample, n)
	for i := 0; i < n; i++ {
		result[i] = v.samples[len(v.samples)-1-i]
	}
	return result
}

// Count returns total violations recorded (ring buffer size)
func (v *ViolationLog) Count() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.samples)
}
