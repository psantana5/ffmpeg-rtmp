package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort = 9501
)

// DockerStats holds current docker statistics
type DockerStats struct {
	mu             sync.RWMutex
	ContainerStats map[string]*ContainerStat
	TotalCPUCores  int
	LastUpdate     time.Time
}

// ContainerStat holds statistics for a single container
type ContainerStat struct {
	Name          string
	ID            string
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsage   string
	NetworkIO     string
	BlockIO       string
}

// DockerStatsJSON represents the JSON output from docker stats
type DockerStatsJSON struct {
	Name       string `json:"Name"`
	Container  string `json:"Container"`
	CPUPerc    string `json:"CPUPerc"`
	MemPerc    string `json:"MemPerc"`
	MemUsage   string `json:"MemUsage"`
	NetIO      string `json:"NetIO"`
	BlockIO    string `json:"BlockIO"`
}

var (
	stats = &DockerStats{ContainerStats: make(map[string]*ContainerStat)}
)

// getCPUCores gets the number of CPU cores from /proc/cpuinfo
func getCPUCores() int {
	data, err := os.ReadFile("/host/proc/cpuinfo")
	if err != nil {
		data, err = os.ReadFile("/proc/cpuinfo")
		if err != nil {
			return 4 // Default fallback
		}
	}

	cores := 0
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}

	if cores == 0 {
		return 4 // Default fallback
	}
	return cores
}

// parsePercentage parses a percentage string like "12.34%" and returns the float value
func parsePercentage(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}

// updateDockerStats updates statistics from Docker CLI
func updateDockerStats() error {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	// Run docker stats command
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{json .}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run docker stats: %w", err)
	}

	// Parse output
	scanner := strings.Split(out.String(), "\n")
	for _, line := range scanner {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var dockerStat DockerStatsJSON
		if err := json.Unmarshal([]byte(line), &dockerStat); err != nil {
			log.Printf("Failed to parse docker stats line: %v", err)
			continue
		}

		// Parse container ID (first 12 chars)
		containerID := dockerStat.Container
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}

		// Store stats
		stats.ContainerStats[dockerStat.Container] = &ContainerStat{
			Name:          dockerStat.Name,
			ID:            containerID,
			CPUPercent:    parsePercentage(dockerStat.CPUPerc),
			MemoryPercent: parsePercentage(dockerStat.MemPerc),
			MemoryUsage:   dockerStat.MemUsage,
			NetworkIO:     dockerStat.NetIO,
			BlockIO:       dockerStat.BlockIO,
		}
	}

	stats.LastUpdate = time.Now()
	return nil
}

// collectStats periodically collects Docker statistics
func collectStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := updateDockerStats(); err != nil {
			log.Printf("Error updating stats: %v", err)
		}
	}
}

// metricsHandler handles the /metrics endpoint
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Exporter metadata
	fmt.Fprintln(w, "# HELP docker_stats_exporter_up Docker stats exporter is running")
	fmt.Fprintln(w, "# TYPE docker_stats_exporter_up gauge")
	fmt.Fprintln(w, "docker_stats_exporter_up 1")

	// Container count
	fmt.Fprintln(w, "# HELP docker_containers_total Total number of containers being monitored")
	fmt.Fprintln(w, "# TYPE docker_containers_total gauge")
	fmt.Fprintf(w, "docker_containers_total %d\n", len(stats.ContainerStats))

	// Container CPU metrics
	fmt.Fprintln(w, "# HELP docker_container_cpu_percent Container CPU usage percentage")
	fmt.Fprintln(w, "# TYPE docker_container_cpu_percent gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_cpu_percent{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.CPUPercent)
	}

	// Container memory metrics
	fmt.Fprintln(w, "# HELP docker_container_memory_percent Container memory usage percentage")
	fmt.Fprintln(w, "# TYPE docker_container_memory_percent gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_memory_percent{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.MemoryPercent)
	}

	fmt.Fprintln(w, "# HELP docker_container_memory_usage Container memory usage")
	fmt.Fprintln(w, "# TYPE docker_container_memory_usage gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_memory_usage{container=\"%s\",id=\"%s\",value=\"%s\"} 1\n",
			cs.Name, cs.ID, cs.MemoryUsage)
	}

	// Network I/O metrics
	fmt.Fprintln(w, "# HELP docker_container_network_io Container network I/O")
	fmt.Fprintln(w, "# TYPE docker_container_network_io gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_network_io{container=\"%s\",id=\"%s\",value=\"%s\"} 1\n",
			cs.Name, cs.ID, cs.NetworkIO)
	}

	// Block I/O metrics
	fmt.Fprintln(w, "# HELP docker_container_block_io Container block I/O")
	fmt.Fprintln(w, "# TYPE docker_container_block_io gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_block_io{container=\"%s\",id=\"%s\",value=\"%s\"} 1\n",
			cs.Name, cs.ID, cs.BlockIO)
	}

	// Exporter metadata
	fmt.Fprintln(w, "# HELP docker_stats_exporter_info Docker stats exporter information")
	fmt.Fprintln(w, "# TYPE docker_stats_exporter_info gauge")
	fmt.Fprintln(w, "docker_stats_exporter_info{version=\"1.0.0\",language=\"go\"} 1")
}

// healthHandler handles the /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	flag.Parse()

	// Override with environment variable if set
	if envPort := os.Getenv("DOCKER_STATS_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	log.Println("Starting Docker Stats Exporter (Go)")

	// Get CPU cores
	stats.TotalCPUCores = getCPUCores()
	log.Printf("Detected %d CPU cores", stats.TotalCPUCores)

	// Start stats collection
	go collectStats(5 * time.Second)

	// Do initial collection
	if err := updateDockerStats(); err != nil {
		log.Printf("Warning: Initial stats collection failed: %v", err)
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
