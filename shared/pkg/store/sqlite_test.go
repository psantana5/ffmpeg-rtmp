package store

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// TestSQLiteConcurrentAccess tests that concurrent database access doesn't cause locks
func TestSQLiteConcurrentAccess(t *testing.T) {
	// Create temporary database
	tmpDB := "/tmp/test_concurrent.db"
	defer os.Remove(tmpDB)
	defer os.Remove(tmpDB + "-shm")
	defer os.Remove(tmpDB + "-wal")

	store, err := NewSQLiteStore(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Register a test node
	node := &models.Node{
		ID:            "test-node-1",
		Address:       "localhost:8081",
		Type:          "worker",
		CPUThreads:    4,
		CPUModel:      "Test CPU",
		HasGPU:        false,
		RAMBytes:      8589934592,
		Status:        "available",
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	if err := store.RegisterNode(node); err != nil {
		t.Fatalf("Failed to register node: %v", err)
	}

	// Create multiple jobs concurrently
	numJobs := 20
	var wg sync.WaitGroup
	errors := make(chan error, numJobs)

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			job := &models.Job{
				ID:         fmt.Sprintf("job-%d", idx),
				Scenario:   "test",
				Confidence: "auto",
				Parameters: map[string]interface{}{"test": true},
				Status:     models.JobStatusPending,
				CreatedAt:  time.Now(),
				RetryCount: 0,
			}
			if err := store.CreateJob(job); err != nil {
				errors <- fmt.Errorf("job %d creation failed: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent job creation error: %v", err)
	}

	// Verify all jobs were created
	jobs := store.GetAllJobs()
	if len(jobs) != numJobs {
		t.Errorf("Expected %d jobs, got %d", numJobs, len(jobs))
	}

	// Test concurrent GetNextJob calls
	numWorkers := 10
	wg2 := sync.WaitGroup{}
	jobsReceived := make(chan string, numWorkers)
	errors2 := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg2.Add(1)
		go func(idx int) {
			defer wg2.Done()
			job, err := store.GetNextJob(fmt.Sprintf("worker-%d", idx))
			if err != nil {
				if err != ErrJobNotFound {
					errors2 <- fmt.Errorf("worker %d GetNextJob failed: %w", idx, err)
				}
			} else {
				jobsReceived <- job.ID
			}
		}(i)
	}

	wg2.Wait()
	close(errors2)
	close(jobsReceived)

	// Check for errors
	for err := range errors2 {
		t.Errorf("Concurrent GetNextJob error: %v", err)
	}

	// Verify we got jobs without duplicates
	receivedJobs := make(map[string]bool)
	for jobID := range jobsReceived {
		if receivedJobs[jobID] {
			t.Errorf("Job %s was assigned to multiple workers", jobID)
		}
		receivedJobs[jobID] = true
	}

	t.Logf("Successfully processed %d jobs across %d workers concurrently", len(receivedJobs), numWorkers)
}

// TestSQLiteBasicOperations tests basic CRUD operations
func TestSQLiteBasicOperations(t *testing.T) {
	tmpDB := "/tmp/test_basic.db"
	defer os.Remove(tmpDB)
	defer os.Remove(tmpDB + "-shm")
	defer os.Remove(tmpDB + "-wal")

	store, err := NewSQLiteStore(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test node registration
	node := &models.Node{
		ID:            "node-1",
		Address:       "localhost:8081",
		Type:          "worker",
		CPUThreads:    4,
		CPUModel:      "Intel i5",
		HasGPU:        true,
		GPUType:       "NVIDIA GTX 1080",
		RAMBytes:      8589934592,
		Status:        "available",
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}

	if err := store.RegisterNode(node); err != nil {
		t.Errorf("Failed to register node: %v", err)
	}

	// Test job creation
	job := &models.Job{
		ID:         "job-1",
		Scenario:   "4K-h265",
		Confidence: "auto",
		Parameters: map[string]interface{}{
			"duration": 3600,
			"bitrate":  "15000k",
		},
		Status:     models.JobStatusPending,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}

	if err := store.CreateJob(job); err != nil {
		t.Errorf("Failed to create job: %v", err)
	}

	// Test GetNextJob
	retrievedJob, err := store.GetNextJob("node-1")
	if err != nil {
		t.Errorf("Failed to get next job: %v", err)
	}
	if retrievedJob.ID != job.ID {
		t.Errorf("Expected job %s, got %s", job.ID, retrievedJob.ID)
	}
	if retrievedJob.Status != models.JobStatusRunning {
		t.Errorf("Expected job status %s, got %s", models.JobStatusRunning, retrievedJob.Status)
	}

	// Test job status update
	if err := store.UpdateJobStatus(job.ID, models.JobStatusCompleted, ""); err != nil {
		t.Errorf("Failed to update job status: %v", err)
	}

	// Verify job was updated
	updatedJob, err := store.GetJob(job.ID)
	if err != nil {
		t.Errorf("Failed to get updated job: %v", err)
	}
	if updatedJob.Status != models.JobStatusCompleted {
		t.Errorf("Expected job status %s, got %s", models.JobStatusCompleted, updatedJob.Status)
	}
}
