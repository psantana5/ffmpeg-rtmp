package scheduler

import (
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

func TestRecoveryManager_RecoverFailedJobs(t *testing.T) {
	st := store.NewMemoryStore()
	rm := NewRecoveryManager(st, 3, 2*time.Minute)

	// Create a failed job with transient error
	job := &models.Job{
		ID:         "job1",
		Scenario:   "test",
		Status:     models.JobStatusFailed,
		Error:      "connection refused",
		RetryCount: 1,
		CreatedAt:  time.Now(),
	}
	st.CreateJob(job)

	// Run recovery
	rm.RecoverFailedJobs()

	// Verify job was reset
	recovered, err := st.GetJob("job1")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if recovered.Status != models.JobStatusPending {
		t.Errorf("Expected status pending, got %s", recovered.Status)
	}

	if recovered.RetryCount != 2 {
		t.Errorf("Expected retry count 2, got %d", recovered.RetryCount)
	}
}

func TestRecoveryManager_RecoverFailedJobs_MaxRetriesExceeded(t *testing.T) {
	st := store.NewMemoryStore()
	rm := NewRecoveryManager(st, 3, 2*time.Minute)

	// Create a failed job that exceeded max retries
	job := &models.Job{
		ID:         "job1",
		Scenario:   "test",
		Status:     models.JobStatusFailed,
		Error:      "timeout",
		RetryCount: 3,
		CreatedAt:  time.Now(),
	}
	st.CreateJob(job)

	// Run recovery
	rm.RecoverFailedJobs()

	// Verify job was NOT reset (still failed)
	recovered, err := st.GetJob("job1")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if recovered.Status != models.JobStatusFailed {
		t.Errorf("Expected status failed, got %s", recovered.Status)
	}
}

func TestRecoveryManager_DetectDeadNodes(t *testing.T) {
	st := store.NewMemoryStore()
	rm := NewRecoveryManager(st, 3, 2*time.Minute)

	// Create a node with old heartbeat
	node := &models.Node{
		ID:            "node1",
		Address:       "test-node",
		Status:        "available",
		LastHeartbeat: time.Now().Add(-5 * time.Minute),
		RegisteredAt:  time.Now().Add(-10 * time.Minute),
	}
	st.RegisterNode(node)

	// Detect dead nodes
	deadNodes := rm.DetectDeadNodes()

	if len(deadNodes) != 1 {
		t.Errorf("Expected 1 dead node, got %d", len(deadNodes))
	}

	if deadNodes[0] != "node1" {
		t.Errorf("Expected node1 to be dead, got %s", deadNodes[0])
	}

	// Verify node was marked offline
	updated, err := st.GetNode("node1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}

	if updated.Status != "offline" {
		t.Errorf("Expected status offline, got %s", updated.Status)
	}
}

func TestRecoveryManager_ReassignJobsFromDeadNodes(t *testing.T) {
	st := store.NewMemoryStore()
	rm := NewRecoveryManager(st, 3, 2*time.Minute)

	// Create a dead node
	node := &models.Node{
		ID:            "node1",
		Address:       "test-node",
		Status:        "offline",
		LastHeartbeat: time.Now().Add(-5 * time.Minute),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(node)

	// Create a job assigned to the dead node
	now := time.Now()
	job := &models.Job{
		ID:        "job1",
		Scenario:  "test",
		Status:    models.JobStatusProcessing,
		NodeID:    "node1",
		CreatedAt: now,
		StartedAt: &now,
	}
	st.CreateJob(job)

	// Reassign jobs
	count := rm.ReassignJobsFromDeadNodes([]string{"node1"})

	if count != 1 {
		t.Errorf("Expected 1 job reassigned, got %d", count)
	}

	// Verify job was reset
	reassigned, err := st.GetJob("job1")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if reassigned.Status != models.JobStatusPending {
		t.Errorf("Expected status pending, got %s", reassigned.Status)
	}

	if reassigned.NodeID != "" {
		t.Errorf("Expected empty node ID, got %s", reassigned.NodeID)
	}
}

func TestRecoveryManager_isTransientFailure(t *testing.T) {
	rm := NewRecoveryManager(nil, 3, 2*time.Minute)

	tests := []struct {
		name      string
		error     string
		expected  bool
	}{
		{"connection refused", "connection refused", true},
		{"timeout", "operation timeout", true},
		{"network error", "network error occurred", true},
		{"stale job", "job is stale", true},
		{"permanent error", "invalid input format", false},
		{"empty error", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.Job{Error: tt.error}
			result := rm.isTransientFailure(job)
			if result != tt.expected {
				t.Errorf("isTransientFailure(%q) = %v, expected %v", tt.error, result, tt.expected)
			}
		})
	}
}

func TestRecoveryManager_RunRecoveryCheck(t *testing.T) {
	st := store.NewMemoryStore()
	rm := NewRecoveryManager(st, 3, 2*time.Minute)

	// Create test data
	deadNode := &models.Node{
		ID:            "node1",
		Address:       "dead-node",
		Status:        "available",
		LastHeartbeat: time.Now().Add(-5 * time.Minute),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(deadNode)

	now := time.Now()
	jobOnDeadNode := &models.Job{
		ID:        "job1",
		Scenario:  "test",
		Status:    models.JobStatusProcessing,
		NodeID:    "node1",
		CreatedAt: now,
		StartedAt: &now,
	}
	st.CreateJob(jobOnDeadNode)

	failedJob := &models.Job{
		ID:         "job2",
		Scenario:   "test",
		Status:     models.JobStatusFailed,
		Error:      "timeout",
		RetryCount: 1,
		CreatedAt:  now,
	}
	st.CreateJob(failedJob)

	// Run full recovery check
	rm.RunRecoveryCheck()

	// Note: UpdateNodeStatus also updates LastHeartbeat, so the node won't be considered dead anymore
	// This is current behavior - in production, nodes stay dead until they reconnect

	// Verify job from dead node was reassigned
	job1, _ := st.GetJob("job1")
	if job1.Status != models.JobStatusPending {
		t.Errorf("Expected job1 pending, got %s", job1.Status)
	}

	// Verify failed job was recovered
	job2, _ := st.GetJob("job2")
	if job2.Status != models.JobStatusPending {
		t.Errorf("Expected job2 pending, got %s", job2.Status)
	}
}
