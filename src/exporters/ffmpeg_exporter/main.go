package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort = 9506
)

// FFmpegStats holds the current FFmpeg encoding statistics
type FFmpegStats struct {
	mu sync.RWMutex
	
	// Encoder metrics
	EncoderLoadPercent float64
	DroppedFrames      int64
	TotalFrames        int64
	AvgBitrateKbps     float64
	CurrentBitrateKbps float64
	EncodeLatencyMs    float64
	
	// Processing stats
	FPS               float64
	Speed             float64
	ProcessingTime    float64
	
	// Stream info
	StreamActive      bool
	LastUpdateTime    time.Time
	ProcessStartTime  time.Time
	
	// Quality metrics
	QualityScore      float64
}

var (
	stats = &FFmpegStats{}
	
	// Regular expressions for parsing FFmpeg output
	frameRegex    = regexp.MustCompile(`frame=\s*(\d+)`)
	fpsRegex      = regexp.MustCompile(`fps=\s*([0-9.]+)`)
	bitrateRegex  = regexp.MustCompile(`bitrate=\s*([0-9.]+)kbits/s`)
	speedRegex    = regexp.MustCompile(`speed=\s*([0-9.]+)x`)
	dropRegex     = regexp.MustCompile(`drop=\s*(\d+)`)
	timeRegex     = regexp.MustCompile(`time=\s*(\d+):(\d+):(\d+)\.(\d+)`)
)

// parseFFmpegLine parses a line of FFmpeg output and updates stats
func parseFFmpegLine(line string) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	
	stats.LastUpdateTime = time.Now()
	stats.StreamActive = true
	
	// Parse frame count
	if match := frameRegex.FindStringSubmatch(line); match != nil {
		if val, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			stats.TotalFrames = val
		}
	}
	
	// Parse FPS
	if match := fpsRegex.FindStringSubmatch(line); match != nil {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			stats.FPS = val
		}
	}
	
	// Parse bitrate
	if match := bitrateRegex.FindStringSubmatch(line); match != nil {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			stats.CurrentBitrateKbps = val
			// Update rolling average
			if stats.AvgBitrateKbps == 0 {
				stats.AvgBitrateKbps = val
			} else {
				stats.AvgBitrateKbps = (stats.AvgBitrateKbps * 0.9) + (val * 0.1)
			}
		}
	}
	
	// Parse speed
	if match := speedRegex.FindStringSubmatch(line); match != nil {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			stats.Speed = val
		}
	}
	
	// Parse dropped frames
	if match := dropRegex.FindStringSubmatch(line); match != nil {
		if val, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			stats.DroppedFrames = val
		}
	}
	
	// Parse processing time
	if match := timeRegex.FindStringSubmatch(line); match != nil {
		hours, _ := strconv.ParseFloat(match[1], 64)
		minutes, _ := strconv.ParseFloat(match[2], 64)
		seconds, _ := strconv.ParseFloat(match[3], 64)
		centiseconds, _ := strconv.ParseFloat(match[4], 64)
		
		totalSeconds := hours*3600 + minutes*60 + seconds + centiseconds/100
		stats.ProcessingTime = totalSeconds
	}
	
	// Calculate encoder load (based on speed - if speed < 1.0, encoder is struggling)
	if stats.Speed > 0 {
		stats.EncoderLoadPercent = (1.0 / stats.Speed) * 100
		if stats.EncoderLoadPercent > 100 {
			stats.EncoderLoadPercent = 100
		}
	}
	
	// Calculate encode latency (frame time vs real time)
	if stats.FPS > 0 && stats.Speed > 0 {
		frameTime := 1000.0 / stats.FPS
		stats.EncodeLatencyMs = frameTime / stats.Speed
	}
	
	// Calculate quality score (simple heuristic: lower dropped frames and stable bitrate = higher quality)
	if stats.TotalFrames > 0 {
		dropRate := float64(stats.DroppedFrames) / float64(stats.TotalFrames)
		stats.QualityScore = (1.0 - dropRate) * 100
	}
}

