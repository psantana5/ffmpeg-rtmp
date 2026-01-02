package models

import (
	"testing"
	"time"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    JobStatus
		to      JobStatus
		wantErr bool
	}{
		// Valid transitions
		{"Queued to Assigned", JobStatusQueued, JobStatusAssigned, false},
		{"Queued to Canceled", JobStatusQueued, JobStatusCanceled, false},
		{"Assigned to Running", JobStatusAssigned, JobStatusRunning, false},
		{"Assigned to Retrying", JobStatusAssigned, JobStatusRetrying, false},
		{"Running to Completed", JobStatusRunning, JobStatusCompleted, false},
		{"Running to Failed", JobStatusRunning, JobStatusFailed, false},
		{"Running to TimedOut", JobStatusRunning, JobStatusTimedOut, false},
		{"TimedOut to Retrying", JobStatusTimedOut, JobStatusRetrying, false},
		{"Retrying to Queued", JobStatusRetrying, JobStatusQueued, false},
		{"Retrying to Failed", JobStatusRetrying, JobStatusFailed, false},

		// Invalid transitions
		{"Queued to Completed", JobStatusQueued, JobStatusCompleted, true},
		{"Queued to Running", JobStatusQueued, JobStatusRunning, true},
		{"Assigned to Completed", JobStatusAssigned, JobStatusCompleted, true},
		{"Completed to Running", JobStatusCompleted, JobStatusRunning, true},
		{"Completed to anything", JobStatusCompleted, JobStatusRetrying, true},
		{"Failed to Running", JobStatusFailed, JobStatusRunning, true},
		{"Canceled to Queued", JobStatusCanceled, JobStatusQueued, true},

		// Legacy state mapping
		{"Pending to Assigned", JobStatusPending, JobStatusAssigned, false},
		{"Processing to Completed", JobStatusProcessing, JobStatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTransition(%v, %v) error = %v, wantErr %v",
					tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestIsTerminalState(t *testing.T) {
	tests := []struct {
		name     string
		state    JobStatus
		expected bool
	}{
		{"Completed is terminal", JobStatusCompleted, true},
		{"Failed is terminal", JobStatusFailed, true},
		{"Canceled is terminal", JobStatusCanceled, true},
		{"Queued is not terminal", JobStatusQueued, false},
		{"Running is not terminal", JobStatusRunning, false},
		{"Retrying is not terminal", JobStatusRetrying, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTerminalState(tt.state)
			if result != tt.expected {
				t.Errorf("IsTerminalState(%v) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}

func TestIsActiveState(t *testing.T) {
	tests := []struct {
		name     string
		state    JobStatus
		expected bool
	}{
		{"Assigned is active", JobStatusAssigned, true},
		{"Running is active", JobStatusRunning, true},
		{"Queued is not active", JobStatusQueued, false},
		{"Completed is not active", JobStatusCompleted, false},
		{"Failed is not active", JobStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsActiveState(tt.state)
			if result != tt.expected {
				t.Errorf("IsActiveState(%v) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}

func TestCanRetry(t *testing.T) {
	tests := []struct {
		name     string
		state    JobStatus
		expected bool
	}{
		{"Failed can retry", JobStatusFailed, true},
		{"TimedOut can retry", JobStatusTimedOut, true},
		{"Completed cannot retry", JobStatusCompleted, false},
		{"Canceled cannot retry", JobStatusCanceled, false},
		{"Running cannot retry", JobStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanRetry(tt.state)
			if result != tt.expected {
				t.Errorf("CanRetry(%v) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}

func TestCalculateTimeout(t *testing.T) {
	config := DefaultJobTimeout()

	tests := []struct {
		name     string
		job      *Job
		expected time.Duration
	}{
		{
			name: "Assigned job uses assigned timeout",
			job: &Job{
				Status: JobStatusAssigned,
				Engine: "ffmpeg",
			},
			expected: config.AssignedTimeout,
		},
		{
			name: "FFmpeg job with duration",
			job: &Job{
				Status: JobStatusRunning,
				Engine: "ffmpeg",
				Parameters: map[string]interface{}{
					"duration": 60.0, // 60 seconds
				},
			},
			expected: 120 * time.Second, // 2x duration
		},
		{
			name: "GStreamer job with duration",
			job: &Job{
				Status: JobStatusRunning,
				Engine: "gstreamer",
				Parameters: map[string]interface{}{
					"duration": 60.0,
				},
			},
			expected: 90 * time.Second, // duration + 30s safety
		},
		{
			name: "Job without duration",
			job: &Job{
				Status: JobStatusRunning,
				Engine: "ffmpeg",
			},
			expected: config.DefaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.CalculateTimeout(tt.job)
			if result != tt.expected {
				t.Errorf("CalculateTimeout() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name        string
		retryCount  int
		expected    time.Duration
		maxExpected time.Duration
	}{
		{"First retry", 0, 5 * time.Second, 5 * time.Second},
		{"Second retry", 1, 10 * time.Second, 10 * time.Second},
		{"Third retry", 2, 20 * time.Second, 20 * time.Second},
		{"Fourth retry (capped)", 3, 40 * time.Second, policy.MaxBackoff},
		{"Many retries (capped)", 10, policy.MaxBackoff, policy.MaxBackoff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.CalculateBackoff(tt.retryCount)
			if result < tt.expected || result > tt.maxExpected {
				t.Errorf("CalculateBackoff(%d) = %v, want range [%v, %v]",
					tt.retryCount, result, tt.expected, tt.maxExpected)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name     string
		job      *Job
		reason   string
		expected bool
	}{
		{
			name: "Failed job under retry limit",
			job: &Job{
				Status:     JobStatusFailed,
				RetryCount: 1,
			},
			reason:   "transient error",
			expected: true,
		},
		{
			name: "Failed job at retry limit",
			job: &Job{
				Status:     JobStatusFailed,
				RetryCount: 3,
			},
			reason:   "transient error",
			expected: false,
		},
		{
			name: "Canceled job",
			job: &Job{
				Status:     JobStatusCanceled,
				RetryCount: 0,
			},
			reason:   "worker died",
			expected: false,
		},
		{
			name: "Worker died",
			job: &Job{
				Status:     JobStatusRunning,
				RetryCount: 1,
			},
			reason:   "worker_died",
			expected: true,
		},
		{
			name: "Non-retryable error",
			job: &Job{
				Status:     JobStatusFailed,
				RetryCount: 0,
				Error:      "non-retryable: invalid input",
			},
			reason:   "validation error",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.ShouldRetry(tt.job, tt.reason)
			if result != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizeState(t *testing.T) {
	tests := []struct {
		name     string
		state    JobStatus
		expected JobStatus
	}{
		{"Pending maps to Queued", JobStatusPending, JobStatusQueued},
		{"Processing maps to Running", JobStatusProcessing, JobStatusRunning},
		{"Paused maps to Assigned", JobStatusPaused, JobStatusAssigned},
		{"Queued stays Queued", JobStatusQueued, JobStatusQueued},
		{"Running stays Running", JobStatusRunning, JobStatusRunning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeState(tt.state)
			if result != tt.expected {
				t.Errorf("normalizeState(%v) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}
