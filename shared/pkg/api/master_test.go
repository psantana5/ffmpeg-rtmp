package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/api"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

// TestRouteOrdering verifies that specific routes like /jobs/next
// are registered before parameterized routes like /jobs/{id}
func TestRouteOrdering(t *testing.T) {
	// Create a test store
	testStore := store.NewMemoryStore()

	// Create test job for /jobs/next endpoint
	testJob := &models.Job{
		ID:         uuid.New().String(),
		Scenario:   "test-scenario",
		Confidence: "auto",
		Status:     models.JobStatusPending,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}
	if err := testStore.CreateJob(testJob); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	// Register test node
	testNode := &models.Node{
		ID:            "test-node-123",
		Address:       "localhost:8081",
		Type:          "test",
		CPUThreads:    4,
		Status:        "available",
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}
	if err := testStore.RegisterNode(testNode); err != nil {
		t.Fatalf("Failed to register test node: %v", err)
	}

	// Create handler and register routes
	handler := api.NewMasterHandler(testStore)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	// Test 1: Verify /jobs/next returns next job (not 404)
	t.Run("JobsNextEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/jobs/next?node_id=test-node-123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
		}

		// Verify response contains a job
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["job"] == nil {
			t.Error("Expected job in response, got nil")
		}
	})

	// Test 2: Verify /jobs/{id} works with actual UUID
	t.Run("JobsGetByIDEndpoint", func(t *testing.T) {
		// Create another test job that we can retrieve
		testJob2 := &models.Job{
			ID:         uuid.New().String(),
			Scenario:   "test-scenario-2",
			Confidence: "high",
			Status:     models.JobStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}
		if err := testStore.CreateJob(testJob2); err != nil {
			t.Fatalf("Failed to create second test job: %v", err)
		}

		req := httptest.NewRequest("GET", "/jobs/"+testJob2.ID, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
		}

		// Verify response contains the correct job
		var job models.Job
		if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
			t.Fatalf("Failed to parse job response: %v", err)
		}

		if job.ID != testJob2.ID {
			t.Errorf("Expected job ID %s, got %s", testJob2.ID, job.ID)
		}
	})

	// Test 3: Verify /jobs/next doesn't match /jobs/{id}
	t.Run("NextNotMatchedByID", func(t *testing.T) {
		// If routes are in wrong order, /jobs/next would be handled by GetJob
		// which would return 404 because there's no job with ID "next"
		req := httptest.NewRequest("GET", "/jobs/next?node_id=test-node-123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should not be 404 if routes are correctly ordered
		if w.Code == http.StatusNotFound {
			t.Error("Route /jobs/next incorrectly matched /jobs/{id} pattern")
		}

		// Should be 200 or return valid response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Response should be for GetNextJob, not GetJob
		bodyStr := w.Body.String()
		if strings.Contains(bodyStr, "job not found") {
			t.Error("Response suggests /jobs/next was handled by GetJob handler")
		}
	})
}

// TestJobLifecycle tests creating and retrieving a job
func TestJobLifecycle(t *testing.T) {
	testStore := store.NewMemoryStore()
	handler := api.NewMasterHandler(testStore)

	// Create a job
	jobReq := `{"scenario":"4K60-h264","confidence":"auto"}`
	req := httptest.NewRequest("POST", "/jobs", strings.NewReader(jobReq))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateJob(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var createdJob models.Job
	if err := json.Unmarshal(w.Body.Bytes(), &createdJob); err != nil {
		t.Fatalf("Failed to parse created job: %v", err)
	}

	// Retrieve the job
	router := mux.NewRouter()
	router.HandleFunc("/jobs/{id}", handler.GetJob).Methods("GET")

	req2 := httptest.NewRequest("GET", "/jobs/"+createdJob.ID, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Response: %s", w2.Code, w2.Body.String())
	}

	var retrievedJob models.Job
	if err := json.Unmarshal(w2.Body.Bytes(), &retrievedJob); err != nil {
		t.Fatalf("Failed to parse retrieved job: %v", err)
	}

	if retrievedJob.ID != createdJob.ID {
		t.Errorf("Expected job ID %s, got %s", createdJob.ID, retrievedJob.ID)
	}
}
