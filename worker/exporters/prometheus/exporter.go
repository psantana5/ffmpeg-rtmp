package prometheus

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// WorkerExporter exports Prometheus metrics for worker nodes
type WorkerExporter struct {
	mu               sync.RWMutex
	nodeID           string
	startTime        time.Time
	activeJobs       int
	heartbeatCount   int64
	
	// Hardware metrics
	cpuUsage         float64
	gpuUsage         float64
	memoryBytes      uint64
	powerWatts       float64
	tempCelsius      float64
	
	// GPU capabilities
	hasGPU           bool
	gpuModel         string
	
	// Input generation metrics
	inputGenerationDurationSeconds float64
	inputFileSizeBytes             int64
	totalInputsGenerated           int64
	
	// Encoder availability metrics (runtime-validated)
	nvencAvailable  bool
	qsvAvailable    bool
	vaapiAvailable  bool
	
	// Bandwidth metrics
	totalInputBytesProcessed   int64
	totalOutputBytesGenerated  int64
	lastJobInputBytes          int64
	lastJobOutputBytes         int64
	lastJobBandwidthMbps       float64
	workerBandwidthUtilization float64
	
	// SLA tracking metrics
	jobsCompletedTotal         int64
	jobsFailedTotal            int64
	jobsSLACompliant           int64   // Jobs completed within SLA targets
	jobsSLAViolation           int64   // Jobs that violated SLA
	currentSLAComplianceRate   float64 // Percentage (0-100)
	
	// Cancellation metrics
	jobsCanceledTotal          int64
	jobsCanceledGracefulTotal  int64 // Terminated with SIGTERM
	jobsCanceledForcefulTotal  int64 // Terminated with SIGKILL
}

// NewWorkerExporter creates a new Prometheus exporter for worker
func NewWorkerExporter(nodeID string, hasGPU bool) *WorkerExporter {
	return &WorkerExporter{
		nodeID:                 nodeID,
		startTime:              time.Now(),
		hasGPU:                 hasGPU,
		currentSLAComplianceRate: 100.0, // Start at 100% compliance
	}
}

