package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// Collector collects metrics from the master node
type Collector struct {
	store      store.Store
	startTime  time.Time
	mu         sync.RWMutex
	jobMetrics map[string]*JobMetrics
}

// JobMetrics tracks metrics for jobs
type JobMetrics struct {
	Created   int64
	Completed int64
	Failed    int64
	Running   int64
	Retried   int64
}

// NewCollector creates a new metrics collector
func NewCollector(s store.Store) *Collector {
	return &Collector{
		store:      s,
		startTime:  time.Now(),
		jobMetrics: make(map[string]*JobMetrics),
	}
}

// ServeHTTP serves Prometheus-compatible metrics
func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Collect current metrics
	nodes := c.store.GetAllNodes()
	jobs := c.store.GetAllJobs()

	// Count nodes by status
	nodesByStatus := make(map[string]int)
	for _, node := range nodes {
		nodesByStatus[node.Status]++
	}

	// Count jobs by status
	jobsByStatus := make(map[string]int)
	totalRetries := 0
	for _, job := range jobs {
		jobsByStatus[string(job.Status)]++
		if job.RetryCount > 0 {
			totalRetries += job.RetryCount
		}
	}

	// Write metrics in Prometheus format
	fmt.Fprintf(w, "# HELP ffmpeg_master_uptime_seconds Time since master started\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_uptime_seconds gauge\n")
	fmt.Fprintf(w, "ffmpeg_master_uptime_seconds %d\n", int64(time.Since(c.startTime).Seconds()))

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_nodes_total Total number of registered nodes\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_nodes_total gauge\n")
	fmt.Fprintf(w, "ffmpeg_master_nodes_total %d\n", len(nodes))

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_nodes_by_status Number of nodes by status\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_nodes_by_status gauge\n")
	for status, count := range nodesByStatus {
		fmt.Fprintf(w, "ffmpeg_master_nodes_by_status{status=\"%s\"} %d\n", status, count)
	}

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_jobs_total Total number of jobs\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_jobs_total gauge\n")
	fmt.Fprintf(w, "ffmpeg_master_jobs_total %d\n", len(jobs))

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_jobs_by_status Number of jobs by status\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_jobs_by_status gauge\n")
	for status, count := range jobsByStatus {
		fmt.Fprintf(w, "ffmpeg_master_jobs_by_status{status=\"%s\"} %d\n", status, count)
	}

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_job_retries_total Total number of job retries\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_job_retries_total counter\n")
	fmt.Fprintf(w, "ffmpeg_master_job_retries_total %d\n", totalRetries)

	// Node capacity metrics
	totalCPUThreads := 0
	totalRAMBytes := uint64(0)
	nodesWithGPU := 0
	for _, node := range nodes {
		totalCPUThreads += node.CPUThreads
		totalRAMBytes += node.RAMTotalBytes
		if node.HasGPU {
			nodesWithGPU++
		}
	}

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_cluster_cpu_threads Total CPU threads in cluster\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_cluster_cpu_threads gauge\n")
	fmt.Fprintf(w, "ffmpeg_master_cluster_cpu_threads %d\n", totalCPUThreads)

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_cluster_ram_bytes Total RAM bytes in cluster\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_cluster_ram_bytes gauge\n")
	fmt.Fprintf(w, "ffmpeg_master_cluster_ram_bytes %d\n", totalRAMBytes)

	fmt.Fprintf(w, "\n# HELP ffmpeg_master_cluster_gpu_nodes Number of nodes with GPU\n")
	fmt.Fprintf(w, "# TYPE ffmpeg_master_cluster_gpu_nodes gauge\n")
	fmt.Fprintf(w, "ffmpeg_master_cluster_gpu_nodes %d\n", nodesWithGPU)
}

// RecordJobCreated increments job creation counter
func (c *Collector) RecordJobCreated() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Metrics are read directly from store
}

// RecordJobRetry increments job retry counter
func (c *Collector) RecordJobRetry(jobID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Metrics are read directly from store
}
