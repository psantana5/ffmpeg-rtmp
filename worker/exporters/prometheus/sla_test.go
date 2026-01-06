package prometheus

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSLATracking(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	slaTarget := GetDefaultSLATarget()
	
	// Record some jobs: 2 compliant, 1 violation
	exporter.RecordJobCompletion(300, false, slaTarget, true)  // 5 min - compliant, SLA-worthy
	exporter.RecordJobCompletion(500, false, slaTarget, true)  // 8.3 min - compliant, SLA-worthy
	exporter.RecordJobCompletion(700, false, slaTarget, true)  // 11.7 min - violation (over 10 min target), SLA-worthy
	
	// Create HTTP test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	exporter.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)
	
	// Verify metrics are present
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_completed_total") {
		t.Error("Missing jobs_completed_total metric")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_sla_compliant_total") {
		t.Error("Missing jobs_sla_compliant_total metric")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_sla_violation_total") {
		t.Error("Missing jobs_sla_violation_total metric")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_worker_sla_compliance_rate") {
		t.Error("Missing sla_compliance_rate metric")
	}
	
	// Verify counts
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_completed_total{node_id=\"test-worker\"} 3") {
		t.Error("Completed jobs count incorrect")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_sla_compliant_total{node_id=\"test-worker\"} 2") {
		t.Error("SLA compliant count incorrect")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_sla_violation_total{node_id=\"test-worker\"} 1") {
		t.Error("SLA violation count incorrect")
	}
	
	// Compliance rate should be 2/3 = 66.67%
	if !strings.Contains(metricsOutput, "ffrtmp_worker_sla_compliance_rate{node_id=\"test-worker\"} 66.67") {
		t.Error("SLA compliance rate calculation incorrect")
	}
}

func TestSLAFailedJobs(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	slaTarget := GetDefaultSLATarget()
	
	// Record 2 successful, 1 failed job
	exporter.RecordJobCompletion(300, false, slaTarget, true)  // Success, SLA-worthy
	exporter.RecordJobCompletion(400, false, slaTarget, true)  // Success, SLA-worthy
	exporter.RecordJobCompletion(200, true, slaTarget, false)   // Failed (doesn't count for SLA)
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	exporter.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)
	
	// Verify completed and failed counts
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_completed_total{node_id=\"test-worker\"} 2") {
		t.Error("Completed jobs count should be 2")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_failed_total{node_id=\"test-worker\"} 1") {
		t.Error("Failed jobs count should be 1")
	}
	
	// Failed jobs don't count toward SLA metrics
	if !strings.Contains(metricsOutput, "ffrtmp_worker_jobs_sla_compliant_total{node_id=\"test-worker\"} 2") {
		t.Error("SLA compliant should only count successful jobs")
	}
}

func TestSLAComplianceRate(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	slaTarget := SLATarget{MaxDurationSeconds: 100, MaxFailureRate: 0.05}
	
	// Test different scenarios
	testCases := []struct {
		name               string
		jobs               []struct{ duration float64; failed bool }
		expectedCompliance float64
	}{
		{
			name: "All compliant",
			jobs: []struct{ duration float64; failed bool }{
				{50, false},
				{75, false},
				{90, false},
			},
			expectedCompliance: 100.0,
		},
		{
			name: "All violations",
			jobs: []struct{ duration float64; failed bool }{
				{150, false},
				{200, false},
				{250, false},
			},
			expectedCompliance: 0.0,
		},
		{
			name: "Mixed",
			jobs: []struct{ duration float64; failed bool }{
				{50, false},  // Compliant
				{150, false}, // Violation
				{80, false},  // Compliant
				{200, false}, // Violation
			},
			expectedCompliance: 50.0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset exporter
			exporter = NewWorkerExporter("test-worker", false)
			
			// Record jobs
			for _, job := range tc.jobs {
				exporter.RecordJobCompletion(job.duration, job.failed, slaTarget, true)
			}
			
			// Check compliance rate
			compliance := exporter.GetSLAComplianceRate()
			if compliance != tc.expectedCompliance {
				t.Errorf("Expected compliance %.2f%%, got %.2f%%", tc.expectedCompliance, compliance)
			}
		})
	}
}

func TestSLADefaultTarget(t *testing.T) {
	target := GetDefaultSLATarget()
	
	if target.MaxDurationSeconds != 600 {
		t.Errorf("Expected default max duration 600s, got %.0f", target.MaxDurationSeconds)
	}
	
	if target.MaxFailureRate != 0.05 {
		t.Errorf("Expected default max failure rate 0.05, got %.2f", target.MaxFailureRate)
	}
}

func TestGetJobCompletionStats(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	slaTarget := GetDefaultSLATarget()
	
	// Record various jobs
	exporter.RecordJobCompletion(300, false, slaTarget, true)  // Completed, compliant, SLA-worthy
	exporter.RecordJobCompletion(700, false, slaTarget, true)  // Completed, violation, SLA-worthy
	exporter.RecordJobCompletion(400, false, slaTarget, true)  // Completed, compliant, SLA-worthy
	exporter.RecordJobCompletion(200, true, slaTarget, false)   // Failed (not counted)
	
	completed, failed, compliant, violation := exporter.GetJobCompletionStats()
	
	if completed != 3 {
		t.Errorf("Expected 3 completed jobs, got %d", completed)
	}
	
	if failed != 1 {
		t.Errorf("Expected 1 failed job, got %d", failed)
	}
	
	if compliant != 2 {
		t.Errorf("Expected 2 compliant jobs, got %d", compliant)
	}
	
	if violation != 1 {
		t.Errorf("Expected 1 violation, got %d", violation)
	}
}

func TestSLAInitialState(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	
	// Before any jobs, compliance should be 100%
	compliance := exporter.GetSLAComplianceRate()
	if compliance != 100.0 {
		t.Errorf("Expected 100%% initial compliance, got %.2f%%", compliance)
	}
	
	completed, failed, compliant, violation := exporter.GetJobCompletionStats()
	if completed != 0 || failed != 0 || compliant != 0 || violation != 0 {
		t.Error("Expected all stats to be 0 initially")
	}
}

func TestSLAExactBoundary(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	slaTarget := SLATarget{MaxDurationSeconds: 600, MaxFailureRate: 0.05}
	
	// Test exact boundary cases
	exporter.RecordJobCompletion(600.0, false, slaTarget, true)  // Exactly at limit - should be compliant
	exporter.RecordJobCompletion(600.1, false, slaTarget, true)  // Just over limit - should be violation
	exporter.RecordJobCompletion(599.9, false, slaTarget, true)  // Just under limit - should be compliant
	
	completed, _, compliant, violation := exporter.GetJobCompletionStats()
	
	if completed != 3 {
		t.Errorf("Expected 3 completed jobs, got %d", completed)
	}
	
	if compliant != 2 {
		t.Errorf("Expected 2 compliant jobs (600.0 and 599.9), got %d", compliant)
	}
	
	if violation != 1 {
		t.Errorf("Expected 1 violation (600.1), got %d", violation)
	}
}
