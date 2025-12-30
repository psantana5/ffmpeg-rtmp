package models

import (
	"time"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusQueued     JobStatus = "queued"
	JobStatusAssigned   JobStatus = "assigned"
	JobStatusProcessing JobStatus = "processing"
	JobStatusRunning    JobStatus = "running"
	JobStatusPaused     JobStatus = "paused"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCanceled   JobStatus = "canceled"
)

// Job represents a workload to be executed on a compute node
type Job struct {
	ID               string                 `json:"id"`
	Scenario         string                 `json:"scenario"`   // e.g., "4K60-h264"
	Confidence       string                 `json:"confidence"` // "auto", "high", "medium", "low"
	Engine           string                 `json:"engine,omitempty"`      // "auto", "ffmpeg", "gstreamer"
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Status           JobStatus              `json:"status"`
	Queue            string                 `json:"queue,omitempty"`    // "live", "default", "batch"
	Priority         string                 `json:"priority,omitempty"` // "high", "medium", "low"
	Progress         int                    `json:"progress,omitempty"` // 0-100%
	NodeID           string                 `json:"node_id,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	StartedAt        *time.Time             `json:"started_at,omitempty"`
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
	RetryCount       int                    `json:"retry_count"`
	Error            string                 `json:"error,omitempty"`
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
