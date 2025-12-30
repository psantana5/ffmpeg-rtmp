package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/agent"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"
)

func main() {
	masterURL := flag.String("master", "http://localhost:8080", "Master node URL")
	register := flag.Bool("register", false, "Register with master node")
	pollInterval := flag.Duration("poll-interval", 10*time.Second, "Job polling interval")
	heartbeatInterval := flag.Duration("heartbeat-interval", 30*time.Second, "Heartbeat interval")
	allowMasterAsWorker := flag.Bool("allow-master-as-worker", false, "Allow registering master node as worker (development mode)")
	apiKeyFlag := flag.String("api-key", "", "API key for authentication (or use FFMPEG_RTMP_API_KEY env var)")
	certFile := flag.String("cert", "", "TLS client certificate file (for mTLS)")
	keyFile := flag.String("key", "", "TLS client key file (for mTLS)")
	caFile := flag.String("ca", "", "CA certificate file to verify server")
	insecureSkipVerify := flag.Bool("insecure-skip-verify", false, "Skip TLS certificate verification (insecure, for development only)")
	flag.Parse()

	// Get API key from flag or environment variable
	apiKey := *apiKeyFlag
	apiKeySource := ""
	if apiKey == "" {
		apiKey = os.Getenv("FFMPEG_RTMP_API_KEY")
		if apiKey != "" {
			apiKeySource = "environment variable"
		}
	} else {
		apiKeySource = "command-line flag"
	}

	log.Println("Starting FFmpeg RTMP Distributed Compute Agent (Production Mode)")
	log.Printf("Master URL: %s", *masterURL)

	// Detect hardware capabilities
	log.Println("Detecting hardware capabilities...")
	caps, err := agent.DetectHardware()
	if err != nil {
		log.Fatalf("Failed to detect hardware: %v", err)
	}

	// Determine node type
	nodeType := agent.DetectNodeType(caps.CPUThreads, caps.RAMBytes)

	log.Println("Hardware detected:")
	log.Printf("  CPU: %s (%d threads)", caps.CPUModel, caps.CPUThreads)
	log.Printf("  RAM: %s", agent.FormatRAM(caps.RAMBytes))
	if caps.HasGPU {
		log.Printf("  GPU: %s", caps.GPUType)
	} else {
		log.Printf("  GPU: Not detected")
	}
	log.Printf("  Node Type: %s", nodeType)
	log.Printf("  OS/Arch: %s/%s", caps.Labels["os"], caps.Labels["arch"])

	// Optimize FFmpeg parameters based on hardware
	log.Println("Optimizing FFmpeg parameters for this hardware...")
	ffmpegOpt := agent.OptimizeFFmpegParameters(caps, nodeType)
	log.Printf("  Recommended Encoder: %s", ffmpegOpt.Encoder)
	log.Printf("  Recommended Preset: %s", ffmpegOpt.Preset)
	if ffmpegOpt.HWAccel != "none" {
		log.Printf("  Hardware Acceleration: %s", ffmpegOpt.HWAccel)
	}
	log.Printf("  Optimization Reason: %s", ffmpegOpt.Reason)

	// Create client with TLS support if certificates provided
	var client *agent.Client
	if *certFile != "" && *keyFile != "" {
		log.Println("Initializing TLS client...")
		tlsConfig, err := tlsutil.LoadClientTLSConfig(*certFile, *keyFile, *caFile)
		if err != nil {
			log.Fatalf("Failed to load TLS config: %v", err)
		}
		if *insecureSkipVerify {
			log.Println("WARNING: TLS certificate verification disabled (insecure)")
			tlsConfig.InsecureSkipVerify = true
		}
		client = agent.NewClientWithTLS(*masterURL, tlsConfig)
		log.Println("TLS enabled")
	} else if strings.HasPrefix(*masterURL, "https://") {
		// HTTPS without client certificates - create TLS config with optional skip verify
		log.Println("Initializing TLS client for HTTPS...")
		tlsConfig, err := tlsutil.LoadClientTLSConfig("", "", *caFile)
		if err != nil {
			log.Fatalf("Failed to load TLS config: %v", err)
		}
		if *insecureSkipVerify {
			log.Println("WARNING: TLS certificate verification disabled (insecure)")
			tlsConfig.InsecureSkipVerify = true
		}
		client = agent.NewClientWithTLS(*masterURL, tlsConfig)
		if *caFile == "" && !*insecureSkipVerify {
			log.Println("WARNING: Using HTTPS without CA certificate - server certificate must be signed by a trusted CA")
		}
	} else {
		client = agent.NewClient(*masterURL)
	}

	// Set API key if provided
	if apiKey != "" {
		client.SetAPIKey(apiKey)
		log.Printf("API authentication enabled (source: %s)", apiKeySource)
	} else {
		log.Println("WARNING: No API key provided (authentication disabled)")
	}

	// Register with master if requested
	if *register {
		log.Println("Registering with master node...")

		// Get node address (hostname or IP)
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}

		// Check if we're trying to register the master as a worker
		if isMasterAsWorker(*masterURL, hostname) {
			log.Println("")
			log.Println("⚠️  WARNING: Master node detected as worker!")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("You are attempting to register the master node as a compute worker.")
			log.Println("This configuration is intended for DEVELOPMENT/TESTING ONLY.")
			log.Println("")
			log.Println("Risks:")
			log.Println("  • Master and worker compete for CPU/memory resources")
			log.Println("  • Heavy workloads may impact master API responsiveness")
			log.Println("  • Not recommended for production environments")
			log.Println("")
			log.Println("Recommended: Run workers on separate machines in production.")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("")

			if !*allowMasterAsWorker {
				log.Println("To proceed, restart with --allow-master-as-worker flag:")
				log.Printf("  %s --register --master %s --allow-master-as-worker\n", os.Args[0], *masterURL)
				log.Println("")
				os.Exit(1)
			}

			// Ask for confirmation
			if !confirmMasterAsWorker() {
				log.Println("Registration cancelled by user.")
				os.Exit(0)
			}

			log.Println("✓ Proceeding with master-as-worker configuration...")
			log.Println("")
		}

		if caps.Labels == nil {
			caps.Labels = make(map[string]string)
		}
		caps.Labels["node_type"] = string(nodeType)

		// Add label if master is also a worker
		if *allowMasterAsWorker {
			caps.Labels["master_as_worker"] = "true"
		}

		reg := &models.NodeRegistration{
			Address:    hostname,
			Type:       nodeType,
			CPUThreads: caps.CPUThreads,
			CPUModel:   caps.CPUModel,
			HasGPU:     caps.HasGPU,
			GPUType:    caps.GPUType,
			RAMBytes:   caps.RAMBytes,
			Labels:     caps.Labels,
		}

		node, err := client.Register(reg)
		if err != nil {
			log.Fatalf("Failed to register with master: %v", err)
		}

		log.Printf("✓ Registered successfully!")
		log.Printf("  Node ID: %s", node.ID)
		log.Printf("  Status: %s", node.Status)
	} else {
		log.Println("Running in standalone mode (no registration)")
		log.Println("Use --register flag to register with master node")
		return
	}

	// Start heartbeat loop
	go func() {
		ticker := time.NewTicker(*heartbeatInterval)
		defer ticker.Stop()

		for range ticker.C {
			if err := client.SendHeartbeat(); err != nil {
				log.Printf("Heartbeat failed: %v", err)
			} else {
				log.Println("Heartbeat sent")
			}
		}
	}()

	// Main job polling loop
	log.Println("Starting job polling loop...")
	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		job, err := client.GetNextJob()
		if err != nil {
			log.Printf("Failed to get next job: %v", err)
			continue
		}

		if job == nil {
			log.Println("No jobs available")
			continue
		}

		log.Printf("Received job: %s (scenario: %s)", job.ID, job.Scenario)

		// Execute job with hardware-optimized parameters
		result := executeJob(job, client.GetNodeID(), ffmpegOpt)

		// Send results
		if err := client.SendResults(result); err != nil {
			log.Printf("Failed to send results: %v", err)
		} else {
			log.Printf("Results sent for job %s (status: %s)", job.ID, result.Status)
		}
	}
}

