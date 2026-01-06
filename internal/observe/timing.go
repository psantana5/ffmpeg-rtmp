package observe

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import "time"

// Timing records start/end timestamps only
type Timing struct {
	Start time.Time // Exported for immutable result creation
	End   time.Time // Exported for immutable result creation
}

// New creates timing with current start time
func NewTiming() *Timing {
	return &Timing{
		Start: time.Now(),
	}
}

// Complete records completion time
func (t *Timing) Complete() {
	t.End = time.Now()
}

// Duration returns execution duration
func (t *Timing) Duration() time.Duration {
	if t.End.IsZero() {
		return time.Since(t.Start)
	}
	return t.End.Sub(t.Start)
}
