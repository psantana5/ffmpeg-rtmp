package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// CreateTenantRequest represents a tenant creation request
type CreateTenantRequest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// UpdateTenantRequest represents a tenant update request
type UpdateTenantRequest struct {
	DisplayName *string                `json:"display_name,omitempty"`
	Description *string                `json:"description,omitempty"`
	IsActive    *bool                  `json:"is_active,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// TenantResponse represents a tenant in API responses
type TenantResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	IsActive    bool                   `json:"is_active"`
	Config      map[string]interface{} `json:"config,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func toTenantResponse(t *models.Tenant) TenantResponse {
	return TenantResponse{
		ID:          t.ID,
		Name:        t.Name,
		DisplayName: t.DisplayName,
		Description: t.Description,
		IsActive:    t.IsActive,
		Config:      t.Config,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// HandleCreateTenant creates a new tenant
func (h *MasterHandler) HandleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Tenant name is required", http.StatusBadRequest)
		return
	}

	tenant := &models.Tenant{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		IsActive:    true,
		Config:      req.Config,
	}

	if err := h.Store.CreateTenant(tenant); err != nil {
		log.Printf("Failed to create tenant: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create tenant: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Created tenant: %s (ID: %s)", tenant.Name, tenant.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toTenantResponse(tenant))
}

// HandleGetTenant retrieves a tenant by ID or name
func (h *MasterHandler) HandleGetTenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrName := vars["id"]

	tenant, err := h.Store.GetTenant(idOrName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Tenant not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTenantResponse(tenant))
}

// HandleListTenants lists all tenants
func (h *MasterHandler) HandleListTenants(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"

	tenants, err := h.Store.ListTenants(activeOnly)
	if err != nil {
		log.Printf("Failed to list tenants: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list tenants: %v", err), http.StatusInternalServerError)
		return
	}

	response := make([]TenantResponse, len(tenants))
	for i, t := range tenants {
		response[i] = toTenantResponse(t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUpdateTenant updates a tenant
func (h *MasterHandler) HandleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrName := vars["id"]

	var req UpdateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	tenant, err := h.Store.GetTenant(idOrName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Tenant not found: %v", err), http.StatusNotFound)
		return
	}

	// Update fields if provided
	if req.DisplayName != nil {
		tenant.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		tenant.Description = *req.Description
	}
	if req.IsActive != nil {
		tenant.IsActive = *req.IsActive
	}
	if req.Config != nil {
		tenant.Config = req.Config
	}

	if err := h.Store.UpdateTenant(tenant); err != nil {
		log.Printf("Failed to update tenant: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update tenant: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Updated tenant: %s (ID: %s)", tenant.Name, tenant.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTenantResponse(tenant))
}

// HandleDeleteTenant soft-deletes a tenant
func (h *MasterHandler) HandleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrName := vars["id"]

	if err := h.Store.DeleteTenant(idOrName); err != nil {
		log.Printf("Failed to delete tenant: %v", err)
		http.Error(w, fmt.Sprintf("Failed to delete tenant: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted tenant: %s", idOrName)

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetTenantStats retrieves statistics for a tenant
func (h *MasterHandler) HandleGetTenantStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrName := vars["id"]

	tenant, err := h.Store.GetTenant(idOrName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Tenant not found: %v", err), http.StatusNotFound)
		return
	}

	stats, err := h.Store.GetTenantStats(tenant.ID)
	if err != nil {
		log.Printf("Failed to get tenant stats: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get tenant stats: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
