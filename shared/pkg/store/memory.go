package store

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

var (
	ErrNodeNotFound = errors.New("node not found")
	ErrJobNotFound  = errors.New("job not found")
)

// MemoryStore is an in-memory implementation of the data store
// Uses a single RWMutex to prevent deadlock issues with nested locks
type MemoryStore struct {
	mu          sync.RWMutex // Single mutex for all operations
	nodes       map[string]*models.Node
	jobs        map[string]*models.Job
	jobQueue    []string // FIFO queue of job IDs
	nextSeqNum  int      // Auto-incrementing sequence number for jobs
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:      make(map[string]*models.Node),
		jobs:       make(map[string]*models.Job),
		nextSeqNum: 1,
		jobQueue: make([]string, 0),
	}
}

// Node operations

// RegisterNode adds or updates a node in the store
func (s *MemoryStore) RegisterNode(node *models.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes[node.ID] = node
	return nil
}

// GetNode retrieves a node by ID
func (s *MemoryStore) GetNode(id string) (*models.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// GetNodeByAddress retrieves a node by address
func (s *MemoryStore) GetNodeByAddress(address string) (*models.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, node := range s.nodes {
		if node.Address == address {
			return node, nil
		}
	}
	return nil, ErrNodeNotFound
}

// GetAllNodes returns all registered nodes
func (s *MemoryStore) GetAllNodes() []*models.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*models.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// UpdateNodeStatus updates the status of a node
func (s *MemoryStore) UpdateNodeStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}

	node.Status = status
	node.LastHeartbeat = time.Now()
	return nil
}

// UpdateNodeHeartbeat updates the last heartbeat time for a node
func (s *MemoryStore) UpdateNodeHeartbeat(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}

	node.LastHeartbeat = time.Now()
	return nil
}

// DeleteNode removes a node from the store
func (s *MemoryStore) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nodes[id]; !ok {
		return ErrNodeNotFound
	}

	delete(s.nodes, id)
	return nil
}

// Job operations

// CreateJob adds a new job to the store and queue
func (s *MemoryStore) CreateJob(job *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate sequence number if not set
	if job.SequenceNumber == 0 {
		job.SequenceNumber = s.nextSeqNum
		s.nextSeqNum++
	}

	s.jobs[job.ID] = job
	s.jobQueue = append(s.jobQueue, job.ID)

	return nil
}

// GetJob retrieves a job by ID
func (s *MemoryStore) GetJob(id string) (*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// GetJobBySequenceNumber retrieves a job by sequence number
func (s *MemoryStore) GetJobBySequenceNumber(seqNum int) (*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, job := range s.jobs {
		if job.SequenceNumber == seqNum {
			return job, nil
		}
	}
	return nil, ErrJobNotFound
}

// GetAllJobs returns all jobs
func (s *MemoryStore) GetAllJobs() []*models.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetNextJob retrieves the next pending job from the queue
func (s *MemoryStore) GetNextJob(nodeID string) (*models.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find first pending job
	for i, jobID := range s.jobQueue {
		// Lock both jobs and nodes for atomic operation
		s.mu.Lock()
		job, ok := s.jobs[jobID]
		if !ok || job.Status != models.JobStatusPending {
			s.mu.Unlock()
			continue
		}

		// Mark job as running and assign to node
		now := time.Now()
		job.Status = models.JobStatusRunning
		job.NodeID = nodeID
		job.StartedAt = &now
		job.LastActivityAt = &now
		s.mu.Unlock()

		// Remove from queue
		s.jobQueue = append(s.jobQueue[:i], s.jobQueue[i+1:]...)

		// Update node status
		s.mu.Lock()
		if node, ok := s.nodes[nodeID]; ok {
			node.Status = "busy"
			node.CurrentJobID = jobID
		}
		s.mu.Unlock()

		return job, nil
	}

	return nil, ErrJobNotFound
}

// UpdateJobStatus updates the status of a job
func (s *MemoryStore) UpdateJobStatus(id string, status models.JobStatus, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	job.Status = status
	if errorMsg != "" {
		job.Error = errorMsg
	}

	if status == models.JobStatusCompleted || status == models.JobStatusFailed {
		now := time.Now()
		job.CompletedAt = &now

		// Update node status back to available
		if job.NodeID != "" {
			s.mu.Lock()
			if node, ok := s.nodes[job.NodeID]; ok {
				node.Status = "available"
				node.CurrentJobID = ""
			}
			s.mu.Unlock()
		}
	}

	return nil
}

// UpdateJobProgress updates the progress percentage of a job
func (s *MemoryStore) UpdateJobProgress(id string, progress int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	job.Progress = progress
	now := time.Now()
	job.LastActivityAt = &now
	return nil
}

// UpdateJobActivity updates the last activity timestamp of a job
func (s *MemoryStore) UpdateJobActivity(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	now := time.Now()
	job.LastActivityAt = &now
	return nil
}

// UpdateJob updates a job's complete state
func (s *MemoryStore) UpdateJob(job *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[job.ID]; !ok {
		return ErrJobNotFound
	}

	s.jobs[job.ID] = job
	return nil
}

// AddStateTransition adds a state transition to a job's history
func (s *MemoryStore) AddStateTransition(id string, from, to models.JobStatus, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	transition := models.StateTransition{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	job.StateTransitions = append(job.StateTransitions, transition)
	job.Status = to

	return nil
}

// PauseJob pauses a running job
func (s *MemoryStore) PauseJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	if job.Status != models.JobStatusProcessing && job.Status != models.JobStatusRunning {
		return fmt.Errorf("cannot pause job in status: %s", job.Status)
	}

	return s.AddStateTransition(id, job.Status, models.JobStatusPaused, "User requested pause")
}

// ResumeJob resumes a paused job
func (s *MemoryStore) ResumeJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	if job.Status != models.JobStatusPaused {
		return fmt.Errorf("cannot resume job in status: %s", job.Status)
	}

	return s.AddStateTransition(id, job.Status, models.JobStatusProcessing, "User requested resume")
}

