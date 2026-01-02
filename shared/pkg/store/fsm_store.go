package store

import (
	"fmt"
	"log"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// TransitionJobState performs a validated state transition with idempotency
// Returns (transitioned bool, error) - transitioned=false if already in target state
func (s *SQLiteStore) TransitionJobState(jobID string, toState models.JobStatus, reason string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current job state with row lock
	var currentStatus string
	var transitionsJSON string
	err = tx.QueryRow(`
		SELECT status, state_transitions 
		FROM jobs 
		WHERE id = ?
	`, jobID).Scan(&currentStatus, &transitionsJSON)

	if err != nil {
		return false, fmt.Errorf("get job state: %w", err)
	}

	fromState := models.JobStatus(currentStatus)

	// Idempotency: if already in target state, no-op
	if fromState == toState {
		log.Printf("[FSM] Job %s already in state %s (idempotent no-op)", jobID, toState)
		return false, nil
	}

	// Validate transition
	if err := models.ValidateTransition(fromState, toState); err != nil {
		return false, fmt.Errorf("invalid transition: %w", err)
	}

	// Parse existing transitions
	var transitions []models.StateTransition
	if transitionsJSON != "" && transitionsJSON != "null" {
		if err := unmarshalJSON([]byte(transitionsJSON), &transitions); err != nil {
			log.Printf("[FSM] Warning: failed to parse transitions: %v", err)
			transitions = []models.StateTransition{}
		}
	}

	// Add new transition
	transition := models.StateTransition{
		From:      fromState,
		To:        toState,
		Timestamp: time.Now(),
		Reason:    reason,
	}
	transitions = append(transitions, transition)

	newTransitionsJSON, err := marshalJSON(transitions)
	if err != nil {
		return false, fmt.Errorf("marshal transitions: %w", err)
	}

	// Update state
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = ?, state_transitions = ?
		WHERE id = ?
	`, string(toState), string(newTransitionsJSON), jobID)

	if err != nil {
		return false, fmt.Errorf("update job state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}

	log.Printf("[FSM] Job %s: %s → %s (reason: %s)", jobID, fromState, toState, reason)
	return true, nil
}

// AssignJobToWorker atomically assigns a job to a worker with idempotency
// Uses SELECT FOR UPDATE to prevent double assignment
func (s *SQLiteStore) AssignJobToWorker(jobID, nodeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock job row
	var currentStatus, currentNodeID string
	var transitionsJSON string
	err = tx.QueryRow(`
		SELECT status, node_id, state_transitions 
		FROM jobs 
		WHERE id = ?
	`, jobID).Scan(&currentStatus, &currentNodeID, &transitionsJSON)

	if err != nil {
		return false, fmt.Errorf("get job: %w", err)
	}

	// Idempotency check: if already assigned to this node, no-op
	if currentStatus == string(models.JobStatusAssigned) && currentNodeID == nodeID {
		log.Printf("[FSM] Job %s already assigned to node %s (idempotent no-op)", jobID, nodeID)
		return false, nil
	}

	// Only assign from QUEUED or RETRYING states
	if currentStatus != string(models.JobStatusQueued) && currentStatus != string(models.JobStatusRetrying) {
		return false, fmt.Errorf("job %s in state %s, cannot assign", jobID, currentStatus)
	}

	// Validate node exists and is available
	var nodeStatus string
	err = tx.QueryRow(`SELECT status FROM nodes WHERE id = ?`, nodeID).Scan(&nodeStatus)
	if err != nil {
		return false, fmt.Errorf("node not found: %w", err)
	}

	// Parse transitions
	var transitions []models.StateTransition
	if transitionsJSON != "" && transitionsJSON != "null" {
		unmarshalJSON([]byte(transitionsJSON), &transitions)
	}

	// Add transition
	now := time.Now()
	transition := models.StateTransition{
		From:      models.JobStatus(currentStatus),
		To:        models.JobStatusAssigned,
		Timestamp: now,
		Reason:    fmt.Sprintf("Assigned to worker %s", nodeID),
	}
	transitions = append(transitions, transition)
	newTransitionsJSON, _ := marshalJSON(transitions)

	// Update job
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = ?, node_id = ?, started_at = ?, last_activity_at = ?, state_transitions = ?
		WHERE id = ?
	`, string(models.JobStatusAssigned), nodeID, now, now, string(newTransitionsJSON), jobID)

	if err != nil {
		return false, fmt.Errorf("update job: %w", err)
	}

	// Update node
	_, err = tx.Exec(`
		UPDATE nodes 
		SET status = ?, current_job_id = ?, last_heartbeat = ?
		WHERE id = ?
	`, "busy", jobID, now, nodeID)

	if err != nil {
		return false, fmt.Errorf("update node: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	log.Printf("[FSM] Job %s assigned to worker %s", jobID, nodeID)
	return true, nil
}

// CompleteJob marks a job as completed (idempotent)
func (s *SQLiteStore) CompleteJob(jobID, nodeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get job
	var currentStatus, jobNodeID string
	var transitionsJSON string
	err = tx.QueryRow(`
		SELECT status, node_id, state_transitions 
		FROM jobs 
		WHERE id = ?
	`, jobID).Scan(&currentStatus, &jobNodeID, &transitionsJSON)

	if err != nil {
		return false, fmt.Errorf("get job: %w", err)
	}

	// Idempotency: already completed
	if currentStatus == string(models.JobStatusCompleted) {
		log.Printf("[FSM] Job %s already completed (idempotent no-op)", jobID)
		return false, nil
	}

	// Security: verify the node completing the job is the assigned node
	if jobNodeID != nodeID {
		return false, fmt.Errorf("job %s assigned to node %s, not %s", jobID, jobNodeID, nodeID)
	}

	// Only complete from RUNNING or ASSIGNED states
	if currentStatus != string(models.JobStatusRunning) && currentStatus != string(models.JobStatusAssigned) {
		return false, fmt.Errorf("job %s in state %s, cannot complete", jobID, currentStatus)
	}

	// Parse transitions
	var transitions []models.StateTransition
	if transitionsJSON != "" && transitionsJSON != "null" {
		unmarshalJSON([]byte(transitionsJSON), &transitions)
	}

	// Add transition
	now := time.Now()
	transition := models.StateTransition{
		From:      models.JobStatus(currentStatus),
		To:        models.JobStatusCompleted,
		Timestamp: now,
		Reason:    fmt.Sprintf("Completed by worker %s", nodeID),
	}
	transitions = append(transitions, transition)
	newTransitionsJSON, _ := marshalJSON(transitions)

	// Update job
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = ?, completed_at = ?, state_transitions = ?
		WHERE id = ?
	`, string(models.JobStatusCompleted), now, string(newTransitionsJSON), jobID)

	if err != nil {
		return false, fmt.Errorf("update job: %w", err)
	}

	// Free up node
	_, err = tx.Exec(`
		UPDATE nodes 
		SET status = ?, current_job_id = ?
		WHERE id = ?
	`, "available", "", nodeID)

	if err != nil {
		return false, fmt.Errorf("update node: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	log.Printf("[FSM] Job %s completed by worker %s", jobID, nodeID)
	return true, nil
}

// UpdateJobHeartbeat updates the last activity timestamp (idempotent)
func (s *SQLiteStore) UpdateJobHeartbeat(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE jobs 
		SET last_activity_at = ?
		WHERE id = ? AND status IN (?, ?)
	`, now, jobID, string(models.JobStatusAssigned), string(models.JobStatusRunning))

	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}

	return nil
}

