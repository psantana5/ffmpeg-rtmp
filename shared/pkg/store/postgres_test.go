package store

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// TestPostgreSQLIntegration tests PostgreSQL store with a real database
// Set DATABASE_DSN environment variable to run: export DATABASE_DSN="postgresql://..."
func TestPostgreSQLIntegration(t *testing.T) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL integration test: DATABASE_DSN not set")
	}

	// Create store
	store, err := NewStore(Config{
		Type: "postgres",
		DSN:  dsn,
	})
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL store: %v", err)
	}
	defer store.Close()

	// Test health check
	if err := store.HealthCheck(); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	t.Run("NodeOperations", func(t *testing.T) {
		testNodeOperations(t, store)
	})

	t.Run("JobOperations", func(t *testing.T) {
		testJobOperations(t, store)
	})

	t.Run("FSMOperations", func(t *testing.T) {
		testFSMOperations(t, store)
	})
}

func testNodeOperations(t *testing.T, store Store) {
	node := &models.Node{
		ID:            fmt.Sprintf("test-node-%d", time.Now().Unix()),
		Name:          "Test Node",
		Address:       fmt.Sprintf("localhost:%d", 8000+time.Now().Unix()%1000),
		Type:          "worker",
		CPUThreads:    4,
		CPUModel:      "Test CPU",
		HasGPU:        true,
		GPUType:       "NVIDIA GTX 1080",
		RAMTotalBytes: 8589934592,
		Status:        "available",
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}

	// Register node
	if err := store.RegisterNode(node); err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	// Get node
	retrieved, err := store.GetNode(node.ID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if retrieved.ID != node.ID {
		t.Errorf("Expected node ID %s, got %s", node.ID, retrieved.ID)
	}

	// Update heartbeat
	if err := store.UpdateNodeHeartbeat(node.ID); err != nil {
		t.Fatalf("UpdateNodeHeartbeat failed: %v", err)
	}

	// Update status
	if err := store.UpdateNodeStatus(node.ID, "busy"); err != nil {
		t.Fatalf("UpdateNodeStatus failed: %v", err)
	}

	// Get all nodes
	nodes := store.GetAllNodes()
	if len(nodes) == 0 {
		t.Error("Expected at least one node")
	}

	// Delete node
	if err := store.DeleteNode(node.ID); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
}

func testJobOperations(t *testing.T, store Store) {
	job := &models.Job{
		ID:         fmt.Sprintf("test-job-%d", time.Now().Unix()),
		Scenario:   "test",
		Confidence: "auto",
		Engine:     "ffmpeg",
		Parameters: map[string]interface{}{
			"bitrate":  "5M",
			"duration": 60,
		},
		Status:    models.JobStatusQueued,
		Queue:     "default",
		Priority:  "medium",
		CreatedAt: time.Now(),
	}

	// Create job
	if err := store.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// Get job
	retrieved, err := store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if retrieved.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, retrieved.ID)
	}
	if retrieved.SequenceNumber == 0 {
		t.Error("Expected sequence number to be set")
	}

	// Get by sequence number
	bySeq, err := store.GetJobBySequenceNumber(retrieved.SequenceNumber)
	if err != nil {
		t.Fatalf("GetJobBySequenceNumber failed: %v", err)
	}
	if bySeq.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, bySeq.ID)
	}

	// Update status
	if err := store.UpdateJobStatus(job.ID, models.JobStatusRunning, ""); err != nil {
		t.Fatalf("UpdateJobStatus failed: %v", err)
	}

	// Update progress
	if err := store.UpdateJobProgress(job.ID, 50); err != nil {
		t.Fatalf("UpdateJobProgress failed: %v", err)
	}

	// Update failure reason
	if err := store.UpdateJobFailureReason(job.ID, models.FailureReasonRuntimeError, "test error"); err != nil {
		t.Fatalf("UpdateJobFailureReason failed: %v", err)
	}

	// Get all jobs
	jobs := store.GetAllJobs()
	if len(jobs) == 0 {
		t.Error("Expected at least one job")
	}
}

