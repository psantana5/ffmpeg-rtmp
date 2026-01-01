package scheduler

import (
	"log"
	"sort"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// PriorityQueueManager manages job prioritization and scheduling
type PriorityQueueManager struct {
	store store.Store
}

// NewPriorityQueueManager creates a new PriorityQueueManager
func NewPriorityQueueManager(st store.Store) *PriorityQueueManager {
	return &PriorityQueueManager{
		store: st,
	}
}

// GetPriorityWeight returns numeric weight for priority levels
func GetPriorityWeight(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 2 // Default to medium
	}
}

// GetQueueWeight returns numeric weight for queue types
func GetQueueWeight(queue string) int {
	switch queue {
	case "live":
		return 10 // Highest priority for live streams
	case "default":
		return 5
	case "batch":
		return 1
	default:
		return 5
	}
}

// SortJobsByPriority sorts jobs by priority (live > high > medium > low > batch)
func (pqm *PriorityQueueManager) SortJobsByPriority(jobs []*models.Job) []*models.Job {
	if len(jobs) == 0 {
		return jobs
	}

	sorted := make([]*models.Job, len(jobs))
	copy(sorted, jobs)

	sort.Slice(sorted, func(i, j int) bool {
		// Calculate total priority score for each job
		scoreI := GetQueueWeight(sorted[i].Queue) * 10 + GetPriorityWeight(sorted[i].Priority)
		scoreJ := GetQueueWeight(sorted[j].Queue) * 10 + GetPriorityWeight(sorted[j].Priority)

		// Higher score = higher priority
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}

		// If same priority, older jobs go first (FIFO within priority)
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	return sorted
}

// GetNextHighPriorityJob returns the highest priority job available
func (pqm *PriorityQueueManager) GetNextHighPriorityJob(nodeID string) (*models.Job, error) {
	// Get all queued and pending jobs
	allJobs := pqm.store.GetAllJobs()
	eligibleJobs := []*models.Job{}

	for _, job := range allJobs {
		if job.Status == models.JobStatusQueued || job.Status == models.JobStatusPending {
			eligibleJobs = append(eligibleJobs, job)
		}
	}

	if len(eligibleJobs) == 0 {
		return nil, nil
	}

	// Sort by priority
	sortedJobs := pqm.SortJobsByPriority(eligibleJobs)

	// Return highest priority job
	highestPriorityJob := sortedJobs[0]
	log.Printf("PriorityQueue: Selected job %s (seq#%d) with queue=%s priority=%s",
		highestPriorityJob.ID, highestPriorityJob.SequenceNumber,
		highestPriorityJob.Queue, highestPriorityJob.Priority)

	// Assign to node
	if err := pqm.store.UpdateJobStatus(highestPriorityJob.ID, models.JobStatusAssigned, ""); err != nil {
		return nil, err
	}

	// Update job with node assignment
	highestPriorityJob.NodeID = nodeID
	if err := pqm.store.UpdateJob(highestPriorityJob); err != nil {
		return nil, err
	}

	return highestPriorityJob, nil
}

// GetQueueStats returns statistics about jobs in each queue/priority
func (pqm *PriorityQueueManager) GetQueueStats() map[string]int {
	stats := make(map[string]int)
	allJobs := pqm.store.GetAllJobs()

	for _, job := range allJobs {
		if job.Status == models.JobStatusQueued || job.Status == models.JobStatusPending {
			key := job.Queue + "_" + job.Priority
			stats[key]++
			stats["total"]++
		}
	}

	return stats
}
