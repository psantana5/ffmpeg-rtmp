package prometheus

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBandwidthMetrics(t *testing.T) {
	// Create exporter
	exporter := NewWorkerExporter("test-worker", false)
	
	// Record some bandwidth metrics
	exporter.RecordJobBandwidth(52428800, 41943040, 62.5) // ~50MB input, ~40MB output, 62.5s duration
	
	// Create HTTP test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	// Serve metrics
	exporter.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)
	
	// Verify metrics are present
	tests := []struct {
		metric string
		shouldContain bool
	}{
		{"ffrtmp_job_input_bytes_total", true},
		{"ffrtmp_job_output_bytes_total", true},
		{"ffrtmp_job_last_input_bytes", true},
		{"ffrtmp_job_last_output_bytes", true},
		{"ffrtmp_job_last_bandwidth_mbps", true},
		{"ffrtmp_worker_bandwidth_utilization", true},
	}
	
	for _, tt := range tests {
		if strings.Contains(metricsOutput, tt.metric) != tt.shouldContain {
			if tt.shouldContain {
				t.Errorf("Expected metric %s to be present in output", tt.metric)
			} else {
				t.Errorf("Expected metric %s to NOT be present in output", tt.metric)
			}
		}
	}
	
	// Verify values are correct
	if !strings.Contains(metricsOutput, "ffrtmp_job_input_bytes_total{node_id=\"test-worker\"} 52428800") {
		t.Error("Input bytes total not correct")
	}
	
	if !strings.Contains(metricsOutput, "ffrtmp_job_output_bytes_total{node_id=\"test-worker\"} 41943040") {
		t.Error("Output bytes total not correct")
	}
	
	// Bandwidth should be calculated correctly
	// Total bytes: 52428800 + 41943040 = 94371840
	// Duration: 62.5s
	// Bytes per second: 94371840 / 62.5 = 1509949.44
	// Mbps: (1509949.44 * 8) / (1024 * 1024) ≈ 11.52 Mbps
	if !strings.Contains(metricsOutput, "ffrtmp_job_last_bandwidth_mbps{node_id=\"test-worker\"} 11.") {
		t.Error("Bandwidth calculation appears incorrect")
	}
}

func TestBandwidthMultipleJobs(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	
	// Record multiple jobs
	exporter.RecordJobBandwidth(10000000, 8000000, 10.0) // Job 1: 10MB + 8MB in 10s
	exporter.RecordJobBandwidth(20000000, 16000000, 15.0) // Job 2: 20MB + 16MB in 15s
	
	// Create HTTP test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	exporter.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)
	
	// Total input should be cumulative
	if !strings.Contains(metricsOutput, "ffrtmp_job_input_bytes_total{node_id=\"test-worker\"} 30000000") {
		t.Error("Cumulative input bytes incorrect")
	}
	
	// Total output should be cumulative
	if !strings.Contains(metricsOutput, "ffrtmp_job_output_bytes_total{node_id=\"test-worker\"} 24000000") {
		t.Error("Cumulative output bytes incorrect")
	}
	
	// Last job values should reflect most recent job
	if !strings.Contains(metricsOutput, "ffrtmp_job_last_input_bytes{node_id=\"test-worker\"} 20000000") {
		t.Error("Last job input bytes incorrect")
	}
}

func TestBandwidthZeroDuration(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	
	// Record with zero duration (should not crash)
	exporter.RecordJobBandwidth(10000000, 8000000, 0)
	
	// Create HTTP test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	exporter.ServeHTTP(w, req)
	
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)
	
	// Bandwidth should be 0 for zero duration
	if !strings.Contains(metricsOutput, "ffrtmp_job_last_bandwidth_mbps{node_id=\"test-worker\"} 0.00") {
		t.Error("Zero duration should result in 0 bandwidth")
	}
}

func TestGetTotalBandwidthBytes(t *testing.T) {
	exporter := NewWorkerExporter("test-worker", false)
	
	// Initially should be 0
	if total := exporter.GetTotalBandwidthBytes(); total != 0 {
		t.Errorf("Initial total should be 0, got %d", total)
	}
	
	// Record bandwidth
	exporter.RecordJobBandwidth(10000000, 8000000, 10.0)
	
	// Should now be sum of input + output
	expected := int64(18000000)
	if total := exporter.GetTotalBandwidthBytes(); total != expected {
		t.Errorf("Expected total %d, got %d", expected, total)
	}
	
	// Add another job
	exporter.RecordJobBandwidth(5000000, 3000000, 5.0)
	
	// Should be cumulative
	expected = int64(26000000)
	if total := exporter.GetTotalBandwidthBytes(); total != expected {
		t.Errorf("Expected cumulative total %d, got %d", expected, total)
	}
}
