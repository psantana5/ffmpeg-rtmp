package models

import (
	"fmt"
	"time"
)

// Strict Job States for FSM
const (
	JobStatusQueued    JobStatus = "queued"     // Job is in queue, not yet assigned
	JobStatusAssigned  JobStatus = "assigned"   // Job assigned to worker, not yet running
	JobStatusRunning   JobStatus = "running"    // Job actively running on worker
	JobStatusCompleted JobStatus = "completed"  // Job finished successfully
	JobStatusFailed    JobStatus = "failed"     // Job failed permanently
	JobStatusTimedOut  JobStatus = "timed_out"  // Job exceeded timeout threshold
	JobStatusRetrying  JobStatus = "retrying"   // Job is being retried after failure
	JobStatusCanceled  JobStatus = "canceled"   // Job explicitly canceled by user
)

// StateTransitionRule defines valid state transitions
type StateTransitionRule struct {
	From        JobStatus
	To          JobStatus
	Description string
}

// validTransitions maps from-state to allowed to-states
var validTransitions = map[JobStatus]map[JobStatus]bool{
	JobStatusQueued: {
		JobStatusAssigned:  true, // Queue → Assigned (worker picks up job)
		JobStatusCanceled:  true, // Queue → Canceled (user cancels)
		JobStatusRetrying:  true, // Queue → Retrying (immediate retry scheduling)
	},
	JobStatusAssigned: {
		JobStatusRunning:   true, // Assigned → Running (worker starts execution)
		JobStatusRetrying:  true, // Assigned → Retrying (worker died before starting)
		JobStatusFailed:    true, // Assigned → Failed (assignment validation failed)
		JobStatusCanceled:  true, // Assigned → Canceled (user cancels)
		JobStatusTimedOut:  true, // Assigned → TimedOut (stuck in assigned state)
	},
	JobStatusRunning: {
		JobStatusCompleted: true, // Running → Completed (successful execution)
		JobStatusFailed:    true, // Running → Failed (execution failed)
		JobStatusTimedOut:  true, // Running → TimedOut (exceeded time limit)
		JobStatusRetrying:  true, // Running → Retrying (worker died mid-execution)
		JobStatusCanceled:  true, // Running → Canceled (user cancels)
	},
	JobStatusRetrying: {
		JobStatusQueued:   true, // Retrying → Queued (ready for reassignment)
		JobStatusFailed:   true, // Retrying → Failed (max retries exceeded)
		JobStatusCanceled: true, // Retrying → Canceled (user cancels)
	},
	JobStatusTimedOut: {
		JobStatusRetrying: true, // TimedOut → Retrying (retry after timeout)
		JobStatusFailed:   true, // TimedOut → Failed (max retries exceeded)
	},
	// Terminal states (no transitions allowed)
	JobStatusCompleted: {},
	JobStatusFailed:    {},
	JobStatusCanceled:  {},
}

// ValidateTransition checks if a state transition is valid
func ValidateTransition(from, to JobStatus) error {
	// Handle legacy states by mapping them
	from = normalizeState(from)
	to = normalizeState(to)

	// Check if from-state is known
	allowedStates, exists := validTransitions[from]
	if !exists {
		return fmt.Errorf("unknown source state: %s", from)
	}

	// Check if transition is allowed
	if !allowedStates[to] {
		return fmt.Errorf("invalid transition from %s to %s", from, to)
	}

	return nil
}

// normalizeState maps legacy states to new FSM states
func normalizeState(state JobStatus) JobStatus {
	switch state {
	case JobStatusPending:
		return JobStatusQueued
	case JobStatusProcessing:
		return JobStatusRunning
	case JobStatusPaused:
		// Paused jobs are treated as assigned but not running
		return JobStatusAssigned
	default:
		return state
	}
}

// IsTerminalState returns true if the state is terminal (no further transitions)
func IsTerminalState(state JobStatus) bool {
	state = normalizeState(state)
	return state == JobStatusCompleted || state == JobStatusFailed || state == JobStatusCanceled
}

