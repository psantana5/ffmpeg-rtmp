package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// MetricsRecorder is an interface for recording metrics
type MetricsRecorder interface {
	RecordScheduleAttempt(result string)
}

// MasterHandler handles master node API requests
type MasterHandler struct {
	store           store.Store
	maxRetries      int
	metricsRecorder MetricsRecorder
	resultsWriter   *ResultsWriter
}

// NewMasterHandler creates a new master handler
func NewMasterHandler(s store.Store) *MasterHandler {
	return &MasterHandler{
		store:         s,
		maxRetries:    0, // No retries by default
		resultsWriter: NewResultsWriter("./test_results"),
	}
}

// NewMasterHandlerWithRetry creates a new master handler with retry support
func NewMasterHandlerWithRetry(s store.Store, maxRetries int) *MasterHandler {
	return &MasterHandler{
		store:         s,
		maxRetries:    maxRetries,
		resultsWriter: NewResultsWriter("./test_results"),
	}
}

// SetMetricsRecorder sets the metrics recorder for the handler
func (h *MasterHandler) SetMetricsRecorder(recorder MetricsRecorder) {
	h.metricsRecorder = recorder
}

// getJobByIDOrSequence retrieves a job by ID (UUID) or sequence number
func (h *MasterHandler) getJobByIDOrSequence(idOrSeq string) (*models.Job, error) {
	// Try to parse as sequence number first
	var seqNum int
	if _, parseErr := fmt.Sscanf(idOrSeq, "%d", &seqNum); parseErr == nil && seqNum > 0 {
		// It's a number, try sequence number lookup
		return h.store.GetJobBySequenceNumber(seqNum)
	}
	// Try UUID lookup
	return h.store.GetJob(idOrSeq)
}

// RegisterRoutes registers all API routes
func (h *MasterHandler) RegisterRoutes(r *mux.Router) {
	// Node routes
	r.HandleFunc("/nodes/register", h.RegisterNode).Methods("POST")
	r.HandleFunc("/nodes/{id}", h.GetNodeDetails).Methods("GET")
	r.HandleFunc("/nodes/{id}", h.RemoveNode).Methods("DELETE")
	r.HandleFunc("/nodes", h.ListNodes).Methods("GET")
	r.HandleFunc("/nodes/{id}/heartbeat", h.NodeHeartbeat).Methods("POST")
	
	// Job routes (register specific routes before parameterized routes)
	r.HandleFunc("/jobs/next", h.GetNextJob).Methods("GET")
	r.HandleFunc("/jobs", h.CreateJob).Methods("POST")
	r.HandleFunc("/jobs", h.ListJobs).Methods("GET")
	r.HandleFunc("/jobs/{id}", h.GetJob).Methods("GET")
	r.HandleFunc("/jobs/{id}/pause", h.PauseJob).Methods("POST")
	r.HandleFunc("/jobs/{id}/resume", h.ResumeJob).Methods("POST")
	r.HandleFunc("/jobs/{id}/cancel", h.CancelJob).Methods("POST")
	r.HandleFunc("/jobs/{id}/retry", h.RetryJob).Methods("POST")
	r.HandleFunc("/jobs/{id}/logs", h.GetJobLogs).Methods("GET")
	
	// Tenant routes (multi-tenancy)
	r.HandleFunc("/tenants", h.CreateTenant).Methods("POST")
	r.HandleFunc("/tenants", h.ListTenants).Methods("GET")
	r.HandleFunc("/tenants/{id}", h.GetTenant).Methods("GET")
	r.HandleFunc("/tenants/{id}", h.UpdateTenant).Methods("PUT")
	r.HandleFunc("/tenants/{id}", h.DeleteTenant).Methods("DELETE")
	r.HandleFunc("/tenants/{id}/stats", h.GetTenantStats).Methods("GET")
	r.HandleFunc("/tenants/{id}/jobs", h.GetTenantJobs).Methods("GET")
	r.HandleFunc("/tenants/{id}/nodes", h.GetTenantNodes).Methods("GET")
	
	// Other routes
	r.HandleFunc("/results", h.ReceiveResults).Methods("POST")
	r.HandleFunc("/health", h.Health).Methods("GET")
}