// CancelJob cancels a job
func (s *MemoryStore) CancelJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}

	if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed || job.Status == models.JobStatusCanceled {
		return fmt.Errorf("cannot cancel job in status: %s", job.Status)
	}

	// Add state transition
	transition := models.StateTransition{
		From:      job.Status,
		To:        models.JobStatusCanceled,
		Timestamp: time.Now(),
		Reason:    "User requested cancel",
	}
	job.StateTransitions = append(job.StateTransitions, transition)
	job.Status = models.JobStatusCanceled

	// Free up node if assigned
	if job.NodeID != "" {
		s.mu.Lock()
		if node, ok := s.nodes[job.NodeID]; ok {
			node.Status = "available"
			node.CurrentJobID = ""
		}
		s.mu.Unlock()
	}

	// Set completed_at
	now := time.Now()
	job.CompletedAt = &now

	return nil
}

// GetQueuedJobs returns jobs in a specific queue with priority filtering
func (s *MemoryStore) GetQueuedJobs(queue string, priority string) []*models.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobs []*models.Job
	for _, job := range s.jobs {
		if (job.Status == models.JobStatusPending || job.Status == models.JobStatusQueued) &&
			job.Queue == queue {
			if priority == "" || job.Priority == priority {
				jobs = append(jobs, job)
			}
		}
	}

	return jobs
}

// TryQueuePendingJob atomically checks if a job is pending with no available workers and queues it
func (s *MemoryStore) TryQueuePendingJob(jobID string) (bool, error) {
s.mu.Lock()
defer s.mu.Unlock()

job, ok := s.jobs[jobID]
if !ok {
return false, fmt.Errorf("job not found: %s", jobID)
}

if job.Status != models.JobStatusPending {
return false, nil // Already processed
}

// Check if any workers are available
availableCount := 0
for _, node := range s.nodes {
if node.Status == "available" {
availableCount++
}
}

if availableCount > 0 {
return false, nil // Workers available
}

// No workers available, queue the job
job.Status = models.JobStatusQueued
return true, nil
}

// RetryJob resets a failed job for retry by updating its status to pending,
// clearing node assignment, and incrementing retry count
func (s *MemoryStore) RetryJob(jobID string, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	// Increment retry count
	job.RetryCount++
	
	// Reset job to pending
	job.Status = models.JobStatusPending
	job.Error = errorMsg
	
	// Clear node assignment
	oldNodeID := job.NodeID
	job.NodeID = ""
	job.StartedAt = nil
	
	// Update the node that was running the job back to available
	if oldNodeID != "" {
		if node, ok := s.nodes[oldNodeID]; ok {
			node.Status = "available"
			node.CurrentJobID = ""
		}
	}
	
	return nil
}

// FSM Methods (for MemoryStore compatibility with production scheduler)

// TransitionJobState performs a validated state transition with idempotency
func (s *MemoryStore) TransitionJobState(jobID string, toState models.JobStatus, reason string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return false, ErrJobNotFound
	}

	fromState := job.Status

	// Idempotency: if already in target state, no-op
	if fromState == toState {
		return false, nil
	}

	// Validate transition
	if err := models.ValidateTransition(fromState, toState); err != nil {
		return false, fmt.Errorf("invalid transition: %w", err)
	}

	// Add transition
	transition := models.StateTransition{
		From:      fromState,
		To:        toState,
		Timestamp: time.Now(),
		Reason:    reason,
	}
	job.StateTransitions = append(job.StateTransitions, transition)
	job.Status = toState

	return true, nil
}

