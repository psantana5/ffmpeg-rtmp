package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// GetJob retrieves a job by ID
func (s *PostgreSQLStore) GetJob(id string) (*models.Job, error) {
	var job models.Job
	var paramsJSON, transitionsJSON []byte
	var nodeID, failureReason, logs sql.NullString
	var startedAt, lastActivityAt, completedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, last_activity_at, completed_at, retry_count, error, failure_reason, logs, state_transitions
		FROM jobs WHERE id = $1
	`, id).Scan(&job.ID, &job.SequenceNumber, &job.Scenario, &job.Confidence, &job.Engine, &paramsJSON, &job.Status,
		&job.Queue, &job.Priority, &job.Progress, &nodeID, &job.CreatedAt,
		&startedAt, &lastActivityAt, &completedAt, &job.RetryCount, &job.Error, &failureReason, &logs, &transitionsJSON)

	if err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if nodeID.Valid {
		job.NodeID = nodeID.String
	}
	if logs.Valid {
		job.Logs = logs.String
	}
	if failureReason.Valid {
		job.FailureReason = models.FailureReason(failureReason.String)
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if lastActivityAt.Valid {
		job.LastActivityAt = &lastActivityAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	// Unmarshal JSON fields
	if len(paramsJSON) > 0 && string(paramsJSON) != "null" {
		if err := json.Unmarshal(paramsJSON, &job.Parameters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
		}
	}

	if len(transitionsJSON) > 0 && string(transitionsJSON) != "null" {
		if err := json.Unmarshal(transitionsJSON, &job.StateTransitions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state_transitions: %w", err)
		}
	}

	return &job, nil
}

// GetJobBySequenceNumber retrieves a job by sequence number
func (s *PostgreSQLStore) GetJobBySequenceNumber(seqNum int) (*models.Job, error) {
	var job models.Job
	var paramsJSON, transitionsJSON []byte
	var nodeID, failureReason, logs sql.NullString
	var startedAt, lastActivityAt, completedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, last_activity_at, completed_at, retry_count, error, failure_reason, logs, state_transitions
		FROM jobs WHERE sequence_number = $1
	`, seqNum).Scan(&job.ID, &job.SequenceNumber, &job.Scenario, &job.Confidence, &job.Engine, &paramsJSON, &job.Status,
		&job.Queue, &job.Priority, &job.Progress, &nodeID, &job.CreatedAt,
		&startedAt, &lastActivityAt, &completedAt, &job.RetryCount, &job.Error, &failureReason, &logs, &transitionsJSON)

	if err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	// Handle nullable fields (same as GetJob)
	if nodeID.Valid {
		job.NodeID = nodeID.String
	}
	if logs.Valid {
		job.Logs = logs.String
	}
	if failureReason.Valid {
		job.FailureReason = models.FailureReason(failureReason.String)
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if lastActivityAt.Valid {
		job.LastActivityAt = &lastActivityAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	if len(paramsJSON) > 0 && string(paramsJSON) != "null" {
		json.Unmarshal(paramsJSON, &job.Parameters)
	}

	if len(transitionsJSON) > 0 && string(transitionsJSON) != "null" {
		json.Unmarshal(transitionsJSON, &job.StateTransitions)
	}

	return &job, nil
}

// GetAllJobs returns all jobs
func (s *PostgreSQLStore) GetAllJobs() []*models.Job {
	rows, err := s.db.Query(`
		SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, progress, node_id,
		       created_at, started_at, last_activity_at, completed_at, retry_count, error, failure_reason, logs, state_transitions
		FROM jobs
		ORDER BY sequence_number ASC
	`)
	if err != nil {
		return []*models.Job{}
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

	return jobs
}

// scanJobRow scans a job row (helper function)
func (s *PostgreSQLStore) scanJobRow(scanner interface {
	Scan(...interface{}) error
}) (*models.Job, error) {
	var job models.Job
	var paramsJSON, transitionsJSON []byte
	var nodeID, failureReason, logs sql.NullString
	var startedAt, lastActivityAt, completedAt sql.NullTime

	err := scanner.Scan(
		&job.ID, &job.SequenceNumber, &job.Scenario, &job.Confidence, &job.Engine,
		&paramsJSON, &job.Status, &job.Queue, &job.Priority, &job.Progress,
		&nodeID, &job.CreatedAt, &startedAt, &lastActivityAt, &completedAt,
		&job.RetryCount, &job.Error, &failureReason, &logs, &transitionsJSON,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if nodeID.Valid {
		job.NodeID = nodeID.String
	}
	if logs.Valid {
		job.Logs = logs.String
	}
	if failureReason.Valid {
		job.FailureReason = models.FailureReason(failureReason.String)
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if lastActivityAt.Valid {
		job.LastActivityAt = &lastActivityAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	// Parse JSON fields
	if len(paramsJSON) > 0 && string(paramsJSON) != "null" {
		json.Unmarshal(paramsJSON, &job.Parameters)
	}

	if len(transitionsJSON) > 0 && string(transitionsJSON) != "null" {
		json.Unmarshal(transitionsJSON, &job.StateTransitions)
	}

	return &job, nil
}

// GetNextJob retrieves the next pending job for a worker (legacy, use scheduler)
func (s *PostgreSQLStore) GetNextJob(nodeID string) (*models.Job, error) {
	return nil, fmt.Errorf("GetNextJob is deprecated, use production scheduler")
}

// UpdateJobStatus updates the status of a job
func (s *PostgreSQLStore) UpdateJobStatus(id string, status models.JobStatus, errorMsg string) error {
	now := time.Now()

	if status == models.JobStatusCompleted || status == models.JobStatusFailed {
		_, err := s.db.Exec(`
			UPDATE jobs 
			SET status = $1, error = $2, completed_at = $3
			WHERE id = $4
		`, status, errorMsg, now, id)
		return err
	}

	_, err := s.db.Exec(`
		UPDATE jobs 
		SET status = $1, error = $2
		WHERE id = $3
	`, status, errorMsg, id)

	return err
}

// UpdateJobProgress updates the progress percentage of a job
func (s *PostgreSQLStore) UpdateJobProgress(id string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	result, err := s.db.Exec(`
		UPDATE jobs SET progress = $1, last_activity_at = $2 WHERE id = $3
	`, progress, time.Now(), id)

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

// UpdateJobActivity updates the last activity timestamp of a job
func (s *PostgreSQLStore) UpdateJobActivity(id string) error {
	result, err := s.db.Exec(`
		UPDATE jobs SET last_activity_at = $1 WHERE id = $2
	`, time.Now(), id)

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

// UpdateJobFailureReason updates the failure_reason field for a job
func (s *PostgreSQLStore) UpdateJobFailureReason(id string, reason models.FailureReason, errorMsg string) error {
	result, err := s.db.Exec(`
		UPDATE jobs SET failure_reason = $1, error = $2 WHERE id = $3
	`, string(reason), errorMsg, id)

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

// UpdateJob updates a job's complete state
func (s *PostgreSQLStore) UpdateJob(job *models.Job) error {
	params, err := json.Marshal(job.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE jobs 
		SET status = $1, retry_count = $2, node_id = $3, started_at = $4, 
		    completed_at = $5, error = $6, parameters = $7
		WHERE id = $8
	`, job.Status, job.RetryCount, job.NodeID, job.StartedAt,
		job.CompletedAt, job.Error, string(params), job.ID)

	return err
}

// Continue in next file...
