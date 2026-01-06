package retry

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration  
	Multiplier     float64       // Backoff multiplier (exponential)
}

// DefaultConfig returns sensible defaults for retries
func DefaultConfig() Config {
	return Config{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
	}
}

// Do executes fn with exponential backoff retries
func Do(ctx context.Context, config Config, fn func() error) error {
	var lastErr error
	backoff := config.InitialBackoff
	
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}
		
		// Execute function
		err := fn()
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Don't sleep after last attempt
		if attempt == config.MaxRetries {
			break
		}
		
		// Sleep with exponential backoff
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(backoff):
		}
		
		// Calculate next backoff
		backoff = time.Duration(float64(backoff) * config.Multiplier)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}
	
	return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, lastErr)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	// Network errors and temporary failures are retryable
	errStr := strings.ToLower(err.Error())
	
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"503",
		"502",
		"504",
		"eof",
		"broken pipe",
	}
	
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	
	return false
}
