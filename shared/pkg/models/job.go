package models

import (
	"fmt"
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
	FailureReasonCapabilityMismatch FailureReason = "capability_mismatch" // Missing GPU/encoder/engine (USER ERROR - not SLA violation)
	FailureReasonRuntimeError       FailureReason = "runtime_error"       // Execution error (NEEDS INSPECTION)
	FailureReasonTimeout            FailureReason = "timeout"             // Job exceeded timeout (PLATFORM - SLA violation)
	FailureReasonUserError          FailureReason = "user_error"          // Invalid parameters/config (USER ERROR - not SLA violation)
	FailureReasonNetworkError       FailureReason = "network_error"       // External network issue (EXTERNAL - not SLA violation)
	FailureReasonInputError         FailureReason = "input_error"         // Corrupt/invalid input file (USER ERROR - not SLA violation)
	FailureReasonPlatformError      FailureReason = "platform_error"      // Platform/scheduler/worker failure (PLATFORM - SLA violation)
	FailureReasonResourceError      FailureReason = "resource_error"      // Resource exhaustion/management failure (PLATFORM - SLA violation)
)

// JobClassification represents the business classification of a job
type JobClassification string

const (
	JobClassificationProduction JobClassification = "production" // Production workload (SLA-worthy)
	JobClassificationTest       JobClassification = "test"       // Test/development (not SLA-worthy)
	JobClassificationBenchmark  JobClassification = "benchmark"  // Performance testing (metrics only)
	JobClassificationDebug      JobClassification = "debug"      // Debugging/troubleshooting (not SLA-worthy)
)

// WrapperConstraints defines resource constraints for the workload wrapper
type WrapperConstraints struct {
	CPUMax      string `json:"cpu_max,omitempty"`       // CPU quota in "quota period" format
	CPUWeight   int    `json:"cpu_weight,omitempty"`    // CPU weight (1-10000, default 100)
	MemoryMaxMB int64  `json:"memory_max_mb,omitempty"` // Memory limit in MB
	IOMax       string `json:"io_max,omitempty"`        // IO max (cgroup v2 only)
}

// Job represents a workload to be executed on a compute node
type Job struct {
	ID               string                 `json:"id"`
	TenantID         string                 `json:"tenant_id,omitempty"`       // Tenant/organization ID
	UserID           string                 `json:"user_id,omitempty"`         // User who created the job
	SequenceNumber   int                    `json:"sequence_number,omitempty"` // Human-friendly job number
	Scenario         string                 `json:"scenario"`                  // e.g., "4K60-h264"
	Confidence       string                 `json:"confidence"`                // "auto", "high", "medium", "low"
	Engine           string                 `json:"engine,omitempty"`          // "auto", "ffmpeg", "gstreamer"
	Classification   JobClassification      `json:"classification,omitempty"`  // "production", "test", "benchmark", "debug"
	WrapperEnabled   bool                   `json:"wrapper_enabled,omitempty"` // Use wrapper for execution
	WrapperConstraints *WrapperConstraints  `json:"wrapper_constraints,omitempty"` // Resource constraints
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Status           JobStatus              `json:"status"`
	Queue            string                 `json:"queue,omitempty"`    // "live", "default", "batch"
	Priority         string                 `json:"priority,omitempty"` // "high", "medium", "low"
	Progress         int                    `json:"progress,omitempty"` // 0-100%
	NodeID           string                 `json:"node_id,omitempty"`
	NodeName         string                 `json:"node_name,omitempty"` // Human-friendly node name (not stored, populated on read)
	CreatedAt        time.Time              `json:"created_at"`
	StartedAt        *time.Time             `json:"started_at,omitempty"`
	LastActivityAt   *time.Time             `json:"last_activity_at,omitempty"` // Tracks last heartbeat/progress update
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
	RetryCount       int                    `json:"retry_count"`
	MaxRetries       int                    `json:"max_retries,omitempty"`  // Max retry attempts (default: 3)
	RetryReason      string                 `json:"retry_reason,omitempty"` // Reason for current retry
	Error            string                 `json:"error,omitempty"`
	FailureReason    FailureReason          `json:"failure_reason,omitempty"` // Explicit failure classification
	Logs             string                 `json:"logs,omitempty"`           // Worker execution logs
	TimeoutAt        *time.Time             `json:"timeout_at,omitempty"`     // Calculated timeout deadline
	StateTransitions []StateTransition      `json:"state_transitions,omitempty"`
	
	// Wrapper results (populated after execution)
	PlatformSLA       bool   `json:"platform_sla_compliant,omitempty"`
	PlatformSLAReason string `json:"platform_sla_reason,omitempty"`
}

