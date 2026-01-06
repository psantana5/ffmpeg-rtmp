package wrapper

import "time"

// WorkloadMetadata describes the workload being wrapped
type WorkloadMetadata struct {
	JobID       string    `json:"job_id"`       // Job identifier
	SLAEligible bool      `json:"sla_eligible"` // Whether this workload counts toward SLA
	Intent      string    `json:"intent"`       // production|test|experiment|soak
	Tags        map[string]string `json:"tags,omitempty"` // Additional tags
	StartedAt   time.Time `json:"started_at"`   // When wrapper attached/started
}

// Intent types for workloads
const (
	IntentProduction  = "production"
	IntentTest        = "test"
	IntentExperiment  = "experiment"
	IntentSoak        = "soak"
)

// Validate checks if metadata is valid
func (m *WorkloadMetadata) Validate() error {
	if m.JobID == "" {
		m.JobID = "unknown"
	}
	
	if m.Intent == "" {
		m.Intent = IntentProduction
	}
	
	if m.StartedAt.IsZero() {
		m.StartedAt = time.Now()
	}
	
	return nil
}
