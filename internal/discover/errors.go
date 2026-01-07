package discover

import (
	"fmt"
	"time"
)

// ErrorType categorizes errors for handling strategy
type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeTransient           // Temporary, retry possible
	ErrorTypePermanent           // Permanent, no retry
	ErrorTypeRateLimit           // Rate limit, backoff needed
	ErrorTypeResource            // Resource exhaustion
)

// DiscoveryError wraps errors with context and categorization
type DiscoveryError struct {
	Type      ErrorType
	Operation string // "scan", "attach", "state_save", etc.
	PID       int
	Message   string
	Err       error
	Timestamp time.Time
	Retryable bool
}

// Error implements error interface
func (e *DiscoveryError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s failed for PID %d: %s: %v", e.Operation, e.PID, e.Message, e.Err)
	}
	return fmt.Sprintf("%s failed for PID %d: %s", e.Operation, e.PID, e.Message)
}

// Unwrap implements error unwrapping
func (e *DiscoveryError) Unwrap() error {
	return e.Err
}

// NewDiscoveryError creates a new discovery error
func NewDiscoveryError(errType ErrorType, operation string, pid int, message string, err error) *DiscoveryError {
	retryable := errType == ErrorTypeTransient || errType == ErrorTypeRateLimit
	
	return &DiscoveryError{
		Type:      errType,
		Operation: operation,
		PID:       pid,
		Message:   message,
		Err:       err,
		Timestamp: time.Now(),
		Retryable: retryable,
	}
}

// ErrorClassifier determines error type from error content
type ErrorClassifier struct{}

// Classify determines error type from error
func (ec *ErrorClassifier) Classify(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}
	
	errStr := err.Error()
	
	// Transient errors (retry possible)
	transientPatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"resource temporarily unavailable",
		"no such process", // PID may have just exited
	}
	
	for _, pattern := range transientPatterns {
		if contains(errStr, pattern) {
			return ErrorTypeTransient
		}
	}
	
	// Resource errors
	resourcePatterns := []string{
		"out of memory",
		"too many open files",
		"no space left",
		"disk quota exceeded",
	}
	
	for _, pattern := range resourcePatterns {
		if contains(errStr, pattern) {
			return ErrorTypeResource
		}
	}
	
	// Rate limit errors
	if contains(errStr, "rate limit") || contains(errStr, "too many requests") {
		return ErrorTypeRateLimit
	}
	
	// Default to permanent
	return ErrorTypePermanent
}

// ErrorMetrics tracks error statistics
type ErrorMetrics struct {
	TotalErrors         int64
	TransientErrors     int64
	PermanentErrors     int64
	RateLimitErrors     int64
	ResourceErrors      int64
	LastError           *DiscoveryError
	ConsecutiveFailures int
}

// RecordError records an error in metrics
func (em *ErrorMetrics) RecordError(err *DiscoveryError) {
	em.TotalErrors++
	em.LastError = err
	em.ConsecutiveFailures++
	
	switch err.Type {
	case ErrorTypeTransient:
		em.TransientErrors++
	case ErrorTypePermanent:
		em.PermanentErrors++
	case ErrorTypeRateLimit:
		em.RateLimitErrors++
	case ErrorTypeResource:
		em.ResourceErrors++
	}
}

// RecordSuccess resets consecutive failure count
func (em *ErrorMetrics) RecordSuccess() {
	em.ConsecutiveFailures = 0
}

// BackoffStrategy calculates retry delay with exponential backoff
type BackoffStrategy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// NewBackoffStrategy creates default backoff strategy
func NewBackoffStrategy() *BackoffStrategy {
	return &BackoffStrategy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
	}
}

// CalculateDelay calculates delay for attempt number
func (bs *BackoffStrategy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return bs.InitialDelay
	}
	
	// Exponential backoff: delay = initial * multiplier^attempt
	delay := float64(bs.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= bs.Multiplier
	}
	
	result := time.Duration(delay)
	if result > bs.MaxDelay {
		result = bs.MaxDelay
	}
	
	return result
}

// contains checks if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive check
	return len(s) >= len(substr) && (s == substr || 
		len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