// JobRequest represents a request to create a new job
type JobRequest struct {
	Scenario       string                 `json:"scenario"`
	Confidence     string                 `json:"confidence,omitempty"`
	Engine         string                 `json:"engine,omitempty"`         // "auto", "ffmpeg", "gstreamer"
	Classification string                 `json:"classification,omitempty"` // "production", "test", "benchmark", "debug"
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	Queue          string                 `json:"queue,omitempty"`    // "live", "default", "batch"
	Priority       string                 `json:"priority,omitempty"` // "high", "medium", "low"
}

// JobResult represents the result of a completed job
type JobResult struct {
	JobID           string                 `json:"job_id"`
	NodeID          string                 `json:"node_id"`
	Status          JobStatus              `json:"status"`
	Progress        int                    `json:"progress,omitempty"` // Final progress 0-100%
	Metrics         map[string]interface{} `json:"metrics,omitempty"`
	AnalyzerOutput  map[string]interface{} `json:"analyzer_output,omitempty"`
	Error           string                 `json:"error,omitempty"`
	Logs            string                 `json:"logs,omitempty"` // Worker execution logs
	CompletedAt     time.Time              `json:"completed_at"`
	QoEScore        float64                `json:"qoe_score,omitempty"`
	EfficiencyScore float64                `json:"efficiency_score,omitempty"`
	EnergyJoules    float64                `json:"energy_joules,omitempty"`
	VMAFScore       float64                `json:"vmaf_score,omitempty"`
}

