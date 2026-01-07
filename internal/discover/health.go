package discover

import (
	"sync"
	"time"
)

// HealthStatus represents the health state of the service
type HealthStatus int

const (
	HealthStatusHealthy HealthStatus = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
)

// String returns string representation of health status
func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// HealthCheck tracks service health
type HealthCheck struct {
	mu sync.RWMutex
	
	// Status
	status           HealthStatus
	lastStatusChange time.Time
	
	// Scan health
	lastSuccessfulScan time.Time
	consecutiveScanFailures int
	totalScanFailures      int64
	
	// Attachment health
	lastSuccessfulAttachment time.Time
	consecutiveAttachFailures int
	totalAttachFailures      int64
	
	// Thresholds
	maxConsecutiveScanFailures   int
	maxConsecutiveAttachFailures int
	maxScanAge                   time.Duration
	
	// Error tracking
	errorMetrics *ErrorMetrics
}

// NewHealthCheck creates a new health check
func NewHealthCheck() *HealthCheck {
	now := time.Now()
	return &HealthCheck{
		status:                       HealthStatusHealthy,
		lastStatusChange:             now,
		lastSuccessfulScan:           now,
		lastSuccessfulAttachment:     now,
		maxConsecutiveScanFailures:   5,
		maxConsecutiveAttachFailures: 10,
		maxScanAge:                   2 * time.Minute,
		errorMetrics:                 &ErrorMetrics{},
	}
}

// RecordScanSuccess records a successful scan
func (hc *HealthCheck) RecordScanSuccess() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.lastSuccessfulScan = time.Now()
	hc.consecutiveScanFailures = 0
	hc.errorMetrics.RecordSuccess()
	hc.updateStatus()
}

// RecordScanFailure records a failed scan
func (hc *HealthCheck) RecordScanFailure(err *DiscoveryError) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.consecutiveScanFailures++
	hc.totalScanFailures++
	hc.errorMetrics.RecordError(err)
	hc.updateStatus()
}

// RecordAttachSuccess records a successful attachment
func (hc *HealthCheck) RecordAttachSuccess() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.lastSuccessfulAttachment = time.Now()
	hc.consecutiveAttachFailures = 0
	hc.updateStatus()
}

// RecordAttachFailure records a failed attachment
func (hc *HealthCheck) RecordAttachFailure(err *DiscoveryError) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.consecutiveAttachFailures++
	hc.totalAttachFailures++
	hc.errorMetrics.RecordError(err)
	hc.updateStatus()
}

// updateStatus updates health status based on current state
// Must be called with lock held
func (hc *HealthCheck) updateStatus() {
	oldStatus := hc.status
	newStatus := HealthStatusHealthy
	
	// Check scan health
	timeSinceLastScan := time.Since(hc.lastSuccessfulScan)
	if timeSinceLastScan > hc.maxScanAge {
		newStatus = HealthStatusUnhealthy
	} else if hc.consecutiveScanFailures >= hc.maxConsecutiveScanFailures {
		newStatus = HealthStatusUnhealthy
	} else if hc.consecutiveScanFailures >= hc.maxConsecutiveScanFailures/2 {
		newStatus = HealthStatusDegraded
	}
	
	// Check attachment health
	if hc.consecutiveAttachFailures >= hc.maxConsecutiveAttachFailures {
		if newStatus < HealthStatusDegraded {
			newStatus = HealthStatusDegraded
		}
	}
	
	// Update status if changed
	if newStatus != oldStatus {
		hc.status = newStatus
		hc.lastStatusChange = time.Now()
	}
}

// GetStatus returns current health status
func (hc *HealthCheck) GetStatus() HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	return hc.status
}

// IsHealthy returns true if service is healthy
func (hc *HealthCheck) IsHealthy() bool {
	return hc.GetStatus() == HealthStatusHealthy
}

// GetHealthReport returns detailed health report
func (hc *HealthCheck) GetHealthReport() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	return map[string]interface{}{
		"status":                        hc.status.String(),
		"status_duration":               time.Since(hc.lastStatusChange).String(),
		"last_successful_scan":          hc.lastSuccessfulScan.Format(time.RFC3339),
		"time_since_last_scan":          time.Since(hc.lastSuccessfulScan).String(),
		"consecutive_scan_failures":     hc.consecutiveScanFailures,
		"total_scan_failures":           hc.totalScanFailures,
		"last_successful_attachment":    hc.lastSuccessfulAttachment.Format(time.RFC3339),
		"time_since_last_attachment":    time.Since(hc.lastSuccessfulAttachment).String(),
		"consecutive_attach_failures":   hc.consecutiveAttachFailures,
		"total_attach_failures":         hc.totalAttachFailures,
		"total_errors":                  hc.errorMetrics.TotalErrors,
		"transient_errors":              hc.errorMetrics.TransientErrors,
		"permanent_errors":              hc.errorMetrics.PermanentErrors,
		"rate_limit_errors":             hc.errorMetrics.RateLimitErrors,
		"resource_errors":               hc.errorMetrics.ResourceErrors,
		"consecutive_failures":          hc.errorMetrics.ConsecutiveFailures,
	}
}

// GetLastError returns the most recent error
func (hc *HealthCheck) GetLastError() *DiscoveryError {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	return hc.errorMetrics.LastError
}
