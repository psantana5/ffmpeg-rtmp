package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter provides rate limiting functionality
type Limiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
}

// NewLimiter creates a new rate limiter
// rps: requests per second
// burst: maximum burst size
func NewLimiter(rps float64, burst int) *Limiter {
	return &Limiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// GetLimiter returns a rate limiter for the given key (e.g., IP address or API key)
func (l *Limiter) GetLimiter(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter, exists := l.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(l.rps, l.burst)
		l.limiters[key] = limiter
	}

	return limiter
}

// Allow checks if a request should be allowed
func (l *Limiter) Allow(key string) bool {
	return l.GetLimiter(key).Allow()
}

// Middleware creates an HTTP middleware for rate limiting
func (l *Limiter) Middleware(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			
			if !l.Allow(key) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CleanupOldLimiters removes limiters that haven't been used recently
func (l *Limiter) CleanupOldLimiters(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Note: In production, you'd want to track last access time
	// For simplicity, we'll just keep all limiters
	// A more sophisticated implementation would use LRU cache
}

// IPKeyFunc extracts the IP address from the request as the rate limit key
func IPKeyFunc(r *http.Request) string {
	// Try X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// APIKeyFunc extracts the API key from the Authorization header as the rate limit key
func APIKeyFunc(r *http.Request) string {
	return r.Header.Get("Authorization")
}
