package main

import (
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
	raplBasePath = "/sys/class/powercap"
	defaultPort  = 9500
)

// RAPLZone represents a RAPL power zone
type RAPLZone struct {
	Name          string
	EnergyPath    string
	MaxRange      int64
	PrevEnergy    int64
	PrevTime      time.Time
	Subzones      map[string]*RAPLZone
	mu            sync.RWMutex
}

// RAPLReader manages RAPL zone discovery and reading
type RAPLReader struct {
	zones map[string]*RAPLZone
	mu    sync.RWMutex
}

// NewRAPLReader creates a new RAPL reader and discovers zones
func NewRAPLReader() (*RAPLReader, error) {
	reader := &RAPLReader{
		zones: make(map[string]*RAPLZone),
	}
	
	if err := reader.discoverZones(); err != nil {
		return nil, err
	}
	
	if len(reader.zones) == 0 {
		return nil, fmt.Errorf("no RAPL zones found")
	}
	
	// Initialize baseline readings
	time.Sleep(100 * time.Millisecond)
	reader.updateAllReadings()
	
	return reader, nil
}

// discoverZones finds all available RAPL zones
func (r *RAPLReader) discoverZones() error {
	if _, err := os.Stat(raplBasePath); os.IsNotExist(err) {
		return fmt.Errorf("RAPL interface not found at %s", raplBasePath)
	}
	
	entries, err := os.ReadDir(raplBasePath)
	if err != nil {
		return fmt.Errorf("failed to read RAPL directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "intel-rapl:") {
			continue
		}
		
		zonePath := filepath.Join(raplBasePath, entry.Name())
		zone, err := r.loadZone(zonePath)
		if err != nil {
			log.Printf("Warning: failed to load zone %s: %v", entry.Name(), err)
			continue
		}
		
		if zone != nil {
			r.zones[zone.Name] = zone
			log.Printf("Discovered RAPL zone: %s", zone.Name)
		}
	}
	
	return nil
}

// loadZone loads a single RAPL zone
func (r *RAPLReader) loadZone(zonePath string) (*RAPLZone, error) {
	namePath := filepath.Join(zonePath, "name")
	nameBytes, err := os.ReadFile(namePath)
	if err != nil {
		return nil, err
	}
	
	name := strings.TrimSpace(string(nameBytes))
	energyPath := filepath.Join(zonePath, "energy_uj")
	
	if _, err := os.Stat(energyPath); err != nil {
		return nil, err
	}
	
	zone := &RAPLZone{
		Name:       name,
		EnergyPath: energyPath,
		Subzones:   make(map[string]*RAPLZone),
		PrevTime:   time.Now(),
	}
	
	// Read max energy range
	maxPath := filepath.Join(zonePath, "max_energy_range_uj")
	if maxBytes, err := os.ReadFile(maxPath); err == nil {
		zone.MaxRange, _ = strconv.ParseInt(strings.TrimSpace(string(maxBytes)), 10, 64)
	}
	
	// Check for subzones
	entries, err := os.ReadDir(zonePath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || !strings.Contains(entry.Name(), "intel-rapl:") {
				continue
			}
			
			subzonePath := filepath.Join(zonePath, entry.Name())
			subzone, err := r.loadZone(subzonePath)
			if err == nil && subzone != nil {
				zone.Subzones[subzone.Name] = subzone
				log.Printf("  Discovered subzone: %s", subzone.Name)
			}
		}
	}
	
	return zone, nil
}

// readEnergyUj reads the current energy counter in microjoules
func (z *RAPLZone) readEnergyUj() (int64, error) {
	data, err := os.ReadFile(z.EnergyPath)
	if err != nil {
		return 0, err
	}
	
	energy, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	
	return energy, nil
}

// getPowerWatts calculates power consumption in watts
func (z *RAPLZone) getPowerWatts() (float64, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	
	currentEnergy, err := z.readEnergyUj()
	if err != nil {
		return 0, err
	}
	
	currentTime := time.Now()
	
	// Need previous reading for power calculation
	if z.PrevTime.IsZero() {
		z.PrevEnergy = currentEnergy
		z.PrevTime = currentTime
		return 0, fmt.Errorf("need baseline reading")
	}
	
	timeDelta := currentTime.Sub(z.PrevTime).Seconds()
	if timeDelta <= 0 {
		return 0, fmt.Errorf("invalid time delta")
	}
	
	energyDelta := currentEnergy - z.PrevEnergy
	
	// Handle counter wraparound
	if energyDelta < 0 && z.MaxRange > 0 {
		energyDelta += z.MaxRange
	}
	
	// Convert microjoules to watts (J/s)
	powerWatts := float64(energyDelta) / 1_000_000 / timeDelta
	
	// Update state
	z.PrevEnergy = currentEnergy
	z.PrevTime = currentTime
	
	return powerWatts, nil
}

// updateAllReadings updates all zone readings
func (r *RAPLReader) updateAllReadings() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, zone := range r.zones {
		zone.getPowerWatts()
		
		for _, subzone := range zone.Subzones {
			subzone.getPowerWatts()
		}
	}
}

// MetricsHandler handles HTTP requests for metrics
type MetricsHandler struct {
	reader *RAPLReader
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	
	h.reader.mu.RLock()
	defer h.reader.mu.RUnlock()
	
	// Write metrics header
	fmt.Fprintln(w, "# HELP rapl_power_watts Current power consumption in watts from RAPL")
	fmt.Fprintln(w, "# TYPE rapl_power_watts gauge")
	
	// Collect metrics from all zones
	for zoneName, zone := range h.reader.zones {
		power, err := zone.getPowerWatts()
		if err == nil && power >= 0 {
			safeName := sanitizeMetricName(zoneName)
			fmt.Fprintf(w, "rapl_power_watts{zone=\"%s\"} %.4f\n", safeName, power)
		}
		
		// Subzones
		for subzoneName, subzone := range zone.Subzones {
			power, err := subzone.getPowerWatts()
			if err == nil && power >= 0 {
				safeName := sanitizeMetricName(fmt.Sprintf("%s_%s", zoneName, subzoneName))
				fmt.Fprintf(w, "rapl_power_watts{zone=\"%s\"} %.4f\n", safeName, power)
			}
		}
	}
	
	// Exporter metadata
	fmt.Fprintln(w, "# HELP cpu_exporter_info CPU exporter information")
	fmt.Fprintln(w, "# TYPE cpu_exporter_info gauge")
	fmt.Fprintln(w, "cpu_exporter_info{version=\"1.0.0\",language=\"go\"} 1")
}

func (h *MetricsHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

// sanitizeMetricName converts zone names to Prometheus-safe metric names
func sanitizeMetricName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	flag.Parse()
	
	// Check for port override via environment variable
	if envPort := os.Getenv("CPU_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}
	
	log.Printf("Starting CPU Power Exporter (Go) on port %d", *port)
	
	// Initialize RAPL reader
	reader, err := NewRAPLReader()
	if err != nil {
		log.Fatalf("Failed to initialize RAPL reader: %v", err)
	}
	
	log.Printf("Successfully initialized %d RAPL zones", len(reader.zones))
	
	// Set up HTTP server
	handler := &MetricsHandler{reader: reader}
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
