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
	"strings"
	"sync"
	"time"
)

const (
	defaultPort = 9504
)

// Exchange rates relative to USD (updated periodically)
var exchangeRates = map[string]float64{
	"USD": 1.0,
	"EUR": 0.92,  // 1 USD = 0.92 EUR
	"SEK": 10.35, // 1 USD = 10.35 SEK
}

// TestResults holds test result data
type TestResults struct {
	Scenarios []Scenario `json:"scenarios"`
}

// Scenario represents a single test scenario with cost metrics
type Scenario struct {
	Name              string    `json:"name"`
	Duration          float64   `json:"duration"`
	Bitrate           string    `json:"bitrate"`
	EncoderType       string    `json:"encoder_type"`
	VMafScore         float64   `json:"vmaf_score,omitempty"`
	PowerWatts        []float64 `json:"power_watts,omitempty"`
	CPUUsageCores     []float64 `json:"cpu_usage_cores,omitempty"`
	StepSeconds       float64   `json:"step_seconds,omitempty"`
	PixelsProcessed   int64     `json:"pixels_processed,omitempty"`
}

// CostConfig holds cost calculation configuration
type CostConfig struct {
	EnergyCostPerKWh float64
	CPUCostPerHour   float64
	Currency         string
	Region           string
}

var (
	resultsData = struct {
		sync.RWMutex
		scenarios []Scenario
		lastLoad  time.Time
	}{}
	resultsDir string
	costConfig CostConfig
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

// calculateEnergyCost calculates energy cost using trapezoidal integration
func calculateEnergyCost(powerWatts []float64, stepSeconds float64, costPerKWh float64) float64 {
	if len(powerWatts) == 0 || stepSeconds == 0 {
		return 0
	}

	// Use trapezoidal rule for integration
	totalEnergyJoules := 0.0
	for i := 0; i < len(powerWatts)-1; i++ {
		// Trapezoidal rule: (f(x) + f(x+h)) * h / 2
		avgPower := (powerWatts[i] + powerWatts[i+1]) / 2.0
		totalEnergyJoules += avgPower * stepSeconds
	}

	// Convert joules to kWh: 1 kWh = 3,600,000 joules
	energyKWh := totalEnergyJoules / 3600000.0

	return energyKWh * costPerKWh
}

// calculateComputeCost calculates compute cost using trapezoidal integration
func calculateComputeCost(cpuUsageCores []float64, stepSeconds float64, costPerCoreHour float64) float64 {
	if len(cpuUsageCores) == 0 || stepSeconds == 0 {
		return 0
	}

	// Use trapezoidal rule for integration
	totalCoreSeconds := 0.0
	for i := 0; i < len(cpuUsageCores)-1; i++ {
		avgCores := (cpuUsageCores[i] + cpuUsageCores[i+1]) / 2.0
		totalCoreSeconds += avgCores * stepSeconds
	}

	// Convert core-seconds to core-hours
	coreHours := totalCoreSeconds / 3600.0

	return coreHours * costPerCoreHour
}

// extractStreamCount extracts the number of concurrent streams from scenario name
func extractStreamCount(scenarioName string) int {
	// Look for patterns like "2 streams", "4 Streams", etc.
	lower := strings.ToLower(scenarioName)
	words := strings.Fields(lower)

	for i, word := range words {
		if strings.Contains(word, "stream") && i > 0 {
			if count, err := strconv.Atoi(words[i-1]); err == nil {
				return count
			}
		}
	}
	return 1 // Default to 1 stream
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
	fmt.Fprintln(w, "# HELP cost_exporter_alive Cost exporter health check (always 1)")
	fmt.Fprintln(w, "# TYPE cost_exporter_alive gauge")
	fmt.Fprintln(w, "cost_exporter_alive 1")

	// Scenarios count
	fmt.Fprintln(w, "# HELP cost_scenarios_total Total number of scenarios loaded")
	fmt.Fprintln(w, "# TYPE cost_scenarios_total gauge")
	fmt.Fprintf(w, "cost_scenarios_total %d\n", len(resultsData.scenarios))

	// Export cost metrics for each scenario
	if len(resultsData.scenarios) > 0 {
		// Total cost (load-aware)
		fmt.Fprintln(w, "# HELP cost_total_load_aware Total cost - load-aware")
		fmt.Fprintln(w, "# TYPE cost_total_load_aware gauge")

		// Energy cost (load-aware)
		fmt.Fprintln(w, "# HELP cost_energy_load_aware Energy cost - load-aware")
		fmt.Fprintln(w, "# TYPE cost_energy_load_aware gauge")

		// Compute cost (load-aware)
		fmt.Fprintln(w, "# HELP cost_compute_load_aware Compute cost - load-aware")
		fmt.Fprintln(w, "# TYPE cost_compute_load_aware gauge")

		// Cost per pixel
		fmt.Fprintln(w, "# HELP cost_per_pixel Cost per pixel - load-aware")
		fmt.Fprintln(w, "# TYPE cost_per_pixel gauge")

		// Cost per watch hour
		fmt.Fprintln(w, "# HELP cost_per_watch_hour Cost per viewer watch hour - load-aware")
		fmt.Fprintln(w, "# TYPE cost_per_watch_hour gauge")

		for _, scenario := range resultsData.scenarios {
			// Skip if missing required data
			if scenario.Bitrate == "" {
				continue
			}

			streamCount := extractStreamCount(scenario.Name)
			stepSeconds := scenario.StepSeconds
			if stepSeconds == 0 {
				stepSeconds = 5.0 // Default
			}

			// Calculate costs if data is available (in USD first)
			energyCostUSD := 0.0
			computeCostUSD := 0.0
			hasData := len(scenario.PowerWatts) > 0 && len(scenario.CPUUsageCores) > 0

			if hasData {
				energyCostUSD = calculateEnergyCost(scenario.PowerWatts, stepSeconds, costConfig.EnergyCostPerKWh)
				computeCostUSD = calculateComputeCost(scenario.CPUUsageCores, stepSeconds, costConfig.CPUCostPerHour)
			}

			totalCostUSD := energyCostUSD + computeCostUSD

			// Export metrics for each currency
			for currency, rate := range exchangeRates {
				energyCost := energyCostUSD * rate
				computeCost := computeCostUSD * rate
				totalCost := totalCostUSD * rate

				labels := fmt.Sprintf("scenario=\"%s\",currency=\"%s\",streams=\"%d\",bitrate=\"%s\",encoder=\"%s\",service=\"cost-analysis\"",
					scenario.Name, currency, streamCount, scenario.Bitrate, scenario.EncoderType)

				// Export metrics
				fmt.Fprintf(w, "cost_total_load_aware{%s} %.8f\n", labels, totalCost)
				fmt.Fprintf(w, "cost_energy_load_aware{%s} %.8f\n", labels, energyCost)
				fmt.Fprintf(w, "cost_compute_load_aware{%s} %.8f\n", labels, computeCost)

				// Cost per pixel (if pixel data available)
				if scenario.PixelsProcessed > 0 && totalCost > 0 {
					costPerPixel := totalCost / float64(scenario.PixelsProcessed)
					fmt.Fprintf(w, "cost_per_pixel{%s} %.12f\n", labels, costPerPixel)
				} else {
					fmt.Fprintf(w, "cost_per_pixel{%s} 0\n", labels)
				}

				// Cost per watch hour (if duration available)
				if scenario.Duration > 0 && totalCost > 0 {
					// Cost per viewer watch hour = total_cost / (duration_hours * stream_count)
					durationHours := scenario.Duration / 3600.0
					watchHours := durationHours * float64(streamCount)
					if watchHours > 0 {
						costPerWatchHour := totalCost / watchHours
						fmt.Fprintf(w, "cost_per_watch_hour{%s} %.8f\n", labels, costPerWatchHour)
					} else {
						fmt.Fprintf(w, "cost_per_watch_hour{%s} 0\n", labels)
					}
				} else {
					fmt.Fprintf(w, "cost_per_watch_hour{%s} 0\n", labels)
				}
			}
		}
	}

	// Exporter info
	fmt.Fprintln(w, "# HELP cost_exporter_info Cost exporter information")
	fmt.Fprintln(w, "# TYPE cost_exporter_info gauge")
	fmt.Fprintf(w, "cost_exporter_info{version=\"1.0.0\",language=\"go\",region=\"%s\"} 1\n", costConfig.Region)
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
	energyCost := flag.Float64("energy-cost", 0.0, "Energy cost per kWh")
	cpuCost := flag.Float64("cpu-cost", 0.50, "CPU cost per hour")
	currency := flag.String("currency", "USD", "Currency code")
	region := flag.String("region", "us-east-1", "AWS region code")
	flag.Parse()

	// Override with environment variables if set
	if envPort := os.Getenv("COST_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	if envResultsDir := os.Getenv("RESULTS_DIR"); envResultsDir != "" {
		*resultsPath = envResultsDir
	}

	if envEnergyCost := os.Getenv("ENERGY_COST"); envEnergyCost != "" {
		if ec, err := strconv.ParseFloat(envEnergyCost, 64); err == nil {
			*energyCost = ec
		}
	}

	if envCPUCost := os.Getenv("CPU_COST"); envCPUCost != "" {
		if cc, err := strconv.ParseFloat(envCPUCost, 64); err == nil {
			*cpuCost = cc
		}
	}

	if envCurrency := os.Getenv("CURRENCY"); envCurrency != "" {
		*currency = envCurrency
	}

	if envRegion := os.Getenv("REGION"); envRegion != "" {
		*region = envRegion
	}

	resultsDir = *resultsPath
	costConfig = CostConfig{
		EnergyCostPerKWh: *energyCost,
		CPUCostPerHour:   *cpuCost,
		Currency:         *currency,
		Region:           *region,
	}

	log.Println("Starting Cost Exporter (Go)")
	log.Printf("Results directory: %s", resultsDir)
	log.Printf("Energy cost: %.4f %s/kWh", costConfig.EnergyCostPerKWh, costConfig.Currency)
	log.Printf("CPU cost: %.2f %s/hour", costConfig.CPUCostPerHour, costConfig.Currency)
	log.Printf("Region: %s", costConfig.Region)

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