// ServeHTTP serves Prometheus-compatible metrics at /metrics
func (e *WorkerExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Update metrics before serving
	e.updateMetrics()

	e.mu.RLock()
	defer e.mu.RUnlock()

	// ffrtmp_worker_cpu_usage
	fmt.Fprintf(w, "# HELP ffrtmp_worker_cpu_usage Worker CPU usage percentage (0-100)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_cpu_usage gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_cpu_usage{node_id=\"%s\"} %.2f\n", e.nodeID, e.cpuUsage)

	// ffrtmp_worker_gpu_usage
	if e.hasGPU {
		fmt.Fprintf(w, "\n# HELP ffrtmp_worker_gpu_usage Worker GPU usage percentage (0-100)\n")
		fmt.Fprintf(w, "# TYPE ffrtmp_worker_gpu_usage gauge\n")
		fmt.Fprintf(w, "ffrtmp_worker_gpu_usage{node_id=\"%s\",gpu_model=\"%s\"} %.2f\n", 
			e.nodeID, e.gpuModel, e.gpuUsage)

		// ffrtmp_worker_power_watts
		fmt.Fprintf(w, "\n# HELP ffrtmp_worker_power_watts Worker power consumption in watts\n")
		fmt.Fprintf(w, "# TYPE ffrtmp_worker_power_watts gauge\n")
		fmt.Fprintf(w, "ffrtmp_worker_power_watts{node_id=\"%s\"} %.2f\n", e.nodeID, e.powerWatts)

		// ffrtmp_worker_temperature_celsius
		fmt.Fprintf(w, "\n# HELP ffrtmp_worker_temperature_celsius Worker GPU temperature in Celsius\n")
		fmt.Fprintf(w, "# TYPE ffrtmp_worker_temperature_celsius gauge\n")
		fmt.Fprintf(w, "ffrtmp_worker_temperature_celsius{node_id=\"%s\"} %.2f\n", e.nodeID, e.tempCelsius)
	}

	// ffrtmp_worker_memory_bytes
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_memory_bytes Worker memory usage in bytes\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_memory_bytes gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_memory_bytes{node_id=\"%s\"} %d\n", e.nodeID, e.memoryBytes)

	// ffrtmp_worker_active_jobs
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_active_jobs Number of active jobs on this worker\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_active_jobs gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_active_jobs{node_id=\"%s\"} %d\n", e.nodeID, e.activeJobs)

	// ffrtmp_worker_heartbeats_total
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_heartbeats_total Total heartbeats sent by worker\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_heartbeats_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_heartbeats_total{node_id=\"%s\"} %d\n", e.nodeID, e.heartbeatCount)

	// Additional info
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_uptime_seconds Worker uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_uptime_seconds gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_uptime_seconds{node_id=\"%s\"} %.0f\n", e.nodeID, time.Since(e.startTime).Seconds())

	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_has_gpu Whether worker has GPU (1=yes, 0=no)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_has_gpu gauge\n")
	hasGPUValue := 0
	if e.hasGPU {
		hasGPUValue = 1
	}
	fmt.Fprintf(w, "ffrtmp_worker_has_gpu{node_id=\"%s\"} %d\n", e.nodeID, hasGPUValue)

	// Input generation metrics
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_input_generation_duration_seconds Duration of last input video generation\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_input_generation_duration_seconds gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_input_generation_duration_seconds{node_id=\"%s\"} %.2f\n", e.nodeID, e.inputGenerationDurationSeconds)

	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_input_file_size_bytes Size of last generated input file\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_input_file_size_bytes gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_input_file_size_bytes{node_id=\"%s\"} %d\n", e.nodeID, e.inputFileSizeBytes)

	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_total_inputs_generated Total number of input videos generated\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_total_inputs_generated counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_total_inputs_generated{node_id=\"%s\"} %d\n", e.nodeID, e.totalInputsGenerated)
	
	// Encoder availability metrics
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_nvenc_available NVENC encoder runtime availability (1=available, 0=unavailable)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_nvenc_available gauge\n")
	nvencValue := 0
	if e.nvencAvailable {
		nvencValue = 1
	}
	fmt.Fprintf(w, "ffrtmp_worker_nvenc_available{node_id=\"%s\"} %d\n", e.nodeID, nvencValue)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_qsv_available Intel QSV encoder runtime availability (1=available, 0=unavailable)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_qsv_available gauge\n")
	qsvValue := 0
	if e.qsvAvailable {
		qsvValue = 1
	}
	fmt.Fprintf(w, "ffrtmp_worker_qsv_available{node_id=\"%s\"} %d\n", e.nodeID, qsvValue)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_vaapi_available VAAPI encoder runtime availability (1=available, 0=unavailable)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_vaapi_available gauge\n")
	vaapiValue := 0
	if e.vaapiAvailable {
		vaapiValue = 1
	}
	fmt.Fprintf(w, "ffrtmp_worker_vaapi_available{node_id=\"%s\"} %d\n", e.nodeID, vaapiValue)
	
	// Bandwidth metrics
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_input_bytes_total Total bytes read from input files across all jobs\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_input_bytes_total counter\n")
	fmt.Fprintf(w, "ffrtmp_job_input_bytes_total{node_id=\"%s\"} %d\n", e.nodeID, e.totalInputBytesProcessed)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_output_bytes_total Total bytes written to output files across all jobs\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_output_bytes_total counter\n")
	fmt.Fprintf(w, "ffrtmp_job_output_bytes_total{node_id=\"%s\"} %d\n", e.nodeID, e.totalOutputBytesGenerated)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_last_input_bytes Size of input file from last completed job\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_last_input_bytes gauge\n")
	fmt.Fprintf(w, "ffrtmp_job_last_input_bytes{node_id=\"%s\"} %d\n", e.nodeID, e.lastJobInputBytes)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_last_output_bytes Size of output file from last completed job\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_last_output_bytes gauge\n")
	fmt.Fprintf(w, "ffrtmp_job_last_output_bytes{node_id=\"%s\"} %d\n", e.nodeID, e.lastJobOutputBytes)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_job_last_bandwidth_mbps Bandwidth utilization for last completed job in Mbps\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_job_last_bandwidth_mbps gauge\n")
	fmt.Fprintf(w, "ffrtmp_job_last_bandwidth_mbps{node_id=\"%s\"} %.2f\n", e.nodeID, e.lastJobBandwidthMbps)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_bandwidth_utilization Worker overall bandwidth utilization as percentage (0-100)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_bandwidth_utilization gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_bandwidth_utilization{node_id=\"%s\"} %.2f\n", e.nodeID, e.workerBandwidthUtilization)
	
	// SLA tracking metrics
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_completed_total Total number of jobs completed successfully\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_completed_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_completed_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsCompletedTotal)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_failed_total Total number of jobs that failed\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_failed_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_failed_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsFailedTotal)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_sla_compliant_total Total number of jobs completed within SLA targets\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_sla_compliant_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_sla_compliant_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsSLACompliant)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_sla_violation_total Total number of jobs that violated SLA targets\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_sla_violation_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_sla_violation_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsSLAViolation)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_sla_compliance_rate Current SLA compliance rate as percentage (0-100)\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_sla_compliance_rate gauge\n")
	fmt.Fprintf(w, "ffrtmp_worker_sla_compliance_rate{node_id=\"%s\"} %.2f\n", e.nodeID, e.currentSLAComplianceRate)
	
	// Cancellation metrics
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_canceled_total Total number of jobs canceled\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_canceled_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_canceled_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsCanceledTotal)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_canceled_graceful_total Jobs terminated gracefully with SIGTERM\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_canceled_graceful_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_canceled_graceful_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsCanceledGracefulTotal)
	
	fmt.Fprintf(w, "\n# HELP ffrtmp_worker_jobs_canceled_forceful_total Jobs terminated forcefully with SIGKILL\n")
	fmt.Fprintf(w, "# TYPE ffrtmp_worker_jobs_canceled_forceful_total counter\n")
	fmt.Fprintf(w, "ffrtmp_worker_jobs_canceled_forceful_total{node_id=\"%s\"} %d\n", e.nodeID, e.jobsCanceledForcefulTotal)
}

