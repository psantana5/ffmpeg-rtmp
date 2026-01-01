package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	// With rate.NewLimiter(10, 2), the limiter starts with 2 tokens in the bucket
	// Each Allow() call consumes 1 token
	limiter := NewLimiter(10, 2) // 10 requests per second, burst of 2

	// First request should pass (2 tokens -> 1 token)
	if !limiter.Allow("test-key") {
		t.Error("First request should be allowed")
	}

	// Second request should pass (1 token -> 0 tokens)
	if !limiter.Allow("test-key") {
		t.Error("Second request should be allowed")
	}

	// Third request should fail (0 tokens, need to wait for refill)
	if limiter.Allow("test-key") {
		t.Error("Third request should be rate limited")
	}

	// Wait for token refill (10 req/s = 100ms per token)
	time.Sleep(150 * time.Millisecond)

	// Should pass after waiting (refilled 1 token)
	if !limiter.Allow("test-key") {
		t.Error("Request after waiting should be allowed")
	}
}

func TestMiddleware(t *testing.T) {
	limiter := NewLimiter(10, 2) // 10 requests per second, burst of 2

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := limiter.Middleware(func(r *http.Request) string {
		return "test-key"
	})

	wrappedHandler := middleware(handler)

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/test", nil)
	rr1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", rr1.Code)
	}

	// Second request should succeed
	req2 := httptest.NewRequest("GET", "/test", nil)
	rr2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Second request should succeed, got status %d", rr2.Code)
	}

	// Third immediate request should be rate limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	rr3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("Third request should be rate limited, got status %d", rr3.Code)
	}
}

func TestIPKeyFunc(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		expectedKey    string
	}{
		{
			name:          "Direct connection",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "",
			expectedKey:   "192.168.1.1:12345",
		},
		{
			name:          "Behind proxy",
			remoteAddr:    "127.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			expectedKey:   "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			key := IPKeyFunc(req)
			if key != tt.expectedKey {
				t.Errorf("Expected key %s, got %s", tt.expectedKey, key)
			}
		})
	}
}
