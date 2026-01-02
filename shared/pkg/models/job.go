package models

import (
	"time"
)

// JobStatus represents the status of a job
type JobStatus string

// Legacy states (for backward compatibility)
const (
	JobStatusPending    JobStatus = "pending"    // Legacy: maps to QUEUED
	JobStatusProcessing JobStatus = "processing" // Legacy: maps to RUNNING
	JobStatusPaused     JobStatus = "paused"     // Legacy: maps to ASSIGNED
)

// Note: Current FSM states defined in fsm.go:
// - JobStatusQueued, JobStatusAssigned, JobStatusRunning
// - JobStatusCompleted, JobStatusFailed, JobStatusTimedOut
// - JobStatusRetrying, JobStatusCanceled, JobStatusRejected

// FailureReason represents the reason for job failure
type FailureReason string

const (
	FailureReasonCapabilityMismatch FailureReason = "capability_mismatch" // Missing GPU/encoder/engine
	FailureReasonRuntimeError       FailureReason = "runtime_error"       // Execution error
	FailureReasonTimeout            FailureReason = "timeout"             // Job exceeded timeout
	FailureReasonUserError          FailureReason = "user_error"          // Invalid parameters/config
)

// Job represents a workload to be executed on a compute node
type Job struct {
	ID               string                 `json:"id"`
	SequenceNumber   int                    `json:"sequence_number,omitempty"`   // Human-friendly job number
	TenantID         string                 `json:"tenant_id,omitempty"`         // Tenant/organization ID
	Scenario         string                 `json:"scenario"`   // e.g., "4K60-h264"
	Confidence       string                 `json:"confidence"` // "auto", "high", "medium", "low"
	Engine           string                 `json:"engine,omitempty"`      // "auto", "ffmpeg", "gstreamer"
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Status           JobStatus              `json:"status"`
	Queue            string                 `json:"queue,omitempty"`    // "live", "default", "batch"
	Priority         string                 `json:"priority,omitempty"` // "high", "medium", "low"
	Progress         int                    `json:"progress,omitempty"` // 0-100%
	NodeID           string                 `json:"node_id,omitempty"`
	NodeName         string                 `json:"node_name,omitempty"`        // Human-friendly node name (not stored, populated on read)
	CreatedAt        time.Time              `json:"created_at"`
	StartedAt        *time.Time             `json:"started_at,omitempty"`
	LastActivityAt   *time.Time             `json:"last_activity_at,omitempty"` // Tracks last heartbeat/progress update
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
	RetryCount       int                    `json:"retry_count"`
	MaxRetries       int                    `json:"max_retries,omitempty"`       // Max retry attempts (default: 3)
	RetryReason      string                 `json:"retry_reason,omitempty"`      // Reason for current retry
	Error            string                 `json:"error,omitempty"`
	FailureReason    FailureReason          `json:"failure_reason,omitempty"`    // Explicit failure classification
	Logs             string                 `json:"logs,omitempty"`              // Worker execution logs
	TimeoutAt        *time.Time             `json:"timeout_at,omitempty"`        // Calculated timeout deadline
	StateTransitions []StateTransition      `json:"state_transitions,omitempty"`
}

// JobRequest represents a request to create a new job
type JobRequest struct {
	Scenario   string                 `json:"scenario"`
	Confidence string                 `json:"confidence,omitempty"`
	Engine     string                 `json:"engine,omitempty"`   // "auto", "ffmpeg", "gstreamer"
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Queue      string                 `json:"queue,omitempty"`    // "live", "default", "batch"
	Priority   string                 `json:"priority,omitempty"` // "high", "medium", "low"
}

// JobResult represents the result of a completed job
type JobResult struct {
	JobID            string                 `json:"job_id"`
	NodeID           string                 `json:"node_id"`
	Status           JobStatus              `json:"status"`
	Progress         int                    `json:"progress,omitempty"` // Final progress 0-100%
	Metrics          map[string]interface{} `json:"metrics,omitempty"`
	AnalyzerOutput   map[string]interface{} `json:"analyzer_output,omitempty"`
	Error            string                 `json:"error,omitempty"`
	Logs             string                 `json:"logs,omitempty"` // Worker execution logs
	CompletedAt      time.Time              `json:"completed_at"`
	QoEScore         float64                `json:"qoe_score,omitempty"`
	EfficiencyScore  float64                `json:"efficiency_score,omitempty"`
	EnergyJoules     float64                `json:"energy_joules,omitempty"`
	VMAFScore        float64                `json:"vmaf_score,omitempty"`
}

// StateTransition tracks job state changes with timestamps
type StateTransition struct {
	From      JobStatus `json:"from"`
	To        JobStatus `json:"to"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}