// updateMetrics updates hardware metrics
func (e *WorkerExporter) updateMetrics() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// CPU usage
	if cpuPercent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
		e.cpuUsage = cpuPercent[0]
	}

	// Memory usage
	if memInfo, err := mem.VirtualMemory(); err == nil {
		e.memoryBytes = memInfo.Used
	}

	// GPU metrics (if available)
	if e.hasGPU {
		e.updateGPUMetrics()
	}
}

// updateGPUMetrics updates GPU-specific metrics using nvidia-smi
func (e *WorkerExporter) updateGPUMetrics() {
	// Try to get GPU metrics using nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,power.draw,temperature.gpu,name", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// Parse output: "utilization, power, temperature, name"
	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) >= 4 {
		if util, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
			e.gpuUsage = util
		}
		if power, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
			e.powerWatts = power
		}
		if temp, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
			e.tempCelsius = temp
		}
		e.gpuModel = strings.TrimSpace(parts[3])
	}
}

// SetActiveJobs sets the number of active jobs
func (e *WorkerExporter) SetActiveJobs(count int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.activeJobs = count
}

// IncrementHeartbeat increments the heartbeat counter
func (e *WorkerExporter) IncrementHeartbeat() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.heartbeatCount++
}

// GetCPUUsage returns current CPU usage
func (e *WorkerExporter) GetCPUUsage() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cpuUsage
}

// GetMemoryUsed returns current memory usage in bytes
func (e *WorkerExporter) GetMemoryUsed() uint64 {
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return vmem.Used
}

// GetMemoryFree returns available memory in bytes
func (e *WorkerExporter) GetMemoryFree() uint64 {
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return vmem.Available
}

// GetCPUCores returns the number of CPU cores
func (e *WorkerExporter) GetCPUCores() int {
	return runtime.NumCPU()
}

// RecordInputGeneration records metrics for input video generation
func (e *WorkerExporter) RecordInputGeneration(durationSeconds float64, fileSizeBytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inputGenerationDurationSeconds = durationSeconds
	e.inputFileSizeBytes = fileSizeBytes
	e.totalInputsGenerated++
}

// SetEncoderAvailability sets the runtime-validated encoder availability
func (e *WorkerExporter) SetEncoderAvailability(nvenc, qsv, vaapi bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nvencAvailable = nvenc
	e.qsvAvailable = qsv
	e.vaapiAvailable = vaapi
}

