package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// PostgreSQLStore implements Store interface using PostgreSQL
type PostgreSQLStore struct {
	db *sql.DB
	mu sync.Mutex // For sequence number generation
}

// NewPostgreSQLStore creates a new PostgreSQL store
func NewPostgreSQLStore(config Config) (*PostgreSQLStore, error) {
	dsn := config.DSN
	if dsn == "" {
		return nil, fmt.Errorf("PostgreSQL DSN is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25) // Default
	}

	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5) // Default
	}

	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute) // Default
	}

	if config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	} else {
		db.SetConnMaxIdleTime(1 * time.Minute) // Default
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgreSQLStore{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates tables if they don't exist
func (s *PostgreSQLStore) initSchema() error {
	schema := `
	-- Tenants table (multi-tenancy support)
	CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		plan TEXT NOT NULL DEFAULT 'free',
		quotas JSONB NOT NULL,
		usage JSONB NOT NULL,
		metadata JSONB,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
	CREATE INDEX IF NOT EXISTS idx_tenants_name ON tenants(name);

	-- Nodes table
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		tenant_id TEXT,
		address TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL,
		cpu_threads INTEGER NOT NULL,
		cpu_model TEXT NOT NULL,
		cpu_load_percent REAL NOT NULL DEFAULT 0.0,
		has_gpu BOOLEAN NOT NULL DEFAULT false,
		gpu_type TEXT,
		gpu_capabilities JSONB,
		ram_total_bytes BIGINT NOT NULL DEFAULT 0,
		ram_free_bytes BIGINT NOT NULL DEFAULT 0,
		labels JSONB,
		status TEXT NOT NULL,
		last_heartbeat TIMESTAMP NOT NULL,
		registered_at TIMESTAMP NOT NULL,
		current_job_id TEXT,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
	CREATE INDEX IF NOT EXISTS idx_nodes_address ON nodes(address);
	CREATE INDEX IF NOT EXISTS idx_nodes_tenant_id ON nodes(tenant_id);

	-- Jobs table
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		sequence_number INTEGER NOT NULL UNIQUE,
		tenant_id TEXT,
		scenario TEXT NOT NULL,
		confidence TEXT,
		engine TEXT NOT NULL DEFAULT 'auto',
		parameters JSONB,
		status TEXT NOT NULL,
		queue TEXT DEFAULT 'default',
		priority TEXT DEFAULT 'medium',
		progress INTEGER DEFAULT 0,
		node_id TEXT,
		created_at TIMESTAMP NOT NULL,
		started_at TIMESTAMP,
		last_activity_at TIMESTAMP,
		completed_at TIMESTAMP,
		retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER DEFAULT 3,
		retry_reason TEXT,
		error TEXT,
		failure_reason TEXT,
		logs TEXT,
		state_transitions JSONB,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_sequence ON jobs(sequence_number);
	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_queue_priority ON jobs(queue, priority, created_at);
	CREATE INDEX IF NOT EXISTS idx_jobs_tenant_id ON jobs(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_jobs_tenant_status ON jobs(tenant_id, status);

	-- Create default tenant if none exists
	INSERT INTO tenants (id, name, display_name, status, plan, quotas, usage, metadata, created_at, updated_at)
	VALUES (
		'default',
		'default',
		'Default Tenant',
		'active',
		'enterprise',
		'{"max_jobs": 1000, "max_workers": 100, "max_jobs_per_hour": 10000, "max_cpu_cores": 256, "max_gpus": 16, "max_storage_gb": 10000, "max_bandwidth_mbps": 100000, "job_timeout_minutes": 1440}'::jsonb,
		'{"active_jobs": 0, "total_jobs": 0, "completed_jobs": 0, "failed_jobs": 0, "active_workers": 0, "cpu_cores_used": 0, "gpus_used": 0, "storage_used_gb": 0, "jobs_this_hour": 0}'::jsonb,
		'{}'::jsonb,
		NOW(),
		NOW()
	)
	ON CONFLICT (id) DO NOTHING;
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *PostgreSQLStore) Close() error {
	return s.db.Close()
}

// HealthCheck verifies database connectivity
func (s *PostgreSQLStore) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.db.PingContext(ctx)
}

// RegisterNode adds or updates a node in the store
func (s *PostgreSQLStore) RegisterNode(node *models.Node) error {
	labels, err := json.Marshal(node.Labels)
	if err != nil {
		return fmt.Errorf("failed to marshal labels: %w", err)
	}

	gpuCaps, err := json.Marshal(node.GPUCapabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal gpu_capabilities: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO nodes 
		(id, name, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type, 
		 gpu_capabilities, ram_total_bytes, ram_free_bytes, labels, status, last_heartbeat, 
		 registered_at, current_job_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			address = EXCLUDED.address,
			type = EXCLUDED.type,
			cpu_threads = EXCLUDED.cpu_threads,
			cpu_model = EXCLUDED.cpu_model,
			cpu_load_percent = EXCLUDED.cpu_load_percent,
			has_gpu = EXCLUDED.has_gpu,
			gpu_type = EXCLUDED.gpu_type,
			gpu_capabilities = EXCLUDED.gpu_capabilities,
			ram_total_bytes = EXCLUDED.ram_total_bytes,
			ram_free_bytes = EXCLUDED.ram_free_bytes,
			labels = EXCLUDED.labels,
			status = EXCLUDED.status,
			last_heartbeat = EXCLUDED.last_heartbeat,
			current_job_id = EXCLUDED.current_job_id
	`, node.ID, node.Name, node.Address, node.Type, node.CPUThreads, node.CPUModel, node.CPULoadPercent,
		node.HasGPU, node.GPUType, string(gpuCaps), node.RAMTotalBytes, node.RAMFreeBytes,
		string(labels), node.Status, node.LastHeartbeat, node.RegisteredAt, node.CurrentJobID)

	return err
}

// GetNode retrieves a node by ID
func (s *PostgreSQLStore) GetNode(id string) (*models.Node, error) {
	var node models.Node
	var labelsJSON, gpuCapsJSON []byte

	err := s.db.QueryRow(`
		SELECT id, name, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type,
		       gpu_capabilities, ram_total_bytes, ram_free_bytes, labels, status, last_heartbeat,
		       registered_at, current_job_id
		FROM nodes WHERE id = $1
	`, id).Scan(&node.ID, &node.Name, &node.Address, &node.Type, &node.CPUThreads, &node.CPUModel,
		&node.CPULoadPercent, &node.HasGPU, &node.GPUType, &gpuCapsJSON, &node.RAMTotalBytes,
		&node.RAMFreeBytes, &labelsJSON, &node.Status, &node.LastHeartbeat,
		&node.RegisteredAt, &node.CurrentJobID)

	if err == sql.ErrNoRows {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &node.Labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}
	}

	if len(gpuCapsJSON) > 0 && string(gpuCapsJSON) != "null" {
		if err := json.Unmarshal(gpuCapsJSON, &node.GPUCapabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal gpu_capabilities: %w", err)
		}
	}

	return &node, nil
}