// RegisterNode handles node registration
func (h *MasterHandler) RegisterNode(w http.ResponseWriter, r *http.Request) {
	var reg models.NodeRegistration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if a node with this address already exists
	existingNode, err := h.store.GetNodeByAddress(reg.Address)
	if err == nil && existingNode != nil {
		// Node with this address already exists - handle re-registration
		// This handles cases where:
		// 1. Agent restarted and tries to re-register
		// 2. Network issues caused registration to fail but succeed server-side
		// 3. Agent crashed without proper deregistration
		
		log.Printf("Node with address %s already exists (ID: %s), handling re-registration...", reg.Address, existingNode.ID)
		
		// Update existing node with new capabilities (hardware may have changed)
		existingNode.Type = reg.Type
		existingNode.CPUThreads = reg.CPUThreads
		existingNode.CPUModel = reg.CPUModel
		existingNode.HasGPU = reg.HasGPU
		existingNode.GPUType = reg.GPUType
		existingNode.GPUCapabilities = reg.GPUCapabilities
		existingNode.RAMTotalBytes = reg.RAMTotalBytes
		existingNode.Labels = reg.Labels
		existingNode.Status = "available" // Reset to available
		existingNode.LastHeartbeat = time.Now()
		existingNode.CurrentJobID = "" // Clear any stale job assignment
		
		// Update in database
		if err := h.store.UpdateNodeHeartbeat(existingNode.ID); err != nil {
			log.Printf("Warning: failed to update heartbeat during re-registration: %v", err)
		}
		
		// Update node status
		if err := h.store.UpdateNodeStatus(existingNode.ID, "available"); err != nil {
			log.Printf("Warning: failed to update status during re-registration: %v", err)
		}
		
		log.Printf("Node re-registered: %s [%s] (%s, %d threads, %s)", existingNode.Name, existingNode.ID, existingNode.Type, existingNode.CPUThreads, existingNode.CPUModel)
		
		// Return the existing node (with updated info)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // 200 OK for re-registration (not 201 Created)
		json.NewEncoder(w).Encode(existingNode)
		return
	}

	// Extract hostname from address for the name field
	name := reg.Address
	// Simple hostname extraction (e.g., "https://hostname:port" -> "hostname")
	if len(name) > 0 {
		// Remove protocol
		if idx := len("https://"); len(name) > idx && name[:idx] == "https://" {
			name = name[idx:]
		} else if idx := len("http://"); len(name) > idx && name[:idx] == "http://" {
			name = name[idx:]
		}
		// Remove port
		for i, ch := range name {
			if ch == ':' {
				name = name[:i]
				break
			}
		}
	}

	// Create new node
	node := &models.Node{
		ID:              uuid.New().String(),
		Name:            name,
		Address:         reg.Address,
		Type:            reg.Type,
		CPUThreads:      reg.CPUThreads,
		CPUModel:        reg.CPUModel,
		HasGPU:          reg.HasGPU,
		GPUType:         reg.GPUType,
		GPUCapabilities: reg.GPUCapabilities,
		RAMTotalBytes:   reg.RAMTotalBytes,
		Labels:          reg.Labels,
		Status:          "available",
		LastHeartbeat:   time.Now(),
		RegisteredAt:    time.Now(),
	}

	if err := h.store.RegisterNode(node); err != nil {
		log.Printf("Error registering node: %v", err)
		http.Error(w, "Failed to register node", http.StatusInternalServerError)
		return
	}

	log.Printf("Node registered: %s [%s] (%s, %d threads, %s)", node.Name, node.ID, node.Type, node.CPUThreads, node.CPUModel)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// ListNodes returns all registered nodes
func (h *MasterHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.store.GetAllNodes()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// NodeHeartbeat updates node heartbeat
func (h *MasterHandler) NodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["id"]

	if err := h.store.UpdateNodeHeartbeat(nodeID); err != nil {
		if err == store.ErrNodeNotFound {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to update heartbeat", http.StatusInternalServerError)
		return
	}

	// Update job activity if the node has a current job
	node, err := h.store.GetNode(nodeID)
	if err == nil && node.CurrentJobID != "" {
		// Update the job's last activity time
		if err := h.store.UpdateJobActivity(node.CurrentJobID); err != nil {
			// Log error but don't fail the heartbeat
			log.Printf("Warning: Failed to update job activity for job %s: %v", node.CurrentJobID, err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// CreateJob creates a new job
func (h *MasterHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req models.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create job
	job := &models.Job{
		ID:         uuid.New().String(),
		Scenario:   req.Scenario,
		Confidence: req.Confidence,
		Engine:     req.Engine,
		Parameters: req.Parameters,
		Queue:      req.Queue,
		Priority:   req.Priority,
		Status:     models.JobStatusPending,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}

	// Set defaults for queue, priority, and engine
	if job.Queue == "" {
		job.Queue = "default"
	}
	if job.Priority == "" {
		job.Priority = "medium"
	}
	if job.Engine == "" {
		job.Engine = "auto"
	}

	// Validate engine value
	validEngines := map[string]bool{
		"auto":       true,
		"ffmpeg":     true,
		"gstreamer":  true,
	}
	if !validEngines[job.Engine] {
		http.Error(w, fmt.Sprintf("Invalid engine '%s'. Valid values: auto, ffmpeg, gstreamer", job.Engine), http.StatusBadRequest)
		return
	}

	if err := h.store.CreateJob(job); err != nil {
		log.Printf("Error creating job: %v", err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	log.Printf("Job created: %s (%s)", job.ID, job.Scenario)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// ListJobs returns all jobs
func (h *MasterHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.store.GetAllJobs()

	// Populate NodeName for each job
	for _, job := range jobs {
		if job.NodeID != "" {
			if node, err := h.store.GetNode(job.NodeID); err == nil {
				job.NodeName = node.Name
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// GetJob retrieves a specific job by ID or sequence number
func (h *MasterHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, err := h.getJobByIDOrSequence(jobID)

	if err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting job: %v", err)
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	// Populate NodeName if NodeID is set
	if job.NodeID != "" {
		if node, err := h.store.GetNode(job.NodeID); err == nil {
			job.NodeName = node.Name
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// GetNextJob retrieves the next pending job for a node
func (h *MasterHandler) GetNextJob(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "node_id parameter is required", http.StatusBadRequest)
		return
	}

	job, err := h.store.GetNextJob(nodeID)
	if err != nil {
		if err == store.ErrJobNotFound {
			// No jobs available - record failed scheduling attempt
			if h.metricsRecorder != nil {
				h.metricsRecorder.RecordScheduleAttempt("no_jobs")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"job": nil,
			})
			return
		}
		// Record failed scheduling attempt due to error
		if h.metricsRecorder != nil {
			h.metricsRecorder.RecordScheduleAttempt("error")
		}
		log.Printf("Error getting next job: %v", err)
		http.Error(w, "Failed to get next job", http.StatusInternalServerError)
		return
	}

	// Record successful scheduling attempt
	if h.metricsRecorder != nil {
		h.metricsRecorder.RecordScheduleAttempt("success")
	}

	log.Printf("Job %s assigned to node %s", job.ID, nodeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job": job,
	})
}

// ReceiveResults receives job results from a compute node
func (h *MasterHandler) ReceiveResults(w http.ResponseWriter, r *http.Request) {
	var result models.JobResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Handle retry logic for failed jobs
	if result.Status == models.JobStatusFailed && h.maxRetries > 0 {
		job, err := h.store.GetJob(result.JobID)
		if err != nil {
			log.Printf("Error getting job for retry check: %v", err)
		} else if job.RetryCount < h.maxRetries {
			// Re-queue job for retry using the RetryJob method
			// RetryJob will: increment retry_count, set status to pending, clear node_id and started_at
			if err := h.store.RetryJob(result.JobID, result.Error); err != nil {
				log.Printf("Error re-queuing job for retry: %v", err)
			} else {
				// Get updated job to report correct retry count
				updatedJob, err := h.store.GetJob(result.JobID)
				if err != nil {
					log.Printf("Error getting updated job: %v", err)
				}
				retryCount := job.RetryCount + 1
				if err == nil {
					retryCount = updatedJob.RetryCount
				}
				
				log.Printf("Job %s failed on node %s (attempt %d/%d) - re-queued for retry",
					result.JobID, result.NodeID, retryCount, h.maxRetries)
				
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":      "retrying",
					"retry":       retryCount,
					"max_retries": h.maxRetries,
				})
				return
			}
		} else {
			log.Printf("Job %s failed after %d attempts - max retries reached",
				result.JobID, job.RetryCount)
		}
	}

	// Update job status
	if err := h.store.UpdateJobStatus(result.JobID, result.Status, result.Error); err != nil {
		log.Printf("Error updating job status: %v", err)
		http.Error(w, "Failed to update job status", http.StatusInternalServerError)
		return
	}
	
	// Update logs if provided
	if result.Logs != "" {
		job, err := h.store.GetJob(result.JobID)
		if err == nil {
			job.Logs = result.Logs
			if err := h.store.UpdateJob(job); err != nil {
				log.Printf("Warning: Failed to update job logs: %v", err)
			}
		}
	}

	// Write results to JSON file for exporters
	if result.Status == models.JobStatusCompleted {
		job, err := h.store.GetJob(result.JobID)
		if err == nil && h.resultsWriter != nil {
			if err := h.resultsWriter.WriteJobResult(job, &result); err != nil {
				log.Printf("Warning: Failed to write job results to file: %v", err)
			}
		}
	}

	log.Printf("Results received for job %s (status: %s)", result.JobID, result.Status)
	if result.Error != "" {
		log.Printf("  Error: %s", result.Error)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// GetNodeDetails retrieves detailed information about a specific node
func (h *MasterHandler) GetNodeDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["id"]

	node, err := h.store.GetNode(nodeID)
	if err != nil {
		if err == store.ErrNodeNotFound {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting node: %v", err)
		http.Error(w, "Failed to get node", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// PauseJob pauses a running job
func (h *MasterHandler) PauseJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDOrSeq := vars["id"]

	// Resolve to actual job ID
	job, err := h.getJobByIDOrSequence(jobIDOrSeq)
	if err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retrieve job: %v", err), http.StatusInternalServerError)
		return
	}
	jobID := job.ID

	if err := h.store.PauseJob(jobID); err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error pausing job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to pause job: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Job %s paused", jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "paused",
		"job_id": jobID,
	})
}

// ResumeJob resumes a paused job
func (h *MasterHandler) ResumeJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDOrSeq := vars["id"]

	// Resolve to actual job ID
	job, err := h.getJobByIDOrSequence(jobIDOrSeq)
	if err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retrieve job: %v", err), http.StatusInternalServerError)
		return
	}
	jobID := job.ID

	if err := h.store.ResumeJob(jobID); err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error resuming job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to resume job: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Job %s resumed", jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "resumed",
		"job_id": jobID,
	})
}

// CancelJob cancels a job
func (h *MasterHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDOrSeq := vars["id"]

	// Resolve to actual job ID
	job, err := h.getJobByIDOrSequence(jobIDOrSeq)
	if err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retrieve job: %v", err), http.StatusInternalServerError)
		return
	}
	jobID := job.ID

	if err := h.store.CancelJob(jobID); err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error canceling job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to cancel job: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Job %s canceled", jobID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "canceled",
		"job_id": jobID,
	})
}

// RetryJob retries a failed job
func (h *MasterHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDOrSeq := vars["id"]

	// Get the job to verify it exists and can be retried
	job, err := h.getJobByIDOrSequence(jobIDOrSeq)
	if err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retrieve job: %v", err), http.StatusInternalServerError)
		return
	}

	// Only allow retry for failed or canceled jobs
	if job.Status != "failed" && job.Status != "canceled" {
		http.Error(w, "Only failed or canceled jobs can be retried", http.StatusBadRequest)
		return
	}

	// Reset job to pending state
	job.Status = "pending"
	job.RetryCount++
	job.NodeID = ""
	job.StartedAt = nil
	job.CompletedAt = nil
	job.Error = ""

	if err := h.store.UpdateJob(job); err != nil {
		log.Printf("Error retrying job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retry job: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Job %s queued for retry (attempt %d)", job.ID, job.RetryCount)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "queued",
		"job_id":      job.ID,
		"retry_count": job.RetryCount,
	})
}

// GetJobLogs retrieves logs for a specific job
func (h *MasterHandler) GetJobLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDOrSeq := vars["id"]

	// Get the job to verify it exists
	job, err := h.getJobByIDOrSequence(jobIDOrSeq)
	if err != nil {
		if err == store.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving job: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retrieve job: %v", err), http.StatusInternalServerError)
		return
	}

	// Return logs from database, fallback to error field if no logs available
	logs := job.Logs
	if logs == "" && job.Error != "" {
		logs = fmt.Sprintf("Error: %s", job.Error)
	}
	if logs == "" {
		logs = "No logs available for this job"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"job_id": job.ID,
		"logs":   logs,
	})
}

// RemoveNode removes a node from the cluster
func (h *MasterHandler) RemoveNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["id"]

	// Get the node to check if it exists
	node, err := h.store.GetNode(nodeID)
	if err != nil {
		if err == store.ErrNodeNotFound {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		log.Printf("Error retrieving node: %v", err)
		http.Error(w, fmt.Sprintf("Failed to retrieve node: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if node is currently processing a job
	if node.Status == "busy" {
		http.Error(w, "Cannot remove node while it is processing a job", http.StatusBadRequest)
		return
	}

	// Remove the node
	if err := h.store.DeleteNode(nodeID); err != nil {
		log.Printf("Error removing node: %v", err)
		http.Error(w, fmt.Sprintf("Failed to remove node: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Node %s (%s) removed from cluster", nodeID, node.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "removed",
		"node_id": nodeID,
	})
}

// Health returns the health status of the master node
func (h *MasterHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// Tenant API stubs
func (h *MasterHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) DeleteTenant(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) GetTenantStats(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) GetTenantJobs(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}

func (h *MasterHandler) GetTenantNodes(w http.ResponseWriter, r *http.Request) {
http.Error(w, "Tenant management not yet implemented", http.StatusNotImplemented)
}
