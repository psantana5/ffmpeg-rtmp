package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	defaultPort = 9600
)

// ExporterConfig defines an exporter to monitor
type ExporterConfig struct {
	Name     string
	URL      string
	JobName  string
	Timeout  time.Duration
}

// ExporterHealth holds health status for an exporter
type ExporterHealth struct {
	Name         string
	URL          string
	Healthy      bool
	LastCheck    time.Time
	LastError    string
	ResponseTime time.Duration
}

var (
	exporters = []ExporterConfig{
		{Name: "nginx-exporter", URL: "http://nginx-exporter:9728/metrics", JobName: "nginx-rtmp", Timeout: 5 * time.Second},
		{Name: "cpu-exporter-go", URL: "http://cpu-exporter-go:9500/metrics", JobName: "cpu-exporter", Timeout: 5 * time.Second},
		{Name: "docker-stats-exporter", URL: "http://docker-stats-exporter:9501/metrics", JobName: "docker-stats", Timeout: 5 * time.Second},
		{Name: "node-exporter", URL: "http://node-exporter:9100/metrics", JobName: "node-exporter", Timeout: 5 * time.Second},
		{Name: "cadvisor", URL: "http://cadvisor:8080/metrics", JobName: "cadvisor", Timeout: 5 * time.Second},
		{Name: "results-exporter", URL: "http://results-exporter:9502/metrics", JobName: "results-exporter", Timeout: 5 * time.Second},
		{Name: "qoe-exporter", URL: "http://qoe-exporter:9503/metrics", JobName: "qoe-exporter", Timeout: 5 * time.Second},
		{Name: "cost-exporter", URL: "http://cost-exporter:9504/metrics", JobName: "cost-exporter", Timeout: 5 * time.Second},
		{Name: "ffmpeg-exporter", URL: "http://ffmpeg-exporter:9506/metrics", JobName: "ffmpeg-exporter", Timeout: 5 * time.Second},
	}

	healthStatus = struct {
		sync.RWMutex
		statuses map[string]*ExporterHealth
	}{
		statuses: make(map[string]*ExporterHealth),
	}
)

// checkExporter checks the health of a single exporter
func checkExporter(config ExporterConfig) *ExporterHealth {
	health := &ExporterHealth{
		Name:      config.Name,
		URL:       config.URL,
		LastCheck: time.Now(),
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	start := time.Now()
	resp, err := client.Get(config.URL)
	health.ResponseTime = time.Since(start)

	if err != nil {
		health.Healthy = false
		health.LastError = err.Error()
		return health
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		health.Healthy = false
		health.LastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return health
	}

	health.Healthy = true
	health.LastError = ""
	return health
}

// monitorExporters periodically checks all exporters
func monitorExporters(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do initial check
	checkAllExporters()

	for range ticker.C {
		checkAllExporters()
	}
}

// checkAllExporters checks all configured exporters
func checkAllExporters() {
	var wg sync.WaitGroup

	for _, exporter := range exporters {
		wg.Add(1)
		go func(exp ExporterConfig) {
			defer wg.Done()

			health := checkExporter(exp)

			healthStatus.Lock()
			healthStatus.statuses[exp.Name] = health
			healthStatus.Unlock()

			if health.Healthy {
				log.Printf("[%s] ✓ Healthy (%.2fms)", exp.Name, float64(health.ResponseTime.Microseconds())/1000.0)
			} else {
				log.Printf("[%s] ✗ Unhealthy: %s", exp.Name, health.LastError)
			}
		}(exporter)
	}

	wg.Wait()
}

// metricsHandler handles the /metrics endpoint
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	healthStatus.RLock()
	defer healthStatus.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Exporter metadata
	fmt.Fprintln(w, "# HELP exporter_health_checker_up Health checker is running")
	fmt.Fprintln(w, "# TYPE exporter_health_checker_up gauge")
	fmt.Fprintln(w, "exporter_health_checker_up 1")

	// Exporter health status
	fmt.Fprintln(w, "# HELP exporter_healthy Whether the exporter is healthy (1) or not (0)")
	fmt.Fprintln(w, "# TYPE exporter_healthy gauge")

	for _, status := range healthStatus.statuses {
		healthValue := 0
		if status.Healthy {
			healthValue = 1
		}
		fmt.Fprintf(w, "exporter_healthy{exporter=\"%s\",url=\"%s\"} %d\n",
			status.Name, status.URL, healthValue)
	}

	// Response time
	fmt.Fprintln(w, "# HELP exporter_response_time_ms Exporter response time in milliseconds")
	fmt.Fprintln(w, "# TYPE exporter_response_time_ms gauge")

	for _, status := range healthStatus.statuses {
		responseTimeMs := float64(status.ResponseTime.Microseconds()) / 1000.0
		fmt.Fprintf(w, "exporter_response_time_ms{exporter=\"%s\",url=\"%s\"} %.2f\n",
			status.Name, status.URL, responseTimeMs)
	}

	// Last check timestamp
	fmt.Fprintln(w, "# HELP exporter_last_check_timestamp Unix timestamp of last health check")
	fmt.Fprintln(w, "# TYPE exporter_last_check_timestamp gauge")

	for _, status := range healthStatus.statuses {
		fmt.Fprintf(w, "exporter_last_check_timestamp{exporter=\"%s\"} %d\n",
			status.Name, status.LastCheck.Unix())
	}

	// Total exporters
	fmt.Fprintln(w, "# HELP exporter_total Total number of exporters being monitored")
	fmt.Fprintln(w, "# TYPE exporter_total gauge")
	fmt.Fprintf(w, "exporter_total %d\n", len(healthStatus.statuses))

	// Healthy exporters
	healthyCount := 0
	for _, status := range healthStatus.statuses {
		if status.Healthy {
			healthyCount++
		}
	}
	fmt.Fprintln(w, "# HELP exporter_healthy_total Total number of healthy exporters")
	fmt.Fprintln(w, "# TYPE exporter_healthy_total gauge")
	fmt.Fprintf(w, "exporter_healthy_total %d\n", healthyCount)

	// Exporter info
	fmt.Fprintln(w, "# HELP exporter_health_checker_info Health checker information")
	fmt.Fprintln(w, "# TYPE exporter_health_checker_info gauge")
	fmt.Fprintln(w, "exporter_health_checker_info{version=\"1.0.0\",language=\"go\"} 1")
}

// healthHandler handles the /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	interval := flag.Int("interval", 30, "Check interval in seconds")
	flag.Parse()

	// Override with environment variable if set
	if envPort := os.Getenv("HEALTH_CHECK_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	log.Println("Starting Exporter Health Checker (Go)")
	log.Printf("Monitoring %d exporters", len(exporters))
	log.Printf("Check interval: %d seconds", *interval)

	// Start monitoring
	go monitorExporters(time.Duration(*interval) * time.Second)

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
