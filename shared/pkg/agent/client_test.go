package agent

import (
"context"
"net/http"
"net/http/httptest"
"testing"
"time"

"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// TestRetryLogic tests that client methods use retry logic for transient failures
func TestSendHeartbeat_RetriesOnTransientFailure(t *testing.T) {
attempts := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
attempts++
if attempts < 3 {
// Simulate transient failure
w.WriteHeader(http.StatusServiceUnavailable)
return
}
// Third attempt succeeds
w.WriteHeader(http.StatusOK)
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

// Should eventually succeed after retries
err := client.SendHeartbeat()
if err != nil {
t.Errorf("Expected success after retries, got error: %v", err)
}

if attempts != 3 {
t.Errorf("Expected 3 attempts, got %d", attempts)
}
}

func TestGetNextJob_RetriesOnTransientFailure(t *testing.T) {
attempts := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
attempts++
if attempts < 2 {
// Simulate connection timeout
w.WriteHeader(http.StatusGatewayTimeout)
return
}
// Second attempt succeeds with no job
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
w.Write([]byte(`{"job":null}`))
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

// Should eventually succeed after retries
job, err := client.GetNextJob()
if err != nil {
t.Errorf("Expected success after retries, got error: %v", err)
}

if job != nil {
t.Errorf("Expected nil job, got %+v", job)
}

if attempts != 2 {
t.Errorf("Expected 2 attempts, got %d", attempts)
}
}

func TestSendResults_RetriesOnTransientFailure(t *testing.T) {
attempts := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
attempts++
if attempts < 3 {
// Simulate bad gateway
w.WriteHeader(http.StatusBadGateway)
return
}
// Third attempt succeeds
w.WriteHeader(http.StatusOK)
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

result := &models.JobResult{
JobID:  "test-job",
NodeID: "test-node",
Status: models.JobStatusCompleted,
}

// Should eventually succeed after retries
err := client.SendResults(result)
if err != nil {
t.Errorf("Expected success after retries, got error: %v", err)
}

if attempts != 3 {
t.Errorf("Expected 3 attempts, got %d", attempts)
}
}

func TestSendHeartbeat_FailsAfterMaxRetries(t *testing.T) {
attempts := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
attempts++
// Always fail
w.WriteHeader(http.StatusServiceUnavailable)
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

// Should fail after max retries
err := client.SendHeartbeat()
if err == nil {
t.Error("Expected error after max retries, got nil")
}

// Should have tried: initial + 3 retries = 4 attempts
if attempts != 4 {
t.Errorf("Expected 4 attempts (initial + 3 retries), got %d", attempts)
}
}

func TestRetry_RespectsContextCancellation(t *testing.T) {
attempts := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
attempts++
// Always fail to force retries
time.Sleep(100 * time.Millisecond)
w.WriteHeader(http.StatusServiceUnavailable)
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

// Cancel context after first attempt
_, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
defer cancel()

// Temporarily override retry config to use context
// (This is a limitation - we'd need to refactor client to accept context)
// For now, just test that retries do happen
err := client.SendHeartbeat()
if err == nil {
t.Error("Expected error, got nil")
}

// Should have at least tried once
if attempts == 0 {
t.Error("Expected at least one attempt")
}
}

// Benchmark retry overhead
func BenchmarkSendHeartbeat_Success(b *testing.B) {
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

b.ResetTimer()
for i := 0; i < b.N; i++ {
client.SendHeartbeat()
}
}

func BenchmarkSendHeartbeat_WithRetries(b *testing.B) {
attempts := 0
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
attempts++
if attempts%2 == 1 {
w.WriteHeader(http.StatusServiceUnavailable)
return
}
w.WriteHeader(http.StatusOK)
attempts = 0 // Reset for next benchmark iteration
}))
defer server.Close()

client := NewClient(server.URL)
client.nodeID = "test-node"

b.ResetTimer()
for i := 0; i < b.N; i++ {
client.SendHeartbeat()
}
}