// RecordJobBandwidth records bandwidth metrics for a completed job
// inputBytes: size of input file in bytes
// outputBytes: size of output file in bytes
// durationSeconds: job execution duration
func (e *WorkerExporter) RecordJobBandwidth(inputBytes, outputBytes int64, durationSeconds float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Update totals
	e.totalInputBytesProcessed += inputBytes
	e.totalOutputBytesGenerated += outputBytes
	
	// Update last job metrics
	e.lastJobInputBytes = inputBytes
	e.lastJobOutputBytes = outputBytes
	
	// Calculate bandwidth in Mbps (total bytes processed / duration)
	if durationSeconds > 0 {
		totalBytes := inputBytes + outputBytes
		bytesPerSecond := float64(totalBytes) / durationSeconds
		e.lastJobBandwidthMbps = (bytesPerSecond * 8) / (1024 * 1024) // Convert to Mbps
	} else {
		e.lastJobBandwidthMbps = 0
	}
	
	// Update worker bandwidth utilization (simplified metric)
	// This is a percentage based on recent activity
	// For now, we'll use the last job bandwidth as an indicator
	// In production, you might want to use a moving average
	e.workerBandwidthUtilization = e.lastJobBandwidthMbps / 10.0 // Normalize to 0-100 scale (assuming 100Mbps baseline)
	if e.workerBandwidthUtilization > 100 {
		e.workerBandwidthUtilization = 100
	}
}

// GetTotalBandwidthBytes returns total input + output bytes processed
func (e *WorkerExporter) GetTotalBandwidthBytes() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalInputBytesProcessed + e.totalOutputBytesGenerated
}

// SLATarget defines SLA targets for job execution
type SLATarget struct {
	MaxDurationSeconds float64 // Maximum duration for job to be SLA-compliant
	MaxFailureRate     float64 // Maximum failure rate (0-1) for SLA compliance
}

// GetDefaultSLATarget returns default SLA targets
// These can be overridden per-scenario or configured globally
func GetDefaultSLATarget() SLATarget {
	return SLATarget{
		MaxDurationSeconds: 600,  // 10 minutes default
		MaxFailureRate:     0.05, // 5% failure rate
	}
}

// RecordJobCompletion records a completed job and tracks SLA compliance
func (e *WorkerExporter) RecordJobCompletion(durationSeconds float64, failed bool, slaTarget SLATarget) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if failed {
		e.jobsFailedTotal++
	} else {
		e.jobsCompletedTotal++
		
		// Check SLA compliance (only for successful jobs)
		if durationSeconds <= slaTarget.MaxDurationSeconds {
			e.jobsSLACompliant++
		} else {
			e.jobsSLAViolation++
		}
	}
	
	// Calculate current SLA compliance rate
	totalSLAEligibleJobs := e.jobsSLACompliant + e.jobsSLAViolation
	if totalSLAEligibleJobs > 0 {
		e.currentSLAComplianceRate = (float64(e.jobsSLACompliant) / float64(totalSLAEligibleJobs)) * 100
	} else {
		e.currentSLAComplianceRate = 100.0 // No jobs yet = 100% compliance
	}
}

// GetSLAComplianceRate returns current SLA compliance rate
func (e *WorkerExporter) GetSLAComplianceRate() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentSLAComplianceRate
}

// GetJobCompletionStats returns job completion statistics
func (e *WorkerExporter) GetJobCompletionStats() (completed, failed, slaCompliant, slaViolation int64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.jobsCompletedTotal, e.jobsFailedTotal, e.jobsSLACompliant, e.jobsSLAViolation
}

// RecordJobCancellation records a canceled job
// graceful: true if terminated with SIGTERM, false if SIGKILL was needed
func (e *WorkerExporter) RecordJobCancellation(graceful bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.jobsCanceledTotal++
	if graceful {
		e.jobsCanceledGracefulTotal++
	} else {
		e.jobsCanceledForcefulTotal++
	}
}

// GetCancellationStats returns cancellation statistics
func (e *WorkerExporter) GetCancellationStats() (total, graceful, forceful int64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.jobsCanceledTotal, e.jobsCanceledGracefulTotal, e.jobsCanceledForcefulTotal
}