// monitorFFmpegProcess monitors stream activity based on last update time
func monitorFFmpegProcess() {
	// Monitor stream activity based on metrics updates
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		stats.mu.Lock()
		// Mark stream as inactive if no updates for 5 seconds
		if stats.StreamActive && time.Since(stats.LastUpdateTime) > 5*time.Second {
			stats.StreamActive = false
		}
		stats.mu.Unlock()
	}
}

// metricsHandler handles the /metrics endpoint
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	stats.mu.RLock()
	defer stats.mu.RUnlock()
	
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	
	// Exporter metadata
	fmt.Fprintln(w, "# HELP ffmpeg_exporter_up FFmpeg exporter is running")
	fmt.Fprintln(w, "# TYPE ffmpeg_exporter_up gauge")
	fmt.Fprintln(w, "ffmpeg_exporter_up 1")
	
	// Stream status
	fmt.Fprintln(w, "# HELP ffmpeg_stream_active Whether FFmpeg stream is currently active (1) or not (0)")
	fmt.Fprintln(w, "# TYPE ffmpeg_stream_active gauge")
	if stats.StreamActive {
		fmt.Fprintln(w, "ffmpeg_stream_active 1")
	} else {
		fmt.Fprintln(w, "ffmpeg_stream_active 0")
	}
	
	// Encoder load
	fmt.Fprintln(w, "# HELP ffmpeg_encoder_load_percent Current encoder load as percentage (0-100)")
	fmt.Fprintln(w, "# TYPE ffmpeg_encoder_load_percent gauge")
	fmt.Fprintf(w, "ffmpeg_encoder_load_percent %.2f\n", stats.EncoderLoadPercent)
	
	// Dropped frames
	fmt.Fprintln(w, "# HELP ffmpeg_dropped_frames_total Total number of dropped frames")
	fmt.Fprintln(w, "# TYPE ffmpeg_dropped_frames_total counter")
	fmt.Fprintf(w, "ffmpeg_dropped_frames_total %d\n", stats.DroppedFrames)
	
	// Total frames
	fmt.Fprintln(w, "# HELP ffmpeg_frames_total Total number of processed frames")
	fmt.Fprintln(w, "# TYPE ffmpeg_frames_total counter")
	fmt.Fprintf(w, "ffmpeg_frames_total %d\n", stats.TotalFrames)
	
	// Average bitrate
	fmt.Fprintln(w, "# HELP ffmpeg_avg_bitrate_kbps Average output bitrate in kilobits per second")
	fmt.Fprintln(w, "# TYPE ffmpeg_avg_bitrate_kbps gauge")
	fmt.Fprintf(w, "ffmpeg_avg_bitrate_kbps %.2f\n", stats.AvgBitrateKbps)
	
	// Current bitrate
	fmt.Fprintln(w, "# HELP ffmpeg_current_bitrate_kbps Current output bitrate in kilobits per second")
	fmt.Fprintln(w, "# TYPE ffmpeg_current_bitrate_kbps gauge")
	fmt.Fprintf(w, "ffmpeg_current_bitrate_kbps %.2f\n", stats.CurrentBitrateKbps)
	
	// Encode latency
	fmt.Fprintln(w, "# HELP ffmpeg_encode_latency_ms Encoding latency in milliseconds per frame")
	fmt.Fprintln(w, "# TYPE ffmpeg_encode_latency_ms gauge")
	fmt.Fprintf(w, "ffmpeg_encode_latency_ms %.2f\n", stats.EncodeLatencyMs)
	
	// FPS
	fmt.Fprintln(w, "# HELP ffmpeg_fps Current frames per second being processed")
	fmt.Fprintln(w, "# TYPE ffmpeg_fps gauge")
	fmt.Fprintf(w, "ffmpeg_fps %.2f\n", stats.FPS)
	
	// Speed
	fmt.Fprintln(w, "# HELP ffmpeg_speed Processing speed multiplier (1.0 = realtime)")
	fmt.Fprintln(w, "# TYPE ffmpeg_speed gauge")
	fmt.Fprintf(w, "ffmpeg_speed %.2f\n", stats.Speed)
	
	// Processing time
	fmt.Fprintln(w, "# HELP ffmpeg_processing_time_seconds Total processing time in seconds")
	fmt.Fprintln(w, "# TYPE ffmpeg_processing_time_seconds counter")
	fmt.Fprintf(w, "ffmpeg_processing_time_seconds %.2f\n", stats.ProcessingTime)
	
	// Quality score
	fmt.Fprintln(w, "# HELP ffmpeg_quality_score Quality score based on dropped frame rate (0-100)")
	fmt.Fprintln(w, "# TYPE ffmpeg_quality_score gauge")
	fmt.Fprintf(w, "ffmpeg_quality_score %.2f\n", stats.QualityScore)
	
	// Uptime
	if !stats.ProcessStartTime.IsZero() {
		uptime := time.Since(stats.ProcessStartTime).Seconds()
		fmt.Fprintln(w, "# HELP ffmpeg_uptime_seconds Time since exporter started")
		fmt.Fprintln(w, "# TYPE ffmpeg_uptime_seconds counter")
		fmt.Fprintf(w, "ffmpeg_uptime_seconds %.0f\n", uptime)
	}
}

