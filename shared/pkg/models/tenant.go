package models

import (
	"errors"
	"fmt"
	"time"
)

// Tenant represents an organization or customer in the system
type Tenant struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Status      TenantStatus           `json:"status"`
	Plan        string                 `json:"plan"` // free, basic, pro, enterprise
	Quotas      TenantQuotas           `json:"quotas"`
	Usage       TenantUsage            `json:"usage"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
}

// TenantStatus represents the operational status of a tenant
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusExpired   TenantStatus = "expired"
	TenantStatusDeleted   TenantStatus = "deleted"
)

// TenantQuotas defines resource limits for a tenant
type TenantQuotas struct {
	MaxJobs           int   `json:"max_jobs"`            // Max concurrent jobs
	MaxWorkers        int   `json:"max_workers"`         // Max workers
	MaxJobsPerHour    int   `json:"max_jobs_per_hour"`   // Rate limit
	MaxCPUCores       int   `json:"max_cpu_cores"`       // Total CPU cores
	MaxGPUs           int   `json:"max_gpus"`            // Total GPUs
	MaxStorageGB      int64 `json:"max_storage_gb"`      // Storage limit
	MaxBandwidthMbps  int   `json:"max_bandwidth_mbps"`  // Bandwidth limit
	JobTimeoutMinutes int   `json:"job_timeout_minutes"` // Max job duration
}

// TenantUsage tracks current resource usage
type TenantUsage struct {
	ActiveJobs      int       `json:"active_jobs"`
	TotalJobs       int       `json:"total_jobs"`
	CompletedJobs   int       `json:"completed_jobs"`
	FailedJobs      int       `json:"failed_jobs"`
	ActiveWorkers   int       `json:"active_workers"`
	CPUCoresUsed    int       `json:"cpu_cores_used"`
	GPUsUsed        int       `json:"gpus_used"`
	StorageUsedGB   int64     `json:"storage_used_gb"`
	LastJobAt       time.Time `json:"last_job_at,omitempty"`
	JobsThisHour    int       `json:"jobs_this_hour"`
	JobsHourResetAt time.Time `json:"jobs_hour_reset_at,omitempty"`
}

// DefaultQuotas returns default quotas based on plan
func DefaultQuotas(plan string) TenantQuotas {
	switch plan {
	case "free":
		return TenantQuotas{
			MaxJobs:           5,
			MaxWorkers:        1,
			MaxJobsPerHour:    10,
			MaxCPUCores:       4,
			MaxGPUs:           0,
			MaxStorageGB:      10,
			MaxBandwidthMbps:  100,
			JobTimeoutMinutes: 30,
		}
	case "basic":
		return TenantQuotas{
			MaxJobs:           20,
			MaxWorkers:        5,
			MaxJobsPerHour:    100,
			MaxCPUCores:       16,
			MaxGPUs:           1,
			MaxStorageGB:      100,
			MaxBandwidthMbps:  1000,
			JobTimeoutMinutes: 120,
		}
	case "pro":
		return TenantQuotas{
			MaxJobs:           100,
			MaxWorkers:        20,
			MaxJobsPerHour:    1000,
			MaxCPUCores:       64,
			MaxGPUs:           4,
			MaxStorageGB:      1000,
			MaxBandwidthMbps:  10000,
			JobTimeoutMinutes: 480,
		}
	case "enterprise":
		return TenantQuotas{
			MaxJobs:           1000,
			MaxWorkers:        100,
			MaxJobsPerHour:    10000,
			MaxCPUCores:       256,
			MaxGPUs:           16,
			MaxStorageGB:      10000,
			MaxBandwidthMbps:  100000,
			JobTimeoutMinutes: 1440, // 24 hours
		}
	default:
		return DefaultQuotas("free")
	}
}

// Validate checks if the tenant configuration is valid
func (t *Tenant) Validate() error {
	if t.ID == "" {
		return errors.New("tenant ID is required")
	}
	if t.Name == "" {
		return errors.New("tenant name is required")
	}
	if len(t.Name) < 3 || len(t.Name) > 50 {
		return errors.New("tenant name must be between 3 and 50 characters")
	}
	if t.Status == "" {
		return errors.New("tenant status is required")
	}
	if !isValidTenantStatus(t.Status) {
		return fmt.Errorf("invalid tenant status: %s", t.Status)
	}
	if t.Plan != "" && !isValidPlan(t.Plan) {
		return fmt.Errorf("invalid plan: %s", t.Plan)
	}
	return nil
}

// IsActive returns true if tenant can use the system
func (t *Tenant) IsActive() bool {
	if t.Status != TenantStatusActive {
		return false
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return false
	}
	return true
}

// CanCreateJob checks if tenant can create a new job
func (t *Tenant) CanCreateJob() (bool, string) {
	if !t.IsActive() {
		return false, "tenant is not active"
	}

	// Check concurrent jobs limit
	if t.Quotas.MaxJobs > 0 && t.Usage.ActiveJobs >= t.Quotas.MaxJobs {
		return false, fmt.Sprintf("max concurrent jobs limit reached (%d/%d)", t.Usage.ActiveJobs, t.Quotas.MaxJobs)
	}

	// Check hourly rate limit
	if t.Quotas.MaxJobsPerHour > 0 {
		// Reset counter if hour has passed
		if time.Now().After(t.Usage.JobsHourResetAt) {
			// Note: This is checked/reset in the handler
		} else if t.Usage.JobsThisHour >= t.Quotas.MaxJobsPerHour {
			return false, fmt.Sprintf("hourly job limit reached (%d/%d)", t.Usage.JobsThisHour, t.Quotas.MaxJobsPerHour)
		}
	}

	return true, ""
}

// CanRegisterWorker checks if tenant can register a new worker
func (t *Tenant) CanRegisterWorker(cpuCores int, hasGPU bool) (bool, string) {
	if !t.IsActive() {
		return false, "tenant is not active"
	}

	// Check worker limit
	if t.Quotas.MaxWorkers > 0 && t.Usage.ActiveWorkers >= t.Quotas.MaxWorkers {
		return false, fmt.Sprintf("max workers limit reached (%d/%d)", t.Usage.ActiveWorkers, t.Quotas.MaxWorkers)
	}

	// Check CPU cores
	if t.Quotas.MaxCPUCores > 0 && (t.Usage.CPUCoresUsed+cpuCores) > t.Quotas.MaxCPUCores {
		return false, fmt.Sprintf("max CPU cores limit reached (%d+%d > %d)", t.Usage.CPUCoresUsed, cpuCores, t.Quotas.MaxCPUCores)
	}

	// Check GPU
	if hasGPU && t.Quotas.MaxGPUs > 0 && t.Usage.GPUsUsed >= t.Quotas.MaxGPUs {
		return false, fmt.Sprintf("max GPUs limit reached (%d/%d)", t.Usage.GPUsUsed, t.Quotas.MaxGPUs)
	}

	return true, ""
}

// IncrementJobCount increments job usage counters
func (t *Tenant) IncrementJobCount() {
	t.Usage.ActiveJobs++
	t.Usage.TotalJobs++
	t.Usage.LastJobAt = time.Now()

	// Reset hourly counter if needed
	if time.Now().After(t.Usage.JobsHourResetAt) {
		t.Usage.JobsThisHour = 1
		t.Usage.JobsHourResetAt = time.Now().Add(1 * time.Hour)
	} else {
		t.Usage.JobsThisHour++
	}
}

// DecrementJobCount decrements active job count
func (t *Tenant) DecrementJobCount() {
	if t.Usage.ActiveJobs > 0 {
		t.Usage.ActiveJobs--
	}
}

// IncrementCompletedJobs increments completed job counter
func (t *Tenant) IncrementCompletedJobs() {
	t.Usage.CompletedJobs++
	t.DecrementJobCount()
}

// IncrementFailedJobs increments failed job counter
func (t *Tenant) IncrementFailedJobs() {
	t.Usage.FailedJobs++
	t.DecrementJobCount()
}

// Helper functions

func isValidTenantStatus(status TenantStatus) bool {
	switch status {
	case TenantStatusActive, TenantStatusSuspended, TenantStatusExpired, TenantStatusDeleted:
		return true
	default:
		return false
	}
}

func isValidPlan(plan string) bool {
	switch plan {
	case "free", "basic", "pro", "enterprise":
		return true
	default:
		return false
	}
}

// NewTenant creates a new tenant with default values
func NewTenant(id, name, plan string) *Tenant {
	if plan == "" {
		plan = "free"
	}

	return &Tenant{
		ID:          id,
		Name:        name,
		DisplayName: name,
		Status:      TenantStatusActive,
		Plan:        plan,
		Quotas:      DefaultQuotas(plan),
		Usage:       TenantUsage{},
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
