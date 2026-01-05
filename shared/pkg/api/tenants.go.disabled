package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// Tenant API handlers

// CreateTenant creates a new tenant
func (h *MasterHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string                 `json:"name"`
		DisplayName string                 `json:"display_name,omitempty"`
		Plan        string                 `json:"plan"` // free, basic, pro, enterprise
		ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "tenant name is required", http.StatusBadRequest)
		return
	}

	if req.Plan == "" {
		req.Plan = "free"
	}

	// Check if tenant already exists
	existing, _ := h.store.GetTenantByName(req.Name)
	if existing != nil {
		http.Error(w, "tenant with this name already exists", http.StatusConflict)
		return
	}

	// Create tenant
	tenant := models.NewTenant(uuid.New().String(), req.Name, req.Plan)
	if req.DisplayName != "" {
		tenant.DisplayName = req.DisplayName
	}
	tenant.ExpiresAt = req.ExpiresAt
	if req.Metadata != nil {
		tenant.Metadata = req.Metadata
	}

	if err := h.store.CreateTenant(tenant); err != nil {
		log.Printf("Failed to create tenant: %v", err)
		http.Error(w, "failed to create tenant", http.StatusInternalServerError)
		return
	}

	log.Printf("Tenant created: %s (plan: %s)", tenant.Name, tenant.Plan)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tenant)
}

// GetTenant retrieves a tenant by ID
func (h *MasterHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenantID := vars["id"]

	tenant, err := h.store.GetTenant(tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenant)
}

// ListTenants retrieves all tenants
func (h *MasterHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.store.ListTenants()
	if err != nil {
		log.Printf("Failed to list tenants: %v", err)
		http.Error(w, "failed to list tenants", http.StatusInternalServerError)
		return
	}

	if tenants == nil {
		tenants = []*models.Tenant{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenants)
}

// UpdateTenant updates an existing tenant
func (h *MasterHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenantID := vars["id"]

	// Get existing tenant
	tenant, err := h.store.GetTenant(tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	var req struct {
		DisplayName *string                 `json:"display_name,omitempty"`
		Status      *models.TenantStatus    `json:"status,omitempty"`
		Plan        *string                 `json:"plan,omitempty"`
		Quotas      *models.TenantQuotas    `json:"quotas,omitempty"`
		Metadata    *map[string]interface{} `json:"metadata,omitempty"`
		ExpiresAt   *time.Time              `json:"expires_at,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields if provided
	updated := false
	if req.DisplayName != nil {
		tenant.DisplayName = *req.DisplayName
		updated = true
	}
	if req.Status != nil {
		tenant.Status = *req.Status
		updated = true
	}
	if req.Plan != nil {
		tenant.Plan = *req.Plan
		// Update quotas based on new plan
		tenant.Quotas = models.DefaultQuotas(*req.Plan)
		updated = true
	}
	if req.Quotas != nil {
		tenant.Quotas = *req.Quotas
		updated = true
	}
	if req.Metadata != nil {
		tenant.Metadata = *req.Metadata
		updated = true
	}
	if req.ExpiresAt != nil {
		tenant.ExpiresAt = req.ExpiresAt
		updated = true
	}

	if !updated {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}

	tenant.UpdatedAt = time.Now()

	if err := h.store.UpdateTenant(tenant); err != nil {
		log.Printf("Failed to update tenant: %v", err)
		http.Error(w, "failed to update tenant", http.StatusInternalServerError)
		return
	}

	log.Printf("Tenant updated: %s", tenant.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenant)
}

// DeleteTenant deletes a tenant (soft delete)
func (h *MasterHandler) DeleteTenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenantID := vars["id"]

	// Cannot delete default tenant
	if tenantID == "default" {
		http.Error(w, "cannot delete default tenant", http.StatusForbidden)
		return
	}

	if err := h.store.DeleteTenant(tenantID); err != nil {
		log.Printf("Failed to delete tenant: %v", err)
		http.Error(w, "failed to delete tenant", http.StatusInternalServerError)
		return
	}

	log.Printf("Tenant deleted: %s", tenantID)

	w.WriteHeader(http.StatusNoContent)
}

// GetTenantStats retrieves usage statistics for a tenant
func (h *MasterHandler) GetTenantStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenantID := vars["id"]

	// Get tenant to verify it exists
	tenant, err := h.store.GetTenant(tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	// Get current stats
	stats, err := h.store.GetTenantStats(tenantID)
	if err != nil {
		log.Printf("Failed to get tenant stats: %v", err)
		http.Error(w, "failed to get tenant stats", http.StatusInternalServerError)
		return
	}

	// Combine with quota information
	response := map[string]interface{}{
		"tenant_id": tenant.ID,
		"name":      tenant.Name,
		"plan":      tenant.Plan,
		"status":    tenant.Status,
		"quotas":    tenant.Quotas,
		"usage":     stats,
		"limits": map[string]interface{}{
			"jobs_available":        tenant.Quotas.MaxJobs - stats.ActiveJobs,
			"workers_available":     tenant.Quotas.MaxWorkers - stats.ActiveWorkers,
			"cpu_cores_available":   tenant.Quotas.MaxCPUCores - stats.CPUCoresUsed,
			"gpus_available":        tenant.Quotas.MaxGPUs - stats.GPUsUsed,
			"jobs_this_hour_remaining": tenant.Quotas.MaxJobsPerHour - stats.JobsThisHour,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTenantJobs retrieves all jobs for a tenant
func (h *MasterHandler) GetTenantJobs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenantID := vars["id"]

	// Verify tenant exists
	if _, err := h.store.GetTenant(tenantID); err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	jobs, err := h.store.GetJobsByTenant(tenantID)
	if err != nil {
		log.Printf("Failed to get tenant jobs: %v", err)
		http.Error(w, "failed to get tenant jobs", http.StatusInternalServerError)
		return
	}

	if jobs == nil {
		jobs = []*models.Job{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// GetTenantNodes retrieves all nodes for a tenant
func (h *MasterHandler) GetTenantNodes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tenantID := vars["id"]

	// Verify tenant exists
	if _, err := h.store.GetTenant(tenantID); err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	nodes, err := h.store.GetNodesByTenant(tenantID)
	if err != nil {
		log.Printf("Failed to get tenant nodes: %v", err)
		http.Error(w, "failed to get tenant nodes", http.StatusInternalServerError)
		return
	}

	if nodes == nil {
		nodes = []*models.Node{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}
