package main

import (
	"encoding/xml"
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
	defaultPort = 9505
	cacheTTL    = 5 * time.Second
)

// NvidiaSMI XML structures
type NvidiaSMILog struct {
	XMLName xml.Name `xml:"nvidia_smi_log"`
	GPUs    []GPU    `xml:"gpu"`
}

type GPU struct {
	ID          string        `xml:"id,attr"`
	ProductName string        `xml:"product_name"`
	UUID        string        `xml:"uuid"`
	Power       PowerReadings `xml:"gpu_power_readings"`
	Temperature Temperature   `xml:"temperature"`
	Utilization Utilization   `xml:"utilization"`
	FBMemory    FBMemory      `xml:"fb_memory_usage"`
	Clocks      Clocks        `xml:"clocks"`
}

type PowerReadings struct {
	PowerDraw  string `xml:"power_draw"`
	PowerLimit string `xml:"power_limit"`
}

type Temperature struct {
	GPUTemp string `xml:"gpu_temp"`
}

type Utilization struct {
	GPUUtil     string `xml:"gpu_util"`
	MemoryUtil  string `xml:"memory_util"`
	EncoderUtil string `xml:"encoder_util"`
	DecoderUtil string `xml:"decoder_util"`
}

type FBMemory struct {
	Used  string `xml:"used"`
	Total string `xml:"total"`
}

type Clocks struct {
	GraphicsClock string `xml:"graphics_clock"`
	SMClock       string `xml:"sm_clock"`
	MemoryClock   string `xml:"mem_clock"`
}

// GPUMetrics represents parsed GPU metrics
type GPUMetrics struct {
	ID                        string
	Name                      string
	UUID                      string
	PowerDrawWatts            float64
	PowerLimitWatts           float64
	TemperatureCelsius        float64
	UtilizationGPUPercent     float64
	UtilizationMemoryPercent  float64
	UtilizationEncoderPercent float64
	UtilizationDecoderPercent float64
	MemoryUsedMB              float64
	MemoryTotalMB             float64
	ClocksGraphicsMHz         float64
	ClocksSMMHz               float64
	ClocksMemoryMHz           float64
}

// GPUExporter manages GPU metrics collection
type GPUExporter struct {
	available    bool
	metricsCache []GPUMetrics
	lastUpdate   time.Time
	mu           sync.RWMutex
}

// NewGPUExporter creates a new GPU exporter
func NewGPUExporter() *GPUExporter {
	exporter := &GPUExporter{}
	exporter.checkAvailability()
	return exporter
}

// checkAvailability checks if nvidia-smi is available
func (e *GPUExporter) checkAvailability() {
	cmd := exec.Command("nvidia-smi", "-L")
	if err := cmd.Run(); err != nil {
		log.Println("Warning: nvidia-smi not available - GPU monitoring disabled")
		e.available = false
		return
	}
	e.available = true
	log.Println("nvidia-smi detected - GPU monitoring enabled")
}

// parseFloat extracts float from string with unit (e.g., "123.45 W" -> 123.45)
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return 0
	}

	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	return val
}

