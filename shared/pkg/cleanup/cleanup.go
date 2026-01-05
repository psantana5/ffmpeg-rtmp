package cleanup

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// CleanupConfig defines retention policies and cleanup intervals
type CleanupConfig struct {
	Enabled            bool
	JobRetentionDays   int
	CleanupInterval    time.Duration
	VacuumInterval     time.Duration
	DeleteBatchSize    int
}

// DefaultConfig returns sensible defaults for cleanup
func DefaultConfig() CleanupConfig {
	return CleanupConfig{
		Enabled:          true,
		JobRetentionDays: 7,
		CleanupInterval:  24 * time.Hour,
		VacuumInterval:   7 * 24 * time.Hour,
		DeleteBatchSize:  100,
	}
}

// Store interface for cleanup operations
type Store interface {
	GetJobs(status string) ([]models.Job, error)
	DeleteJob(id string) error
	Vacuum() error
}

// CleanupManager handles automatic cleanup of old jobs and maintenance
type CleanupManager struct {
	config       CleanupConfig
	store        Store
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	
	mu           sync.RWMutex
	stats        CleanupStats
}

// CleanupStats tracks cleanup operations
type CleanupStats struct {
	LastCleanupTime      time.Time
	LastVacuumTime       time.Time
	TotalJobsDeleted     int64
	TotalVacuumRuns      int64
	LastCleanupDuration  time.Duration
	LastVacuumDuration   time.Duration
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(config CleanupConfig, store Store) *CleanupManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &CleanupManager{
		config: config,
		store:  store,
		ctx:    ctx,
		cancel: cancel,
		stats:  CleanupStats{},
	}
}

// Start begins the automatic cleanup process
func (cm *CleanupManager) Start() {
	if !cm.config.Enabled {
		log.Println("[Cleanup] Cleanup manager disabled")
		return
	}

	log.Printf("[Cleanup] Starting cleanup manager (retention: %d days, interval: %v)\n",
		cm.config.JobRetentionDays, cm.config.CleanupInterval)

	cm.wg.Add(2)
	go cm.cleanupLoop()
	go cm.vacuumLoop()
}

// Stop gracefully stops the cleanup manager
func (cm *CleanupManager) Stop() {
	log.Println("[Cleanup] Stopping cleanup manager...")
	cm.cancel()
	cm.wg.Wait()
	log.Println("[Cleanup] Cleanup manager stopped")
}

// cleanupLoop runs periodic job cleanup
func (cm *CleanupManager) cleanupLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.config.CleanupInterval)
	defer ticker.Stop()

	// Run initial cleanup after short delay
	time.Sleep(5 * time.Minute)
	cm.cleanupOldJobs()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.cleanupOldJobs()
		}
	}
}

// vacuumLoop runs periodic database vacuum
func (cm *CleanupManager) vacuumLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.config.VacuumInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.vacuum()
		}
	}
}

// cleanupOldJobs deletes completed/failed jobs older than retention period
func (cm *CleanupManager) cleanupOldJobs() {
	startTime := time.Now()
	log.Println("[Cleanup] Starting job cleanup...")

	cutoffTime := time.Now().Add(-time.Duration(cm.config.JobRetentionDays) * 24 * time.Hour)
	deletedCount := 0

	// Cleanup completed jobs
	if err := cm.cleanupJobsByStatus("completed", cutoffTime, &deletedCount); err != nil {
		log.Printf("[Cleanup] Error cleaning completed jobs: %v\n", err)
	}

	// Cleanup failed jobs
	if err := cm.cleanupJobsByStatus("failed", cutoffTime, &deletedCount); err != nil {
		log.Printf("[Cleanup] Error cleaning failed jobs: %v\n", err)
	}

	// Cleanup canceled jobs
	if err := cm.cleanupJobsByStatus("canceled", cutoffTime, &deletedCount); err != nil {
		log.Printf("[Cleanup] Error cleaning canceled jobs: %v\n", err)
	}

	duration := time.Since(startTime)
	
	cm.mu.Lock()
	cm.stats.LastCleanupTime = time.Now()
	cm.stats.LastCleanupDuration = duration
	cm.stats.TotalJobsDeleted += int64(deletedCount)
	cm.mu.Unlock()

	log.Printf("[Cleanup] Job cleanup complete: deleted %d jobs in %v\n", deletedCount, duration)
}

// cleanupJobsByStatus deletes jobs of a specific status older than cutoff time
func (cm *CleanupManager) cleanupJobsByStatus(status string, cutoffTime time.Time, deletedCount *int) error {
	jobs, err := cm.store.GetJobs(status)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		// Check if job is older than retention period
		// Use CompletedAt if available, otherwise CreatedAt
		compareTime := job.CreatedAt
		if job.CompletedAt != nil {
			compareTime = *job.CompletedAt
		}
		
		if compareTime.Before(cutoffTime) {
			if err := cm.store.DeleteJob(job.ID); err != nil {
				log.Printf("[Cleanup] Failed to delete job %s: %v\n", job.ID, err)
				continue
			}
			*deletedCount++
			
			// Rate limit deletions to avoid overloading database
			if *deletedCount%cm.config.DeleteBatchSize == 0 {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	return nil
}

// vacuum performs database maintenance
func (cm *CleanupManager) vacuum() {
	startTime := time.Now()
	log.Println("[Cleanup] Starting database vacuum...")

	if err := cm.store.Vacuum(); err != nil {
		log.Printf("[Cleanup] Database vacuum failed: %v\n", err)
		return
	}

	duration := time.Since(startTime)
	
	cm.mu.Lock()
	cm.stats.LastVacuumTime = time.Now()
	cm.stats.LastVacuumDuration = duration
	cm.stats.TotalVacuumRuns++
	cm.mu.Unlock()

	log.Printf("[Cleanup] Database vacuum complete in %v\n", duration)
}

// CleanupNow triggers an immediate cleanup run
func (cm *CleanupManager) CleanupNow() {
	log.Println("[Cleanup] Manual cleanup triggered")
	cm.cleanupOldJobs()
}

// VacuumNow triggers an immediate vacuum run
func (cm *CleanupManager) VacuumNow() {
	log.Println("[Cleanup] Manual vacuum triggered")
	cm.vacuum()
}

// GetStats returns current cleanup statistics
func (cm *CleanupManager) GetStats() CleanupStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.stats
}
