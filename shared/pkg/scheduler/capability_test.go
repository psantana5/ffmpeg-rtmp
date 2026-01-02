package scheduler

import (
	"testing"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

func TestCapabilityFiltering_GPUJobOnCPUCluster(t *testing.T) {
	// Test: GPU job on CPU-only cluster should be rejected
	st := store.NewMemoryStore()
	config := DefaultSchedulerConfig()
	config.SchedulingInterval = 100 * time.Millisecond

	sched := NewProductionScheduler(st, config)

	// Register CPU-only worker
	cpuWorker := &models.Node{
		ID:            "cpu-worker",
		Name:          "cpu-only",
		Address:       "http://localhost:8080",
		Status:        "available",
		CPUThreads:    8,
		HasGPU:        false,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(cpuWorker)

	// Create GPU job
	gpuJob := &models.Job{
		ID:             "gpu-job",
		SequenceNumber: 1,
		Scenario:       "4K60-h264-nvenc",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now(),
		Engine:         "ffmpeg",
		Parameters: map[string]interface{}{
			"codec":   "h264_nvenc",
			"bitrate": "10M",
		},
	}
	st.CreateJob(gpuJob)

	// Run scheduling cycle
	sched.runSchedulingCycle()

	// Verify job was rejected
	updatedJob, err := st.GetJob("gpu-job")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if updatedJob.Status != models.JobStatusRejected {
		t.Errorf("Expected job to be rejected, got status: %s", updatedJob.Status)
	}

	if updatedJob.FailureReason != models.FailureReasonCapabilityMismatch {
		t.Errorf("Expected failure reason capability_mismatch, got: %s", updatedJob.FailureReason)
	}

	// Verify worker is still available (not assigned)
	updatedWorker, _ := st.GetNode("cpu-worker")
	if updatedWorker.Status != "available" {
		t.Errorf("Expected worker to remain available, got: %s", updatedWorker.Status)
	}
}

func TestCapabilityFiltering_MixedCluster(t *testing.T) {
	// Test: GPU job on mixed cluster should be assigned to GPU worker
	st := store.NewMemoryStore()
	config := DefaultSchedulerConfig()

	sched := NewProductionScheduler(st, config)

	// Register CPU-only worker
	cpuWorker := &models.Node{
		ID:            "cpu-worker",
		Name:          "cpu-only",
		Address:       "http://localhost:8080",
		Status:        "available",
		CPUThreads:    8,
		HasGPU:        false,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(cpuWorker)

	// Register GPU worker
	gpuWorker := &models.Node{
		ID:              "gpu-worker",
		Name:            "gpu-enabled",
		Address:         "http://localhost:8081",
		Status:          "available",
		CPUThreads:      16,
		HasGPU:          true,
		GPUType:         "NVIDIA RTX 3090",
		GPUCapabilities: []string{"nvenc_h264", "nvenc_h265"},
		LastHeartbeat:   time.Now(),
		RegisteredAt:    time.Now(),
	}
	st.RegisterNode(gpuWorker)

	// Create GPU job
	gpuJob := &models.Job{
		ID:             "gpu-job",
		SequenceNumber: 1,
		Scenario:       "4K60-h264-nvenc",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now(),
		Engine:         "ffmpeg",
		Parameters: map[string]interface{}{
			"codec":   "h264_nvenc",
			"bitrate": "10M",
		},
	}
	st.CreateJob(gpuJob)

	// Run scheduling cycle
	sched.runSchedulingCycle()

	// Verify job was assigned to GPU worker
	updatedJob, err := st.GetJob("gpu-job")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if updatedJob.Status != models.JobStatusAssigned {
		t.Errorf("Expected job to be assigned, got status: %s", updatedJob.Status)
	}

	if updatedJob.NodeID != "gpu-worker" {
		t.Errorf("Expected job assigned to gpu-worker, got: %s", updatedJob.NodeID)
	}

	// Verify GPU worker is busy
	updatedGPUWorker, _ := st.GetNode("gpu-worker")
	if updatedGPUWorker.Status != "busy" {
		t.Errorf("Expected GPU worker to be busy, got: %s", updatedGPUWorker.Status)
	}

	// Verify CPU worker is still available
	updatedCPUWorker, _ := st.GetNode("cpu-worker")
	if updatedCPUWorker.Status != "available" {
		t.Errorf("Expected CPU worker to remain available, got: %s", updatedCPUWorker.Status)
	}
}

func TestRejection_NoRetry(t *testing.T) {
	// Test: Rejected jobs should never be retried
	st := store.NewMemoryStore()
	config := DefaultSchedulerConfig()

	sched := NewProductionScheduler(st, config)

	// Register CPU-only worker
	cpuWorker := &models.Node{
		ID:            "cpu-worker",
		Name:          "cpu-only",
		Address:       "http://localhost:8080",
		Status:        "available",
		CPUThreads:    8,
		HasGPU:        false,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(cpuWorker)

	// Create GPU job
	gpuJob := &models.Job{
		ID:             "gpu-job",
		SequenceNumber: 1,
		Scenario:       "4K60-h264-nvenc",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now(),
		MaxRetries:     3,
		Parameters: map[string]interface{}{
			"codec": "h264_nvenc",
		},
	}
	st.CreateJob(gpuJob)

	// Run scheduling cycle
	sched.runSchedulingCycle()

	// Verify job was rejected
	rejectedJob, _ := st.GetJob("gpu-job")
	if rejectedJob.Status != models.JobStatusRejected {
		t.Errorf("Expected job to be rejected, got: %s", rejectedJob.Status)
	}

	// Verify ShouldRetry returns false for rejected jobs
	if config.RetryPolicy.ShouldRetry(rejectedJob, "capability_mismatch") {
		t.Error("Rejected jobs should never be retried")
	}

	// Run cleanup cycle (which handles retries)
	sched.runCleanupCycle()

	// Verify job is still rejected and not retrying
	finalJob, _ := st.GetJob("gpu-job")
	if finalJob.Status != models.JobStatusRejected {
		t.Errorf("Rejected job should remain rejected, got: %s", finalJob.Status)
	}

	if finalJob.RetryCount > 0 {
		t.Errorf("Rejected job should have 0 retries, got: %d", finalJob.RetryCount)
	}
}

func TestRejection_DoesNotBlockOtherJobs(t *testing.T) {
	// Test: Rejection of one job should not affect other jobs
	st := store.NewMemoryStore()
	config := DefaultSchedulerConfig()

	sched := NewProductionScheduler(st, config)

	// Register CPU-only worker
	cpuWorker := &models.Node{
		ID:            "cpu-worker",
		Name:          "cpu-only",
		Address:       "http://localhost:8080",
		Status:        "available",
		CPUThreads:    8,
		HasGPU:        false,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(cpuWorker)

	// Create GPU job (will be rejected)
	gpuJob := &models.Job{
		ID:             "gpu-job",
		SequenceNumber: 1,
		Scenario:       "4K60-h264-nvenc",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now(),
		Parameters: map[string]interface{}{
			"codec": "h264_nvenc",
		},
	}
	st.CreateJob(gpuJob)

	// Create CPU job (should be assigned)
	cpuJob := &models.Job{
		ID:             "cpu-job",
		SequenceNumber: 2,
		Scenario:       "1080p-h264",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now().Add(1 * time.Second),
		Engine:         "ffmpeg",
		Parameters: map[string]interface{}{
			"codec":   "libx264",
			"bitrate": "5M",
		},
	}
	st.CreateJob(cpuJob)

	// Run scheduling cycle
	sched.runSchedulingCycle()

	// Verify GPU job was rejected
	rejectedJob, _ := st.GetJob("gpu-job")
	if rejectedJob.Status != models.JobStatusRejected {
		t.Errorf("GPU job should be rejected, got: %s", rejectedJob.Status)
	}

	// Verify CPU job was assigned
	assignedJob, _ := st.GetJob("cpu-job")
	if assignedJob.Status != models.JobStatusAssigned {
		t.Errorf("CPU job should be assigned, got: %s", assignedJob.Status)
	}

	if assignedJob.NodeID != "cpu-worker" {
		t.Errorf("CPU job should be assigned to cpu-worker, got: %s", assignedJob.NodeID)
	}

	// Verify worker is busy with CPU job
	worker, _ := st.GetNode("cpu-worker")
	if worker.CurrentJobID != "cpu-job" {
		t.Errorf("Worker should be running cpu-job, got: %s", worker.CurrentJobID)
	}
}

func TestCapabilityFiltering_CPUJobOnGPUWorker(t *testing.T) {
	// Test: CPU jobs can run on GPU workers (backwards compatible)
	st := store.NewMemoryStore()
	config := DefaultSchedulerConfig()

	sched := NewProductionScheduler(st, config)

	// Register GPU worker only
	gpuWorker := &models.Node{
		ID:              "gpu-worker",
		Name:            "gpu-enabled",
		Address:         "http://localhost:8081",
		Status:          "available",
		CPUThreads:      16,
		HasGPU:          true,
		GPUType:         "NVIDIA RTX 3090",
		GPUCapabilities: []string{"nvenc_h264", "nvenc_h265"},
		LastHeartbeat:   time.Now(),
		RegisteredAt:    time.Now(),
	}
	st.RegisterNode(gpuWorker)

	// Create CPU job
	cpuJob := &models.Job{
		ID:             "cpu-job",
		SequenceNumber: 1,
		Scenario:       "1080p-h264",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now(),
		Engine:         "ffmpeg",
		Parameters: map[string]interface{}{
			"codec":   "libx264",
			"bitrate": "5M",
		},
	}
	st.CreateJob(cpuJob)

	// Run scheduling cycle
	sched.runSchedulingCycle()

	// Verify CPU job was assigned to GPU worker
	updatedJob, err := st.GetJob("cpu-job")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if updatedJob.Status != models.JobStatusAssigned {
		t.Errorf("Expected CPU job to be assigned to GPU worker, got status: %s", updatedJob.Status)
	}

	if updatedJob.NodeID != "gpu-worker" {
		t.Errorf("Expected job assigned to gpu-worker, got: %s", updatedJob.NodeID)
	}
}

func TestSchedulerMetrics_RejectedJobs(t *testing.T) {
	// Test: Scheduler metrics should track rejected jobs separately
	st := store.NewMemoryStore()
	config := DefaultSchedulerConfig()

	sched := NewProductionScheduler(st, config)

	// Register CPU-only worker
	cpuWorker := &models.Node{
		ID:            "cpu-worker",
		Name:          "cpu-only",
		Address:       "http://localhost:8080",
		Status:        "available",
		CPUThreads:    8,
		HasGPU:        false,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	st.RegisterNode(cpuWorker)

	// Create GPU job
	gpuJob := &models.Job{
		ID:             "gpu-job",
		SequenceNumber: 1,
		Scenario:       "4K60-h264-nvenc",
		Status:         models.JobStatusQueued,
		CreatedAt:      time.Now(),
		Parameters: map[string]interface{}{
			"codec": "h264_nvenc",
		},
	}
	st.CreateJob(gpuJob)

	initialMetrics := sched.GetMetrics()
	initialAttempts := initialMetrics.AssignmentAttempts
	initialFailures := initialMetrics.AssignmentFailures

	// Run scheduling cycle
	sched.runSchedulingCycle()

	finalMetrics := sched.GetMetrics()

	// Rejection should NOT count as assignment failure
	if finalMetrics.AssignmentFailures != initialFailures {
		t.Errorf("Rejection should not increment AssignmentFailures, initial: %d, final: %d",
			initialFailures, finalMetrics.AssignmentFailures)
	}

	// Rejection should NOT count as assignment attempt
	if finalMetrics.AssignmentAttempts != initialAttempts {
		t.Errorf("Rejection should not increment AssignmentAttempts, initial: %d, final: %d",
			initialAttempts, finalMetrics.AssignmentAttempts)
	}
}