// isMasterAsWorker checks if the agent is trying to register on the same machine as master
func isMasterAsWorker(masterURL, hostname string) bool {
	// Check if master URL contains localhost or 127.0.0.1
	if strings.Contains(masterURL, "localhost") || strings.Contains(masterURL, "127.0.0.1") {
		return true
	}

	// Check if master URL contains the local hostname
	if strings.Contains(masterURL, hostname) {
		return true
	}

	return false
}

// confirmMasterAsWorker prompts the user for confirmation
func confirmMasterAsWorker() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Do you want to continue? (yes/no): ")

	response, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v", err)
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y"
}

// executeJob executes a job and returns the result
func executeJob(job *models.Job, nodeID string, ffmpegOpt *agent.FFmpegOptimization) *models.JobResult {
	log.Printf("Executing job %s (scenario: %s)...", job.ID, job.Scenario)
	startTime := time.Now()

	// Execute the actual job based on parameters
	metrics, analyzerOutput, err := executeFFmpegJob(job, ffmpegOpt)
	
	duration := time.Since(startTime).Seconds()
	
	if err != nil {
		log.Printf("Job %s failed: %v", job.ID, err)
		return &models.JobResult{
			JobID:       job.ID,
			NodeID:      nodeID,
			Status:      models.JobStatusFailed,
			Error:       err.Error(),
			CompletedAt: time.Now(),
			Metrics: map[string]interface{}{
				"duration": duration,
			},
		}
	}

	// Add duration to metrics
	if metrics == nil {
		metrics = make(map[string]interface{})
	}
	metrics["duration"] = duration
	metrics["scenario"] = job.Scenario

	log.Printf("Job %s completed successfully in %.2f seconds", job.ID, duration)
	return &models.JobResult{
		JobID:          job.ID,
		NodeID:         nodeID,
		Status:         models.JobStatusCompleted,
		CompletedAt:    time.Now(),
		Metrics:        metrics,
		AnalyzerOutput: analyzerOutput,
	}
}

