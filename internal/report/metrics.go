package report

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import "sync/atomic"

// Metrics are counters only. No interpretation.
type Metrics struct {
	JobsStarted   atomic.Uint64
	JobsCompleted atomic.Uint64
	JobsFailed    atomic.Uint64
	JobsAttached  atomic.Uint64
}

var globalMetrics = &Metrics{}

// Global returns global metrics instance
func Global() *Metrics {
	return globalMetrics
}

// IncrStarted increments jobs started counter
func (m *Metrics) IncrStarted() {
	m.JobsStarted.Add(1)
}

// IncrCompleted increments jobs completed counter
func (m *Metrics) IncrCompleted() {
	m.JobsCompleted.Add(1)
}

// IncrFailed increments jobs failed counter
func (m *Metrics) IncrFailed() {
	m.JobsFailed.Add(1)
}

// IncrAttached increments jobs attached counter
func (m *Metrics) IncrAttached() {
	m.JobsAttached.Add(1)
}