// IsActiveState returns true if the job is actively being processed
func IsActiveState(state JobStatus) bool {
	state = normalizeState(state)
	return state == JobStatusAssigned || state == JobStatusRunning
}

// CanRetry returns true if the job can be retried from this state
func CanRetry(state JobStatus) bool {
	state = normalizeState(state)
	return state == JobStatusFailed || state == JobStatusTimedOut
}

// JobTimeout represents timeout configuration for jobs
type JobTimeout struct {
	FFmpegBaseDuration time.Duration // Base duration for FFmpeg jobs
	FFmpegSafetyFactor float64       // Multiplier for FFmpeg timeout (e.g., 2.0 = 2x duration)
	GStreamerSafety    time.Duration // Safety buffer for GStreamer (e.g., 30s)
	DefaultTimeout     time.Duration // Timeout when duration unknown
	AssignedTimeout    time.Duration // Max time job can stay in assigned state
}

// DefaultJobTimeout returns default timeout configuration
func DefaultJobTimeout() *JobTimeout {
	return &JobTimeout{
		FFmpegBaseDuration: 0, // Will be set from job parameters
		FFmpegSafetyFactor: 2.0,
		GStreamerSafety:    30 * time.Second,
		DefaultTimeout:     30 * time.Minute,
		AssignedTimeout:    5 * time.Minute,
	}
}

// CalculateTimeout calculates the timeout for a job based on its parameters
func (jt *JobTimeout) CalculateTimeout(job *Job) time.Duration {
	// For assigned jobs, use assigned timeout
	if job.Status == JobStatusAssigned {
		return jt.AssignedTimeout
	}

	// Try to extract duration from job parameters
	if job.Parameters != nil {
		if durationVal, ok := job.Parameters["duration"]; ok {
			switch v := durationVal.(type) {
			case float64:
				duration := time.Duration(v) * time.Second
				
				// For FFmpeg jobs: 2x duration + safety
				if job.Engine == "ffmpeg" || job.Engine == "auto" {
					return time.Duration(float64(duration) * jt.FFmpegSafetyFactor)
				}
				
				// For GStreamer jobs: duration + 30s safety
				if job.Engine == "gstreamer" {
					return duration + jt.GStreamerSafety
				}
				
				return duration
			case int:
				duration := time.Duration(v) * time.Second
				if job.Engine == "ffmpeg" || job.Engine == "auto" {
					return time.Duration(float64(duration) * jt.FFmpegSafetyFactor)
				}
				if job.Engine == "gstreamer" {
					return duration + jt.GStreamerSafety
				}
				return duration
			}
		}
	}

	// Default timeout when duration unknown
	return jt.DefaultTimeout
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries      int           // Maximum number of retries
	InitialBackoff  time.Duration // Initial backoff duration
	MaxBackoff      time.Duration // Maximum backoff duration
	BackoffMultiplier float64     // Multiplier for exponential backoff
}

// DefaultRetryPolicy returns default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        3,
		InitialBackoff:    5 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}
}

// CalculateBackoff calculates the backoff duration for a given retry count
func (rp *RetryPolicy) CalculateBackoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		return rp.InitialBackoff
	}

	// Exponential backoff: initialBackoff * (multiplier ^ retryCount)
	backoff := float64(rp.InitialBackoff)
	for i := 0; i < retryCount; i++ {
		backoff *= rp.BackoffMultiplier
	}

	duration := time.Duration(backoff)
	if duration > rp.MaxBackoff {
		return rp.MaxBackoff
	}
	return duration
}

// ShouldRetry determines if a job should be retried
func (rp *RetryPolicy) ShouldRetry(job *Job, reason string) bool {
	// Never retry if max retries exceeded
	if job.RetryCount >= rp.MaxRetries {
		return false
	}

	// Never retry canceled jobs
	if job.Status == JobStatusCanceled {
		return false
	}

	// Never retry if explicitly marked as non-retryable
	if job.Error != "" && contains(job.Error, "non-retryable") {
		return false
	}

	// Retry failed, timed out, or jobs on dead workers
	return CanRetry(job.Status) || reason == "worker_died" || reason == "worker_timeout"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
