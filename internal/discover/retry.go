package discover

import (
	"context"
	"log"
	"sync"
	"time"
)

// RetryItem represents an attachment that failed and needs retry
type RetryItem struct {
	Process      *Process
	Attempt      int
	LastAttempt  time.Time
	NextAttempt  time.Time
	LastError    *DiscoveryError
	MaxAttempts  int
}

// RetryQueue manages failed attachments for retry
type RetryQueue struct {
	items    map[int]*RetryItem // PID -> RetryItem
	mu       sync.RWMutex
	backoff  *BackoffStrategy
	logger   *log.Logger
	
	// Configuration
	maxAttempts int
	enabled     bool
}

// NewRetryQueue creates a new retry queue
func NewRetryQueue(maxAttempts int, logger *log.Logger) *RetryQueue {
	if maxAttempts <= 0 {
		maxAttempts = 3 // Default: 3 retry attempts
	}
	
	return &RetryQueue{
		items:       make(map[int]*RetryItem),
		backoff:     NewBackoffStrategy(),
		logger:      logger,
		maxAttempts: maxAttempts,
		enabled:     true,
	}
}

// Add adds a failed attachment to retry queue
func (rq *RetryQueue) Add(proc *Process, err *DiscoveryError) {
	if !rq.enabled {
		return
	}
	
	rq.mu.Lock()
	defer rq.mu.Unlock()
	
	// Check if already in queue
	if item, exists := rq.items[proc.PID]; exists {
		// Update existing item
		item.Attempt++
		item.LastAttempt = time.Now()
		item.LastError = err
		item.NextAttempt = time.Now().Add(rq.backoff.CalculateDelay(item.Attempt))
		
		if item.Attempt >= rq.maxAttempts {
			rq.logger.Printf("[retry] PID %d exceeded max attempts (%d), moving to dead letter", 
				proc.PID, rq.maxAttempts)
			delete(rq.items, proc.PID)
		} else {
			rq.logger.Printf("[retry] PID %d retry scheduled for %v (attempt %d/%d)",
				proc.PID, item.NextAttempt.Format("15:04:05"), item.Attempt+1, rq.maxAttempts)
		}
	} else {
		// Add new item
		nextAttempt := time.Now().Add(rq.backoff.CalculateDelay(0))
		rq.items[proc.PID] = &RetryItem{
			Process:     proc,
			Attempt:     0,
			LastAttempt: time.Now(),
			NextAttempt: nextAttempt,
			LastError:   err,
			MaxAttempts: rq.maxAttempts,
		}
		
		rq.logger.Printf("[retry] PID %d added to retry queue, next attempt at %v",
			proc.PID, nextAttempt.Format("15:04:05"))
	}
}

// GetReadyItems returns items ready for retry
func (rq *RetryQueue) GetReadyItems() []*RetryItem {
	if !rq.enabled {
		return nil
	}
	
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	
	now := time.Now()
	ready := make([]*RetryItem, 0)
	
	for _, item := range rq.items {
		if now.After(item.NextAttempt) {
			ready = append(ready, item)
		}
	}
	
	return ready
}

// Remove removes an item from queue (successful retry)
func (rq *RetryQueue) Remove(pid int) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	
	delete(rq.items, pid)
}

// Size returns number of items in queue
func (rq *RetryQueue) Size() int {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	
	return len(rq.items)
}

// GetStats returns retry queue statistics
func (rq *RetryQueue) GetStats() map[string]interface{} {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	
	stats := map[string]interface{}{
		"queue_size":   len(rq.items),
		"max_attempts": rq.maxAttempts,
		"enabled":      rq.enabled,
	}
	
	// Count items by attempt
	attemptCounts := make(map[int]int)
	for _, item := range rq.items {
		attemptCounts[item.Attempt]++
	}
	stats["by_attempt"] = attemptCounts
	
	return stats
}

// StartRetryWorker starts background worker to process retries
func (rq *RetryQueue) StartRetryWorker(ctx context.Context, retryFunc func(*Process) error) {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			readyItems := rq.GetReadyItems()
			
			for _, item := range readyItems {
				// Try to reattach
				rq.logger.Printf("[retry] Retrying attachment to PID %d (attempt %d/%d)",
					item.Process.PID, item.Attempt+1, rq.maxAttempts)
				
				err := retryFunc(item.Process)
				if err == nil {
					// Success - remove from queue
					rq.Remove(item.Process.PID)
					rq.logger.Printf("[retry] PID %d attachment succeeded on retry", item.Process.PID)
				} else {
					// Failed - will be rescheduled via Add()
					rq.logger.Printf("[retry] PID %d attachment failed on retry: %v", item.Process.PID, err)
				}
			}
		}
	}
}
