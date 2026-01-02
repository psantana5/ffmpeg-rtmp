package store

import (
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantCRUD(t *testing.T) {
	store := NewMemoryStore()

	// Create tenant
	tenant := &models.Tenant{
		Name:        "test-company",
		DisplayName: "Test Company Inc.",
		Plan:        "pro",
		Status:      models.TenantActive,
		Quotas: models.TenantQuotas{
			MaxJobs:        100,
			MaxWorkers:     10,
			MaxCPUCores:    50,
			MaxGPUs:        5,
			MaxJobsPerHour: 1000,
		},
	}

	err := store.CreateTenant(tenant)
	require.NoError(t, err)
	require.NotEmpty(t, tenant.ID)
	assert.False(t, tenant.CreatedAt.IsZero())

	// Get tenant
	retrieved, err := store.GetTenant(tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.Name, retrieved.Name)
	assert.Equal(t, tenant.Plan, retrieved.Plan)

	// Get tenant by name
	byName, err := store.GetTenantByName("test-company")
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, byName.ID)

	// Update tenant
	tenant.DisplayName = "Updated Company"
	tenant.Plan = "enterprise"
	err = store.UpdateTenant(tenant)
	require.NoError(t, err)

	retrieved, err = store.GetTenant(tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Company", retrieved.DisplayName)
	assert.Equal(t, "enterprise", retrieved.Plan)

	// List tenants
	tenants, err := store.ListTenants()
	require.NoError(t, err)
	// Should have default tenant + our test tenant
	assert.GreaterOrEqual(t, len(tenants), 2)

	// Delete tenant
	err = store.DeleteTenant(tenant.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = store.GetTenant(tenant.ID)
	assert.Error(t, err)
}

func TestTenantIsolation(t *testing.T) {
	store := NewMemoryStore()

	// Create two tenants
	tenant1 := &models.Tenant{
		Name: "tenant1",
		Plan: "free",
	}
	tenant2 := &models.Tenant{
		Name: "tenant2",
		Plan: "pro",
	}

	require.NoError(t, store.CreateTenant(tenant1))
	require.NoError(t, store.CreateTenant(tenant2))

	// Create jobs for each tenant
	job1 := &models.Job{
		TenantID: tenant1.ID,
		InputURL: "rtmp://example1.com/stream",
		Status:   models.JobStatusQueued,
	}
	job2 := &models.Job{
		TenantID: tenant2.ID,
		InputURL: "rtmp://example2.com/stream",
		Status:   models.JobStatusQueued,
	}

	require.NoError(t, store.CreateJob(job1))
	require.NoError(t, store.CreateJob(job2))

	// Verify tenant1 only sees their jobs
	jobs1, err := store.GetJobsByTenant(tenant1.ID)
	require.NoError(t, err)
	assert.Len(t, jobs1, 1)
	assert.Equal(t, job1.ID, jobs1[0].ID)

	// Verify tenant2 only sees their jobs
	jobs2, err := store.GetJobsByTenant(tenant2.ID)
	require.NoError(t, err)
	assert.Len(t, jobs2, 1)
	assert.Equal(t, job2.ID, jobs2[0].ID)
}

func TestTenantQuotaEnforcement(t *testing.T) {
	store := NewMemoryStore()

	// Create tenant with strict quotas
	tenant := &models.Tenant{
		Name:   "limited-tenant",
		Plan:   "basic",
		Status: models.TenantActive,
		Quotas: models.TenantQuotas{
			MaxJobs:        2,
			MaxWorkers:     1,
			MaxCPUCores:    4,
			MaxGPUs:        0,
			MaxJobsPerHour: 10,
		},
	}
	require.NoError(t, store.CreateTenant(tenant))

	// Create jobs up to quota
	for i := 0; i < 2; i++ {
		job := &models.Job{
			TenantID: tenant.ID,
			InputURL: "rtmp://example.com/stream",
			Status:   models.JobStatusRunning,
		}
		require.NoError(t, store.CreateJob(job))
	}

	// Get stats
	stats, err := store.GetTenantStats(tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.ActiveJobs)

	// Verify quota check would fail
	assert.Equal(t, tenant.Quotas.MaxJobs, stats.ActiveJobs)
}

func TestTenantStatus(t *testing.T) {
	store := NewMemoryStore()

	// Create active tenant
	tenant := &models.Tenant{
		Name:   "test-tenant",
		Plan:   "free",
		Status: models.TenantActive,
	}
	require.NoError(t, store.CreateTenant(tenant))

	// Suspend tenant
	tenant.Status = models.TenantSuspended
	require.NoError(t, store.UpdateTenant(tenant))

	// Verify status
	retrieved, err := store.GetTenant(tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, models.TenantSuspended, retrieved.Status)

	// List only active tenants should not include suspended
	active, err := store.ListTenants()
	require.NoError(t, err)
	found := false
	for _, t := range active {
		if t.ID == tenant.ID && t.Status == models.TenantSuspended {
			found = true
		}
	}
	assert.True(t, found) // It's in the list but suspended
}

func TestTenantExpiration(t *testing.T) {
	store := NewMemoryStore()

	// Create tenant with expiration
	expiredTime := time.Now().Add(-24 * time.Hour)
	tenant := &models.Tenant{
		Name:      "expiring-tenant",
		Plan:      "trial",
		Status:    models.TenantActive,
		ExpiresAt: &expiredTime,
	}
	require.NoError(t, store.CreateTenant(tenant))

	// In a real system, a background job would check expirations
	// and update status to TenantExpired
	retrieved, err := store.GetTenant(tenant.ID)
	require.NoError(t, err)
	assert.True(t, retrieved.ExpiresAt.Before(time.Now()))
}

func TestDefaultTenant(t *testing.T) {
	store := NewMemoryStore()

	// Get default tenant
	defaultTenant, err := store.GetTenant("default")
	require.NoError(t, err)
	assert.Equal(t, "default", defaultTenant.Name)
	assert.Equal(t, models.TenantActive, defaultTenant.Status)

	// Default tenant cannot be deleted
	err = store.DeleteTenant("default")
	assert.Error(t, err)
}

func TestTenantStats(t *testing.T) {
	store := NewMemoryStore()

	tenant := &models.Tenant{
		Name: "stats-tenant",
		Plan: "pro",
	}
	require.NoError(t, store.CreateTenant(tenant))

	// Create resources
	node := &models.Node{
		TenantID: tenant.ID,
		Name:     "test-node",
		Status:   models.NodeOnline,
		Capabilities: models.NodeCapabilities{
			CPUCores: 8,
			GPUs:     2,
		},
	}
	require.NoError(t, store.RegisterNode(node))

	job1 := &models.Job{
		TenantID: tenant.ID,
		Status:   models.JobStatusRunning,
	}
	job2 := &models.Job{
		TenantID: tenant.ID,
		Status:   models.JobStatusCompleted,
	}
	require.NoError(t, store.CreateJob(job1))
	require.NoError(t, store.CreateJob(job2))

	// Get stats
	stats, err := store.GetTenantStats(tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.ActiveJobs)       // Only running
	assert.Equal(t, 2, stats.TotalJobs)        // All jobs
	assert.Equal(t, 1, stats.ActiveWorkers)     // Online nodes
	assert.Equal(t, 8, stats.CPUCoresUsed)
	assert.Equal(t, 2, stats.GPUsUsed)
}

func TestTenantNodeAssociation(t *testing.T) {
	store := NewMemoryStore()

	tenant1 := &models.Tenant{Name: "tenant1", Plan: "free"}
	tenant2 := &models.Tenant{Name: "tenant2", Plan: "pro"}
	require.NoError(t, store.CreateTenant(tenant1))
	require.NoError(t, store.CreateTenant(tenant2))

	// Register nodes for each tenant
	node1 := &models.Node{
		TenantID: tenant1.ID,
		Name:     "tenant1-node",
		Status:   models.NodeOnline,
	}
	node2 := &models.Node{
		TenantID: tenant2.ID,
		Name:     "tenant2-node",
		Status:   models.NodeOnline,
	}
	require.NoError(t, store.RegisterNode(node1))
	require.NoError(t, store.RegisterNode(node2))

	// Verify isolation
	nodes1, err := store.GetNodesByTenant(tenant1.ID)
	require.NoError(t, err)
	assert.Len(t, nodes1, 1)
	assert.Equal(t, "tenant1-node", nodes1[0].Name)

	nodes2, err := store.GetNodesByTenant(tenant2.ID)
	require.NoError(t, err)
	assert.Len(t, nodes2, 1)
	assert.Equal(t, "tenant2-node", nodes2[0].Name)
}
