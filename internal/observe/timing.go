package observe

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import "time"

// Timing records start/end timestamps only
type Timing struct {
	StartedAt  time.Time
	CompletedAt time.Time
}

// New creates timing with current start time
func NewTiming() *Timing {
	return &Timing{
		StartedAt: time.Now(),
	}
}

// Complete records completion time
func (t *Timing) Complete() {
	t.CompletedAt = time.Now()
}

// Duration returns execution duration
func (t *Timing) Duration() time.Duration {
	if t.CompletedAt.IsZero() {
		return time.Since(t.StartedAt)
	}
	return t.CompletedAt.Sub(t.StartedAt)
}
