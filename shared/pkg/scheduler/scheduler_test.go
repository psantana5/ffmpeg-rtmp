package scheduler

import (
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// TestCheckStaleJobs_BatchJobs tests stale detection for batch jobs
func TestCheckStaleJobs_BatchJobs(t *testing.T) {
	// Create in-memory store
	st := store.NewMemoryStore()

	// Create a batch job that started 35 minutes ago (stale)
	now := time.Now()
	staleStartTime := now.Add(-35 * time.Minute)
	staleJob := &models.Job{
		ID:        "batch-stale-job",
		Scenario:  "test-batch",
		Status:    models.JobStatusProcessing,
		Queue:     "batch", // batch job
		StartedAt: &staleStartTime,
		CreatedAt: now,
	}
	st.CreateJob(staleJob)

	// Create a batch job that started 10 minutes ago (not stale)
	activeStartTime := now.Add(-10 * time.Minute)
	activeJob := &models.Job{
		ID:        "batch-active-job",
		Scenario:  "test-batch",
		Status:    models.JobStatusProcessing,
		Queue:     "batch", // batch job
		StartedAt: &activeStartTime,
		CreatedAt: now,
	}
	st.CreateJob(activeJob)

	// Run stale job check
	scheduler := New(st, 5*time.Second)
	scheduler.checkStaleJobs()

	// Check that stale job is marked as failed
	updatedStaleJob, err := st.GetJob("batch-stale-job")
	if err != nil {
		t.Fatalf("Failed to get stale job: %v", err)
	}
	if updatedStaleJob.Status != models.JobStatusFailed {
		t.Errorf("Expected stale batch job to be marked as failed, got %s", updatedStaleJob.Status)
	}

	// Check that active job is still processing
	updatedActiveJob, err := st.GetJob("batch-active-job")
	if err != nil {
		t.Fatalf("Failed to get active job: %v", err)
	}
	if updatedActiveJob.Status != models.JobStatusProcessing {
		t.Errorf("Expected active batch job to still be processing, got %s", updatedActiveJob.Status)
	}
}

// TestCheckStaleJobs_LiveJobs tests stale detection for live jobs
func TestCheckStaleJobs_LiveJobs(t *testing.T) {
	// Create in-memory store
	st := store.NewMemoryStore()

	// Create a live job with no activity for 10 minutes (stale)
	now := time.Now()
	staleStartTime := now.Add(-40 * time.Minute)
	staleActivityTime := now.Add(-10 * time.Minute)
	staleJob := &models.Job{
		ID:             "live-stale-job",
		Scenario:       "live-stream",
		Status:         models.JobStatusProcessing,
		Queue:          "live", // live job
		StartedAt:      &staleStartTime,
		LastActivityAt: &staleActivityTime,
		CreatedAt:      now,
	}
	st.CreateJob(staleJob)

	// Create a live job with recent activity (not stale)
	activeStartTime := now.Add(-50 * time.Minute)
	activeActivityTime := now.Add(-2 * time.Minute)
	activeJob := &models.Job{
		ID:             "live-active-job",
		Scenario:       "live-stream",
		Status:         models.JobStatusProcessing,
		Queue:          "live", // live job
		StartedAt:      &activeStartTime,
		LastActivityAt: &activeActivityTime,
		CreatedAt:      now,
	}
	st.CreateJob(activeJob)

	// Run stale job check
	scheduler := New(st, 5*time.Second)
	scheduler.checkStaleJobs()

	// Check that stale job is marked as failed
	updatedStaleJob, err := st.GetJob("live-stale-job")
	if err != nil {
		t.Fatalf("Failed to get stale job: %v", err)
	}
	if updatedStaleJob.Status != models.JobStatusFailed {
		t.Errorf("Expected stale live job to be marked as failed, got %s", updatedStaleJob.Status)
	}

	// Check that active job is still processing
	updatedActiveJob, err := st.GetJob("live-active-job")
	if err != nil {
		t.Fatalf("Failed to get active job: %v", err)
	}
	if updatedActiveJob.Status != models.JobStatusProcessing {
		t.Errorf("Expected active live job to still be processing, got %s", updatedActiveJob.Status)
	}
}

// TestCheckStaleJobs_LiveJobLongRunning tests that live jobs can run for a long time if they show activity
func TestCheckStaleJobs_LiveJobLongRunning(t *testing.T) {
	// Create in-memory store
	st := store.NewMemoryStore()

	// Create a live job that started 2 hours ago but has recent activity (not stale)
	now := time.Now()
	startTime := now.Add(-2 * time.Hour)
	activityTime := now.Add(-1 * time.Minute)
	longRunningJob := &models.Job{
		ID:             "live-long-running-job",
		Scenario:       "live-stream",
		Status:         models.JobStatusProcessing,
		Queue:          "live", // live job
		StartedAt:      &startTime,
		LastActivityAt: &activityTime,
		CreatedAt:      now.Add(-2 * time.Hour),
	}
	st.CreateJob(longRunningJob)

	// Run stale job check
	scheduler := New(st, 5*time.Second)
	scheduler.checkStaleJobs()

	// Check that long-running job is still processing (not marked as failed)
	updatedJob, err := st.GetJob("live-long-running-job")
	if err != nil {
		t.Fatalf("Failed to get long-running job: %v", err)
	}
	if updatedJob.Status != models.JobStatusProcessing {
		t.Errorf("Expected long-running live job to still be processing, got %s", updatedJob.Status)
	}
}

// TestCheckStaleJobs_DefaultQueue tests that default queue jobs behave like batch jobs
func TestCheckStaleJobs_DefaultQueue(t *testing.T) {
	// Create in-memory store
	st := store.NewMemoryStore()

	// Create a default queue job that started 35 minutes ago (stale)
	now := time.Now()
	staleStartTime := now.Add(-35 * time.Minute)
	staleJob := &models.Job{
		ID:        "default-stale-job",
		Scenario:  "test-default",
		Status:    models.JobStatusProcessing,
		Queue:     "default", // default queue (treated like batch)
		StartedAt: &staleStartTime,
		CreatedAt: now,
	}
	st.CreateJob(staleJob)

	// Run stale job check
	scheduler := New(st, 5*time.Second)
	scheduler.checkStaleJobs()

	// Check that stale job is marked as failed
	updatedStaleJob, err := st.GetJob("default-stale-job")
	if err != nil {
		t.Fatalf("Failed to get stale job: %v", err)
	}
	if updatedStaleJob.Status != models.JobStatusFailed {
		t.Errorf("Expected stale default queue job to be marked as failed, got %s", updatedStaleJob.Status)
	}
}
