package scheduler

import (
	"log"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// Scheduler manages background job scheduling tasks
type Scheduler struct {
	store         store.Store
	checkInterval time.Duration
	stopCh        chan struct{}
}

// New creates a new Scheduler instance
func New(st store.Store, checkInterval time.Duration) *Scheduler {
	if checkInterval <= 0 {
		checkInterval = 5 * time.Second // default: check every 5 seconds
	}
	return &Scheduler{
		store:         st,
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background scheduling loop
func (s *Scheduler) Start() {
	log.Printf("📅 Scheduler started (check interval: %v)", s.checkInterval)
	go s.run()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	log.Println("📅 Stopping scheduler...")
	close(s.stopCh)
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processPendingJobs()
			s.checkStaleJobs()
		case <-s.stopCh:
			log.Println("📅 Scheduler stopped")
			return
		}
	}
}

// processPendingJobs checks pending jobs and queues them if no workers available
func (s *Scheduler) processPendingJobs() {
	// Get all pending jobs
	allJobs := s.store.GetAllJobs()

	pendingJobs := []*models.Job{}
	for _, job := range allJobs {
		if job.Status == models.JobStatusPending {
			pendingJobs = append(pendingJobs, job)
		}
	}

	if len(pendingJobs) == 0 {
		return
	}

	log.Printf("📅 Scheduler: Found %d pending jobs", len(pendingJobs))

	// Get all available nodes
	nodes := s.store.GetAllNodes()
	log.Printf("📅 Scheduler: Total nodes registered: %d", len(nodes))

	availableNodes := []*models.Node{}
	for _, node := range nodes {
		log.Printf("📅 Scheduler: Node %s status: %s", node.ID, node.Status)
		if node.Status == "available" {
			availableNodes = append(availableNodes, node)
		}
	}

	log.Printf("📅 Scheduler: Available nodes: %d", len(availableNodes))

	// If no available workers, queue all pending jobs
	if len(availableNodes) == 0 {
		log.Printf("📋 Scheduler: No available workers - queuing %d pending jobs", len(pendingJobs))
		for _, job := range pendingJobs {
			if err := s.store.UpdateJobStatus(job.ID, models.JobStatusQueued, ""); err != nil {
				log.Printf("❌ Scheduler: failed to queue job %s: %v", job.ID, err)
			} else {
				log.Printf("📋 Scheduler: Job %s queued (no workers available)", job.ID)
			}
		}
	} else {
		log.Printf("📅 Scheduler: %d workers available - jobs will be picked up via GetNextJob", len(availableNodes))
	}
}

// checkStaleJobs finds jobs that have been processing for too long and marks them as failed
func (s *Scheduler) checkStaleJobs() {
	// Get all processing jobs
	allJobs := s.store.GetAllJobs()

	staleThreshold := 30 * time.Minute // Jobs processing for > 30 minutes are considered stale
	now := time.Now()

	for _, job := range allJobs {
		if job.Status != models.JobStatusProcessing {
			continue
		}

		// Check if job has been processing for too long
		if job.StartedAt != nil && job.StartedAt.Add(staleThreshold).Before(now) {
			log.Printf("⚠️  Scheduler: Job %s is stale (processing for %v), marking as failed", 
				job.ID, now.Sub(*job.StartedAt))
			
			// Mark as failed
			if err := s.store.UpdateJobStatus(job.ID, models.JobStatusFailed, "Job stale - exceeded 30 minute timeout"); err != nil {
				log.Printf("❌ Scheduler: failed to fail stale job %s: %v", job.ID, err)
			}
		}
	}
}