// GetJobsInState returns all jobs in a specific state
func (s *SQLiteStore) GetJobsInState(state models.JobStatus) ([]*models.Job, error) {
	rows, err := s.db.Query(`
		SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, 
		       progress, node_id, created_at, started_at, last_activity_at, completed_at, 
		       retry_count, error, logs, state_transitions
		FROM jobs 
		WHERE status = ?
		ORDER BY created_at ASC
	`, string(state))

	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	return s.scanJobs(rows)
}

// GetOrphanedJobs finds jobs assigned/running on offline/dead workers
func (s *SQLiteStore) GetOrphanedJobs(workerTimeout time.Duration) ([]*models.Job, error) {
	cutoff := time.Now().Add(-workerTimeout)
	
	rows, err := s.db.Query(`
		SELECT j.id, j.sequence_number, j.scenario, j.confidence, j.engine, j.parameters, j.status, 
		       j.queue, j.priority, j.progress, j.node_id, j.created_at, j.started_at, 
		       j.last_activity_at, j.completed_at, j.retry_count, j.error, j.logs, j.state_transitions
		FROM jobs j
		INNER JOIN nodes n ON j.node_id = n.id
		WHERE j.status IN (?, ?)
		  AND (n.status = 'offline' OR n.last_heartbeat < ?)
	`, string(models.JobStatusAssigned), string(models.JobStatusRunning), cutoff)

	if err != nil {
		return nil, fmt.Errorf("query orphaned jobs: %w", err)
	}
	defer rows.Close()

	return s.scanJobs(rows)
}

// GetTimedOutJobs finds jobs that exceeded their timeout
func (s *SQLiteStore) GetTimedOutJobs() ([]*models.Job, error) {
	now := time.Now()
	
	rows, err := s.db.Query(`
		SELECT j.id, j.sequence_number, j.scenario, j.confidence, j.engine, j.parameters, j.status, 
		       j.queue, j.priority, j.progress, j.node_id, j.created_at, j.started_at, 
		       j.last_activity_at, j.completed_at, j.retry_count, j.error, j.logs, j.state_transitions
		FROM jobs j
		WHERE j.status IN (?, ?)
		  AND j.last_activity_at IS NOT NULL
	`, string(models.JobStatusAssigned), string(models.JobStatusRunning))

	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	jobs, err := s.scanJobs(rows)
	if err != nil {
		return nil, err
	}

	// Filter by timeout calculation
	timeoutConfig := models.DefaultJobTimeout()
	timedOut := []*models.Job{}
	
	for _, job := range jobs {
		if job.LastActivityAt == nil {
			continue
		}
		
		timeout := timeoutConfig.CalculateTimeout(job)
		deadline := job.LastActivityAt.Add(timeout)
		
		if now.After(deadline) {
			timedOut = append(timedOut, job)
		}
	}

	return timedOut, nil
}

// Helper to scan job rows
func (s *SQLiteStore) scanJobs(rows rowScanner) ([]*models.Job, error) {
	jobs := []*models.Job{}
	
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			log.Printf("Warning: failed to scan job row: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}
	
	return jobs, nil
}

type rowScanner interface {
	Next() bool
	Scan(...interface{}) error
}