// StateTransition tracks job state changes with timestamps
type StateTransition struct {
	From      JobStatus `json:"from"`
	To        JobStatus `json:"to"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// IsSLAWorthy returns true if the job should be counted towards SLA compliance
func (j *Job) IsSLAWorthy() bool {
	// Production jobs are always SLA-worthy
	if j.Classification == JobClassificationProduction {
		return true
	}

	// Test, benchmark, and debug jobs are NOT SLA-worthy
	if j.Classification == JobClassificationTest ||
		j.Classification == JobClassificationBenchmark ||
		j.Classification == JobClassificationDebug {
		return false
	}

	// If no classification specified, use heuristics
	// - Jobs with "test" in scenario name are not SLA-worthy
	// - Jobs with duration < 10 seconds are likely tests
	// - Jobs in "batch" queue with low priority are not SLA-worthy

	// Check scenario name for test indicators
	if len(j.Scenario) > 0 {
		lowerScenario := j.Scenario
		if len(lowerScenario) > 4 && lowerScenario[:4] == "test" {
			return false
		}
		if len(lowerScenario) > 5 && lowerScenario[:5] == "debug" {
			return false
		}
		if len(lowerScenario) > 9 && lowerScenario[:9] == "benchmark" {
			return false
		}
	}

	// Check duration parameter (if < 10s, likely a test)
	if params := j.Parameters; params != nil {
		if durationVal, ok := params["duration"]; ok {
			switch v := durationVal.(type) {
			case int:
				if v < 10 {
					return false
				}
			case float64:
				if v < 10 {
					return false
				}
			case string:
				// Parse string duration
				if len(v) > 0 && v[0] >= '0' && v[0] <= '9' {
					var dur float64
					if _, err := fmt.Sscanf(v, "%f", &dur); err == nil && dur < 10 {
						return false
					}
				}
			}
		}
	}

	// Batch queue with low priority is not SLA-worthy
	if j.Queue == "batch" && j.Priority == "low" {
		return false
	}

	// Default: treat as SLA-worthy (conservative approach)
	return true
}

// GetSLACategory returns a descriptive category for the job's SLA classification
func (j *Job) GetSLACategory() string {
	if j.IsSLAWorthy() {
		return "production"
	}

	// Determine why it's not SLA-worthy
	if j.Classification == JobClassificationTest {
		return "test"
	}
	if j.Classification == JobClassificationBenchmark {
		return "benchmark"
	}
	if j.Classification == JobClassificationDebug {
		return "debug"
	}

	// Heuristic-based classification
	if len(j.Scenario) > 4 && j.Scenario[:4] == "test" {
		return "test"
	}
	if len(j.Scenario) > 5 && j.Scenario[:5] == "debug" {
		return "debug"
	}
	if len(j.Scenario) > 9 && j.Scenario[:9] == "benchmark" {
		return "benchmark"
	}

	// Check duration
	if params := j.Parameters; params != nil {
		if durationVal, ok := params["duration"]; ok {
			switch v := durationVal.(type) {
			case int:
				if v < 10 {
					return "test"
				}
			case float64:
				if v < 10 {
					return "test"
				}
			}
		}
	}

	if j.Queue == "batch" && j.Priority == "low" {
		return "batch"
	}

	return "other"
}

// SLATimingTargets defines timing targets for platform SLA
type SLATimingTargets struct {
	MaxQueueTimeSeconds  float64 // Max time job should wait in queue (default: 30s)
	MaxStartDelaySeconds float64 // Max time from assignment to start (default: 60s)
	MaxProcessingSeconds float64 // Max total processing time (default: 600s)
}

// GetDefaultSLATimingTargets returns default SLA timing targets
func GetDefaultSLATimingTargets() SLATimingTargets {
	return SLATimingTargets{
		MaxQueueTimeSeconds:  30.0,  // Job should be assigned within 30s
		MaxStartDelaySeconds: 60.0,  // Job should start within 60s of assignment
		MaxProcessingSeconds: 600.0, // Job should complete within 10 minutes
	}
}

// CalculatePlatformSLACompliance checks if the platform met its SLA obligations
// Returns (compliant bool, reason string)
func (j *Job) CalculatePlatformSLACompliance(targets SLATimingTargets) (bool, string) {
	// If job is not SLA-worthy, return compliant by default
	if !j.IsSLAWorthy() {
		return true, "not_sla_worthy"
	}

	// Check if job failed due to platform error (SLA violation)
	if j.Status == JobStatusFailed || j.Status == JobStatusTimedOut {
		switch j.FailureReason {
		case FailureReasonPlatformError:
			return false, "platform_failure"
		case FailureReasonResourceError:
			return false, "resource_management_failure"
		case FailureReasonTimeout:
			// Check if it's a platform timeout or reasonable processing time
			if j.StartedAt != nil && j.CompletedAt != nil {
				processingTime := j.CompletedAt.Sub(*j.StartedAt).Seconds()
				if processingTime > targets.MaxProcessingSeconds {
					return false, "platform_timeout_exceeded"
				}
			}
			return true, "user_timeout_reasonable"
		case FailureReasonUserError, FailureReasonCapabilityMismatch,
			FailureReasonNetworkError, FailureReasonInputError:
			// These are NOT platform failures - platform behaved correctly
			return true, "external_failure_platform_ok"
		case FailureReasonRuntimeError:
			// Runtime errors need inspection - default to platform compliant
			// unless we can prove it's a platform issue
			return true, "runtime_error_external"
		default:
			// Unknown failure reason - assume platform compliant
			return true, "unknown_failure_external"
		}
	}

	// Check timing SLAs for successful or in-progress jobs
	now := time.Now()

	// 1. Check queue time (created → assigned)
	if j.StartedAt != nil {
		// Job has been assigned
		queueTime := j.StartedAt.Sub(j.CreatedAt).Seconds()
		if queueTime > targets.MaxQueueTimeSeconds {
			return false, "queue_time_exceeded"
		}
	} else if j.Status != JobStatusCompleted {
		// Job still waiting to be assigned
		queueTime := now.Sub(j.CreatedAt).Seconds()
		if queueTime > targets.MaxQueueTimeSeconds {
			return false, "queue_time_exceeded"
		}
	}

	// 2. Check start delay (assigned → started)
	// Note: In our system, assignment happens when worker picks up the job
	// So this is essentially checking if there was a delay between assignment and execution

	// 3. Check total processing time (started → completed)
	if j.StartedAt != nil && j.CompletedAt != nil {
		processingTime := j.CompletedAt.Sub(*j.StartedAt).Seconds()
		if processingTime > targets.MaxProcessingSeconds {
			return false, "processing_time_exceeded"
		}
	}

	// Platform met all SLA obligations
	return true, "compliant"
}

// IsPlatformFailure returns true if the job failed due to platform issues
func (j *Job) IsPlatformFailure() bool {
	return j.FailureReason == FailureReasonPlatformError ||
		j.FailureReason == FailureReasonResourceError ||
		j.FailureReason == FailureReasonTimeout
}

// IsExternalFailure returns true if the job failed due to external/user issues
func (j *Job) IsExternalFailure() bool {
	return j.FailureReason == FailureReasonUserError ||
		j.FailureReason == FailureReasonCapabilityMismatch ||
		j.FailureReason == FailureReasonNetworkError ||
		j.FailureReason == FailureReasonInputError
}
