package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// FSM Operations for PostgreSQL Store

// TransitionJobState performs a validated state transition with idempotency
func (s *PostgreSQLStore) TransitionJobState(jobID string, toState models.JobStatus, reason string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Get current job state
	var currentStatus string
	var transitionsJSON []byte
	err = tx.QueryRow("SELECT status, state_transitions FROM jobs WHERE id = $1 FOR UPDATE", jobID).
		Scan(&currentStatus, &transitionsJSON)

	if err == sql.ErrNoRows {
		return false, ErrJobNotFound
	}
	if err != nil {
		return false, err
	}

	fromState := models.JobStatus(currentStatus)

	// Idempotency: if already in target state, no-op
	if fromState == toState {
		return false, nil
	}

	// Validate transition
	if err := models.ValidateTransition(fromState, toState); err != nil {
		return false, fmt.Errorf("invalid transition: %w", err)
	}

	// Parse existing transitions
	var transitions []models.StateTransition
	if len(transitionsJSON) > 0 && string(transitionsJSON) != "null" {
		json.Unmarshal(transitionsJSON, &transitions)
	}

	// Add new transition
	transition := models.StateTransition{
		From:      fromState,
		To:        toState,
		Timestamp: time.Now(),
		Reason:    reason,
	}
	transitions = append(transitions, transition)

	newTransitionsJSON, err := json.Marshal(transitions)
	if err != nil {
		return false, err
	}

	// Update job status
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = $1, state_transitions = $2, last_activity_at = $3
		WHERE id = $4
	`, toState, string(newTransitionsJSON), time.Now(), jobID)

	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

// AssignJobToWorker atomically assigns a job to a worker with idempotency
func (s *PostgreSQLStore) AssignJobToWorker(jobID, nodeID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Get job with lock
	var currentStatus, currentNodeID string
	var transitionsJSON []byte
	err = tx.QueryRow(`
		SELECT status, COALESCE(node_id, ''), state_transitions 
		FROM jobs WHERE id = $1 FOR UPDATE
	`, jobID).Scan(&currentStatus, &currentNodeID, &transitionsJSON)

	if err == sql.ErrNoRows {
		return false, ErrJobNotFound
	}
	if err != nil {
		return false, err
	}

	// Idempotency check
	if currentStatus == string(models.JobStatusAssigned) && currentNodeID == nodeID {
		return false, nil
	}

	// Only assign from QUEUED or RETRYING states
	if currentStatus != string(models.JobStatusQueued) && currentStatus != string(models.JobStatusRetrying) {
		return false, fmt.Errorf("job %s in state %s, cannot assign", jobID, currentStatus)
	}

	// Validate node exists
	var nodeExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM nodes WHERE id = $1)", nodeID).Scan(&nodeExists)
	if err != nil || !nodeExists {
		return false, fmt.Errorf("node not found: %s", nodeID)
	}

	// Parse transitions
	var transitions []models.StateTransition
	if len(transitionsJSON) > 0 && string(transitionsJSON) != "null" {
		json.Unmarshal(transitionsJSON, &transitions)
	}

	// Add state transition
	now := time.Now()
	transition := models.StateTransition{
		From:      models.JobStatus(currentStatus),
		To:        models.JobStatusAssigned,
		Timestamp: now,
		Reason:    fmt.Sprintf("Assigned to node %s", nodeID),
	}
	transitions = append(transitions, transition)
	newTransitionsJSON, _ := json.Marshal(transitions)

	// Update job
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = $1, node_id = $2, started_at = $3, last_activity_at = $4, state_transitions = $5
		WHERE id = $6
	`, models.JobStatusAssigned, nodeID, now, now, string(newTransitionsJSON), jobID)

	if err != nil {
		return false, err
	}

	// Update node
	_, err = tx.Exec(`
		UPDATE nodes 
		SET status = $1, current_job_id = $2
		WHERE id = $3
	`, "busy", jobID, nodeID)

	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

// CompleteJob marks a job as completed and frees the worker
func (s *PostgreSQLStore) CompleteJob(jobID, nodeID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Get job with lock
	var currentStatus, currentNodeID string
	err = tx.QueryRow(`
		SELECT status, COALESCE(node_id, '') 
		FROM jobs WHERE id = $1 FOR UPDATE
	`, jobID).Scan(&currentStatus, &currentNodeID)

	if err == sql.ErrNoRows {
		return false, ErrJobNotFound
	}
	if err != nil {
		return false, err
	}

	// Idempotency check
	if currentStatus == string(models.JobStatusCompleted) {
		return false, nil
	}

	// Validate node matches
	if currentNodeID != nodeID {
		return false, fmt.Errorf("job %s not assigned to node %s", jobID, nodeID)
	}

	// Update job
	now := time.Now()
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = $1, completed_at = $2
		WHERE id = $3
	`, models.JobStatusCompleted, now, jobID)

	if err != nil {
		return false, err
	}

	// Free node
	_, err = tx.Exec(`
		UPDATE nodes 
		SET status = $1, current_job_id = NULL
		WHERE id = $2
	`, "available", nodeID)

	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

// UpdateJobHeartbeat updates the last activity timestamp
func (s *PostgreSQLStore) UpdateJobHeartbeat(jobID string) error {
	return s.UpdateJobActivity(jobID)
}

// GetJobsInState returns all jobs in a specific state
func (s *PostgreSQLStore) GetJobsInState(state models.JobStatus) ([]*models.Job, error) {
	rows, err := s.db.Query(`
		SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, 
		       progress, node_id, created_at, started_at, last_activity_at, completed_at, 
		       retry_count, error, failure_reason, logs, state_transitions
		FROM jobs 
		WHERE status = $1
		ORDER BY created_at ASC
	`, string(state))

	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetOrphanedJobs returns jobs assigned to dead workers
func (s *PostgreSQLStore) GetOrphanedJobs(workerTimeout time.Duration) ([]*models.Job, error) {
	cutoff := time.Now().Add(-workerTimeout)

	rows, err := s.db.Query(`
		SELECT j.id, j.sequence_number, j.scenario, j.confidence, j.engine, j.parameters, j.status, 
		       j.queue, j.priority, j.progress, j.node_id, j.created_at, j.started_at, 
		       j.last_activity_at, j.completed_at, j.retry_count, j.error, j.failure_reason, 
		       j.logs, j.state_transitions
		FROM jobs j
		INNER JOIN nodes n ON j.node_id = n.id
		WHERE j.status IN ($1, $2)
		  AND n.last_heartbeat < $3
		ORDER BY j.created_at ASC
	`, models.JobStatusAssigned, models.JobStatusRunning, cutoff)

	if err != nil {
		return nil, fmt.Errorf("query orphaned jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetTimedOutJobs returns jobs that have exceeded their timeout
func (s *PostgreSQLStore) GetTimedOutJobs() ([]*models.Job, error) {
	// Use simple timeout logic: jobs in ASSIGNED/RUNNING state for > 1 hour without activity
	cutoff := time.Now().Add(-1 * time.Hour)

	rows, err := s.db.Query(`
		SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, 
		       progress, node_id, created_at, started_at, last_activity_at, completed_at, 
		       retry_count, error, failure_reason, logs, state_transitions
		FROM jobs 
		WHERE status IN ($1, $2)
		  AND COALESCE(last_activity_at, started_at, created_at) < $3
		ORDER BY created_at ASC
	`, models.JobStatusAssigned, models.JobStatusRunning, cutoff)

	if err != nil {
		return nil, fmt.Errorf("query timed out jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}