// getGPUMetrics queries nvidia-smi and returns parsed metrics
func (e *GPUExporter) getGPUMetrics() ([]GPUMetrics, error) {
	if !e.available {
		return []GPUMetrics{}, nil
	}

	cmd := exec.Command("nvidia-smi", "-q", "-x")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query nvidia-smi: %w", err)
	}

	var smiLog NvidiaSMILog
	if err := xml.Unmarshal(output, &smiLog); err != nil {
		return nil, fmt.Errorf("failed to parse nvidia-smi XML: %w", err)
	}

	metrics := make([]GPUMetrics, 0, len(smiLog.GPUs))

	for _, gpu := range smiLog.GPUs {
		m := GPUMetrics{
			ID:                        gpu.ID,
			Name:                      gpu.ProductName,
			UUID:                      gpu.UUID,
			PowerDrawWatts:            parseFloat(gpu.Power.PowerDraw),
			PowerLimitWatts:           parseFloat(gpu.Power.PowerLimit),
			TemperatureCelsius:        parseFloat(gpu.Temperature.GPUTemp),
			UtilizationGPUPercent:     parseFloat(gpu.Utilization.GPUUtil),
			UtilizationMemoryPercent:  parseFloat(gpu.Utilization.MemoryUtil),
			UtilizationEncoderPercent: parseFloat(gpu.Utilization.EncoderUtil),
			UtilizationDecoderPercent: parseFloat(gpu.Utilization.DecoderUtil),
			MemoryUsedMB:              parseFloat(gpu.FBMemory.Used),
			MemoryTotalMB:             parseFloat(gpu.FBMemory.Total),
			ClocksGraphicsMHz:         parseFloat(gpu.Clocks.GraphicsClock),
			ClocksSMMHz:               parseFloat(gpu.Clocks.SMClock),
			ClocksMemoryMHz:           parseFloat(gpu.Clocks.MemoryClock),
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// getMetricsWithCache returns cached metrics or fetches new ones
func (e *GPUExporter) getMetricsWithCache() []GPUMetrics {
	e.mu.RLock()
	if time.Since(e.lastUpdate) < cacheTTL && e.metricsCache != nil {
		defer e.mu.RUnlock()
		return e.metricsCache
	}
	e.mu.RUnlock()

	metrics, err := e.getGPUMetrics()
	if err != nil {
		log.Printf("Error getting GPU metrics: %v", err)
		return e.metricsCache // Return cached data on error
	}

	e.mu.Lock()
	e.metricsCache = metrics
	e.lastUpdate = time.Now()
	e.mu.Unlock()

	return metrics
}

// MetricsHandler handles HTTP requests
type MetricsHandler struct {
	exporter *GPUExporter
}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/metrics":
		h.handleMetrics(w, r)
	case "/health":
		h.handleHealth(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *MetricsHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Exporter alive metric
	fmt.Fprintln(w, "# HELP gpu_exporter_alive GPU exporter health check (always 1)")
	fmt.Fprintln(w, "# TYPE gpu_exporter_alive gauge")
	fmt.Fprintln(w, "gpu_exporter_alive 1")

	metrics := h.exporter.getMetricsWithCache()

	// GPU count
	fmt.Fprintln(w, "# HELP gpu_count Number of GPUs detected")
	fmt.Fprintln(w, "# TYPE gpu_count gauge")
	fmt.Fprintf(w, "gpu_count %d\n", len(metrics))

	if len(metrics) == 0 {
		return
	}

	// Define all metric types
	fmt.Fprintln(w, "# HELP gpu_power_draw_watts GPU power draw in watts")
	fmt.Fprintln(w, "# TYPE gpu_power_draw_watts gauge")
	fmt.Fprintln(w, "# HELP gpu_power_limit_watts GPU power limit in watts")
	fmt.Fprintln(w, "# TYPE gpu_power_limit_watts gauge")
	fmt.Fprintln(w, "# HELP gpu_temperature_celsius GPU temperature in Celsius")
	fmt.Fprintln(w, "# TYPE gpu_temperature_celsius gauge")
	fmt.Fprintln(w, "# HELP gpu_utilization_percent GPU utilization percentage")
	fmt.Fprintln(w, "# TYPE gpu_utilization_percent gauge")
	fmt.Fprintln(w, "# HELP gpu_memory_utilization_percent GPU memory utilization percentage")
	fmt.Fprintln(w, "# TYPE gpu_memory_utilization_percent gauge")
	fmt.Fprintln(w, "# HELP gpu_encoder_utilization_percent GPU encoder utilization percentage")
	fmt.Fprintln(w, "# TYPE gpu_encoder_utilization_percent gauge")
	fmt.Fprintln(w, "# HELP gpu_decoder_utilization_percent GPU decoder utilization percentage")
	fmt.Fprintln(w, "# TYPE gpu_decoder_utilization_percent gauge")
	fmt.Fprintln(w, "# HELP gpu_memory_used_bytes GPU memory used in bytes")
	fmt.Fprintln(w, "# TYPE gpu_memory_used_bytes gauge")
	fmt.Fprintln(w, "# HELP gpu_memory_total_bytes GPU total memory in bytes")
	fmt.Fprintln(w, "# TYPE gpu_memory_total_bytes gauge")
	fmt.Fprintln(w, "# HELP gpu_clocks_graphics_mhz GPU graphics clock in MHz")
	fmt.Fprintln(w, "# TYPE gpu_clocks_graphics_mhz gauge")
	fmt.Fprintln(w, "# HELP gpu_clocks_sm_mhz GPU SM clock in MHz")
	fmt.Fprintln(w, "# TYPE gpu_clocks_sm_mhz gauge")
	fmt.Fprintln(w, "# HELP gpu_clocks_memory_mhz GPU memory clock in MHz")
	fmt.Fprintln(w, "# TYPE gpu_clocks_memory_mhz gauge")

	// Export metrics for each GPU
	for _, m := range metrics {
		labels := fmt.Sprintf("gpu_id=\"%s\",gpu_name=\"%s\",gpu_uuid=\"%s\"", m.ID, m.Name, m.UUID)

		fmt.Fprintf(w, "gpu_power_draw_watts{%s} %.2f\n", labels, m.PowerDrawWatts)
		fmt.Fprintf(w, "gpu_power_limit_watts{%s} %.2f\n", labels, m.PowerLimitWatts)
		fmt.Fprintf(w, "gpu_temperature_celsius{%s} %.1f\n", labels, m.TemperatureCelsius)
		fmt.Fprintf(w, "gpu_utilization_percent{%s} %.1f\n", labels, m.UtilizationGPUPercent)
		fmt.Fprintf(w, "gpu_memory_utilization_percent{%s} %.1f\n", labels, m.UtilizationMemoryPercent)
		fmt.Fprintf(w, "gpu_encoder_utilization_percent{%s} %.1f\n", labels, m.UtilizationEncoderPercent)
		fmt.Fprintf(w, "gpu_decoder_utilization_percent{%s} %.1f\n", labels, m.UtilizationDecoderPercent)
		fmt.Fprintf(w, "gpu_memory_used_bytes{%s} %.0f\n", labels, m.MemoryUsedMB*1024*1024)
		fmt.Fprintf(w, "gpu_memory_total_bytes{%s} %.0f\n", labels, m.MemoryTotalMB*1024*1024)
		fmt.Fprintf(w, "gpu_clocks_graphics_mhz{%s} %.0f\n", labels, m.ClocksGraphicsMHz)
		fmt.Fprintf(w, "gpu_clocks_sm_mhz{%s} %.0f\n", labels, m.ClocksSMMHz)
		fmt.Fprintf(w, "gpu_clocks_memory_mhz{%s} %.0f\n", labels, m.ClocksMemoryMHz)
	}

	// Exporter info
	fmt.Fprintln(w, "# HELP gpu_exporter_info GPU exporter information")
	fmt.Fprintln(w, "# TYPE gpu_exporter_info gauge")
	fmt.Fprintln(w, "gpu_exporter_info{version=\"1.0.0\",language=\"go\"} 1")
}

func (h *MetricsHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	flag.Parse()

	// Check for port override via environment variable
	if envPort := os.Getenv("GPU_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	log.Printf("Starting GPU Power Exporter (Go) on port %d", *port)

	// Initialize GPU exporter
	exporter := NewGPUExporter()

	// Set up HTTP server
	handler := &MetricsHandler{exporter: exporter}
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	log.Printf("Exporter ready at http://0.0.0.0:%d/metrics", *port)
	log.Printf("Health endpoint at http://0.0.0.0:%d/health", *port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
