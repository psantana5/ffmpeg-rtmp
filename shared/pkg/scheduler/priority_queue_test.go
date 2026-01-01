package scheduler

import (
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

func TestPriorityQueueManager_GetPriorityWeight(t *testing.T) {
	tests := []struct {
		priority string
		expected int
	}{
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"invalid", 2}, // defaults to medium
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			weight := GetPriorityWeight(tt.priority)
			if weight != tt.expected {
				t.Errorf("GetPriorityWeight(%q) = %d, expected %d", tt.priority, weight, tt.expected)
			}
		})
	}
}

func TestPriorityQueueManager_GetQueueWeight(t *testing.T) {
	tests := []struct {
		queue    string
		expected int
	}{
		{"live", 10},
		{"default", 5},
		{"batch", 1},
		{"invalid", 5}, // defaults to default
	}

	for _, tt := range tests {
		t.Run(tt.queue, func(t *testing.T) {
			weight := GetQueueWeight(tt.queue)
			if weight != tt.expected {
				t.Errorf("GetQueueWeight(%q) = %d, expected %d", tt.queue, weight, tt.expected)
			}
		})
	}
}

func TestPriorityQueueManager_SortJobsByPriority(t *testing.T) {
	st := store.NewMemoryStore()
	pqm := NewPriorityQueueManager(st)

	now := time.Now()

	// Create jobs with different priorities
	jobs := []*models.Job{
		{
			ID:        "job1",
			Queue:     "batch",
			Priority:  "low",
			CreatedAt: now,
		},
		{
			ID:        "job2",
			Queue:     "live",
			Priority:  "high",
			CreatedAt: now.Add(1 * time.Second),
		},
		{
			ID:        "job3",
			Queue:     "default",
			Priority:  "medium",
			CreatedAt: now.Add(2 * time.Second),
		},
		{
			ID:        "job4",
			Queue:     "live",
			Priority:  "medium",
			CreatedAt: now.Add(3 * time.Second),
		},
	}

	sorted := pqm.SortJobsByPriority(jobs)

	// Expected order: live/high, live/medium, default/medium, batch/low
	expectedOrder := []string{"job2", "job4", "job3", "job1"}

	for i, expectedID := range expectedOrder {
		if sorted[i].ID != expectedID {
			t.Errorf("Position %d: expected %s, got %s", i, expectedID, sorted[i].ID)
		}
	}
}

func TestPriorityQueueManager_SortJobsByPriority_SamePriorityFIFO(t *testing.T) {
	st := store.NewMemoryStore()
	pqm := NewPriorityQueueManager(st)

	now := time.Now()

	// Create jobs with same priority but different timestamps
	jobs := []*models.Job{
		{
			ID:        "job1",
			Queue:     "default",
			Priority:  "high",
			CreatedAt: now.Add(2 * time.Second),
		},
		{
			ID:        "job2",
			Queue:     "default",
			Priority:  "high",
			CreatedAt: now, // Oldest
		},
		{
			ID:        "job3",
			Queue:     "default",
			Priority:  "high",
			CreatedAt: now.Add(1 * time.Second),
		},
	}

	sorted := pqm.SortJobsByPriority(jobs)

	// Expected order: job2 (oldest), job3, job1 (newest)
	expectedOrder := []string{"job2", "job3", "job1"}

	for i, expectedID := range expectedOrder {
		if sorted[i].ID != expectedID {
			t.Errorf("Position %d: expected %s, got %s", i, expectedID, sorted[i].ID)
		}
	}
}

func TestPriorityQueueManager_GetNextHighPriorityJob(t *testing.T) {
	st := store.NewMemoryStore()
	pqm := NewPriorityQueueManager(st)

	now := time.Now()

	// Create jobs with different priorities
	lowPriorityJob := &models.Job{
		ID:        "job1",
		Scenario:  "test",
		Queue:     "batch",
		Priority:  "low",
		Status:    models.JobStatusQueued,
		CreatedAt: now,
	}
	st.CreateJob(lowPriorityJob)

	highPriorityJob := &models.Job{
		ID:        "job2",
		Scenario:  "test",
		Queue:     "live",
		Priority:  "high",
		Status:    models.JobStatusQueued,
		CreatedAt: now.Add(1 * time.Second),
	}
	st.CreateJob(highPriorityJob)

	// Get next job
	job, err := pqm.GetNextHighPriorityJob("node1")
	if err != nil {
		t.Fatalf("GetNextHighPriorityJob failed: %v", err)
	}

	if job == nil {
		t.Fatal("Expected a job, got nil")
	}

	// Should get the high priority job
	if job.ID != "job2" {
		t.Errorf("Expected job2, got %s", job.ID)
	}

	if job.Status != models.JobStatusAssigned {
		t.Errorf("Expected status assigned, got %s", job.Status)
	}

	if job.NodeID != "node1" {
		t.Errorf("Expected node_id node1, got %s", job.NodeID)
	}
}

func TestPriorityQueueManager_GetNextHighPriorityJob_NoJobs(t *testing.T) {
	st := store.NewMemoryStore()
	pqm := NewPriorityQueueManager(st)

	// Get next job when no jobs available
	job, err := pqm.GetNextHighPriorityJob("node1")
	if err != nil {
		t.Fatalf("GetNextHighPriorityJob failed: %v", err)
	}

	if job != nil {
		t.Errorf("Expected nil job, got %v", job)
	}
}

func TestPriorityQueueManager_GetQueueStats(t *testing.T) {
	st := store.NewMemoryStore()
	pqm := NewPriorityQueueManager(st)

	now := time.Now()

	// Create jobs with different queues/priorities
	jobs := []*models.Job{
		{ID: "job1", Scenario: "test", Queue: "live", Priority: "high", Status: models.JobStatusQueued, CreatedAt: now},
		{ID: "job2", Scenario: "test", Queue: "live", Priority: "high", Status: models.JobStatusQueued, CreatedAt: now},
		{ID: "job3", Scenario: "test", Queue: "default", Priority: "medium", Status: models.JobStatusQueued, CreatedAt: now},
		{ID: "job4", Scenario: "test", Queue: "batch", Priority: "low", Status: models.JobStatusQueued, CreatedAt: now},
		{ID: "job5", Scenario: "test", Queue: "default", Priority: "high", Status: models.JobStatusCompleted, CreatedAt: now}, // Should not count
	}

	for _, job := range jobs {
		st.CreateJob(job)
	}

	stats := pqm.GetQueueStats()

	// Verify counts
	if stats["live_high"] != 2 {
		t.Errorf("Expected 2 live_high jobs, got %d", stats["live_high"])
	}

	if stats["default_medium"] != 1 {
		t.Errorf("Expected 1 default_medium job, got %d", stats["default_medium"])
	}

	if stats["batch_low"] != 1 {
		t.Errorf("Expected 1 batch_low job, got %d", stats["batch_low"])
	}

	if stats["total"] != 4 {
		t.Errorf("Expected 4 total jobs, got %d", stats["total"])
	}

	// Completed job should not be counted
	if stats["default_high"] != 0 {
		t.Errorf("Expected 0 default_high jobs, got %d", stats["default_high"])
	}
}
