package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// SQLiteStore is a SQLite-based implementation of the data store
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Configure SQLite connection string with parameters for concurrent access
	// - _journal_mode=WAL: Enable Write-Ahead Logging for better concurrency
	// - _busy_timeout=10000: Wait up to 10 seconds when database is locked
	// - _synchronous=NORMAL: Balance between safety and performance
	// - _cache_size=-8000: 8MB memory cache for better performance
	// - _txlock=immediate: Acquire write lock at transaction start to reduce conflicts
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_cache_size=-8000&_txlock=immediate", dbPath)
	
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool limits to prevent too many concurrent writes
	// Single writer for SQLite to avoid lock contention
	db.SetMaxOpenConns(1)  // Serialize writes to avoid SQLITE_BUSY
	db.SetMaxIdleConns(1)  // Keep one connection ready
	db.SetConnMaxLifetime(30 * time.Minute)

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Explicitly enable WAL mode via PRAGMA
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	
	// Set busy timeout via PRAGMA as well
	if _, err := db.Exec("PRAGMA busy_timeout=10000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	return store, nil
}

// initSchema creates the database schema
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		address TEXT NOT NULL,
		type TEXT NOT NULL,
		cpu_threads INTEGER NOT NULL,
		cpu_model TEXT NOT NULL,
		has_gpu BOOLEAN NOT NULL,
		gpu_type TEXT,
		ram_bytes INTEGER NOT NULL,
		labels TEXT,
		status TEXT NOT NULL,
		last_heartbeat DATETIME NOT NULL,
		registered_at DATETIME NOT NULL,
		current_job_id TEXT
	);

	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		scenario TEXT NOT NULL,
		confidence TEXT,
		parameters TEXT,
		status TEXT NOT NULL,
		node_id TEXT,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		retry_count INTEGER NOT NULL,
		error TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
	`

	_, err := s.db.Exec(schema)
	return err
}

// RegisterNode adds or updates a node in the store
func (s *SQLiteStore) RegisterNode(node *models.Node) error {
	labels, err := json.Marshal(node.Labels)
	if err != nil {
		return fmt.Errorf("failed to marshal labels: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO nodes 
		(id, address, type, cpu_threads, cpu_model, has_gpu, gpu_type, ram_bytes, 
		 labels, status, last_heartbeat, registered_at, current_job_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, node.ID, node.Address, node.Type, node.CPUThreads, node.CPUModel,
		node.HasGPU, node.GPUType, node.RAMBytes, string(labels),
		node.Status, node.LastHeartbeat, node.RegisteredAt, node.CurrentJobID)

	return err
}

// GetNode retrieves a node by ID
func (s *SQLiteStore) GetNode(id string) (*models.Node, error) {
	var node models.Node
	var labelsJSON string

	err := s.db.QueryRow(`
		SELECT id, address, type, cpu_threads, cpu_model, has_gpu, gpu_type, ram_bytes,
		       labels, status, last_heartbeat, registered_at, current_job_id
		FROM nodes WHERE id = ?
	`, id).Scan(&node.ID, &node.Address, &node.Type, &node.CPUThreads, &node.CPUModel,
		&node.HasGPU, &node.GPUType, &node.RAMBytes, &labelsJSON,
		&node.Status, &node.LastHeartbeat, &node.RegisteredAt, &node.CurrentJobID)

	if err == sql.ErrNoRows {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
		return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
	}

	return &node, nil
}

// GetAllNodes returns all registered nodes
func (s *SQLiteStore) GetAllNodes() []*models.Node {
	rows, err := s.db.Query(`
		SELECT id, address, type, cpu_threads, cpu_model, has_gpu, gpu_type, ram_bytes,
		       labels, status, last_heartbeat, registered_at, current_job_id
		FROM nodes
	`)
	if err != nil {
		return []*models.Node{}
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		var node models.Node
		var labelsJSON string

		if err := rows.Scan(&node.ID, &node.Address, &node.Type, &node.CPUThreads,
			&node.CPUModel, &node.HasGPU, &node.GPUType, &node.RAMBytes, &labelsJSON,
			&node.Status, &node.LastHeartbeat, &node.RegisteredAt, &node.CurrentJobID); err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
			continue
		}

		nodes = append(nodes, &node)
	}

	return nodes
}

// UpdateNodeStatus updates the status of a node
func (s *SQLiteStore) UpdateNodeStatus(id, status string) error {
	result, err := s.db.Exec(`
		UPDATE nodes SET status = ?, last_heartbeat = ? WHERE id = ?
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
func (s *SQLiteStore) UpdateNodeHeartbeat(id string) error {
	result, err := s.db.Exec(`
		UPDATE nodes SET last_heartbeat = ? WHERE id = ?
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

// CreateJob adds a new job to the store
func (s *SQLiteStore) CreateJob(job *models.Job) error {
	params, err := json.Marshal(job.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO jobs 
		(id, scenario, confidence, parameters, status, node_id, created_at, 
		 started_at, completed_at, retry_count, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.Scenario, job.Confidence, string(params), job.Status,
		job.NodeID, job.CreatedAt, job.StartedAt, job.CompletedAt,
		job.RetryCount, job.Error)

	return err
}

// GetJob retrieves a job by ID
func (s *SQLiteStore) GetJob(id string) (*models.Job, error) {
	var job models.Job
	var paramsJSON sql.NullString
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, scenario, confidence, parameters, status, node_id, created_at,
		       started_at, completed_at, retry_count, error
		FROM jobs WHERE id = ?
	`, id).Scan(&job.ID, &job.Scenario, &job.Confidence, &paramsJSON, &job.Status,
		&job.NodeID, &job.CreatedAt, &startedAt, &completedAt, &job.RetryCount, &job.Error)

	if err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	if paramsJSON.Valid {
		if err := json.Unmarshal([]byte(paramsJSON.String), &job.Parameters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
		}
	}

	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return &job, nil
}

