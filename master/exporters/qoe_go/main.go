package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	defaultPort = 9503
)

// TestResults holds test result data
type TestResults struct {
	Scenarios []Scenario `json:"scenarios"`
}

// Scenario represents a single test scenario with QoE metrics
type Scenario struct {
	Name              string  `json:"name"`
	Duration          float64 `json:"duration"`
	Bitrate           string  `json:"bitrate"`
	EncoderType       string  `json:"encoder_type"`
	VMafScore         float64 `json:"vmaf_score,omitempty"`
	PSNRScore         float64 `json:"psnr_score,omitempty"`
	SSIMScore         float64 `json:"ssim_score,omitempty"`
	AvgFPS            float64 `json:"avg_fps,omitempty"`
	DroppedFrames     int     `json:"dropped_frames,omitempty"`
	TotalFrames       int     `json:"total_frames,omitempty"`
	PowerWatts        []float64 `json:"power_watts,omitempty"`
	CPUUsageCores     []float64 `json:"cpu_usage_cores,omitempty"`
	Resolution        string  `json:"resolution,omitempty"`
	PixelsProcessed   int64   `json:"pixels_processed,omitempty"`
}

var (
	resultsData = struct {
		sync.RWMutex
		scenarios []Scenario
		lastLoad  time.Time
	}{}
	resultsDir string
	cacheTTL   = 60 * time.Second
)

// loadLatestResults loads the most recent test results JSON file
func loadLatestResults() error {
	resultsData.Lock()
	defer resultsData.Unlock()

	// Check cache
	if time.Since(resultsData.lastLoad) < cacheTTL && len(resultsData.scenarios) > 0 {
		return nil
	}

	// Find most recent results file
	pattern := filepath.Join(resultsDir, "test_results_*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find results files: %w", err)
	}

	if len(matches) == 0 {
		log.Printf("No test results found in %s", resultsDir)
		return nil
	}

	// Get most recent file
	var latestFile string
	var latestTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if latestFile == "" || info.ModTime().After(latestTime) {
			latestFile = match
			latestTime = info.ModTime()
		}
	}

	// Load the file
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return fmt.Errorf("failed to read results file %s: %w", latestFile, err)
	}

	var results TestResults
	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("failed to parse results file %s: %w", latestFile, err)
	}

	resultsData.scenarios = results.Scenarios
	resultsData.lastLoad = time.Now()

	log.Printf("Loaded %d scenarios from %s", len(results.Scenarios), filepath.Base(latestFile))
	return nil
}

// calculateQualityPerWatt calculates quality per watt efficiency
func calculateQualityPerWatt(vmaf float64, powerWatts []float64) float64 {
	if len(powerWatts) == 0 || vmaf == 0 {
		return 0
	}

	// Calculate average power
	var sum float64
	for _, p := range powerWatts {
		sum += p
	}
	avgPower := sum / float64(len(powerWatts))

	if avgPower == 0 {
		return 0
	}

	return vmaf / avgPower
}

// calculateQoEEfficiency calculates QoE efficiency score
func calculateQoEEfficiency(scenario Scenario) float64 {
	if len(scenario.PowerWatts) == 0 || scenario.VMafScore == 0 || scenario.PixelsProcessed == 0 || scenario.Duration == 0 {
		return 0
	}

	// Calculate total energy in joules
	var totalEnergy float64
	stepSeconds := 5.0 // Assume 5 second intervals
	for _, power := range scenario.PowerWatts {
		totalEnergy += power * stepSeconds
	}

	if totalEnergy == 0 {
		return 0
	}

	// QoE efficiency = (quality * pixels) / energy
	// This gives us "quality-weighted pixels per joule"
	qualityWeight := scenario.VMafScore / 100.0 // Normalize VMAF to 0-1
	return (qualityWeight * float64(scenario.PixelsProcessed)) / totalEnergy
}

