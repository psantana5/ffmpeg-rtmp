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
type MemoryStore struct {
	nodes    map[string]*models.Node
	jobs     map[string]*models.Job
	jobQueue []string // FIFO queue of job IDs
	nodesMu  sync.RWMutex
	jobsMu   sync.RWMutex
	queueMu  sync.Mutex
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:    make(map[string]*models.Node),
		jobs:     make(map[string]*models.Job),
		jobQueue: make([]string, 0),
	}
}

// Node operations

// RegisterNode adds or updates a node in the store
func (s *MemoryStore) RegisterNode(node *models.Node) error {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()

	s.nodes[node.ID] = node
	return nil
}

// GetNode retrieves a node by ID
func (s *MemoryStore) GetNode(id string) (*models.Node, error) {
	s.nodesMu.RLock()
	defer s.nodesMu.RUnlock()

	node, ok := s.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// GetAllNodes returns all registered nodes
func (s *MemoryStore) GetAllNodes() []*models.Node {
	s.nodesMu.RLock()
	defer s.nodesMu.RUnlock()

	nodes := make([]*models.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// UpdateNodeStatus updates the status of a node
func (s *MemoryStore) UpdateNodeStatus(id, status string) error {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()

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
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()

	node, ok := s.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}

	node.LastHeartbeat = time.Now()
	return nil
}

// Job operations

// CreateJob adds a new job to the store and queue
func (s *MemoryStore) CreateJob(job *models.Job) error {
	s.jobsMu.Lock()
	s.jobs[job.ID] = job
	s.jobsMu.Unlock()

	// Add to queue
	s.queueMu.Lock()
	s.jobQueue = append(s.jobQueue, job.ID)
	s.queueMu.Unlock()

	return nil
}

// GetJob retrieves a job by ID
func (s *MemoryStore) GetJob(id string) (*models.Job, error) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// GetAllJobs returns all jobs
func (s *MemoryStore) GetAllJobs() []*models.Job {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetNextJob retrieves the next pending job from the queue
func (s *MemoryStore) GetNextJob(nodeID string) (*models.Job, error) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	// Find first pending job
	for i, jobID := range s.jobQueue {
		// Lock both jobs and nodes for atomic operation
		s.jobsMu.Lock()
		job, ok := s.jobs[jobID]
		if !ok || job.Status != models.JobStatusPending {
			s.jobsMu.Unlock()
			continue
		}

		// Mark job as running and assign to node
		now := time.Now()
		job.Status = models.JobStatusRunning
		job.NodeID = nodeID
		job.StartedAt = &now
		s.jobsMu.Unlock()

		// Remove from queue
		s.jobQueue = append(s.jobQueue[:i], s.jobQueue[i+1:]...)

		// Update node status
		s.nodesMu.Lock()
		if node, ok := s.nodes[nodeID]; ok {
			node.Status = "busy"
			node.CurrentJobID = jobID
		}
		s.nodesMu.Unlock()

		return job, nil
	}

	return nil, ErrJobNotFound
}

// UpdateJobStatus updates the status of a job
func (s *MemoryStore) UpdateJobStatus(id string, status models.JobStatus, errorMsg string) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

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
			s.nodesMu.Lock()
			if node, ok := s.nodes[job.NodeID]; ok {
				node.Status = "available"
				node.CurrentJobID = ""
			}
			s.nodesMu.Unlock()
		}
	}

	return nil
}

// UpdateJobProgress updates the progress percentage of a job
func (s *MemoryStore) UpdateJobProgress(id string, progress int) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

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
	return nil
}

// AddStateTransition adds a state transition to a job's history
func (s *MemoryStore) AddStateTransition(id string, from, to models.JobStatus, reason string) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

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
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

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
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

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
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

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
		s.nodesMu.Lock()
		if node, ok := s.nodes[job.NodeID]; ok {
			node.Status = "available"
			node.CurrentJobID = ""
		}
		s.nodesMu.Unlock()
	}

	// Set completed_at
	now := time.Now()
	job.CompletedAt = &now

	return nil
}

// GetQueuedJobs returns jobs in a specific queue with priority filtering
func (s *MemoryStore) GetQueuedJobs(queue string, priority string) []*models.Job {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

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
