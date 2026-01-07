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
	defaultPort = 9502
)

// TestResults holds test result data
type TestResults struct {
	Scenarios []Scenario `json:"scenarios"`
}

// Scenario represents a single test scenario
type Scenario struct {
	Name          string  `json:"name"`
	Duration      float64 `json:"duration"`
	Bitrate       string  `json:"bitrate"`
	EncoderType   string  `json:"encoder_type"`
	VMafScore     float64 `json:"vmaf_score,omitempty"`
	PSNRScore     float64 `json:"psnr_score,omitempty"`
	AvgFPS        float64 `json:"avg_fps,omitempty"`
	DroppedFrames int     `json:"dropped_frames,omitempty"`
	TotalFrames   int     `json:"total_frames,omitempty"`
	StartTime     float64 `json:"start_time,omitempty"`
	EndTime       float64 `json:"end_time,omitempty"`
}

var (
	resultsData = struct {
		sync.RWMutex
		scenarios []Scenario
		lastLoad  time.Time
	}{}
	resultsDir string
	cacheTTL   = 10 * time.Second // Fast refresh for real-time dashboards
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

	// Get most recent file (assumes lexicographic sorting works with timestamp format)
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
	fmt.Fprintln(w, "# HELP results_exporter_up Results exporter is running")
	fmt.Fprintln(w, "# TYPE results_exporter_up gauge")
	fmt.Fprintln(w, "results_exporter_up 1")

	// Scenarios count
	fmt.Fprintln(w, "# HELP results_scenarios_total Total number of scenarios loaded")
	fmt.Fprintln(w, "# TYPE results_scenarios_total gauge")
	fmt.Fprintf(w, "results_scenarios_total %d\n", len(resultsData.scenarios))

	// Export metrics for each scenario
	if len(resultsData.scenarios) > 0 {
		// Duration
		fmt.Fprintln(w, "# HELP results_scenario_duration_seconds Test scenario duration in seconds")
		fmt.Fprintln(w, "# TYPE results_scenario_duration_seconds gauge")

		for _, scenario := range resultsData.scenarios {
			labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
				scenario.Name, scenario.Bitrate, scenario.EncoderType)
			fmt.Fprintf(w, "results_scenario_duration_seconds{%s} %.2f\n", labels, scenario.Duration)
		}

		// FPS
		fmt.Fprintln(w, "# HELP results_scenario_avg_fps Average frames per second")
		fmt.Fprintln(w, "# TYPE results_scenario_avg_fps gauge")

		for _, scenario := range resultsData.scenarios {
			if scenario.AvgFPS > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "results_scenario_avg_fps{%s} %.2f\n", labels, scenario.AvgFPS)
			}
		}

		// Dropped frames
		fmt.Fprintln(w, "# HELP results_scenario_dropped_frames Total dropped frames")
		fmt.Fprintln(w, "# TYPE results_scenario_dropped_frames counter")

		for _, scenario := range resultsData.scenarios {
			if scenario.TotalFrames > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "results_scenario_dropped_frames{%s} %d\n", labels, scenario.DroppedFrames)
			}
		}

		// Total frames
		fmt.Fprintln(w, "# HELP results_scenario_total_frames Total processed frames")
		fmt.Fprintln(w, "# TYPE results_scenario_total_frames counter")

		for _, scenario := range resultsData.scenarios {
			if scenario.TotalFrames > 0 {
				labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
					scenario.Name, scenario.Bitrate, scenario.EncoderType)
				fmt.Fprintf(w, "results_scenario_total_frames{%s} %d\n", labels, scenario.TotalFrames)
			}
		}

		// VMAF Score (if available)
		hasVmaf := false
		for _, scenario := range resultsData.scenarios {
			if scenario.VMafScore > 0 {
				hasVmaf = true
				break
			}
		}

		if hasVmaf {
			fmt.Fprintln(w, "# HELP results_scenario_vmaf_score VMAF quality score (0-100)")
			fmt.Fprintln(w, "# TYPE results_scenario_vmaf_score gauge")

			for _, scenario := range resultsData.scenarios {
				if scenario.VMafScore > 0 {
					labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
						scenario.Name, scenario.Bitrate, scenario.EncoderType)
					fmt.Fprintf(w, "results_scenario_vmaf_score{%s} %.2f\n", labels, scenario.VMafScore)
				}
			}
		}

		// PSNR Score (if available)
		hasPsnr := false
		for _, scenario := range resultsData.scenarios {
			if scenario.PSNRScore > 0 {
				hasPsnr = true
				break
			}
		}

		if hasPsnr {
			fmt.Fprintln(w, "# HELP results_scenario_psnr_score PSNR quality score (dB)")
			fmt.Fprintln(w, "# TYPE results_scenario_psnr_score gauge")

			for _, scenario := range resultsData.scenarios {
				if scenario.PSNRScore > 0 {
					labels := fmt.Sprintf("scenario=\"%s\",bitrate=\"%s\",encoder=\"%s\"",
						scenario.Name, scenario.Bitrate, scenario.EncoderType)
					fmt.Fprintf(w, "results_scenario_psnr_score{%s} %.2f\n", labels, scenario.PSNRScore)
				}
			}
		}
	}

	// Exporter info
	fmt.Fprintln(w, "# HELP results_exporter_info Results exporter information")
	fmt.Fprintln(w, "# TYPE results_exporter_info gauge")
	fmt.Fprintln(w, "results_exporter_info{version=\"1.0.0\",language=\"go\"} 1")
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
	if envPort := os.Getenv("RESULTS_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	if envResultsDir := os.Getenv("RESULTS_DIR"); envResultsDir != "" {
		*resultsPath = envResultsDir
	}

	resultsDir = *resultsPath

	log.Println("Starting Results Exporter (Go)")
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
