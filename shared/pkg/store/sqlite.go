package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// SQLiteStore is a SQLite-based implementation of the data store
type SQLiteStore struct {
	db *sql.DB
	mu sync.Mutex
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
		cpu_load_percent REAL DEFAULT 0,
		has_gpu BOOLEAN NOT NULL,
		gpu_type TEXT,
		gpu_capabilities TEXT,
		ram_total_bytes INTEGER NOT NULL,
		ram_free_bytes INTEGER DEFAULT 0,
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
		engine TEXT NOT NULL DEFAULT 'auto',
		parameters TEXT,
		status TEXT NOT NULL,
		queue TEXT DEFAULT 'default',
		priority TEXT DEFAULT 'medium',
		progress INTEGER DEFAULT 0,
		node_id TEXT,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		retry_count INTEGER NOT NULL,
		error TEXT,
		state_transitions TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_queue_priority ON jobs(queue, priority, created_at);
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

	gpuCaps, err := json.Marshal(node.GPUCapabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal gpu_capabilities: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO nodes 
		(id, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type, 
		 gpu_capabilities, ram_total_bytes, ram_free_bytes, labels, status, last_heartbeat, 
		 registered_at, current_job_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, node.ID, node.Address, node.Type, node.CPUThreads, node.CPUModel, node.CPULoadPercent,
		node.HasGPU, node.GPUType, string(gpuCaps), node.RAMTotalBytes, node.RAMFreeBytes,
		string(labels), node.Status, node.LastHeartbeat, node.RegisteredAt, node.CurrentJobID)

	return err
}

// GetNode retrieves a node by ID
func (s *SQLiteStore) GetNode(id string) (*models.Node, error) {
	var node models.Node
	var labelsJSON, gpuCapsJSON string

	err := s.db.QueryRow(`
		SELECT id, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type,
		       gpu_capabilities, ram_total_bytes, ram_free_bytes, labels, status, last_heartbeat,
		       registered_at, current_job_id
		FROM nodes WHERE id = ?
	`, id).Scan(&node.ID, &node.Address, &node.Type, &node.CPUThreads, &node.CPUModel,
		&node.CPULoadPercent, &node.HasGPU, &node.GPUType, &gpuCapsJSON, &node.RAMTotalBytes,
		&node.RAMFreeBytes, &labelsJSON, &node.Status, &node.LastHeartbeat,
		&node.RegisteredAt, &node.CurrentJobID)

	if err == sql.ErrNoRows {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
		return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
	}

	if gpuCapsJSON != "" && gpuCapsJSON != "null" {
		if err := json.Unmarshal([]byte(gpuCapsJSON), &node.GPUCapabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal gpu_capabilities: %w", err)
		}
	}

	return &node, nil
}

// GetAllNodes returns all registered nodes
func (s *SQLiteStore) GetAllNodes() []*models.Node {
	rows, err := s.db.Query(`
		SELECT id, address, type, cpu_threads, cpu_model, cpu_load_percent, has_gpu, gpu_type,
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
		var labelsJSON, gpuCapsJSON string

		if err := rows.Scan(&node.ID, &node.Address, &node.Type, &node.CPUThreads,
			&node.CPUModel, &node.CPULoadPercent, &node.HasGPU, &node.GPUType, &gpuCapsJSON,
			&node.RAMTotalBytes, &node.RAMFreeBytes, &labelsJSON, &node.Status,
			&node.LastHeartbeat, &node.RegisteredAt, &node.CurrentJobID); err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
			continue
		}

		if gpuCapsJSON != "" && gpuCapsJSON != "null" {
			if err := json.Unmarshal([]byte(gpuCapsJSON), &node.GPUCapabilities); err != nil {
				continue
			}
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

	transitions, err := json.Marshal(job.StateTransitions)
	if err != nil {
		return fmt.Errorf("failed to marshal state_transitions: %w", err)
	}

	// Set defaults for new fields
	if job.Queue == "" {
		job.Queue = "default"
	}
	if job.Priority == "" {
		job.Priority = "medium"
	}
	if job.Engine == "" {
		job.Engine = "auto"
	}

	_, err = s.db.Exec(`
		INSERT INTO jobs 
		(id, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id, 
		 created_at, started_at, completed_at, retry_count, error, state_transitions)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.Scenario, job.Confidence, job.Engine, string(params), job.Status, job.Queue,
		job.Priority, job.Progress, job.NodeID, job.CreatedAt, job.StartedAt,
		job.CompletedAt, job.RetryCount, job.Error, string(transitions))

	return err
}

// GetJob retrieves a job by ID
func (s *SQLiteStore) GetJob(id string) (*models.Job, error) {
	var job models.Job
	var paramsJSON, transitionsJSON sql.NullString
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, completed_at, retry_count, error, state_transitions
		FROM jobs WHERE id = ?
	`, id).Scan(&job.ID, &job.Scenario, &job.Confidence, &job.Engine, &paramsJSON, &job.Status,
		&job.Queue, &job.Priority, &job.Progress, &job.NodeID, &job.CreatedAt, 
		&startedAt, &completedAt, &job.RetryCount, &job.Error, &transitionsJSON)

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

	if transitionsJSON.Valid && transitionsJSON.String != "" && transitionsJSON.String != "null" {
		if err := json.Unmarshal([]byte(transitionsJSON.String), &job.StateTransitions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state_transitions: %w", err)
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
		SELECT id, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, completed_at, retry_count, error, state_transitions
		FROM jobs ORDER BY created_at DESC
	`)
	if err != nil {
		return []*models.Job{}
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var paramsJSON, transitionsJSON sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(&job.ID, &job.Scenario, &job.Confidence, &job.Engine, &paramsJSON,
			&job.Status, &job.Queue, &job.Priority, &job.Progress, &job.NodeID, &job.CreatedAt,
			&startedAt, &completedAt, &job.RetryCount, &job.Error, &transitionsJSON); err != nil {
			continue
		}

		if paramsJSON.Valid {
			if err := json.Unmarshal([]byte(paramsJSON.String), &job.Parameters); err != nil {
				continue
			}
		}

		if transitionsJSON.Valid && transitionsJSON.String != "" && transitionsJSON.String != "null" {
			if err := json.Unmarshal([]byte(transitionsJSON.String), &job.StateTransitions); err != nil {
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

// GetNextJob retrieves the next pending job from the queue with priority scheduling
// Priority order: live > default > batch, then high > medium > low, then FIFO
func (s *SQLiteStore) GetNextJob(nodeID string) (*models.Job, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get node capabilities for GPU filtering
	var node models.Node
	var gpuCapsJSON string
	err = tx.QueryRow(`
		SELECT has_gpu, gpu_capabilities FROM nodes WHERE id = ?
	`, nodeID).Scan(&node.HasGPU, &gpuCapsJSON)
	
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	if gpuCapsJSON != "" && gpuCapsJSON != "null" {
		json.Unmarshal([]byte(gpuCapsJSON), &node.GPUCapabilities)
	}

	// Select job with priority: queue (live>default>batch), priority (high>medium>low), then FIFO
	// Queue priority: live=3, default=2, batch=1
	// Priority: high=3, medium=2, low=1
	var job models.Job
	var paramsJSON, transitionsJSON sql.NullString
	var startedAt, completedAt sql.NullTime

	query := `
		SELECT id, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, completed_at, retry_count, error, state_transitions
		FROM jobs 
		WHERE status IN (?, ?)
		ORDER BY 
			CASE queue 
				WHEN 'live' THEN 3 
				WHEN 'default' THEN 2 
				WHEN 'batch' THEN 1 
				ELSE 2 
			END DESC,
			CASE priority 
				WHEN 'high' THEN 3 
				WHEN 'medium' THEN 2 
				WHEN 'low' THEN 1 
				ELSE 2 
			END DESC,
			created_at ASC
		LIMIT 1
	`

	err = tx.QueryRow(query, models.JobStatusPending, models.JobStatusQueued).Scan(
		&job.ID, &job.Scenario, &job.Confidence, &job.Engine, &paramsJSON, &job.Status, &job.Queue,
		&job.Priority, &job.Progress, &job.NodeID, &job.CreatedAt, &startedAt, &completedAt,
		&job.RetryCount, &job.Error, &transitionsJSON)

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

	if transitionsJSON.Valid && transitionsJSON.String != "" && transitionsJSON.String != "null" {
		if err := json.Unmarshal([]byte(transitionsJSON.String), &job.StateTransitions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state_transitions: %w", err)
		}
	}

	// GPU filtering: check if job requires GPU capabilities
	requiresGPU := false
	if job.Parameters != nil {
		if codec, ok := job.Parameters["codec"].(string); ok {
			requiresGPU = strings.Contains(codec, "nvenc") || strings.Contains(codec, "qsv") || strings.Contains(codec, "videotoolbox")
		}
		if hwaccel, ok := job.Parameters["hwaccel"].(string); ok && hwaccel != "none" {
			requiresGPU = true
		}
	}

	// If job requires GPU but node doesn't have one, skip this job
	if requiresGPU && !node.HasGPU {
		return nil, ErrJobNotFound // No suitable job for this node
	}

	// Update job status to assigned then processing
	now := time.Now()
	
	// Add state transition
	transition := models.StateTransition{
		From:      job.Status,
		To:        models.JobStatusAssigned,
		Timestamp: now,
		Reason:    fmt.Sprintf("Assigned to node %s", nodeID),
	}
	job.StateTransitions = append(job.StateTransitions, transition)
	
	transitionsJSON2, _ := json.Marshal(job.StateTransitions)
	
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = ?, node_id = ?, started_at = ?, state_transitions = ?
		WHERE id = ?
	`, models.JobStatusProcessing, nodeID, now, string(transitionsJSON2), job.ID)

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

	job.Status = models.JobStatusProcessing
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

// UpdateJobProgress updates the progress percentage of a job
func (s *SQLiteStore) UpdateJobProgress(id string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	result, err := s.db.Exec(`
		UPDATE jobs SET progress = ? WHERE id = ?
	`, progress, id)

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

	return nil
}

// AddStateTransition adds a state transition to a job's history
func (s *SQLiteStore) AddStateTransition(id string, from, to models.JobStatus, reason string) error {
	// Get current job
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}

	// Add new transition
	transition := models.StateTransition{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	job.StateTransitions = append(job.StateTransitions, transition)
	job.Status = to

	// Update job
	transitionsJSON, err := json.Marshal(job.StateTransitions)
	if err != nil {
		return fmt.Errorf("failed to marshal state_transitions: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE jobs SET status = ?, state_transitions = ? WHERE id = ?
	`, to, string(transitionsJSON), id)

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

	return nil
}

// PauseJob pauses a running job
func (s *SQLiteStore) PauseJob(id string) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}

	if job.Status != models.JobStatusProcessing && job.Status != models.JobStatusRunning {
		return fmt.Errorf("cannot pause job in status: %s", job.Status)
	}

	return s.AddStateTransition(id, job.Status, models.JobStatusPaused, "User requested pause")
}

// ResumeJob resumes a paused job
func (s *SQLiteStore) ResumeJob(id string) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}

	if job.Status != models.JobStatusPaused {
		return fmt.Errorf("cannot resume job in status: %s", job.Status)
	}

	return s.AddStateTransition(id, job.Status, models.JobStatusProcessing, "User requested resume")
}