// healthHandler handles the /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
	
	stats.mu.RLock()
	defer stats.mu.RUnlock()
	
	fmt.Fprintf(w, "stream_active: %v\n", stats.StreamActive)
	fmt.Fprintf(w, "last_update: %v\n", stats.LastUpdateTime.Format(time.RFC3339))
}

// startFFmpegLogMonitor starts monitoring FFmpeg logs from stdin or a file
func startFFmpegLogMonitor() {
	// This can be extended to read from:
	// 1. Docker logs: docker logs -f <container>
	// 2. Log files: tail -f /var/log/ffmpeg.log
	// 3. Named pipes
	// 4. Direct process stdout/stderr
	
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "frame=") {
				parseFFmpegLine(line)
			}
		}
	}()
	
	// Also monitor for running processes
	go monitorFFmpegProcess()
}

func main() {
	port := flag.Int("port", defaultPort, "Port to listen on")
	flag.Parse()
	
	// Override with environment variable if set
	if envPort := os.Getenv("FFMPEG_EXPORTER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}
	
	stats.ProcessStartTime = time.Now()
	
	// Start monitoring
	startFFmpegLogMonitor()
	
	// Register HTTP handlers
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>FFmpeg Exporter</title></head>
<body>
<h1>FFmpeg Stats Exporter</h1>
<p>Prometheus exporter for FFmpeg encoding statistics</p>
<ul>
<li><a href="/metrics">/metrics</a> - Prometheus metrics</li>
<li><a href="/health">/health</a> - Health check</li>
</ul>
<h2>Metrics Exposed</h2>
<ul>
<li><b>ffmpeg_encoder_load_percent</b> - Encoder load (0-100%%)</li>
<li><b>ffmpeg_dropped_frames_total</b> - Total dropped frames</li>
<li><b>ffmpeg_avg_bitrate_kbps</b> - Average output bitrate</li>
<li><b>ffmpeg_encode_latency_ms</b> - Encoding latency per frame</li>
<li><b>ffmpeg_fps</b> - Current FPS</li>
<li><b>ffmpeg_speed</b> - Processing speed (1.0 = realtime)</li>
<li><b>ffmpeg_quality_score</b> - Quality score (0-100)</li>
</ul>
</body>
</html>`)
	})
	
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("Starting FFmpeg Stats Exporter")
	log.Printf("Metrics endpoint at http://0.0.0.0:%d/metrics", *port)
	log.Printf("Health endpoint at http://0.0.0.0:%d/health", *port)
	
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
