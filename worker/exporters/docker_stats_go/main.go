package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const (
	defaultPort = 9501
)

// DockerStats holds current docker statistics
type DockerStats struct {
	mu                 sync.RWMutex
	EngineStats        *EngineStats
	ContainerStats     map[string]*ContainerStat
	TotalCPUCores      int
	LastUpdate         time.Time
}

// EngineStats holds Docker engine (dockerd) statistics
type EngineStats struct {
	CPUPercent    float64
	MemoryPercent float64
	MemoryKB      float64
}

// ContainerStat holds statistics for a single container
type ContainerStat struct {
	Name          string
	ID            string
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsageMB float64
	MemoryLimitMB float64
	NetworkRxMB   float64
	NetworkTxMB   float64
	BlockReadMB   float64
	BlockWriteMB  float64
}

var (
	stats        = &DockerStats{ContainerStats: make(map[string]*ContainerStat)}
	dockerClient *client.Client
)

// getCPUCores gets the number of CPU cores from /proc/cpuinfo
func getCPUCores() int {
	data, err := os.ReadFile("/host/proc/cpuinfo")
	if err != nil {
		// Fallback to environment-based detection
		return 4
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

// updateDockerStats updates statistics from Docker API
func updateDockerStats(ctx context.Context) error {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	// Get container list
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Update each container's stats
	for _, cont := range containers {
		// Get container stats
		statsResp, err := dockerClient.ContainerStats(ctx, cont.ID, false)
		if err != nil {
			log.Printf("Failed to get stats for container %s: %v", cont.Names[0], err)
			continue
		}

		var containerStats types.StatsJSON
		if err := json.NewDecoder(statsResp.Body).Decode(&containerStats); err != nil {
			statsResp.Body.Close()
			log.Printf("Failed to decode stats for container %s: %v", cont.Names[0], err)
			continue
		}
		statsResp.Body.Close()

		// Calculate CPU percentage
		cpuPercent := 0.0
		cpuDelta := float64(containerStats.CPUStats.CPUUsage.TotalUsage - containerStats.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(containerStats.CPUStats.SystemUsage - containerStats.PreCPUStats.SystemUsage)
		if systemDelta > 0.0 && cpuDelta > 0.0 {
			cpuPercent = (cpuDelta / systemDelta) * float64(len(containerStats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
		}

		// Calculate memory usage
		memUsage := float64(containerStats.MemoryStats.Usage) / (1024 * 1024) // MB
		memLimit := float64(containerStats.MemoryStats.Limit) / (1024 * 1024)  // MB
		memPercent := 0.0
		if memLimit > 0 {
			memPercent = (memUsage / memLimit) * 100.0
		}

		// Calculate network I/O
		networkRx := 0.0
		networkTx := 0.0
		for _, network := range containerStats.Networks {
			networkRx += float64(network.RxBytes) / (1024 * 1024) // MB
			networkTx += float64(network.TxBytes) / (1024 * 1024) // MB
		}

		// Calculate block I/O
		blockRead := 0.0
		blockWrite := 0.0
		for _, bioEntry := range containerStats.BlkioStats.IoServiceBytesRecursive {
			if bioEntry.Op == "Read" || bioEntry.Op == "read" {
				blockRead += float64(bioEntry.Value) / (1024 * 1024) // MB
			} else if bioEntry.Op == "Write" || bioEntry.Op == "write" {
				blockWrite += float64(bioEntry.Value) / (1024 * 1024) // MB
			}
		}

		// Get container name (remove leading slash)
		name := cont.Names[0]
		if strings.HasPrefix(name, "/") {
			name = name[1:]
		}

		// Store stats
		stats.ContainerStats[cont.ID] = &ContainerStat{
			Name:          name,
			ID:            cont.ID[:12],
			CPUPercent:    cpuPercent,
			MemoryPercent: memPercent,
			MemoryUsageMB: memUsage,
			MemoryLimitMB: memLimit,
			NetworkRxMB:   networkRx,
			NetworkTxMB:   networkTx,
			BlockReadMB:   blockRead,
			BlockWriteMB:  blockWrite,
		}
	}

	stats.LastUpdate = time.Now()
	return nil
}

// collectStats periodically collects Docker statistics
func collectStats(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := updateDockerStats(ctx); err != nil {
				log.Printf("Error updating stats: %v", err)
			}
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

	fmt.Fprintln(w, "# HELP docker_container_memory_usage_mb Container memory usage in MB")
	fmt.Fprintln(w, "# TYPE docker_container_memory_usage_mb gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_memory_usage_mb{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.MemoryUsageMB)
	}

	fmt.Fprintln(w, "# HELP docker_container_memory_limit_mb Container memory limit in MB")
	fmt.Fprintln(w, "# TYPE docker_container_memory_limit_mb gauge")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_memory_limit_mb{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.MemoryLimitMB)
	}

	// Network I/O metrics
	fmt.Fprintln(w, "# HELP docker_container_network_rx_mb Container network received in MB")
	fmt.Fprintln(w, "# TYPE docker_container_network_rx_mb counter")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_network_rx_mb{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.NetworkRxMB)
	}

	fmt.Fprintln(w, "# HELP docker_container_network_tx_mb Container network transmitted in MB")
	fmt.Fprintln(w, "# TYPE docker_container_network_tx_mb counter")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_network_tx_mb{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.NetworkTxMB)
	}

	// Block I/O metrics
	fmt.Fprintln(w, "# HELP docker_container_block_read_mb Container block read in MB")
	fmt.Fprintln(w, "# TYPE docker_container_block_read_mb counter")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_block_read_mb{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.BlockReadMB)
	}

	fmt.Fprintln(w, "# HELP docker_container_block_write_mb Container block write in MB")
	fmt.Fprintln(w, "# TYPE docker_container_block_write_mb counter")
	for _, cs := range stats.ContainerStats {
		fmt.Fprintf(w, "docker_container_block_write_mb{container=\"%s\",id=\"%s\"} %.2f\n",
			cs.Name, cs.ID, cs.BlockWriteMB)
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

	// Initialize Docker client
	var err error
	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	// Get CPU cores
	stats.TotalCPUCores = getCPUCores()
	log.Printf("Detected %d CPU cores", stats.TotalCPUCores)

	// Start stats collection
	ctx := context.Background()
	go collectStats(ctx, 5*time.Second)

	// Do initial collection
	if err := updateDockerStats(ctx); err != nil {
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