// CancelJob cancels a job
func (s *SQLiteStore) CancelJob(id string) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}

	if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed || job.Status == models.JobStatusCanceled {
		return fmt.Errorf("cannot cancel job in status: %s", job.Status)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Add state transition
	err = s.AddStateTransition(id, job.Status, models.JobStatusCanceled, "User requested cancel")
	if err != nil {
		return err
	}

	// Free up node if assigned
	if job.NodeID != "" {
		_, err = tx.Exec(`
			UPDATE nodes 
			SET status = ?, current_job_id = ?
			WHERE id = ?
		`, "available", "", job.NodeID)
		if err != nil {
			return err
		}
	}

	// Set completed_at
	now := time.Now()
	_, err = tx.Exec(`
		UPDATE jobs SET completed_at = ? WHERE id = ?
	`, now, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetQueuedJobs returns jobs in a specific queue with priority filtering
func (s *SQLiteStore) GetQueuedJobs(queue string, priority string) []*models.Job {
	query := `
		SELECT id, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, completed_at, retry_count, error, state_transitions
		FROM jobs 
		WHERE status IN (?, ?) AND queue = ?
	`
	args := []interface{}{models.JobStatusPending, models.JobStatusQueued, queue}

	if priority != "" {
		query += " AND priority = ?"
		args = append(args, priority)
	}

	query += " ORDER BY created_at ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []*models.Job{}
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var paramsJSON, transitionsJSON sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(&job.ID, &job.Scenario, &job.Confidence, &job.Engine, &paramsJSON,
			&job.Status, &job.Queue, &job.Priority, &job.Progress, &job.NodeID,
			&job.CreatedAt, &startedAt, &completedAt, &job.RetryCount, &job.Error,
			&transitionsJSON); err != nil {
			continue
		}

		if paramsJSON.Valid {
			json.Unmarshal([]byte(paramsJSON.String), &job.Parameters)
		}

		if transitionsJSON.Valid && transitionsJSON.String != "" && transitionsJSON.String != "null" {
			json.Unmarshal([]byte(transitionsJSON.String), &job.StateTransitions)
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
	UpdateJobProgress(id string, progress int) error
	AddStateTransition(id string, from, to models.JobStatus, reason string) error
	PauseJob(id string) error
	ResumeJob(id string) error
	CancelJob(id string) error
	GetQueuedJobs(queue string, priority string) []*models.Job
	TryQueuePendingJob(jobID string) (bool, error)
	RetryJob(jobID string, errorMsg string) error
}

// Ensure both implementations satisfy the interface
var _ Store = (*MemoryStore)(nil)
var _ Store = (*SQLiteStore)(nil)

// TryQueuePendingJob atomically checks if a job is pending with no available workers and queues it
// Returns true if job was queued, false if already queued or picked up
func (s *SQLiteStore) TryQueuePendingJob(jobID string) (bool, error) {
s.mu.Lock()
defer s.mu.Unlock()

// Check if job is still pending
var status string
err := s.db.QueryRow("SELECT status FROM jobs WHERE id = ?", jobID).Scan(&status)
if err != nil {
return false, err
}

if status != string(models.JobStatusPending) {
return false, nil // Already processed
}

// Check if any workers are available
var availableCount int
err = s.db.QueryRow("SELECT COUNT(*) FROM nodes WHERE status = 'available'").Scan(&availableCount)
if err != nil {
return false, err
}

if availableCount > 0 {
return false, nil // Workers available, let GetNextJob handle it
}

// No workers available, queue the job
result, err := s.db.Exec("UPDATE jobs SET status = ? WHERE id = ? AND status = ?",
models.JobStatusQueued, jobID, models.JobStatusPending)
if err != nil {
return false, err
}

rows, _ := result.RowsAffected()
return rows > 0, nil
}

// RetryJob resets a failed job for retry by updating its status to pending,
// clearing node assignment, and incrementing retry count
func (s *SQLiteStore) RetryJob(jobID string, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current job to check retry count
	var retryCount int
	err = tx.QueryRow("SELECT retry_count FROM jobs WHERE id = ?", jobID).Scan(&retryCount)
	if err != nil {
		return fmt.Errorf("failed to get job for retry: %w", err)
	}

	// Update job: increment retry_count, set status to pending, clear node_id and started_at, update error
	_, err = tx.Exec(`
		UPDATE jobs
		SET status = ?,
		    retry_count = ?,
		    node_id = NULL,
		    started_at = NULL,
		    error = ?
		WHERE id = ?
	`, models.JobStatusPending, retryCount+1, errorMsg, jobID)
	
	if err != nil {
		return fmt.Errorf("failed to update job for retry: %w", err)
	}

	// Update the node that was running the job back to available
	_, err = tx.Exec(`
		UPDATE nodes 
		SET status = 'available', current_job_id = NULL 
		WHERE current_job_id = ?
	`, jobID)
	
	if err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	return tx.Commit()
}