// GetNodeByAddress retrieves a node by address
func (s *PostgreSQLStore) GetNodeByAddress(address string) (*models.Node, error) {
	var node models.Node
	var labelsJSON, gpuCapsJSON []byte

	err := s.db.QueryRow(`
		SELECT id, name, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type,
		       gpu_capabilities, ram_total_bytes, ram_free_bytes, labels, status, last_heartbeat,
		       registered_at, current_job_id
		FROM nodes WHERE address = $1
	`, address).Scan(&node.ID, &node.Name, &node.Address, &node.Type, &node.CPUThreads, &node.CPUModel,
		&node.CPULoadPercent, &node.HasGPU, &node.GPUType, &gpuCapsJSON, &node.RAMTotalBytes,
		&node.RAMFreeBytes, &labelsJSON, &node.Status, &node.LastHeartbeat,
		&node.RegisteredAt, &node.CurrentJobID)

	if err == sql.ErrNoRows {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(labelsJSON) > 0 {
		if err := json.Unmarshal(labelsJSON, &node.Labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}
	}

	if len(gpuCapsJSON) > 0 && string(gpuCapsJSON) != "null" {
		if err := json.Unmarshal(gpuCapsJSON, &node.GPUCapabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal gpu_capabilities: %w", err)
		}
	}

	return &node, nil
}

// GetAllNodes returns all registered nodes
func (s *PostgreSQLStore) GetAllNodes() []*models.Node {
	rows, err := s.db.Query(`
		SELECT id, name, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type,
		       gpu_capabilities, ram_total_bytes, ram_free_bytes, labels, status, last_heartbeat,
		       registered_at, current_job_id
		FROM nodes
	`)
	if err != nil {
		return []*models.Node{}
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		var node models.Node
		var labelsJSON, gpuCapsJSON []byte

		if err := rows.Scan(&node.ID, &node.Name, &node.Address, &node.Type, &node.CPUThreads,
			&node.CPUModel, &node.CPULoadPercent, &node.HasGPU, &node.GPUType, &gpuCapsJSON,
			&node.RAMTotalBytes, &node.RAMFreeBytes, &labelsJSON, &node.Status,
			&node.LastHeartbeat, &node.RegisteredAt, &node.CurrentJobID); err != nil {
			continue
		}

		if len(labelsJSON) > 0 {
			json.Unmarshal(labelsJSON, &node.Labels)
		}

		if len(gpuCapsJSON) > 0 && string(gpuCapsJSON) != "null" {
			json.Unmarshal(gpuCapsJSON, &node.GPUCapabilities)
		}

		nodes = append(nodes, &node)
	}

	return nodes
}

// UpdateNodeStatus updates the status of a node
func (s *PostgreSQLStore) UpdateNodeStatus(id, status string) error {
	result, err := s.db.Exec(`
		UPDATE nodes SET status = $1, last_heartbeat = $2 WHERE id = $3
	`, status, time.Now(), id)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNodeNotFound
	}

	return nil
}