// executeFFmpegJob executes an FFmpeg transcoding job based on job parameters
func executeFFmpegJob(job *models.Job, ffmpegOpt *agent.FFmpegOptimization) (metrics map[string]interface{}, analyzerOutput map[string]interface{}, err error) {
	// Extract parameters from job
	params := job.Parameters
	if params == nil {
		params = make(map[string]interface{})
	}

	// Apply hardware-optimized parameters (job parameters take precedence)
	params = agent.ApplyOptimizationToParameters(params, ffmpegOpt)
	
	log.Printf("Using optimized FFmpeg parameters: encoder=%s, preset=%s", 
		params["codec"], params["preset"])

	// Get input file (use test pattern if not specified)
	inputFile := filepath.Join(os.TempDir(), "test_input.mp4")
	if input, ok := params["input"].(string); ok && input != "" {
		inputFile = input
	}

	// Get output file
	outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("job_%s_output.mp4", job.ID))
	if output, ok := params["output"].(string); ok && output != "" {
		outputFile = output
	}

	// Get transcode parameters with defaults
	bitrate := "2000k"
	if b, ok := params["bitrate"].(string); ok && b != "" {
		bitrate = b
	}

	codec := "libx264"
	if c, ok := params["codec"].(string); ok && c != "" {
		// Map common codec names
		switch c {
		case "h264":
			codec = "libx264"
		case "h265", "hevc":
			codec = "libx265"
		case "vp9":
			codec = "libvpx-vp9"
		default:
			codec = c
		}
	}

	preset := "medium"
	if p, ok := params["preset"].(string); ok && p != "" {
		preset = p
	}

	duration := 0
	if d, ok := params["duration"].(float64); ok {
		duration = int(d)
	} else if d, ok := params["duration"].(int); ok {
		duration = d
	}

	// Create test input if it doesn't exist
	if _, statErr := os.Stat(inputFile); os.IsNotExist(statErr) {
		log.Printf("Input file %s not found, creating test video...", inputFile)
		if err := createTestVideo(inputFile, duration); err != nil {
			return nil, nil, fmt.Errorf("failed to create test input: %w", err)
		}
	}

	log.Printf("Transcoding: %s -> %s", inputFile, outputFile)
	log.Printf("  Codec: %s, Bitrate: %s, Preset: %s", codec, bitrate, preset)

	// Verify FFmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Build FFmpeg command
	args := []string{
		"-i", inputFile,
		"-c:v", codec,
		"-b:v", bitrate,
		"-preset", preset,
		"-y", // Overwrite output
	}

	// Add duration limit if specified
	if duration > 0 {
		args = append([]string{"-t", fmt.Sprintf("%d", duration)}, args...)
	}

	args = append(args, outputFile)

	// Execute FFmpeg
	cmd := exec.Command(ffmpegPath, args...)
	
	// Capture output for metrics
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("Running: ffmpeg %s", strings.Join(args, " "))
	
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		log.Printf("FFmpeg stderr: %s", stderr.String())
		return nil, nil, fmt.Errorf("ffmpeg execution failed: %w", err)
	}
	execDuration := time.Since(startTime).Seconds()

	// Verify output was created
	outputInfo, err := os.Stat(outputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("output file not created: %w", err)
	}

	log.Printf("✓ Transcoding completed: %s (%.2f MB in %.2f seconds)", 
		outputFile, float64(outputInfo.Size())/1024/1024, execDuration)

	// Parse FFmpeg output for metrics
	metrics = parseFFmpegMetrics(stderr.String(), execDuration, outputInfo.Size())

	// Generate analyzer output
	analyzerOutput = map[string]interface{}{
		"scenario":            job.Scenario,
		"input_file":          inputFile,
		"output_file":         outputFile,
		"output_size":         outputInfo.Size(),
		"codec":               codec,
		"bitrate":             bitrate,
		"preset":              preset,
		"exec_duration":       execDuration,
		"status":              "success",
		"optimization_reason": ffmpegOpt.Reason,
		"hwaccel":             ffmpegOpt.HWAccel,
	}

	return metrics, analyzerOutput, nil
}

