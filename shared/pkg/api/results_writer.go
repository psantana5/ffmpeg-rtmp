package api

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// ResultsWriter writes job results to JSON files for exporters
type ResultsWriter struct {
	outputDir string
}

// NewResultsWriter creates a new results writer
func NewResultsWriter(outputDir string) *ResultsWriter {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: Failed to create results directory %s: %v", outputDir, err)
	}
	return &ResultsWriter{
		outputDir: outputDir,
	}
}

// WriteJobResult writes a completed job result to a JSON file
func (w *ResultsWriter) WriteJobResult(job *models.Job, result *models.JobResult) error {
	if w.outputDir == "" {
		return nil // Skip if no output directory configured
	}

	// Create scenario structure matching what exporters expect
	scenario := map[string]interface{}{
		"name":         job.Scenario,
		"duration":     0.0,
		"bitrate":      "0k",
		"encoder_type": job.Engine,
		"start_time":   job.CreatedAt.Unix(),
	}

	// Add duration if job completed
	if job.CompletedAt != nil && job.StartedAt != nil {
		scenario["duration"] = job.CompletedAt.Sub(*job.StartedAt).Seconds()
		scenario["end_time"] = job.CompletedAt.Unix()
	}

	// Extract metrics from result
	if result.Metrics != nil {
		// Map common metrics
		if val, ok := result.Metrics["bitrate"]; ok {
			scenario["bitrate"] = val
		}
		if val, ok := result.Metrics["fps"]; ok {
			scenario["avg_fps"] = val
		}
		if val, ok := result.Metrics["frames"]; ok {
			scenario["total_frames"] = val
		}
		if val, ok := result.Metrics["dropped_frames"]; ok {
			scenario["dropped_frames"] = val
		}
		if val, ok := result.Metrics["resolution"]; ok {
			scenario["resolution"] = val
		}
		if val, ok := result.Metrics["pixels_processed"]; ok {
			scenario["pixels_processed"] = val
		}
	}

	// Add QoE metrics
	if result.VMAFScore > 0 {
		scenario["vmaf_score"] = result.VMAFScore
	}
	if result.QoEScore > 0 {
		scenario["qoe_score"] = result.QoEScore
	}
	if result.EfficiencyScore > 0 {
		scenario["efficiency_score"] = result.EfficiencyScore
	}
	if result.EnergyJoules > 0 {
		scenario["energy_joules"] = result.EnergyJoules
		// Convert to power samples (estimate)
		if duration, ok := scenario["duration"].(float64); ok && duration > 0 {
			avgPower := result.EnergyJoules / duration
			scenario["power_watts"] = []float64{avgPower}
			scenario["step_seconds"] = duration
		}
	}

	// Read existing file or create new
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("test_results_%s.json", timestamp)
	resultFilepath := filepath.Join(w.outputDir, filename)

	// Check if a recent file exists (within last hour)
	existingFile := w.findRecentFile()
	var scenarios []map[string]interface{}

	if existingFile != "" {
		// Append to existing file
		data, err := os.ReadFile(existingFile)
		if err == nil {
			var existing map[string]interface{}
			if err := json.Unmarshal(data, &existing); err == nil {
				if existingScenarios, ok := existing["scenarios"].([]interface{}); ok {
					for _, s := range existingScenarios {
						if sm, ok := s.(map[string]interface{}); ok {
							scenarios = append(scenarios, sm)
						}
					}
				}
			}
		}
		resultFilepath = existingFile
	}

	scenarios = append(scenarios, scenario)

	// Write file
	output := map[string]interface{}{
		"test_date":      time.Now().Format(time.RFC3339),
		"test_metadata": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"platform":  "distributed",
			"source":    "scheduler",
		},
		"scenarios": scenarios,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(resultFilepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write results file: %w", err)
	}

	log.Printf("Job results written to %s", resultFilepath)
	return nil
}

// findRecentFile finds a results file created within the last hour
func (w *ResultsWriter) findRecentFile() string {
	pattern := filepath.Join(w.outputDir, "test_results_*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	// Find most recent file within last hour
	cutoff := time.Now().Add(-1 * time.Hour)
	var mostRecent string
	var mostRecentTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) && info.ModTime().After(mostRecentTime) {
			mostRecent = match
			mostRecentTime = info.ModTime()
		}
	}

	return mostRecent
}
