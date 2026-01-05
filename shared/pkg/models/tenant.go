package models

import (
	"time"
)

// Tenant represents an organization or customer in the system
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"` // URL-friendly identifier
	Plan      string    `json:"plan"` // "free", "pro", "enterprise"
	Status    string    `json:"status"` // "active", "suspended", "deleted"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TenantQuota represents resource limits for a tenant
type TenantQuota struct {
	TenantID           string    `json:"tenant_id"`
	MaxConcurrentJobs  int       `json:"max_concurrent_jobs"`  // Max jobs running simultaneously
	MaxTotalJobs       int       `json:"max_total_jobs"`       // Max total jobs (lifetime or period)
	MaxWorkers         int       `json:"max_workers"`          // Max workers registered
	MaxCPUCores        int       `json:"max_cpu_cores"`        // Max CPU cores across all workers
	MaxGPUs            int       `json:"max_gpus"`             // Max GPUs across all workers
	MaxStorageGB       int       `json:"max_storage_gb"`       // Max storage for results/logs
	MaxAPIRequestsPerHour int    `json:"max_api_requests_per_hour"` // API rate limit
	UpdatedAt          time.Time `json:"updated_at"`
}

// TenantUsage tracks current resource usage for a tenant
type TenantUsage struct {
	TenantID           string    `json:"tenant_id"`
	CurrentJobs        int       `json:"current_jobs"`         // Jobs currently running
	TotalJobsToday     int       `json:"total_jobs_today"`     // Jobs submitted today
	TotalJobsLifetime  int       `json:"total_jobs_lifetime"`  // Total jobs ever
	CurrentWorkers     int       `json:"current_workers"`      // Workers currently registered
	CurrentCPUCores    int       `json:"current_cpu_cores"`    // Total CPU cores
	CurrentGPUs        int       `json:"current_gpus"`         // Total GPUs
	StorageUsedGB      float64   `json:"storage_used_gb"`      // Current storage usage
	APIRequestsLastHour int      `json:"api_requests_last_hour"` // API calls in last hour
	LastUpdated        time.Time `json:"last_updated"`
}

// TenantCost tracks cost accumulation for billing
type TenantCost struct {
	TenantID          string    `json:"tenant_id"`
	Month             string    `json:"month"` // "2026-01"
	ComputeHours      float64   `json:"compute_hours"`       // Total compute hours
	CPUHours          float64   `json:"cpu_hours"`           // CPU-only hours
	GPUHours          float64   `json:"gpu_hours"`           // GPU hours
	StorageGBHours    float64   `json:"storage_gb_hours"`    // GB-hours of storage
	NetworkGB         float64   `json:"network_gb"`          // GB of network transfer
	JobsCompleted     int       `json:"jobs_completed"`      // Completed jobs count
	EstimatedCostUSD  float64   `json:"estimated_cost_usd"`  // Estimated cost
	Currency          string    `json:"currency"`            // "USD", "EUR", etc.
	LastUpdated       time.Time `json:"last_updated"`
}

// TenantAPIKey represents an API key for tenant authentication
type TenantAPIKey struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Name       string    `json:"name"`       // Human-friendly name
	KeyHash    string    `json:"key_hash"`   // Hashed API key (never store plain)
	KeyPrefix  string    `json:"key_prefix"` // First 8 chars for identification (e.g., "ffrtmp_prod_abc123...")
	Scopes     []string  `json:"scopes"`     // ["jobs:read", "jobs:write", "nodes:read"]
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by"` // User ID who created the key
	Status     string    `json:"status"`     // "active", "revoked"
}

// TenantRequest represents a request to create a new tenant
type TenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"` // Auto-generated if not provided
	Plan string `json:"plan,omitempty"` // Defaults to "free"
}

// DefaultTenantQuotas returns default quotas based on plan
func DefaultTenantQuotas(plan string) *TenantQuota {
	quotas := map[string]*TenantQuota{
		"free": {
			MaxConcurrentJobs:     3,
			MaxTotalJobs:          100,
			MaxWorkers:            1,
			MaxCPUCores:           8,
			MaxGPUs:               0,
			MaxStorageGB:          5,
			MaxAPIRequestsPerHour: 100,
		},
		"pro": {
			MaxConcurrentJobs:     20,
			MaxTotalJobs:          10000,
			MaxWorkers:            10,
			MaxCPUCores:           64,
			MaxGPUs:               4,
			MaxStorageGB:          100,
			MaxAPIRequestsPerHour: 1000,
		},
		"enterprise": {
			MaxConcurrentJobs:     -1, // Unlimited
			MaxTotalJobs:          -1, // Unlimited
			MaxWorkers:            -1, // Unlimited
			MaxCPUCores:           -1, // Unlimited
			MaxGPUs:               -1, // Unlimited
			MaxStorageGB:          -1, // Unlimited
			MaxAPIRequestsPerHour: -1, // Unlimited
		},
	}

	if q, ok := quotas[plan]; ok {
		return q
	}
	return quotas["free"] // Default to free plan
}
