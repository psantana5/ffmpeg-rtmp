package scheduler

import (
	"fmt"
	"strings"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// CapabilityRequirements represents what a job needs from a worker
type CapabilityRequirements struct {
	RequiresGPU       bool
	RequiredEncoder   string // e.g., "nvenc_h264", "h264"
	RequiredEngine    string // "ffmpeg", "gstreamer", or "auto"
	MinCPUThreads     int
	MinRAMBytes       uint64
}

// ExtractJobRequirements analyzes a job and extracts capability requirements
func ExtractJobRequirements(job *models.Job) *CapabilityRequirements {
	req := &CapabilityRequirements{
		RequiredEngine: job.Engine,
	}

	if job.Parameters == nil {
		return req
	}

	// Check for GPU encoder requirements
	if codec, ok := job.Parameters["codec"].(string); ok {
		req.RequiredEncoder = codec
		
		// GPU encoders
		if strings.Contains(codec, "nvenc") || 
		   strings.Contains(codec, "qsv") || 
		   strings.Contains(codec, "videotoolbox") ||
		   strings.Contains(codec, "vaapi") {
			req.RequiresGPU = true
		}
	}

	// Check for hardware acceleration
	if hwaccel, ok := job.Parameters["hwaccel"].(string); ok {
		if hwaccel != "none" && hwaccel != "" {
			req.RequiresGPU = true
		}
	}

	// Extract resource requirements (optional)
	if threads, ok := job.Parameters["threads"].(float64); ok {
		req.MinCPUThreads = int(threads)
	} else if threads, ok := job.Parameters["threads"].(int); ok {
		req.MinCPUThreads = threads
	}

	return req
}

// CanNodeSatisfyJob checks if a node has the capabilities to run a job
func CanNodeSatisfyJob(node *models.Node, requirements *CapabilityRequirements) (bool, string) {
	// Check GPU requirement
	if requirements.RequiresGPU && !node.HasGPU {
		return false, fmt.Sprintf("job requires GPU but node %s has no GPU", node.Name)
	}

	// Check specific GPU encoder availability
	if requirements.RequiredEncoder != "" && requirements.RequiresGPU {
		if !hasGPUCapability(node, requirements.RequiredEncoder) {
			return false, fmt.Sprintf("job requires encoder %s but node %s lacks this capability", 
				requirements.RequiredEncoder, node.Name)
		}
	}

	// Check engine compatibility
	if requirements.RequiredEngine != "" && requirements.RequiredEngine != "auto" {
		// For now, assume all nodes support both ffmpeg and gstreamer
		// In a real system, you'd check node.Labels or a capabilities field
	}

	// Check CPU threads
	if requirements.MinCPUThreads > 0 && node.CPUThreads < requirements.MinCPUThreads {
		return false, fmt.Sprintf("job requires %d CPU threads but node %s only has %d", 
			requirements.MinCPUThreads, node.Name, node.CPUThreads)
	}

	// Check RAM
	if requirements.MinRAMBytes > 0 && node.RAMTotalBytes < requirements.MinRAMBytes {
		return false, fmt.Sprintf("job requires %d bytes RAM but node %s only has %d", 
			requirements.MinRAMBytes, node.Name, node.RAMTotalBytes)
	}

	return true, ""
}

// hasGPUCapability checks if a node supports a specific GPU encoder
func hasGPUCapability(node *models.Node, encoder string) bool {
	if !node.HasGPU {
		return false
	}

	// If no explicit capabilities listed, assume node supports common encoders
	if len(node.GPUCapabilities) == 0 {
		return true
	}

	// Check for exact match or prefix match
	// Handle both "nvenc_h264" and "h264_nvenc" formats
	for _, cap := range node.GPUCapabilities {
		if encoder == cap || 
		   strings.Contains(encoder, cap) || 
		   strings.Contains(cap, encoder) ||
		   (strings.Contains(encoder, "nvenc") && strings.Contains(cap, "nvenc")) ||
		   (strings.Contains(encoder, "qsv") && strings.Contains(cap, "qsv")) {
			return true
		}
	}

	return false
}

// FindCompatibleWorkers returns workers that can satisfy job requirements
func FindCompatibleWorkers(job *models.Job, availableWorkers []*models.Node) ([]*models.Node, string) {
	requirements := ExtractJobRequirements(job)
	compatible := []*models.Node{}
	var rejectionReason string

	for _, worker := range availableWorkers {
		canRun, reason := CanNodeSatisfyJob(worker, requirements)
		if canRun {
			compatible = append(compatible, worker)
		} else if rejectionReason == "" {
			// Store first rejection reason for logging
			rejectionReason = reason
		}
	}

	return compatible, rejectionReason
}

// ValidateClusterCapabilities checks if ANY worker in the cluster can run the job
func ValidateClusterCapabilities(job *models.Job, allWorkers []*models.Node) (bool, string) {
	requirements := ExtractJobRequirements(job)
	
	// Check if any worker (available or not) can satisfy requirements
	for _, worker := range allWorkers {
		canRun, _ := CanNodeSatisfyJob(worker, requirements)
		if canRun {
			return true, ""
		}
	}

	// No worker can satisfy requirements
	reason := "no workers in cluster can satisfy job requirements"
	
	if requirements.RequiresGPU {
		reason = fmt.Sprintf("job requires GPU (encoder: %s) but no workers have GPU capabilities", 
			requirements.RequiredEncoder)
	} else if requirements.MinCPUThreads > 0 {
		reason = fmt.Sprintf("job requires %d CPU threads but no workers meet this requirement", 
			requirements.MinCPUThreads)
	}

	return false, reason
}
