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

// GetJobMetrics returns aggregated job statistics optimized for metrics endpoint
// This avoids loading all jobs into memory
func (s *PostgreSQLStore) GetJobMetrics() (*JobMetrics, error) {
	metrics := &JobMetrics{
		JobsByState:       make(map[models.JobStatus]int),
		JobsByEngine:      make(map[string]int),
		CompletedByEngine: make(map[string]int),
		QueueByPriority:   make(map[string]int),
		QueueByType:       make(map[string]int),
	}

	// Count jobs by state
	rows, err := s.db.Query(`
		SELECT status, COUNT(*) as count
		FROM jobs
		GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to count jobs by state: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		metrics.JobsByState[models.JobStatus(status)] = count
		metrics.TotalJobs += count

		// Count active jobs
		if status == string(models.JobStatusProcessing) || status == string(models.JobStatusAssigned) {
			metrics.ActiveJobs += count
		}
		// Count queued jobs
		if status == string(models.JobStatusQueued) || status == string(models.JobStatusPending) {
			metrics.QueueLength += count
		}
	}

	// Count jobs by engine
	rows, err = s.db.Query(`
		SELECT engine, COUNT(*) as count
		FROM jobs
		WHERE engine != ''
		GROUP BY engine
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to count jobs by engine: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var engine string
		var count int
		if err := rows.Scan(&engine, &count); err != nil {
			continue
		}
		metrics.JobsByEngine[engine] = count
	}

	// Count completed jobs by engine
	rows, err = s.db.Query(`
		SELECT engine, COUNT(*) as count
		FROM jobs
		WHERE status = 'completed' AND engine != ''
		GROUP BY engine
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to count completed by engine: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var engine string
		var count int
		if err := rows.Scan(&engine, &count); err != nil {
			continue
		}
		// Normalize "auto" to "ffmpeg" for completed jobs
		if engine == "auto" {
			engine = "ffmpeg"
		}
		metrics.CompletedByEngine[engine] += count
	}

	// Count queued jobs by priority
	rows, err = s.db.Query(`
		SELECT priority, COUNT(*) as count
		FROM jobs
		WHERE status IN ('queued', 'pending')
		GROUP BY priority
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to count queue by priority: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var priority string
		var count int
		if err := rows.Scan(&priority, &count); err != nil {
			continue
		}
		metrics.QueueByPriority[priority] = count
	}

	// Count queued jobs by type
	rows, err = s.db.Query(`
		SELECT queue, COUNT(*) as count
		FROM jobs
		WHERE status IN ('queued', 'pending')
		GROUP BY queue
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to count queue by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var queueType string
		var count int
		if err := rows.Scan(&queueType, &count); err != nil {
			continue
		}
		metrics.QueueByType[queueType] = count
	}

	// Calculate average duration for completed/failed jobs
	var totalDuration sql.NullFloat64
	var jobCount sql.NullInt64
	err = s.db.QueryRow(`
		SELECT 
			AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) as avg_duration,
			COUNT(*) as count
		FROM jobs
		WHERE (status = 'completed' OR status = 'failed') 
		  AND completed_at IS NOT NULL 
		  AND created_at IS NOT NULL
	`).Scan(&totalDuration, &jobCount)
	
	if err == nil && totalDuration.Valid && jobCount.Valid && jobCount.Int64 > 0 {
		metrics.AvgDuration = totalDuration.Float64
	}

	return metrics, nil
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

// AddStateTransition adds a state transition to a job's history
func (s *PostgreSQLStore) AddStateTransition(id string, from, to models.JobStatus, reason string) error {
// Use TransitionJobState which already handles this
_, err := s.TransitionJobState(id, to, reason)
return err
}

// PauseJob pauses a running job
func (s *PostgreSQLStore) PauseJob(id string) error {
_, err := s.TransitionJobState(id, models.JobStatusAssigned, "Job paused by user")
return err
}

// ResumeJob resumes a paused job
func (s *PostgreSQLStore) ResumeJob(id string) error {
_, err := s.TransitionJobState(id, models.JobStatusRunning, "Job resumed by user")
return err
}

// CancelJob cancels a job
func (s *PostgreSQLStore) CancelJob(id string) error {
_, err := s.TransitionJobState(id, models.JobStatusCanceled, "Job canceled by user")
return err
}

// RetryJob retries a failed job
func (s *PostgreSQLStore) RetryJob(jobID string, errorMsg string) error {
tx, err := s.db.Begin()
if err != nil {
return err
}
defer tx.Rollback()

// Get current job
var retryCount int
var status string
err = tx.QueryRow("SELECT retry_count, status FROM jobs WHERE id = $1 FOR UPDATE", jobID).
Scan(&retryCount, &status)
if err != nil {
return err
}

// Increment retry count and set to queued
_, err = tx.Exec(`
UPDATE jobs 
SET status = $1, retry_count = $2, error = $3, node_id = NULL
WHERE id = $4
`, models.JobStatusQueued, retryCount+1, errorMsg, jobID)

if err != nil {
return err
}

return tx.Commit()
}

// TryQueuePendingJob atomically queues a pending job (for legacy scheduler)
func (s *PostgreSQLStore) TryQueuePendingJob(jobID string) (bool, error) {
result, err := s.db.Exec(`
UPDATE jobs 
SET status = $1
WHERE id = $2 AND status = $3
`, models.JobStatusQueued, jobID, models.JobStatusPending)

if err != nil {
return false, err
}

rows, err := result.RowsAffected()
if err != nil {
return false, err
}

return rows > 0, nil
}

// GetQueuedJobs returns queued jobs filtered by queue and priority
func (s *PostgreSQLStore) GetQueuedJobs(queue string, priority string) []*models.Job {
query := `
SELECT id, sequence_number, scenario, confidence, engine, parameters, status, queue, priority, 
       progress, node_id, created_at, started_at, last_activity_at, completed_at, 
       retry_count, error, failure_reason, logs, state_transitions
FROM jobs 
WHERE status IN ($1, $2)
`
args := []interface{}{models.JobStatusQueued, models.JobStatusRetrying}

if queue != "" {
query += " AND queue = $" + fmt.Sprintf("%d", len(args)+1)
args = append(args, queue)
}

if priority != "" {
query += " AND priority = $" + fmt.Sprintf("%d", len(args)+1)
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
job, err := s.scanJobRow(rows)
if err != nil {
continue
}
jobs = append(jobs, job)
}

return jobs
}