// createTestVideo creates a test video using FFmpeg's test source
func createTestVideo(outputPath string, duration int) error {
	if duration <= 0 {
		duration = 10 // Default 10 seconds
	}

	// Verify FFmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%d:size=1280x720:rate=30", duration),
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-y",
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	
	// Capture output for error reporting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		log.Printf("FFmpeg stderr: %s", stderr.String())
		return fmt.Errorf("failed to create test video: %w", err)
	}

	log.Printf("✓ Created test video: %s (%d seconds)", outputPath, duration)
	return nil
}

// parseFFmpegMetrics extracts metrics from FFmpeg output
func parseFFmpegMetrics(ffmpegOutput string, duration float64, outputSize int64) map[string]interface{} {
	metrics := map[string]interface{}{
		"transcode_duration_sec": duration,
		"output_size_bytes":      outputSize,
		"output_size_mb":         float64(outputSize) / 1024 / 1024,
	}

	// Parse frame count
	if strings.Contains(ffmpegOutput, "frame=") {
		// Extract frame count (example: "frame= 300")
		lines := strings.Split(ffmpegOutput, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.Contains(lines[i], "frame=") {
				fields := strings.Fields(lines[i])
				for j, field := range fields {
					if strings.HasPrefix(field, "frame=") {
						if j+1 < len(fields) {
							metrics["frames_encoded"] = fields[j+1]
						}
						break
					}
				}
				break
			}
		}
	}

	// Parse FPS
	if strings.Contains(ffmpegOutput, "fps=") {
		lines := strings.Split(ffmpegOutput, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.Contains(lines[i], "fps=") {
				fields := strings.Fields(lines[i])
				for j, field := range fields {
					if strings.HasPrefix(field, "fps=") {
						if j+1 < len(fields) {
							metrics["encoding_fps"] = fields[j+1]
						}
						break
					}
				}
				break
			}
		}
	}

	return metrics
}
