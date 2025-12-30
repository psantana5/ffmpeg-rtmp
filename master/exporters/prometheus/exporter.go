package prometheus

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// MasterExporter exports Prometheus metrics for the master node
type MasterExporter struct {
	store             store.Store
	startTime         time.Time
	mu                sync.RWMutex
	scheduleAttempts  map[string]int64 // result -> count
	jobDurations      []float64
	jobWaitTimes      []float64
}

// NewMasterExporter creates a new Prometheus exporter for master
func NewMasterExporter(s store.Store) *MasterExporter {
	return &MasterExporter{
		store:            s,
		startTime:        time.Now(),
		scheduleAttempts: make(map[string]int64),
		jobDurations:     make([]float64, 0),
		jobWaitTimes:     make([]float64, 0),
	}
}

// ServeHTTP serves Prometheus-compatible metrics at /metrics
func (e *MasterExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Collect current state
	nodes := e.store.GetAllNodes()
	jobs := e.store.GetAllJobs()

	// Count jobs by state
	jobsByState := make(map[models.JobStatus]int)
	activeJobs := 0
	queueLength := 0
	var totalDuration float64
	jobCount := 0
	
	// Count jobs by engine (new)
	jobsByEngine := map[string]int{
		"ffmpeg":    0,
		"gstreamer": 0,
		"auto":      0,
	}
	completedByEngine := map[string]int{
		"ffmpeg":    0,
		"gstreamer": 0,
	}

	for _, job := range jobs {
		jobsByState[job.Status]++
		
		// Track engine distribution
		if job.Engine != "" {
			jobsByEngine[job.Engine]++
		}
		
		if job.Status == models.JobStatusProcessing || job.Status == models.JobStatusAssigned {
			activeJobs++
		}
		
		if job.Status == models.JobStatusQueued {
			queueLength++
		}

		// Calculate durations for completed jobs
		if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed {
			if job.CompletedAt != nil && !job.CreatedAt.IsZero() {
				duration := job.CompletedAt.Sub(job.CreatedAt).Seconds()
				totalDuration += duration
				jobCount++
			}
			
			// Track completed jobs by actual engine used (from job results)
			// Note: This is the engine that was actually selected, not the preference
			if job.Status == models.JobStatusCompleted && job.Engine != "" && job.Engine != "auto" {
				completedByEngine[job.Engine]++
			}
		}
	}

	// Calculate average duration
	avgDuration := 0.0
	if jobCount > 0 {
		avgDuration = totalDuration / float64(jobCount)
	}

	// ffrtmp_jobs_total{state}
	fmt.Fprintf(w, "# HELP ffrtmp_jobs_total Total number of jobs by state\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_jobs_total counter\n")
	for state, count := range jobsByState {
		fmt.Fprintf(w, "ffrtmp_jobs_total{state=\"%s\"} %d\n", state, count)
	}

	// ffrtmp_active_jobs
	fmt.Fprintf(w, "\n# HELP ffrtmp_active_jobs Number of currently active jobs\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_active_jobs gauge\n")
	fmt.Fprintf(w, "ffrtmp_active_jobs %d\n", activeJobs)

	// ffrtmp_queue_length
	fmt.Fprintf(w, "\n# HELP ffrtmp_queue_length Number of jobs in queue\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_queue_length gauge\n")
	fmt.Fprintf(w, "ffrtmp_queue_length %d\n", queueLength)

	// ffrtmp_job_duration_seconds
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_duration_seconds Average job duration in seconds\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_duration_seconds gauge\n")
	fmt.Fprintf(w, "ffrtmp_job_duration_seconds %.2f\n", avgDuration)

	// ffrtmp_job_wait_time_seconds (placeholder - would need scheduling timestamps)
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_wait_time_seconds Average job wait time in queue\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_wait_time_seconds gauge\n")
	fmt.Fprintf(w, "ffrtmp_job_wait_time_seconds %.2f\n", 0.0)

	// ffrtmp_schedule_attempts_total{result}
	e.mu.RLock()
	fmt.Fprintf(w, "\n# HELP ffrtmp_schedule_attempts_total Total scheduling attempts by result\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_schedule_attempts_total counter\n")
	for result, count := range e.scheduleAttempts {
		fmt.Fprintf(w, "ffrtmp_schedule_attempts_total{result=\"%s\"} %d\n", result, count)
	}
	e.mu.RUnlock()

	// Additional useful metrics
	fmt.Fprintf(w, "\n# HELP ffrtmp_master_uptime_seconds Master uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_master_uptime_seconds gauge\n")
	fmt.Fprintf(w, "ffrtmp_master_uptime_seconds %.0f\n", time.Since(e.startTime).Seconds())

	fmt.Fprintf(w, "\n# HELP ffrtmp_nodes_total Total number of registered worker nodes\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_nodes_total gauge\n")
	fmt.Fprintf(w, "ffrtmp_nodes_total %d\n", len(nodes))

	// Node status breakdown
	// Initialize all possible statuses to ensure metrics always exist
	nodesByStatus := map[string]int{
		"available": 0,
		"busy":      0,
		"offline":   0,
	}
	for _, node := range nodes {
		nodesByStatus[node.Status]++
	}
	fmt.Fprintf(w, "\n# HELP ffrtmp_nodes_by_status Worker nodes by status\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_nodes_by_status gauge\n")
	// Always export all statuses (even if count is 0)
	for _, status := range []string{"available", "busy", "offline"} {
		fmt.Fprintf(w, "ffrtmp_nodes_by_status{status=\"%s\"} %d\n", status, nodesByStatus[status])
	}

	// Queue breakdown by priority and type
	// Initialize all possible values to ensure metrics always exist
	queueByPriority := map[string]int{
		"high":   0,
		"medium": 0,
		"low":    0,
	}
	queueByType := map[string]int{
		"live":    0,
		"default": 0,
		"batch":   0,
	}
	for _, job := range jobs {
		if job.Status == models.JobStatusQueued {
			queueByPriority[job.Priority]++
			queueByType[job.Queue]++
		}
	}
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_queue_by_priority Jobs in queue by priority\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_queue_by_priority gauge\n")
	// Always export all priorities (even if count is 0)
	for _, priority := range []string{"high", "medium", "low"} {
		fmt.Fprintf(w, "ffrtmp_queue_by_priority{priority=\"%s\"} %d\n", priority, queueByPriority[priority])
	}
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_queue_by_type Jobs in queue by type\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_queue_by_type gauge\n")
	// Always export all queue types (even if count is 0)
	for _, queueType := range []string{"live", "default", "batch"} {
		fmt.Fprintf(w, "ffrtmp_queue_by_type{type=\"%s\"} %d\n", queueType, queueByType[queueType])
	}
	
	// Engine metrics (new)
	fmt.Fprintf(w, "\n# HELP ffrtmp_jobs_by_engine Total jobs by transcoding engine preference\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_jobs_by_engine counter\n")
	for _, engine := range []string{"ffmpeg", "gstreamer", "auto"} {
		fmt.Fprintf(w, "ffrtmp_jobs_by_engine{engine=\"%s\"} %d\n", engine, jobsByEngine[engine])
	}
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_jobs_completed_by_engine Completed jobs by actual engine used\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_jobs_completed_by_engine counter\n")
	for _, engine := range []string{"ffmpeg", "gstreamer"} {
		fmt.Fprintf(w, "ffrtmp_jobs_completed_by_engine{engine=\"%s\"} %d\n", engine, completedByEngine[engine])
	}
}

// RecordScheduleAttempt records a scheduling attempt
func (e *MasterExporter) RecordScheduleAttempt(result string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scheduleAttempts[result]++
}

// RecordJobDuration records a job duration
func (e *MasterExporter) RecordJobDuration(duration float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.jobDurations = append(e.jobDurations, duration)
	// Keep only last 1000 entries
	if len(e.jobDurations) > 1000 {
		e.jobDurations = e.jobDurations[1:]
	}
}

// RecordJobWaitTime records a job wait time
func (e *MasterExporter) RecordJobWaitTime(waitTime float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.jobWaitTimes = append(e.jobWaitTimes, waitTime)
	// Keep only last 1000 entries
	if len(e.jobWaitTimes) > 1000 {
		e.jobWaitTimes = e.jobWaitTimes[1:]
	}
}
