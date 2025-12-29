package models

import (
	"time"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Job represents a workload to be executed on a compute node
type Job struct {
	ID            string                 `json:"id"`
	Scenario      string                 `json:"scenario"`       // e.g., "4K60-h264"
	Confidence    string                 `json:"confidence"`     // "auto", "high", "medium", "low"
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	Status        JobStatus              `json:"status"`
	NodeID        string                 `json:"node_id,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	StartedAt     *time.Time             `json:"started_at,omitempty"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	RetryCount    int                    `json:"retry_count"`
	Error         string                 `json:"error,omitempty"`
}

// JobRequest represents a request to create a new job
type JobRequest struct {
	Scenario   string                 `json:"scenario"`
	Confidence string                 `json:"confidence,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// JobResult represents the result of a completed job
type JobResult struct {
	JobID            string                 `json:"job_id"`
	NodeID           string                 `json:"node_id"`
	Status           JobStatus              `json:"status"`
	Metrics          map[string]interface{} `json:"metrics,omitempty"`
	AnalyzerOutput   map[string]interface{} `json:"analyzer_output,omitempty"`
	Error            string                 `json:"error,omitempty"`
	CompletedAt      time.Time              `json:"completed_at"`
}