// AssignJobToWorker atomically assigns a job to a worker with idempotency
func (s *MemoryStore) AssignJobToWorker(jobID, nodeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return false, ErrJobNotFound
	}

	// Idempotency check
	if job.Status == models.JobStatusAssigned && job.NodeID == nodeID {
		return false, nil
	}

	// Only assign from QUEUED or RETRYING states
	if job.Status != models.JobStatusQueued && job.Status != models.JobStatusRetrying {
		return false, fmt.Errorf("job %s in state %s, cannot assign", jobID, job.Status)
	}

	// Validate node exists
	node, ok := s.nodes[nodeID]
	if !ok {
		return false, fmt.Errorf("node not found: %s", nodeID)
	}

	// Add transition
	now := time.Now()
	transition := models.StateTransition{
		From:      job.Status,
		To:        models.JobStatusAssigned,
		Timestamp: now,
		Reason:    fmt.Sprintf("Assigned to worker %s", nodeID),
	}
	job.StateTransitions = append(job.StateTransitions, transition)
	job.Status = models.JobStatusAssigned
	job.NodeID = nodeID
	job.StartedAt = &now
	job.LastActivityAt = &now

	// Update node
	node.Status = "busy"
	node.CurrentJobID = jobID
	node.LastHeartbeat = now

	return true, nil
}

// CompleteJob marks a job as completed (idempotent)
func (s *MemoryStore) CompleteJob(jobID, nodeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return false, ErrJobNotFound
	}

	// Idempotency: already completed
	if job.Status == models.JobStatusCompleted {
		return false, nil
	}

	// Security: verify the node completing the job is the assigned node
	if job.NodeID != nodeID {
		return false, fmt.Errorf("job %s assigned to node %s, not %s", jobID, job.NodeID, nodeID)
	}

	// Only complete from RUNNING or ASSIGNED states
	if job.Status != models.JobStatusRunning && job.Status != models.JobStatusAssigned {
		return false, fmt.Errorf("job %s in state %s, cannot complete", jobID, job.Status)
	}

	// Add transition
	now := time.Now()
	transition := models.StateTransition{
		From:      job.Status,
		To:        models.JobStatusCompleted,
		Timestamp: now,
		Reason:    fmt.Sprintf("Completed by worker %s", nodeID),
	}
	job.StateTransitions = append(job.StateTransitions, transition)
	job.Status = models.JobStatusCompleted
	job.CompletedAt = &now

	// Free up node
	if node, ok := s.nodes[nodeID]; ok {
		node.Status = "available"
		node.CurrentJobID = ""
	}

	return true, nil
}

// UpdateJobHeartbeat updates the last activity timestamp
func (s *MemoryStore) UpdateJobHeartbeat(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	// Only update heartbeat for active jobs
	if job.Status == models.JobStatusAssigned || job.Status == models.JobStatusRunning {
		now := time.Now()
		job.LastActivityAt = &now
	}

	return nil
}

// GetJobsInState returns all jobs in a specific state
func (s *MemoryStore) GetJobsInState(state models.JobStatus) ([]*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := []*models.Job{}
	for _, job := range s.jobs {
		if job.Status == state {
			result = append(result, job)
		}
	}

	return result, nil
}

// GetOrphanedJobs finds jobs assigned/running on offline/dead workers
func (s *MemoryStore) GetOrphanedJobs(workerTimeout time.Duration) ([]*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-workerTimeout)
	result := []*models.Job{}

	for _, job := range s.jobs {
		if job.Status != models.JobStatusAssigned && job.Status != models.JobStatusRunning {
			continue
		}

		if job.NodeID == "" {
			continue
		}

		// Check if node is offline or hasn't sent heartbeat
		node, ok := s.nodes[job.NodeID]
		if !ok || node.Status == "offline" || node.LastHeartbeat.Before(cutoff) {
			result = append(result, job)
		}
	}

	return result, nil
}

// GetTimedOutJobs finds jobs that exceeded their timeout
func (s *MemoryStore) GetTimedOutJobs() ([]*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	timeoutConfig := models.DefaultJobTimeout()
	result := []*models.Job{}

	for _, job := range s.jobs {
		if job.Status != models.JobStatusAssigned && job.Status != models.JobStatusRunning {
			continue
		}

		if job.LastActivityAt == nil {
			continue
		}

		timeout := timeoutConfig.CalculateTimeout(job)
		deadline := job.LastActivityAt.Add(timeout)

		if now.After(deadline) {
			result = append(result, job)
		}
	}

	return result, nil
}

