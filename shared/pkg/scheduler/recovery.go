package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// RecoveryManager handles job recovery and reassignment
type RecoveryManager struct {
	store                store.Store
	maxRetries           int
	nodeFailureThreshold time.Duration
}

// NewRecoveryManager creates a new RecoveryManager
func NewRecoveryManager(st store.Store, maxRetries int, nodeFailureThreshold time.Duration) *RecoveryManager {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if nodeFailureThreshold <= 0 {
		// Default: 90s = 3 missed heartbeats @ 30s interval
		nodeFailureThreshold = 90 * time.Second
	}
	return &RecoveryManager{
		store:                st,
		maxRetries:           maxRetries,
		nodeFailureThreshold: nodeFailureThreshold,
	}
}

// RecoverFailedJobs identifies and recovers jobs that should be retried
func (rm *RecoveryManager) RecoverFailedJobs() {
	allJobs := rm.store.GetAllJobs()
	recoveredCount := 0

	for _, job := range allJobs {
		if job.Status != models.JobStatusFailed {
			continue
		}

		// Check if job is eligible for retry
		if job.RetryCount >= rm.maxRetries {
			log.Printf("Recovery: Job %s (seq#%d) exceeded max retries (%d/%d), skipping",
				job.ID, job.SequenceNumber, job.RetryCount, rm.maxRetries)
			continue
		}

		// Check if failure was due to transient issues
		if rm.isTransientFailure(job) {
			log.Printf("Recovery: Retrying job %s (seq#%d) - attempt %d/%d",
				job.ID, job.SequenceNumber, job.RetryCount+1, rm.maxRetries)

			// Reset job to pending with incremented retry count
			if err := rm.store.RetryJob(job.ID, fmt.Sprintf("Retry after transient failure: %s", job.Error)); err != nil {
				log.Printf("Recovery: Failed to reset job %s: %v", job.ID, err)
				continue
			}

			recoveredCount++
			log.Printf("Recovery: Job %s (seq#%d) reset to pending for retry", job.ID, job.SequenceNumber)
		}
	}

	if recoveredCount > 0 {
		log.Printf("Recovery: Recovered %d jobs for retry", recoveredCount)
	}
}

// DetectDeadNodes identifies nodes that have not sent heartbeats recently
func (rm *RecoveryManager) DetectDeadNodes() []string {
	nodes := rm.store.GetAllNodes()
	deadNodes := []string{}
	now := time.Now()

	for _, node := range nodes {
		if node.Status == "offline" {
			continue // Already marked offline
		}

		timeSinceHeartbeat := now.Sub(node.LastHeartbeat)
		if timeSinceHeartbeat > rm.nodeFailureThreshold {
			log.Printf("Recovery: Node %s (%s) failed - no heartbeat for %v (threshold: %v)",
				node.ID, node.Address, timeSinceHeartbeat, rm.nodeFailureThreshold)
			deadNodes = append(deadNodes, node.ID)

			// Mark node as offline
			if err := rm.store.UpdateNodeStatus(node.ID, "offline"); err != nil {
				log.Printf("Recovery: Failed to mark node %s offline: %v", node.ID, err)
			}
		}
	}

	return deadNodes
}

// ReassignJobsFromDeadNodes reassigns jobs from dead nodes to pending queue
func (rm *RecoveryManager) ReassignJobsFromDeadNodes(deadNodeIDs []string) int {
	if len(deadNodeIDs) == 0 {
		return 0
	}

	allJobs := rm.store.GetAllJobs()
	reassignedCount := 0

	for _, job := range allJobs {
		// Only reassign jobs that are processing or assigned
		if job.Status != models.JobStatusProcessing && job.Status != models.JobStatusAssigned {
			continue
		}

		// Check if job was on a dead node
		isOnDeadNode := false
		for _, nodeID := range deadNodeIDs {
			if job.NodeID == nodeID {
				isOnDeadNode = true
				break
			}
		}

		if !isOnDeadNode {
			continue
		}

		log.Printf("Recovery: Reassigning job %s (seq#%d) from dead node %s",
			job.ID, job.SequenceNumber, job.NodeID)

		// Reassign job
		if err := rm.store.RetryJob(job.ID, fmt.Sprintf("Reassigning from dead node: %s", job.NodeID)); err != nil {
			log.Printf("Recovery: Failed to reassign job %s: %v", job.ID, err)
			continue
		}

		reassignedCount++
	}

	if reassignedCount > 0 {
		log.Printf("Recovery: Reassigned %d jobs from dead nodes", reassignedCount)
	}

	return reassignedCount
}

// isTransientFailure checks if a job failure was likely transient
func (rm *RecoveryManager) isTransientFailure(job *models.Job) bool {
	if job.Error == "" {
		return false
	}

	// Check for common transient error patterns
	transientPatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"network error",
		"no such host",
		"broken pipe",
		"connection reset",
		"node unavailable",
		"worker died",
		"stale",
	}

	errorLower := job.Error
	for _, pattern := range transientPatterns {
		if contains(errorLower, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// RunRecoveryCheck performs a complete recovery check cycle
func (rm *RecoveryManager) RunRecoveryCheck() {
	log.Println("Recovery: Starting recovery check cycle")

	// Detect dead nodes
	deadNodes := rm.DetectDeadNodes()

	// Reassign jobs from dead nodes
	if len(deadNodes) > 0 {
		rm.ReassignJobsFromDeadNodes(deadNodes)
	}

	// Recover failed jobs eligible for retry
	rm.RecoverFailedJobs()

	log.Println("Recovery: Recovery check cycle completed")
}