// UpdateNodeHeartbeat updates the last heartbeat time for a node
func (s *PostgreSQLStore) UpdateNodeHeartbeat(id string) error {
	result, err := s.db.Exec(`
		UPDATE nodes SET last_heartbeat = $1 WHERE id = $2
	`, time.Now(), id)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNodeNotFound
	}

	return nil
}

// DeleteNode removes a node from the store
func (s *PostgreSQLStore) DeleteNode(id string) error {
	result, err := s.db.Exec(`
		DELETE FROM nodes WHERE id = $1
	`, id)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNodeNotFound
	}

	return nil
}

// CreateJob adds a new job to the store
func (s *PostgreSQLStore) CreateJob(job *models.Job) error {
	params, err := json.Marshal(job.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	transitions, err := json.Marshal(job.StateTransitions)
	if err != nil {
		return fmt.Errorf("failed to marshal state_transitions: %w", err)
	}

	// Set defaults
	if job.Queue == "" {
		job.Queue = "default"
	}
	if job.Priority == "" {
		job.Priority = "medium"
	}
	if job.Engine == "" {
		job.Engine = "auto"
	}

	// Generate sequence number if not set (protected by mutex for concurrency)
	needsSequenceNumber := job.SequenceNumber == 0
	if needsSequenceNumber {
		s.mu.Lock()
		defer s.mu.Unlock()

		var maxSeq sql.NullInt64
		err := s.db.QueryRow("SELECT MAX(sequence_number) FROM jobs").Scan(&maxSeq)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get max sequence number: %w", err)
		}
		if maxSeq.Valid {
			job.SequenceNumber = int(maxSeq.Int64) + 1
		} else {
			job.SequenceNumber = 1
		}
	}

	_, err = s.db.Exec(`
		INSERT INTO jobs 
		(id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id, 
		 created_at, started_at, last_activity_at, completed_at, retry_count, error, failure_reason, logs, state_transitions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`, job.ID, job.SequenceNumber, job.Scenario, job.Confidence, job.Engine, string(params), job.Status, job.Queue,
		job.Priority, job.Progress, job.NodeID, job.CreatedAt, job.StartedAt, job.LastActivityAt,
		job.CompletedAt, job.RetryCount, job.Error, string(job.FailureReason), job.Logs, string(transitions))

	return err
}

// Implement remaining methods in next file...