// GetAllJobs returns all jobs
func (s *SQLiteStore) GetAllJobs() []*models.Job {
	rows, err := s.db.Query(`
		SELECT id, scenario, confidence, parameters, status, node_id, created_at,
		       started_at, completed_at, retry_count, error
		FROM jobs ORDER BY created_at DESC
	`)
	if err != nil {
		return []*models.Job{}
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var paramsJSON sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(&job.ID, &job.Scenario, &job.Confidence, &paramsJSON,
			&job.Status, &job.NodeID, &job.CreatedAt, &startedAt, &completedAt,
			&job.RetryCount, &job.Error); err != nil {
			continue
		}

		if paramsJSON.Valid {
			if err := json.Unmarshal([]byte(paramsJSON.String), &job.Parameters); err != nil {
				continue
			}
		}

		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		jobs = append(jobs, &job)
	}

	return jobs
}

// GetNextJob retrieves the next pending job from the queue
func (s *SQLiteStore) GetNextJob(nodeID string) (*models.Job, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get first pending job
	var job models.Job
	var paramsJSON sql.NullString

	err = tx.QueryRow(`
		SELECT id, scenario, confidence, parameters, status, node_id, created_at,
		       started_at, completed_at, retry_count, error
		FROM jobs 
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1
	`, models.JobStatusPending).Scan(&job.ID, &job.Scenario, &job.Confidence,
		&paramsJSON, &job.Status, &job.NodeID, &job.CreatedAt,
		&job.StartedAt, &job.CompletedAt, &job.RetryCount, &job.Error)

	if err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	if paramsJSON.Valid {
		if err := json.Unmarshal([]byte(paramsJSON.String), &job.Parameters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
		}
	}

	// Update job status
	now := time.Now()
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = ?, node_id = ?, started_at = ?
		WHERE id = ?
	`, models.JobStatusRunning, nodeID, now, job.ID)

	if err != nil {
		return nil, err
	}

	// Update node status
	_, err = tx.Exec(`
		UPDATE nodes 
		SET status = ?, current_job_id = ?
		WHERE id = ?
	`, "busy", job.ID, nodeID)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	job.Status = models.JobStatusRunning
	job.NodeID = nodeID
	job.StartedAt = &now

	return &job, nil
}

// UpdateJobStatus updates the status of a job
func (s *SQLiteStore) UpdateJobStatus(id string, status models.JobStatus, errorMsg string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current job to find node ID
	var nodeID sql.NullString
	err = tx.QueryRow(`SELECT node_id FROM jobs WHERE id = ?`, id).Scan(&nodeID)
	if err == sql.ErrNoRows {
		return ErrJobNotFound
	}
	if err != nil {
		return err
	}

	// Update job
	now := time.Now()
	var result sql.Result
	if status == models.JobStatusCompleted || status == models.JobStatusFailed {
		result, err = tx.Exec(`
			UPDATE jobs 
			SET status = ?, error = ?, completed_at = ?
			WHERE id = ?
		`, status, errorMsg, now, id)
	} else {
		result, err = tx.Exec(`
			UPDATE jobs 
			SET status = ?, error = ?
			WHERE id = ?
		`, status, errorMsg, id)
	}

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrJobNotFound
	}

	// Update node status if job is complete
	if nodeID.Valid && (status == models.JobStatusCompleted || status == models.JobStatusFailed) {
		_, err = tx.Exec(`
			UPDATE nodes 
			SET status = ?, current_job_id = ?
			WHERE id = ?
		`, "available", "", nodeID.String)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Store interface that both MemoryStore and SQLiteStore implement
type Store interface {
	RegisterNode(node *models.Node) error
	GetNode(id string) (*models.Node, error)
	GetAllNodes() []*models.Node
	UpdateNodeStatus(id, status string) error
	UpdateNodeHeartbeat(id string) error
	CreateJob(job *models.Job) error
	GetJob(id string) (*models.Job, error)
	GetAllJobs() []*models.Job
	GetNextJob(nodeID string) (*models.Job, error)
	UpdateJobStatus(id string, status models.JobStatus, errorMsg string) error
}

// Ensure both implementations satisfy the interface
var _ Store = (*MemoryStore)(nil)
var _ Store = (*SQLiteStore)(nil)
