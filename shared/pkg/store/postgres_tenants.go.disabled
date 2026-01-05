package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// CreateTenant creates a new tenant
func (s *PostgreSQLStore) CreateTenant(tenant *models.Tenant) error {
	if err := tenant.Validate(); err != nil {
		return fmt.Errorf("invalid tenant: %w", err)
	}

	quotasJSON, err := json.Marshal(tenant.Quotas)
	if err != nil {
		return fmt.Errorf("failed to marshal quotas: %w", err)
	}

	usageJSON, err := json.Marshal(tenant.Usage)
	if err != nil {
		return fmt.Errorf("failed to marshal usage: %w", err)
	}

	metadataJSON, err := json.Marshal(tenant.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO tenants (
			id, name, display_name, status, plan, 
			quotas, usage, metadata, created_at, updated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = s.db.Exec(query,
		tenant.ID,
		tenant.Name,
		tenant.DisplayName,
		tenant.Status,
		tenant.Plan,
		quotasJSON,
		usageJSON,
		metadataJSON,
		tenant.CreatedAt,
		tenant.UpdatedAt,
		tenant.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	return nil
}

// GetTenant retrieves a tenant by ID
func (s *PostgreSQLStore) GetTenant(id string) (*models.Tenant, error) {
	query := `
		SELECT id, name, display_name, status, plan,
			   quotas, usage, metadata, created_at, updated_at, expires_at
		FROM tenants
		WHERE id = $1
	`

	var tenant models.Tenant
	var quotasJSON, usageJSON, metadataJSON []byte

	err := s.db.QueryRow(query, id).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.DisplayName,
		&tenant.Status,
		&tenant.Plan,
		&quotasJSON,
		&usageJSON,
		&metadataJSON,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
		&tenant.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(quotasJSON, &tenant.Quotas); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quotas: %w", err)
	}
	if err := json.Unmarshal(usageJSON, &tenant.Usage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal usage: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &tenant.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &tenant, nil
}

// GetTenantByName retrieves a tenant by name
func (s *PostgreSQLStore) GetTenantByName(name string) (*models.Tenant, error) {
	query := `
		SELECT id, name, display_name, status, plan,
			   quotas, usage, metadata, created_at, updated_at, expires_at
		FROM tenants
		WHERE name = $1
	`

	var tenant models.Tenant
	var quotasJSON, usageJSON, metadataJSON []byte

	err := s.db.QueryRow(query, name).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.DisplayName,
		&tenant.Status,
		&tenant.Plan,
		&quotasJSON,
		&usageJSON,
		&metadataJSON,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
		&tenant.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tenant not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant by name: %w", err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(quotasJSON, &tenant.Quotas); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quotas: %w", err)
	}
	if err := json.Unmarshal(usageJSON, &tenant.Usage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal usage: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &tenant.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &tenant, nil
}

// ListTenants retrieves all tenants
func (s *PostgreSQLStore) ListTenants() ([]*models.Tenant, error) {
	query := `
		SELECT id, name, display_name, status, plan,
			   quotas, usage, metadata, created_at, updated_at, expires_at
		FROM tenants
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*models.Tenant

	for rows.Next() {
		var tenant models.Tenant
		var quotasJSON, usageJSON, metadataJSON []byte

		err := rows.Scan(
			&tenant.ID,
			&tenant.Name,
			&tenant.DisplayName,
			&tenant.Status,
			&tenant.Plan,
			&quotasJSON,
			&usageJSON,
			&metadataJSON,
			&tenant.CreatedAt,
			&tenant.UpdatedAt,
			&tenant.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(quotasJSON, &tenant.Quotas); err != nil {
			return nil, fmt.Errorf("failed to unmarshal quotas: %w", err)
		}
		if err := json.Unmarshal(usageJSON, &tenant.Usage); err != nil {
			return nil, fmt.Errorf("failed to unmarshal usage: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &tenant.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		tenants = append(tenants, &tenant)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tenants: %w", err)
	}

	return tenants, nil
}

// UpdateTenant updates an existing tenant
func (s *PostgreSQLStore) UpdateTenant(tenant *models.Tenant) error {
	if err := tenant.Validate(); err != nil {
		return fmt.Errorf("invalid tenant: %w", err)
	}

	quotasJSON, err := json.Marshal(tenant.Quotas)
	if err != nil {
		return fmt.Errorf("failed to marshal quotas: %w", err)
	}

	usageJSON, err := json.Marshal(tenant.Usage)
	if err != nil {
		return fmt.Errorf("failed to marshal usage: %w", err)
	}

	metadataJSON, err := json.Marshal(tenant.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE tenants
		SET name = $2, display_name = $3, status = $4, plan = $5,
		    quotas = $6, usage = $7, metadata = $8, updated_at = $9, expires_at = $10
		WHERE id = $1
	`

	result, err := s.db.Exec(query,
		tenant.ID,
		tenant.Name,
		tenant.DisplayName,
		tenant.Status,
		tenant.Plan,
		quotasJSON,
		usageJSON,
		metadataJSON,
		tenant.UpdatedAt,
		tenant.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant not found: %s", tenant.ID)
	}

	return nil
}

// DeleteTenant deletes a tenant (soft delete by setting status to deleted)
func (s *PostgreSQLStore) DeleteTenant(id string) error {
	query := `
		UPDATE tenants
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := s.db.Exec(query, models.TenantStatusDeleted, id)
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant not found: %s", id)
	}

	return nil
}

// UpdateTenantUsage updates the usage statistics for a tenant
func (s *PostgreSQLStore) UpdateTenantUsage(id string, usage *models.TenantUsage) error {
	usageJSON, err := json.Marshal(usage)
	if err != nil {
		return fmt.Errorf("failed to marshal usage: %w", err)
	}

	query := `
		UPDATE tenants
		SET usage = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := s.db.Exec(query, usageJSON, id)
	if err != nil {
		return fmt.Errorf("failed to update tenant usage: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant not found: %s", id)
	}

	return nil
}

// GetTenantStats retrieves usage statistics for a tenant
func (s *PostgreSQLStore) GetTenantStats(id string) (*models.TenantUsage, error) {
	query := `SELECT usage FROM tenants WHERE id = $1`

	var usageJSON []byte
	err := s.db.QueryRow(query, id).Scan(&usageJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant stats: %w", err)
	}

	var usage models.TenantUsage
	if err := json.Unmarshal(usageJSON, &usage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal usage: %w", err)
	}

	return &usage, nil
}

// GetJobsByTenant retrieves all jobs for a specific tenant
func (s *PostgreSQLStore) GetJobsByTenant(tenantID string) ([]*models.Job, error) {
	query := `
		SELECT id, sequence_number, tenant_id, scenario, confidence, engine, parameters,
		       status, queue, priority, progress, node_id, created_at, started_at,
		       last_activity_at, completed_at, retry_count, max_retries, retry_reason,
		       error, failure_reason, logs, state_transitions
		FROM jobs
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs by tenant: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating jobs: %w", err)
	}

	return jobs, nil
}

// GetNodesByTenant retrieves all nodes for a specific tenant
func (s *PostgreSQLStore) GetNodesByTenant(tenantID string) ([]*models.Node, error) {
	query := `
		SELECT id, name, tenant_id, address, type, cpu_threads, cpu_model, cpu_load_percent,
		       has_gpu, gpu_type, gpu_capabilities, ram_total_bytes, ram_free_bytes,
		       labels, status, last_heartbeat, registered_at, current_job_id
		FROM nodes
		WHERE tenant_id = $1
		ORDER BY registered_at DESC
	`

	rows, err := s.db.Query(query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by tenant: %w", err)
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		var node models.Node
		var tenantID sql.NullString
		var labelsJSON, gpuCapsJSON []byte

		if err := rows.Scan(&node.ID, &node.Name, &tenantID, &node.Address, &node.Type, &node.CPUThreads,
			&node.CPUModel, &node.CPULoadPercent, &node.HasGPU, &node.GPUType, &gpuCapsJSON,
			&node.RAMTotalBytes, &node.RAMFreeBytes, &labelsJSON, &node.Status,
			&node.LastHeartbeat, &node.RegisteredAt, &node.CurrentJobID); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		if tenantID.Valid {
			node.TenantID = tenantID.String
		}

		if len(labelsJSON) > 0 {
			json.Unmarshal(labelsJSON, &node.Labels)
		}

		if len(gpuCapsJSON) > 0 && string(gpuCapsJSON) != "null" {
			json.Unmarshal(gpuCapsJSON, &node.GPUCapabilities)
		}

		nodes = append(nodes, &node)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating nodes: %w", err)
	}

	return nodes, nil
}