// metricsHandler handles the /metrics endpoint
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Load latest results
	if err := loadLatestResults(); err != nil {
		log.Printf("Error loading results: %v", err)
	}

	resultsData.RLock()
	defer resultsData.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Exporter metadata
	fmt.Fprintln(w, "# HELP qoe_exporter_up QoE exporter is running")
	fmt.Fprintln(w, "# TYPE qoe_exporter_up gauge")
	fmt.Fprintln(w, "qoe_exporter_up 1")

	// Scenarios count
	fmt.Fprintln(w, "# HELP qoe_scenarios_total Total number of scenarios loaded")
	fmt.Fprintln(w, "# TYPE qoe_scenarios_total gauge")
	fmt.Fprintf(w, "qoe_scenarios_total %d\n", len(resultsData.scenarios))

	// Export QoE metrics for each scenario
	if len(resultsData.scenarios) > 0 {
		// VMAF Score
		fmt.Fprintln(w, "# HELP qoe_vmaf_score VMAF quality score (0-100)")
		fmt.Fprintln(w, "# TYPE qoe_vmaf_score gauge")

		for _, scenario := range resultsData.scenarios {
			if scenario.VMafScore > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "qoe_vmaf_score{%s} %.2f\n", labels, scenario.VMafScore)
			}
		}

		// PSNR Score
		fmt.Fprintln(w, "# HELP qoe_psnr_score PSNR quality score (dB)")
		fmt.Fprintln(w, "# TYPE qoe_psnr_score gauge")

		for _, scenario := range resultsData.scenarios {
			if scenario.PSNRScore > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "qoe_psnr_score{%s} %.2f\n", labels, scenario.PSNRScore)
			}
		}

		// SSIM Score
		fmt.Fprintln(w, "# HELP qoe_ssim_score SSIM quality score (0-1)")
		fmt.Fprintln(w, "# TYPE qoe_ssim_score gauge")

		for _, scenario := range resultsData.scenarios {
			if scenario.SSIMScore > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "qoe_ssim_score{%s} %.4f\n", labels, scenario.SSIMScore)
			}
		}

		// Quality per Watt
		fmt.Fprintln(w, "# HELP qoe_quality_per_watt Quality per watt efficiency (VMAF/Watt)")
		fmt.Fprintln(w, "# TYPE qoe_quality_per_watt gauge")

		for _, scenario := range resultsData.scenarios {
			qpw := calculateQualityPerWatt(scenario.VMafScore, scenario.PowerWatts)
			if qpw > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "qoe_quality_per_watt{%s} %.4f\n", labels, qpw)
			}
		}

		// QoE Efficiency Score
		fmt.Fprintln(w, "# HELP qoe_efficiency_score QoE efficiency score (quality-weighted pixels per joule)")
		fmt.Fprintln(w, "# TYPE qoe_efficiency_score gauge")

		for _, scenario := range resultsData.scenarios {
			efficiency := calculateQoEEfficiency(scenario)
			if efficiency > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "qoe_efficiency_score{%s} %.2f\n", labels, efficiency)
			}
		}

		// Drop rate
		fmt.Fprintln(w, "# HELP qoe_drop_rate Frame drop rate (dropped/total)")
		fmt.Fprintln(w, "# TYPE qoe_drop_rate gauge")

		for _, scenario := range resultsData.scenarios {
			if scenario.TotalFrames > 0 {
				dropRate := float64(scenario.DroppedFrames) / float64(scenario.TotalFrames)
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "qoe_drop_rate{%s} %.6f\n", labels, dropRate)
			}
		}
	}

	// Exporter info
	fmt.Fprintln(w, "# HELP qoe_exporter_info QoE exporter information")
	fmt.Fprintln(w, "# TYPE qoe_exporter_info gauge")
	fmt.Fprintln(w, "qoe_exporter_info{version=\"1.0.0\",language=\"go\"} 1")
}

// healthHandler handles the /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	resultsPath := flag.String("results-dir", "./test_results", "Directory containing test results")
	flag.Parse()

	// Override with environment variable if set
	if envPort := os.Getenv("QOE_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	if envResultsDir := os.Getenv("RESULTS_DIR"); envResultsDir != "" {
		*resultsPath = envResultsDir
	}

	resultsDir = *resultsPath

	log.Println("Starting QoE Exporter (Go)")
	log.Printf("Results directory: %s", resultsDir)

	// Check if results directory exists
	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		log.Printf("Warning: Results directory %s does not exist", resultsDir)
	}

	// Load initial results
	if err := loadLatestResults(); err != nil {
		log.Printf("Warning: Initial results load failed: %v", err)
	}

	// Register HTTP handlers
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/health", healthHandler)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("Metrics endpoint: http://0.0.0.0:%d/metrics", *port)
	log.Printf("Health endpoint: http://0.0.0.0:%d/health", *port)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
