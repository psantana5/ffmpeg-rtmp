package scheduler

import (
	"fmt"
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
	log.Printf("Scheduler started (check interval: %v)", s.checkInterval)
	go s.run()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
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
			log.Println("Scheduler stopped")
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

	log.Printf("Scheduler: Found %d pending jobs", len(pendingJobs))

	// Use atomic TryQueuePendingJob to avoid race conditions
	queuedCount := 0
	for _, job := range pendingJobs {
		queued, err := s.store.TryQueuePendingJob(job.ID)
		if err != nil {
			log.Printf("Scheduler: failed to queue job %s: %v", job.ID, err)
			continue
		}
		if queued {
			queuedCount++
			log.Printf("Scheduler: Job %s queued (no workers available)", job.ID)
		}
	}

	if queuedCount > 0 {
		log.Printf("Scheduler: Queued %d jobs (no workers available)", queuedCount)
	}
}

// checkStaleJobs finds jobs that have been processing for too long and marks them as failed
func (s *Scheduler) checkStaleJobs() {
	// Get all processing jobs
	allJobs := s.store.GetAllJobs()

	// Stale thresholds
	batchStaleThreshold := 30 * time.Minute  // Batch jobs stale after 30 minutes
	liveStaleThreshold := 5 * time.Minute    // Live jobs stale if no activity for 5 minutes
	now := time.Now()

	for _, job := range allJobs {
		if job.Status != models.JobStatusProcessing {
			continue
		}

		// Determine if this is a live job (queue="live")
		isLiveJob := job.Queue == "live"

		if isLiveJob {
			// For live jobs: check last activity time (heartbeat-based staleness)
			// Live jobs can run indefinitely but must show activity
			if job.LastActivityAt != nil {
				timeSinceActivity := now.Sub(*job.LastActivityAt)
				if timeSinceActivity > liveStaleThreshold {
					log.Printf("Scheduler: Live job %s is stale (no activity for %v), marking as failed", 
						job.ID, timeSinceActivity)
					
					// Mark as failed
					if err := s.store.UpdateJobStatus(job.ID, models.JobStatusFailed, 
						fmt.Sprintf("Live job stale - no activity for %v (threshold: %v)", 
							timeSinceActivity, liveStaleThreshold)); err != nil {
						log.Printf("Scheduler: failed to fail stale live job %s: %v", job.ID, err)
					}
				}
			} else if job.StartedAt != nil {
				// Defensive fallback: if LastActivityAt is not set (shouldn't happen for new jobs),
				// use StartedAt as the activity reference point
				timeSinceStart := now.Sub(*job.StartedAt)
				if timeSinceStart > liveStaleThreshold {
					log.Printf("Scheduler: Live job %s is stale (no activity since start for %v), marking as failed", 
						job.ID, timeSinceStart)
					
					if err := s.store.UpdateJobStatus(job.ID, models.JobStatusFailed, 
						fmt.Sprintf("Live job stale - no activity since start for %v (threshold: %v)", 
							timeSinceStart, liveStaleThreshold)); err != nil {
						log.Printf("Scheduler: failed to fail stale live job %s: %v", job.ID, err)
					}
				}
			}
		} else {
			// For batch jobs: check total processing time (time-based staleness)
			// Batch jobs are expected to complete within a fixed time
			if job.StartedAt != nil && job.StartedAt.Add(batchStaleThreshold).Before(now) {
				log.Printf("Scheduler: Batch job %s is stale (processing for %v), marking as failed", 
					job.ID, now.Sub(*job.StartedAt))
				
				// Mark as failed
				if err := s.store.UpdateJobStatus(job.ID, models.JobStatusFailed, 
					fmt.Sprintf("Batch job stale - exceeded %v timeout", batchStaleThreshold)); err != nil {
					log.Printf("Scheduler: failed to fail stale batch job %s: %v", job.ID, err)
				}
			}
		}
	}
}