func testFSMOperations(t *testing.T, store Store) {
	// Create node for assignment
	node := &models.Node{
		ID:            fmt.Sprintf("fsm-node-%d", time.Now().Unix()),
		Address:       fmt.Sprintf("localhost:%d", 9000+time.Now().Unix()%1000),
		Type:          "worker",
		CPUThreads:    4,
		CPUModel:      "Test CPU",
		Status:        "available",
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	if err := store.RegisterNode(node); err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}
	defer store.DeleteNode(node.ID)

	// Create job
	job := &models.Job{
		ID:         fmt.Sprintf("fsm-job-%d", time.Now().Unix()),
		Scenario:   "fsm-test",
		Confidence: "auto",
		Engine:     "ffmpeg",
		Status:     models.JobStatusQueued,
		CreatedAt:  time.Now(),
	}
	if err := store.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// Test state transition
	changed, err := store.TransitionJobState(job.ID, models.JobStatusQueued, "test transition")
	if err != nil {
		t.Fatalf("TransitionJobState failed: %v", err)
	}
	if changed {
		t.Error("Expected no change when transitioning to same state")
	}

	// Test assignment
	assigned, err := store.AssignJobToWorker(job.ID, node.ID)
	if err != nil {
		t.Fatalf("AssignJobToWorker failed: %v", err)
	}
	if !assigned {
		t.Error("Expected job to be assigned")
	}

	// Verify job is assigned
	retrieved, _ := store.GetJob(job.ID)
	if retrieved.Status != models.JobStatusAssigned {
		t.Errorf("Expected status Assigned, got %s", retrieved.Status)
	}
	if retrieved.NodeID != node.ID {
		t.Errorf("Expected node ID %s, got %s", node.ID, retrieved.NodeID)
	}

	// Test GetJobsInState
	assignedJobs, err := store.GetJobsInState(models.JobStatusAssigned)
	if err != nil {
		t.Fatalf("GetJobsInState failed: %v", err)
	}
	found := false
	for _, j := range assignedJobs {
		if j.ID == job.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Job not found in assigned state")
	}

	// Test completion
	completed, err := store.CompleteJob(job.ID, node.ID)
	if err != nil {
		t.Fatalf("CompleteJob failed: %v", err)
	}
	if !completed {
		t.Error("Expected job to be completed")
	}

	// Verify completion
	retrieved, _ = store.GetJob(job.ID)
	if retrieved.Status != models.JobStatusCompleted {
		t.Errorf("Expected status Completed, got %s", retrieved.Status)
	}
}

// TestPostgreSQLConcurrency tests concurrent operations
func TestPostgreSQLConcurrency(t *testing.T) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL concurrency test: DATABASE_DSN not set")
	}

	store, err := NewStore(Config{
		Type: "postgres",
		DSN:  dsn,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create 20 jobs concurrently (same as SQLite test)
	numJobs := 20
	errors := make(chan error, numJobs)
	done := make(chan bool, numJobs)

	for i := 0; i < numJobs; i++ {
		go func(idx int) {
			job := &models.Job{
				ID:         fmt.Sprintf("concurrent-job-%d-%d", time.Now().Unix(), idx),
				Scenario:   "concurrent-test",
				Confidence: "auto",
				Engine:     "ffmpeg",
				Status:     models.JobStatusQueued,
				CreatedAt:  time.Now(),
			}
			if err := store.CreateJob(job); err != nil {
				errors <- fmt.Errorf("job %d failed: %w", idx, err)
			}
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < numJobs; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent job creation error: %v", err)
	}

	// Verify all jobs created
	jobs := store.GetAllJobs()
	t.Logf("Created %d jobs concurrently, total jobs in DB: %d", numJobs, len(jobs))
	
	// Note: We don't check exact count because there might be jobs from other tests
	// Just verify no errors occurred
}
