package prometheus

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/psantana5/ffmpeg-rtmp/pkg/bandwidth"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// MasterExporter exports Prometheus metrics for the master node
type MasterExporter struct {
	store             store.Store
	bandwidthMonitor  *bandwidth.BandwidthMonitor
	startTime         time.Time
	mu                sync.RWMutex
	scheduleAttempts  map[string]int64 // result -> count
	jobDurations      []float64
	jobWaitTimes      []float64
}

// NewMasterExporter creates a new Prometheus exporter for master
func NewMasterExporter(s store.Store, bw *bandwidth.BandwidthMonitor) *MasterExporter {
	return &MasterExporter{
		store:            s,
		bandwidthMonitor: bw,
		startTime:        time.Now(),
		scheduleAttempts: make(map[string]int64),
		jobDurations:     make([]float64, 0),
		jobWaitTimes:     make([]float64, 0),
	}
}

// ServeHTTP serves Prometheus-compatible metrics at /metrics
func (e *MasterExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// First, write our custom metrics
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
		
		// Count active jobs (processing or assigned)
		if job.Status == models.JobStatusProcessing || job.Status == models.JobStatusAssigned {
			activeJobs++
		}
		
		// Count queued jobs (includes both pending and queued states)
		if job.Status == models.JobStatusQueued || job.Status == models.JobStatusPending {
			queueLength++
		}

		// Calculate durations for completed jobs
		if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed {
			if job.CompletedAt != nil && !job.CreatedAt.IsZero() {
				duration := job.CompletedAt.Sub(job.CreatedAt).Seconds()
				totalDuration += duration
				jobCount++
			}
			
			// Track completed jobs by actual engine used
			// If engine is "auto", treat it as ffmpeg (the default implementation)
			if job.Status == models.JobStatusCompleted && job.Engine != "" {
				engine := job.Engine
				if engine == "auto" {
					engine = "ffmpeg" // Auto defaults to ffmpeg
				}
				completedByEngine[engine]++
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
		// Count jobs in queue (both pending and queued states)
		if job.Status == models.JobStatusQueued || job.Status == models.JobStatusPending {
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
	
	// Bandwidth metrics (simple aggregated totals for backward compatibility)
	if e.bandwidthMonitor != nil {
		stats := e.bandwidthMonitor.GetStats()
		
		fmt.Fprintf(w, "\n# HELP scheduler_http_bandwidth_bytes_total Total bandwidth by direction\n")
		fmt.Fprintf(w, "# TYPE scheduler_http_bandwidth_bytes_total counter\n")
		fmt.Fprintf(w, "scheduler_http_bandwidth_bytes_total{direction=\"inbound\"} %d\n", stats.TotalBytesReceived)
		fmt.Fprintf(w, "scheduler_http_bandwidth_bytes_total{direction=\"outbound\"} %d\n", stats.TotalBytesSent)
		
		fmt.Fprintf(w, "\n# HELP scheduler_http_requests_total Total HTTP requests processed\n")
		fmt.Fprintf(w, "# TYPE scheduler_http_requests_total counter\n")
		fmt.Fprintf(w, "scheduler_http_requests_total %d\n", stats.TotalRequests)
		
		if stats.TotalRequests > 0 {
			avgReqSize := float64(stats.TotalBytesReceived) / float64(stats.TotalRequests)
			avgRespSize := float64(stats.TotalBytesSent) / float64(stats.TotalRequests)
			
			fmt.Fprintf(w, "\n# HELP scheduler_http_request_size_bytes_avg Average request size in bytes\n")
			fmt.Fprintf(w, "# TYPE scheduler_http_request_size_bytes_avg gauge\n")
			fmt.Fprintf(w, "scheduler_http_request_size_bytes_avg %.2f\n", avgReqSize)
			
			fmt.Fprintf(w, "\n# HELP scheduler_http_response_size_bytes_avg Average response size in bytes\n")
			fmt.Fprintf(w, "# TYPE scheduler_http_response_size_bytes_avg gauge\n")
			fmt.Fprintf(w, "scheduler_http_response_size_bytes_avg %.2f\n", avgRespSize)
		}
	}
	
	// Now append the Prometheus-registered metrics (from bandwidth monitor)
	// This includes the detailed metrics with labels and histograms
	fmt.Fprintf(w, "\n")
	
	// Gather metrics from Prometheus default registry
	metricFamilies, err := promclient.DefaultGatherer.Gather()
	if err != nil {
		// Log error but don't fail the request
		fmt.Fprintf(w, "# Error gathering Prometheus metrics: %v\n", err)
		return
	}
	
	// Write Prometheus metrics using text encoder
	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.FmtText)
	for _, mf := range metricFamilies {
		// Skip metrics we've already written manually (to avoid duplicates)
		if mf.GetName() == "scheduler_http_bandwidth_bytes_total" || 
		   mf.GetName() == "scheduler_http_requests_total" ||
		   mf.GetName() == "scheduler_http_request_size_bytes_avg" ||
		   mf.GetName() == "scheduler_http_response_size_bytes_avg" {
			continue
		}
		
		// Write metric family
		if err := encoder.Encode(mf); err != nil {
			// Log error but continue
			fmt.Fprintf(w, "# Error encoding metric %s: %v\n", mf.GetName(), err)
		}
	}
	
	// Write the buffer to response
	w.Write(buf.Bytes())
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
