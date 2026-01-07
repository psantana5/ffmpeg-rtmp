package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/psantana5/ffmpeg-rtmp/pkg/agent"
	"github.com/psantana5/ffmpeg-rtmp/pkg/logging"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/shutdown"
	tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"
	"github.com/psantana5/ffmpeg-rtmp/worker/exporters/prometheus"
	"github.com/psantana5/ffmpeg-rtmp/worker/pkg/resources"
)

var logger *logging.Logger

func main() {
	masterURL := flag.String("master", "http://localhost:8080", "Master node URL")
	register := flag.Bool("register", false, "Register with master node")
	pollInterval := flag.Duration("poll-interval", 10*time.Second, "Job polling interval")
	heartbeatInterval := flag.Duration("heartbeat-interval", 30*time.Second, "Heartbeat interval")
	allowMasterAsWorker := flag.Bool("allow-master-as-worker", false, "Allow registering master node as worker (development mode)")
	skipConfirmation := flag.Bool("skip-confirmation", false, "Skip confirmation prompts (for automated testing)")
	apiKeyFlag := flag.String("api-key", "", "API key for authentication (or use FFMPEG_RTMP_API_KEY env var)")
	certFile := flag.String("cert", "", "TLS client certificate file (for mTLS)")
	keyFile := flag.String("key", "", "TLS client key file (for mTLS)")
	caFile := flag.String("ca", "", "CA certificate file to verify server")
	insecureSkipVerify := flag.Bool("insecure-skip-verify", false, "Skip TLS certificate verification (insecure, for development only)")
	metricsPort := flag.String("metrics-port", "9091", "Prometheus metrics port")
	generateInput := flag.Bool("generate-input", true, "Automatically generate input videos for jobs (default: true)")
	maxConcurrentJobs := flag.Int("max-concurrent-jobs", 1, "Maximum number of concurrent jobs to process (default: 1)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	
	// Auto-attach flags
	enableAutoAttach := flag.Bool("enable-auto-attach", false, "Enable automatic discovery and attachment to running FFmpeg processes")
	autoAttachScanInterval := flag.Duration("auto-attach-scan-interval", 10*time.Second, "Scan interval for auto-attach (default: 10s)")
	autoAttachCPUQuota := flag.Int("auto-attach-cpu-quota", 0, "Default CPU quota for auto-attached processes (0=unlimited)")
	autoAttachMemLimit := flag.Int("auto-attach-memory-limit", 0, "Default memory limit in MB for auto-attached processes (0=unlimited)")
	
	flag.Parse()

	// Initialize file logger: /var/log/ffrtmp/worker/agent.log
	var err error
	logger, err = logging.NewFileLogger("worker", "agent", logging.ParseLevel(*logLevel), false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

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

	logger.Info("Starting FFmpeg RTMP Distributed Compute Agent (Production Mode)")
	logger.Info(fmt.Sprintf("Master URL: %s", *masterURL))

	// Detect hardware capabilities
	log.Println("Detecting hardware capabilities...")
	caps, err := agent.DetectHardware()
	if err != nil {
		log.Fatalf("Failed to detect hardware: %v", err)
	}

	// Detect encoder capabilities with runtime validation
	log.Println("Detecting and validating encoders...")
	encoderCaps := agent.DetectEncoders()
	log.Printf("Selected H.264 encoder: %s", encoderCaps.SelectedH264)
	log.Printf("Selected H.265 encoder: %s", encoderCaps.SelectedH265)
	log.Printf("Reason: %s", encoderCaps.GetEncoderReason())
	
	// Log validation results
	nvencAvailable := encoderCaps.ValidationResults["h264_nvenc"]
	qsvAvailable := encoderCaps.ValidationResults["h264_qsv"]
	vaapiAvailable := encoderCaps.ValidationResults["h264_vaapi"]
	
	log.Printf("Encoder Runtime Validation:")
	log.Printf("  NVENC: %v", nvencAvailable)
	log.Printf("  QSV: %v", qsvAvailable)
	log.Printf("  VAAPI: %v", vaapiAvailable)
	
	// Log validation failures
	for encoder, reason := range encoderCaps.ValidationReasons {
		if reason != "" {
			log.Printf("  %s validation failed: %s", encoder, reason)
		}
	}

	// Determine node type
	nodeType := agent.DetectNodeType(caps.CPUThreads, caps.RAMTotalBytes)

	log.Println("Hardware detected:")
	log.Printf("  CPU: %s (%d threads)", caps.CPUModel, caps.CPUThreads)
	log.Printf("  RAM: %s", agent.FormatRAM(caps.RAMTotalBytes))
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

	// Create engine selector for dual-engine support
	log.Println("Initializing transcoding engines...")
	engineSelector := agent.NewEngineSelector(caps, nodeType)
	availableEngines := engineSelector.GetAvailableEngines()
	log.Printf("  Available engines: %v", availableEngines)

	// Create input generator
	inputGenerator := agent.NewInputGenerator(encoderCaps)
	log.Printf("Input generation: %s", func() string {
		if *generateInput {
			return "enabled"
		}
		return "disabled"
	}())

	// Start Prometheus metrics exporter
	hostname, _ := os.Hostname()
	nodeID := fmt.Sprintf("%s:%s", hostname, *metricsPort)
	metricsExporter := prometheus.NewWorkerExporter(nodeID, caps.HasGPU)
	
	metricsRouter := mux.NewRouter()
	metricsRouter.Handle("/metrics", metricsExporter).Methods("GET")
	
	// Health check endpoint (liveness probe)
	metricsRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	}).Methods("GET")
	
	// Readiness check endpoint (readiness probe)
	// Note: Master reachability check is deferred until after registration
	var readyClient *agent.Client // Will be set after client creation
	metricsRouter.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check if worker can accept jobs
		ready := true
		checks := make(map[string]string)
		
		// Check 1: FFmpeg availability
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			checks["ffmpeg"] = "not_found"
			ready = false
		} else {
			checks["ffmpeg"] = "available"
		}
		
		// Check 2: Disk space (require at least 10% free)
		diskInfo, err := resources.CheckDiskSpace("/tmp")
		if err != nil {
			checks["disk_space"] = fmt.Sprintf("error: %v", err)
			ready = false
		} else if diskInfo.UsedPercent > 90 {
			checks["disk_space"] = fmt.Sprintf("low: %.1f%% used", diskInfo.UsedPercent)
			ready = false
		} else {
			checks["disk_space"] = fmt.Sprintf("ok: %.1f%% used, %d MB available", diskInfo.UsedPercent, diskInfo.AvailableMB)
		}
		
		// Check 3: Master reachability (skip if not registered yet)
		if readyClient != nil && readyClient.GetNodeID() != "" {
			// Try to send a heartbeat as a connectivity test
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			done := make(chan error, 1)
			go func() {
				done <- readyClient.SendHeartbeat()
			}()
			
			select {
			case err := <-done:
				if err != nil {
					checks["master"] = fmt.Sprintf("unreachable: %v", err)
					ready = false
				} else {
					checks["master"] = "reachable"
				}
			case <-ctx.Done():
				checks["master"] = "timeout"
				ready = false
			}
		} else {
			checks["master"] = "not_registered"
		}
		
		if ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready","checks":` + toJSON(checks) + `,"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready","checks":` + toJSON(checks) + `,"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		}
	}).Methods("GET")
	
	// Wrapper metrics endpoint (Prometheus format)
	metricsRouter.HandleFunc("/wrapper/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		// Import from internal/report
		metricsOutput := getWrapperMetrics()
		w.Write([]byte(metricsOutput))
	}).Methods("GET")
	
	// Violations endpoint (debugging)
	metricsRouter.HandleFunc("/violations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		violationsOutput := getWrapperViolations()
		w.Write([]byte(violationsOutput))
	}).Methods("GET")
	
	// Set encoder availability metrics
	metricsExporter.SetEncoderAvailability(nvencAvailable, qsvAvailable, vaapiAvailable)
	
	metricsServer := &http.Server{
		Addr:    ":" + *metricsPort,
		Handler: metricsRouter,
	}
	
	go func() {
		log.Printf("✓ Prometheus metrics endpoint: http://localhost:%s/metrics", *metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

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
		
		// Auto-enable InsecureSkipVerify for localhost when no CA is provided
		// This handles common development scenarios with self-signed certificates
		isLocalhost := isLocalhostURL(*masterURL)
		
		if *insecureSkipVerify {
			log.Println("WARNING: TLS certificate verification disabled (insecure)")
			tlsConfig.InsecureSkipVerify = true
		} else if *caFile == "" && isLocalhost {
			log.Println("Using self-signed certificate mode for localhost")
			log.Println("  → TLS certificate verification disabled for localhost/127.0.0.1")
			log.Println("  → For production, use --ca flag to verify server certificates")
			tlsConfig.InsecureSkipVerify = true
		}
		
		client = agent.NewClientWithTLS(*masterURL, tlsConfig)
		if *caFile == "" && !*insecureSkipVerify && !isLocalhost {
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
	
	// Set client for readiness checks
	readyClient = client

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

			// Ask for confirmation (skip in automated testing)
			if !*skipConfirmation {
				if !confirmMasterAsWorker() {
					log.Println("Registration cancelled by user.")
					os.Exit(0)
				}
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
			Address:       hostname,
			Type:          nodeType,
			CPUThreads:    caps.CPUThreads,
			CPUModel:      caps.CPUModel,
			HasGPU:        caps.HasGPU,
			GPUType:       caps.GPUType,
			RAMTotalBytes: caps.RAMTotalBytes,
			Labels:        caps.Labels,
		}

		node, err := client.Register(reg)
		if err != nil {
			log.Fatalf("Failed to register with master: %v", err)
		}

		// Check if this was a re-registration (node already existed)
		log.Printf("✓ Registered successfully!")
		log.Printf("  Node ID: %s", node.ID)
		log.Printf("  Node Name: %s", node.Name)
		log.Printf("  Status: %s", node.Status)
		if node.RegisteredAt.Before(time.Now().Add(-1 * time.Minute)) {
			// Node was registered more than 1 minute ago - this is a re-registration
			log.Printf("  Note: Reconnected to existing registration (registered at: %s)", node.RegisteredAt.Format(time.RFC3339))
		}
	} else {
		log.Println("Running in standalone mode (no registration)")
		log.Println("Use --register flag to register with master node")
		return
	}

	// Initialize graceful shutdown manager
	shutdownMgr := shutdown.New(30 * time.Second)
	
	// Register shutdown handlers
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Stopping metrics server...")
		return metricsServer.Shutdown(ctx)
	})
	
	shutdownMgr.Register(func(ctx context.Context) error {
		logger.Info("Closing logger...")
		return logger.Close()
	})
	
	// Start auto-attach service if enabled
	var autoAttachService interface {
		Start(context.Context) error
		Stop()
	}
	
	if *enableAutoAttach {
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("Auto-Attach Service Enabled")
		log.Printf("  Scan Interval: %v", *autoAttachScanInterval)
		log.Printf("  CPU Quota: %d", *autoAttachCPUQuota)
		log.Printf("  Memory Limit: %d MB", *autoAttachMemLimit)
		log.Println("  This will automatically discover and govern running FFmpeg processes")
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		
		// Note: Import and use internal/discover package
		// This would require importing the package from the main project
		// For now, log that it would be enabled
		log.Println("⚠️  Auto-attach requires internal/discover package")
	}
	
	// Start shutdown signal handler
	go func() {
		shutdownMgr.Wait()
		logger.Info("Shutdown signal received")
	}()

	// Start heartbeat loop
	heartbeatTicker := time.NewTicker(*heartbeatInterval)
	defer heartbeatTicker.Stop()
	
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		for {
			select {
			case <-heartbeatTicker.C:
				if err := client.SendHeartbeat(); err != nil {
					log.Printf("Heartbeat failed: %v", err)
				} else {
					log.Println("Heartbeat sent")
					metricsExporter.IncrementHeartbeat()
				}
			case <-shutdownMgr.Done():
				logger.Info("Stopping heartbeat loop")
				return
			}
		}
	}()

	// Main job polling loop with concurrent job processing
	log.Printf("Starting job polling loop (max concurrent jobs: %d)...", *maxConcurrentJobs)
	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	// Semaphore to limit concurrent jobs
	jobSemaphore := make(chan struct{}, *maxConcurrentJobs)
	activeJobsCount := 0
	var activeJobsMutex sync.Mutex
	var wg sync.WaitGroup

	for {
		select {
		case <-ticker.C:
			// Check if we can accept more jobs
			activeJobsMutex.Lock()
			currentActive := activeJobsCount
			activeJobsMutex.Unlock()

			if currentActive >= *maxConcurrentJobs {
				log.Printf("At max concurrent jobs (%d/%d), waiting...", currentActive, *maxConcurrentJobs)
				continue
			}

			job, err := client.GetNextJob()
			if err != nil {
				log.Printf("Failed to get next job: %v", err)
				continue
			}

			if job == nil {
				// log.Println("No jobs available")  // Too verbose, comment out
				continue
			}

			log.Printf("Received job: %s (scenario: %s)", job.ID, job.Scenario)

			// Acquire semaphore slot
			jobSemaphore <- struct{}{}
			
			// Increment active jobs counter
			activeJobsMutex.Lock()
			activeJobsCount++
			currentActive = activeJobsCount
			activeJobsMutex.Unlock()
			metricsExporter.SetActiveJobs(currentActive)

			// Execute job concurrently
			wg.Add(1)
			go func(j *models.Job) {
				defer wg.Done()
				defer func() {
					// Release semaphore slot
					<-jobSemaphore
					
					// Decrement active jobs counter
					activeJobsMutex.Lock()
					activeJobsCount--
					currentActive := activeJobsCount
					activeJobsMutex.Unlock()
					metricsExporter.SetActiveJobs(currentActive)
				}()

				// Execute job with hardware-optimized parameters
				result := executeJob(j, client, ffmpegOpt, engineSelector, inputGenerator, *generateInput, metricsExporter)

				// Send results
				if err := client.SendResults(result); err != nil {
					log.Printf("Failed to send results: %v", err)
				} else {
					log.Printf("Results sent for job %s (status: %s)", j.ID, result.Status)
				}
			}(job)
			
		case <-shutdownMgr.Done():
			logger.Info("Shutdown signal received - stopping job polling")
			logger.Info("Waiting for active jobs to complete...")
			
			// Wait for all jobs to complete with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			
			select {
			case <-done:
				logger.Info("All jobs completed successfully")
			case <-time.After(30 * time.Second):
				logger.Warn("Timeout waiting for jobs - some jobs may not have completed")
			}
			
			// Wait for heartbeat to stop
			<-heartbeatDone
			
			// Execute shutdown handlers
			shutdownMgr.Shutdown()
			
			logger.Info("Worker agent shutdown complete")
			return
		}
	}
}

// isLocalhostURL checks if a URL points to localhost
// Returns true only if the hostname component is localhost, 127.0.0.1, or ::1
func isLocalhostURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	
	// Get hostname without port
	hostname := parsedURL.Hostname()
	
	// Check for localhost variations
	return hostname == "localhost" || 
		   hostname == "127.0.0.1" || 
		   hostname == "::1"
}

// isMasterAsWorker checks if the agent is trying to register on the same machine as master
func isMasterAsWorker(masterURL, hostname string) bool {
	// Check if master URL points to localhost
	if isLocalhostURL(masterURL) {
		return true
	}

	// Check if master URL contains the local hostname
	parsedURL, err := url.Parse(masterURL)
	if err == nil {
		urlHostname := parsedURL.Hostname()
		if urlHostname == hostname {
			return true
		}
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
func executeJob(job *models.Job, client *agent.Client, ffmpegOpt *agent.FFmpegOptimization, engineSelector *agent.EngineSelector, inputGenerator *agent.InputGenerator, generateInputFlag bool, metricsExporter *prometheus.WorkerExporter) *models.JobResult {
	log.Printf("╔════════════════════════════════════════════════════════════════╗")
	log.Printf("║ EXECUTING JOB: %s", job.ID)
	log.Printf("╠════════════════════════════════════════════════════════════════╣")
	log.Printf("║ Scenario: %s", job.Scenario)
	log.Printf("║ Job Parameters:")
	if job.Parameters != nil {
		for key, value := range job.Parameters {
			log.Printf("║   %s: %v", key, value)
		}
	} else {
		log.Printf("║   (no parameters specified - using defaults)")
	}
	log.Printf("╚════════════════════════════════════════════════════════════════╝")
	
	startTime := time.Now()

	// Check disk space before starting job
	log.Println("\n>>> RESOURCE CHECK PHASE <<<")
	diskInfo, err := resources.CheckDiskSpace("/tmp")
	if err != nil {
		log.Printf("⚠️  WARNING: Failed to check disk space: %v", err)
	} else {
		log.Printf("Disk space: %.1f%% used (%d MB available)", diskInfo.UsedPercent, diskInfo.AvailableMB)
		if diskInfo.UsedPercent > 95 {
			errMsg := fmt.Sprintf("insufficient disk space: %.1f%% used", diskInfo.UsedPercent)
			log.Printf("❌ %s", errMsg)
			return &models.JobResult{
				JobID:       job.ID,
				NodeID:      client.GetNodeID(),
				Status:      models.JobStatusFailed,
				Error:       errMsg,
				Logs:        fmt.Sprintf("=== Resource Check Failed ===\n%s\n", errMsg),
				CompletedAt: time.Now(),
			}
		}
	}
	
	// Parse resource limits from job parameters
	limits := resources.DefaultLimits()
	if job.Parameters != nil {
		if resourceLimits, ok := job.Parameters["resource_limits"].(map[string]interface{}); ok {
			if maxCPU, ok := resourceLimits["max_cpu_percent"].(float64); ok {
				limits.MaxCPUPercent = int(maxCPU)
			}
			if maxMem, ok := resourceLimits["max_memory_mb"].(float64); ok {
				limits.MaxMemoryMB = int(maxMem)
			}
			if maxDisk, ok := resourceLimits["max_disk_mb"].(float64); ok {
				limits.MaxDiskMB = int(maxDisk)
			}
			if timeout, ok := resourceLimits["timeout_sec"].(float64); ok {
				limits.TimeoutSec = int(timeout)
			}
		}
	}
	log.Printf("Resource limits: CPU=%d%%, Memory=%dMB, Disk=%dMB, Timeout=%ds", 
		limits.MaxCPUPercent, limits.MaxMemoryMB, limits.MaxDiskMB, limits.TimeoutSec)

	// Generate input video if needed
	var inputGenResult *agent.InputGenerationResult
	var generatedInputPath string
	if agent.ShouldGenerateInput(job, generateInputFlag) {
		log.Println("\n>>> INPUT GENERATION PHASE <<<")
		var err error
		inputGenResult, err = inputGenerator.GenerateInput(job)
		if err != nil {
			errMsg := fmt.Sprintf("input generation failed: %v", err)
			log.Printf("❌ Failed to generate input: %v", err)
			return &models.JobResult{
				JobID:       job.ID,
				NodeID:      client.GetNodeID(),
				Status:      models.JobStatusFailed,
				Error:       errMsg,
				Logs:        fmt.Sprintf("=== Input Generation Failed ===\n%s\n", errMsg),
				CompletedAt: time.Now(),
			}
		}
		
		// Record metrics
		metricsExporter.RecordInputGeneration(inputGenResult.GenerationTime, inputGenResult.FileSizeBytes)
		
		generatedInputPath = inputGenResult.FilePath
		
		// Set input path in job parameters if not already set
		if job.Parameters == nil {
			job.Parameters = make(map[string]interface{})
		}
		if _, exists := job.Parameters["input"]; !exists {
			job.Parameters["input"] = inputGenResult.FilePath
			log.Printf("✓ Input path added to job parameters: %s", inputGenResult.FilePath)
		}
	} else {
		log.Println("\n>>> SKIPPING INPUT GENERATION <<<")
		if job.Parameters != nil {
			if input, ok := job.Parameters["input"].(string); ok {
				log.Printf("Using existing input file: %s", input)
			}
		}
	}

	// Select the best engine for this job
	log.Println("\n>>> ENGINE SELECTION PHASE <<<")
	selectedEngine, reason := engineSelector.SelectEngine(job)
	log.Printf("Selected Engine: %s", selectedEngine.Name())
	log.Printf("Selection Reason: %s", reason)

	// Execute the actual job with the selected engine
	log.Println("\n>>> TRANSCODING EXECUTION PHASE <<<")
	
	// Get input file path for bandwidth tracking
	var inputFilePath string
	if job.Parameters != nil {
		if input, ok := job.Parameters["input"].(string); ok {
			inputFilePath = input
		}
	}
	
	// Get input file size before execution
	inputFileSize := getFileSize(inputFilePath)
	if inputFileSize > 0 {
		log.Printf("Input file size: %.2f MB", float64(inputFileSize)/(1024*1024))
	}
	
	metrics, analyzerOutput, executionLogs, cancelResult, err := executeEngineJob(job, client, selectedEngine, ffmpegOpt, limits, metricsExporter)
	
	// Extract output file path for cleanup (if present in job parameters)
	var outputFilePath string
	if job.Parameters != nil {
		if output, ok := job.Parameters["output"].(string); ok {
			outputFilePath = output
		}
	}
	
	// Cleanup generated input if needed
	log.Println("\n>>> CLEANUP PHASE <<<")
	persistInputs := os.Getenv("PERSIST_INPUTS") == "true"
	persistOutputs := os.Getenv("PERSIST_OUTPUTS") == "true"
	
	if generatedInputPath != "" && !persistInputs {
		log.Printf("PERSIST_INPUTS=false, cleaning up generated input...")
		if cleanupErr := inputGenerator.CleanupInput(generatedInputPath); cleanupErr != nil {
			log.Printf("⚠️  Warning: failed to cleanup input file: %v", cleanupErr)
		}
	} else if generatedInputPath != "" && persistInputs {
		log.Printf("PERSIST_INPUTS=true, keeping input file: %s", generatedInputPath)
	} else {
		log.Printf("No generated input to cleanup")
	}
	
	// Cleanup temporary output files (test artifacts) if configured
	if outputFilePath != "" && !persistOutputs {
		// Only cleanup files in /tmp with job_*_output.mp4 pattern (test artifacts)
		// NEVER delete user-specified output paths or files outside /tmp
		if strings.HasPrefix(outputFilePath, os.TempDir()) && 
		   strings.Contains(filepath.Base(outputFilePath), "_output.mp4") &&
		   strings.HasPrefix(filepath.Base(outputFilePath), "job_") {
			log.Printf("PERSIST_OUTPUTS=false, cleaning up temporary output: %s", outputFilePath)
			if cleanupErr := os.Remove(outputFilePath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				log.Printf("⚠️  Warning: failed to cleanup output file: %v", cleanupErr)
			} else if cleanupErr == nil {
				log.Printf("✓ Cleaned up temporary output file")
			}
		} else {
			log.Printf("Keeping output file (not a temporary test artifact): %s", outputFilePath)
		}
	} else if outputFilePath != "" && persistOutputs {
		log.Printf("PERSIST_OUTPUTS=true, keeping output file: %s", outputFilePath)
	}
	
	duration := time.Since(startTime).Seconds()
	
	// Check if job was canceled
	if cancelResult != nil && cancelResult.WasCanceled {
		// Job was canceled - don't record for SLA, return canceled status
		log.Printf("\n╔════════════════════════════════════════════════════════════════╗")
		log.Printf("║ 🛑 JOB CANCELED: %s", job.ID)
		log.Printf("║ Duration before cancellation: %.2f seconds", duration)
		terminationType := "gracefully (SIGTERM)"
		if !cancelResult.Graceful {
			terminationType = "forcefully (SIGKILL)"
		}
		log.Printf("║ Termination: %s", terminationType)
		log.Printf("╚════════════════════════════════════════════════════════════════╝\n")
		return &models.JobResult{
			JobID:       job.ID,
			NodeID:      client.GetNodeID(),
			Status:      models.JobStatusCanceled,
			Error:       "Job canceled by user",
			Logs:        executionLogs,
			CompletedAt: time.Now(),
			Metrics: map[string]interface{}{
				"duration": duration,
				"engine":   selectedEngine.Name(),
				"canceled": true,
				"graceful_termination": cancelResult.Graceful,
			},
		}
	}
	
	if err != nil {
		// Set completion time and failure reason
		now := time.Now()
		job.CompletedAt = &now
		
		// Determine failure reason based on error type
		// This is critical for platform SLA calculation
		errorStr := err.Error()
		if strings.Contains(errorStr, "invalid") || strings.Contains(errorStr, "bad parameter") {
			job.FailureReason = models.FailureReasonUserError
		} else if strings.Contains(errorStr, "network") || strings.Contains(errorStr, "connection") {
			job.FailureReason = models.FailureReasonNetworkError
		} else if strings.Contains(errorStr, "input") || strings.Contains(errorStr, "corrupt") {
			job.FailureReason = models.FailureReasonInputError
		} else if strings.Contains(errorStr, "resource") || strings.Contains(errorStr, "out of memory") {
			job.FailureReason = models.FailureReasonResourceError
		} else if strings.Contains(errorStr, "timeout") {
			job.FailureReason = models.FailureReasonTimeout
		} else {
			// Default to runtime error - will be treated as external by SLA logic
			job.FailureReason = models.FailureReasonRuntimeError
		}
		
		job.Status = models.JobStatusFailed
		
		// Record for platform SLA tracking
		slaTargets := models.GetDefaultSLATimingTargets()
		metricsExporter.RecordJobCompletion(job, slaTargets)
		
		// Check if this was a platform failure
		isPlatformFailure := job.IsPlatformFailure()
		failureType := "External"
		if isPlatformFailure {
			failureType = "Platform"
		}
		
		log.Printf("\n╔════════════════════════════════════════════════════════════════╗")
		log.Printf("║ ❌ JOB FAILED: %s", job.ID)
		log.Printf("║ Error: %v", err)
		log.Printf("║ Duration: %.2f seconds", duration)
		log.Printf("║ Failure Reason: %s", job.FailureReason)
		log.Printf("║ Failure Type: %s (%s our fault)", failureType, 
			map[bool]string{true: "IS", false: "NOT"}[isPlatformFailure])
		log.Printf("╚════════════════════════════════════════════════════════════════╝\n")
		return &models.JobResult{
			JobID:       job.ID,
			NodeID:      client.GetNodeID(),
			Status:      models.JobStatusFailed,
			Error:       err.Error(),
			Logs:        executionLogs,
			CompletedAt: time.Now(),
			Metrics: map[string]interface{}{
				"duration": duration,
				"engine":   selectedEngine.Name(),
			},
		}
	}

	// Add duration and engine to metrics
	if metrics == nil {
		metrics = make(map[string]interface{})
	}
	metrics["duration"] = duration
	metrics["scenario"] = job.Scenario
	metrics["engine"] = selectedEngine.Name()
	
	// Add SLA tracking to metrics
	slaTargets := models.GetDefaultSLATimingTargets()
	platformCompliant, slaReason := job.CalculatePlatformSLACompliance(slaTargets)
	
	// Calculate timing metrics
	var queueTime, processingTime float64
	if job.StartedAt != nil {
		queueTime = job.StartedAt.Sub(job.CreatedAt).Seconds()
		if job.CompletedAt != nil {
			processingTime = job.CompletedAt.Sub(*job.StartedAt).Seconds()
		}
	}
	
	metrics["platform_sla_compliant"] = platformCompliant
	metrics["platform_sla_reason"] = slaReason
	metrics["sla_worthy"] = job.IsSLAWorthy()
	metrics["sla_category"] = job.GetSLACategory()
	metrics["queue_time_seconds"] = queueTime
	metrics["processing_time_seconds"] = processingTime
	
	// Add input generation metrics if applicable
	if inputGenResult != nil {
		metrics["input_generation_duration_sec"] = inputGenResult.GenerationTime
		metrics["input_file_size_bytes"] = inputGenResult.FileSizeBytes
		metrics["input_encoder_used"] = inputGenResult.EncoderUsed
	}
	
	// Track bandwidth metrics (reuse outputFilePath from cleanup section)
	if job.Parameters != nil {
		if output, ok := job.Parameters["output"].(string); ok {
			if outputFilePath == "" {
				outputFilePath = output
			}
		}
	}
	
	// Get file sizes and record bandwidth
	inputBandwidthSize := getFileSize(inputFilePath)
	outputFileSize := getFileSize(outputFilePath)
	
	if inputBandwidthSize > 0 || outputFileSize > 0 {
		// Record bandwidth metrics
		metricsExporter.RecordJobBandwidth(inputBandwidthSize, outputFileSize, duration)
		
		// Add to job metrics
		metrics["input_file_bytes"] = inputBandwidthSize
		metrics["output_file_bytes"] = outputFileSize
		if duration > 0 {
			totalBytes := inputBandwidthSize + outputFileSize
			bandwidthMbps := (float64(totalBytes) * 8) / (duration * 1024 * 1024)
			metrics["bandwidth_mbps"] = bandwidthMbps
		}
		
		log.Printf("Bandwidth tracking: input=%.2f MB, output=%.2f MB", 
			float64(inputBandwidthSize)/(1024*1024), float64(outputFileSize)/(1024*1024))
	}
	
	// Set completion time
	now := time.Now()
	job.CompletedAt = &now
	job.Status = models.JobStatusCompleted
	
	// Record successful job completion for platform SLA tracking
	slaTargets2 := models.GetDefaultSLATimingTargets()
	metricsExporter.RecordJobCompletion(job, slaTargets2)

	// Check if platform met SLA obligations
	platformCompliant2, slaReason2 := job.CalculatePlatformSLACompliance(slaTargets2)
	
	slaStatus := "✅ PLATFORM SLA MET"
	if !platformCompliant2 {
		slaStatus = fmt.Sprintf("⚠️  PLATFORM SLA VIOLATED (%s)", slaReason2)
	}
	if !job.IsSLAWorthy() {
		slaCategory := job.GetSLACategory()
		slaStatus = fmt.Sprintf("ℹ️  NOT SLA-WORTHY (%s)", slaCategory)
	}

	// Calculate timing metrics
	var queueTime2, processingTime2 float64
	if job.StartedAt != nil {
		queueTime2 = job.StartedAt.Sub(job.CreatedAt).Seconds()
		processingTime2 = job.CompletedAt.Sub(*job.StartedAt).Seconds()
	}

	log.Printf("\n╔════════════════════════════════════════════════════════════════╗")
	log.Printf("║ ✅ JOB COMPLETED: %s", job.ID)
	log.Printf("║ Total Duration: %.2f seconds", duration)
	log.Printf("║ Queue Time: %.2f seconds (target: %.0fs)", queueTime2, slaTargets2.MaxQueueTimeSeconds)
	log.Printf("║ Processing Time: %.2f seconds (target: %.0fs)", processingTime2, slaTargets2.MaxProcessingSeconds)
	log.Printf("║ Platform SLA: %s", slaStatus)
	log.Printf("║ Engine Used: %s", selectedEngine.Name())
	if inputGenResult != nil {
		log.Printf("║ Input Generation: %.2f seconds", inputGenResult.GenerationTime)
	}
	log.Printf("╚════════════════════════════════════════════════════════════════╝\n")
	return &models.JobResult{
		JobID:          job.ID,
		NodeID:         client.GetNodeID(),
		Status:         models.JobStatusCompleted,
		Logs:           executionLogs,
		CompletedAt:    time.Now(),
		Metrics:        metrics,
		AnalyzerOutput: analyzerOutput,
	}
}

// CancellationResult indicates how a job was canceled
type CancellationResult struct {
	WasCanceled bool
	Graceful    bool // true if SIGTERM worked, false if SIGKILL was needed
}

// monitorJobCancellation periodically checks if a job has been canceled
// If canceled, it gracefully terminates the process (SIGTERM, then SIGKILL after 30s)
func monitorJobCancellation(jobID string, client *agent.Client, cmd *exec.Cmd, canceledChan chan CancellationResult, doneChan chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-doneChan:
			// Job completed normally
			return
		case <-ticker.C:
			// Check job status
			job, err := client.GetJob(jobID)
			if err != nil {
				log.Printf("WARNING: Failed to check job status for cancellation: %v", err)
				continue
			}
			
			if job.Status == models.JobStatusCanceled {
				log.Printf("🛑 Job %s has been canceled, terminating process...", jobID)
				
				// Try graceful termination first (SIGTERM)
				if cmd.Process != nil {
					pgid, err := syscall.Getpgid(cmd.Process.Pid)
					if err == nil {
						log.Printf("Sending SIGTERM to process group %d", pgid)
						syscall.Kill(-pgid, syscall.SIGTERM)
						
						// Wait 30 seconds for graceful shutdown
						gracefulTimer := time.NewTimer(30 * time.Second)
						select {
						case <-doneChan:
							gracefulTimer.Stop()
							log.Printf("✓ Process terminated gracefully with SIGTERM")
							canceledChan <- CancellationResult{WasCanceled: true, Graceful: true}
							return
						case <-gracefulTimer.C:
							// Force kill after timeout
							log.Printf("⚠️  Process did not terminate gracefully, sending SIGKILL")
							syscall.Kill(-pgid, syscall.SIGKILL)
							canceledChan <- CancellationResult{WasCanceled: true, Graceful: false}
							return
						}
					} else {
						// Fallback: kill just the process
						log.Printf("Could not get process group, killing process directly")
						cmd.Process.Kill()
						canceledChan <- CancellationResult{WasCanceled: true, Graceful: false}
						return
					}
				}
			}
		}
	}
}

// executeEngineJob executes a job using the selected engine with resource limits
func executeEngineJob(job *models.Job, client *agent.Client, engine agent.Engine, ffmpegOpt *agent.FFmpegOptimization, limits *resources.ResourceLimits, metricsExporter *prometheus.WorkerExporter) (metrics map[string]interface{}, analyzerOutput map[string]interface{}, logs string, cancelResult *CancellationResult, err error) {
	// Build command using the selected engine
	args, err := engine.BuildCommand(job, client.GetMasterURL())
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("failed to build command: %w", err)
	}

	// Determine the command to run based on engine
	var cmdPath string
	var cmdName string
	
	if engine.Name() == "gstreamer" {
		cmdPath, err = exec.LookPath("gst-launch-1.0")
		cmdName = "gst-launch-1.0"
		if err != nil {
			return nil, nil, "", nil, fmt.Errorf("gst-launch-1.0 not found in PATH: %w", err)
		}
	} else {
		cmdPath, err = exec.LookPath("ffmpeg")
		cmdName = "ffmpeg"
		if err != nil {
			return nil, nil, "", nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
		}
	}

	// Check if wrapper is enabled for this job
	if job.WrapperEnabled {
		return executeWithWrapperPath(job, cmdPath, cmdName, args, limits, metricsExporter)
	}

	// Get duration for timeout calculation
	duration := 0
	if job.Parameters != nil {
		if d, ok := job.Parameters["duration"].(float64); ok {
			duration = int(d)
		} else if d, ok := job.Parameters["duration"].(int); ok {
			duration = d
		} else if d, ok := job.Parameters["duration_seconds"].(float64); ok {
			duration = int(d)
		} else if d, ok := job.Parameters["duration_seconds"].(int); ok {
			duration = d
		}
	}

	// Determine timeout from either duration or resource limits
	var timeoutDuration time.Duration
	if limits.TimeoutSec > 0 {
		timeoutDuration = time.Duration(limits.TimeoutSec) * time.Second
		log.Printf("→ Using resource limit timeout: %d seconds", limits.TimeoutSec)
	} else if engine.Name() == "gstreamer" && duration > 0 {
		// GStreamer needs explicit timeout since it runs continuously
		timeoutDuration = time.Duration(duration+30) * time.Second
		log.Printf("→ GStreamer job will timeout after %d seconds (duration: %d + 30s buffer)", duration+30, duration)
	} else if duration > 0 {
		// FFmpeg handles duration internally, but add safety timeout
		timeoutDuration = time.Duration(duration*2 + 60) * time.Second // 2x duration + 1 minute buffer
	} else {
		// No duration specified - use default 10 minute timeout
		timeoutDuration = 10 * time.Minute
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	// Execute the command with context
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	
	// Set up process group for easier cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	
	// Capture output for metrics
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("Running: %s %s", cmdName, strings.Join(args, " "))
	
	// Start the command
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, nil, "", nil, fmt.Errorf("failed to start %s: %w", cmdName, err)
	}
	
	pid := cmd.Process.Pid
	log.Printf("Process started with PID: %d", pid)
	
	// Set process priority (nice value) for lower priority
	if err := resources.SetProcessPriority(pid, 10); err != nil {
		log.Printf("WARNING: Failed to set process priority: %v", err)
	} else {
		log.Printf("Set process priority: nice=10 (lower than normal)")
	}
	
	// Create cgroup for resource limits (best effort, will warn if fails)
	cgroupMgr, err := resources.NewCgroupManager()
	cgroupPath := ""
	if err != nil {
		log.Printf("WARNING: Failed to initialize cgroup manager: %v", err)
	} else {
		cgroupPath, err = cgroupMgr.CreateCgroup(job.ID, limits)
		if err != nil {
			log.Printf("WARNING: Failed to create cgroup: %v", err)
		} else if cgroupPath != "" {
			// Add process to cgroup
			if err := cgroupMgr.AddProcessToCgroup(cgroupPath, pid); err != nil {
				log.Printf("WARNING: Failed to add process to cgroup: %v", err)
			} else {
				log.Printf("✓ Process added to cgroup: %s", cgroupPath)
			}
		}
	}
	
	// Monitor process for resource limits and timeout
	doneChan := make(chan struct{})
	canceledChan := make(chan CancellationResult, 1)
	go resources.MonitorProcess(cmd, limits, doneChan)
	
	// Monitor for job cancellation (check every 5 seconds)
	go monitorJobCancellation(job.ID, client, cmd, canceledChan, doneChan)
	
	// Wait for command to complete
	execErr := cmd.Wait()
	close(doneChan) // Stop monitoring
	execDuration := time.Since(startTime).Seconds()
	
	// Check if job was canceled
	var cancelResultData CancellationResult
	select {
	case cancelResultData = <-canceledChan:
		// Job was canceled
	default:
		// Job completed normally or failed
	}
	
	// Cleanup cgroup
	if cgroupPath != "" && cgroupMgr != nil {
		if err := cgroupMgr.RemoveCgroup(cgroupPath); err != nil {
			log.Printf("WARNING: Failed to remove cgroup: %v", err)
		}
	}
	
	// Capture all logs (combine stdout and stderr for comprehensive trace)
	var logBuffer bytes.Buffer
	logBuffer.WriteString(fmt.Sprintf("=== Command Execution ===\n"))
	logBuffer.WriteString(fmt.Sprintf("Command: %s %s\n", cmdName, strings.Join(args, " ")))
	logBuffer.WriteString(fmt.Sprintf("Duration: %.2f seconds\n", execDuration))
	logBuffer.WriteString(fmt.Sprintf("\n=== STDOUT ===\n%s\n", stdout.String()))
	logBuffer.WriteString(fmt.Sprintf("=== STDERR ===\n%s\n", stderr.String()))
	// Check if job was canceled
	if cancelResultData.WasCanceled {
		terminationType := "gracefully (SIGTERM)"
		if !cancelResultData.Graceful {
			terminationType = "forcefully (SIGKILL)"
		}
		log.Printf("⚠️  Job was canceled and terminated %s", terminationType)
		logBuffer.WriteString(fmt.Sprintf("\n=== CANCELED ===\nJob canceled by user, terminated %s\n", terminationType))
		
		// Record cancellation metrics
		metricsExporter.RecordJobCancellation(cancelResultData.Graceful)
		
		// Clean up partial output files
		if job.Parameters != nil {
			if output, ok := job.Parameters["output"].(string); ok {
				if output != "" {
					log.Printf("Cleaning up partial output file: %s", output)
					os.Remove(output)
				}
			}
		}
		
		return nil, nil, logBuffer.String(), &cancelResultData, fmt.Errorf("job was canceled")
	}
	
	if execErr != nil {
		// Check if context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("⚠️  %s timed out after expected duration", cmdName)
			logBuffer.WriteString(fmt.Sprintf("\n=== TIMEOUT ===\nContext deadline exceeded\n"))
			// For GStreamer with duration, timeout is expected - not an error
			if engine.Name() == "gstreamer" && duration > 0 {
				log.Printf("✓ GStreamer completed via timeout (expected behavior)")
				logBuffer.WriteString("GStreamer completed via timeout (expected behavior)\n")
				// Not an error - continue to metrics
			} else {
				log.Printf("%s stderr: %s", cmdName, stderr.String())
				return nil, nil, logBuffer.String(), nil, fmt.Errorf("%s execution timeout: %w", cmdName, execErr)
			}
		} else {
			log.Printf("%s stderr: %s", cmdName, stderr.String())
			logBuffer.WriteString(fmt.Sprintf("\n=== ERROR ===\n%v\n", execErr))
			// Check for specific GStreamer errors
			stderrStr := stderr.String()
			if engine.Name() == "gstreamer" {
				if strings.Contains(stderrStr, "Resource not found") {
					return nil, nil, logBuffer.String(), nil, fmt.Errorf("GStreamer pipeline error: RTMP server not reachable or stream rejected")
				}
				if strings.Contains(stderrStr, "Could not connect") {
					return nil, nil, logBuffer.String(), nil, fmt.Errorf("GStreamer pipeline error: Failed to connect to RTMP server")
				}
				if strings.Contains(stderrStr, "No such element") {
					return nil, nil, logBuffer.String(), nil, fmt.Errorf("GStreamer pipeline error: Missing GStreamer plugin (check gst-inspect-1.0)")
				}
			}
			return nil, nil, logBuffer.String(), nil, fmt.Errorf("%s execution failed: %w", cmdName, execErr)
		}
	}

	// Determine output mode
	outputMode := "file"
	if job.Parameters != nil {
		if mode, ok := job.Parameters["output_mode"].(string); ok {
			outputMode = mode
		}
	}

	// Log completion
	if outputMode == "rtmp" || outputMode == "stream" {
		log.Printf("✓ %s streaming completed (%.2f seconds)", engine.Name(), execDuration)
		logBuffer.WriteString(fmt.Sprintf("\n✓ %s streaming completed (%.2f seconds)\n", engine.Name(), execDuration))
	} else {
		log.Printf("✓ %s transcoding completed (%.2f seconds)", engine.Name(), execDuration)
		logBuffer.WriteString(fmt.Sprintf("\n✓ %s transcoding completed (%.2f seconds)\n", engine.Name(), execDuration))
	}

	// Parse output for metrics (works for both engines)
	if engine.Name() == "ffmpeg" {
		metrics = parseFFmpegMetrics(stderr.String(), execDuration, 0)
	} else {
		// GStreamer metrics
		metrics = make(map[string]interface{})
		metrics["exec_duration"] = execDuration
	}

	// Generate analyzer output
	analyzerOutput = map[string]interface{}{
		"scenario":      job.Scenario,
		"engine":        engine.Name(),
		"output_mode":   outputMode,
		"exec_duration": execDuration,
		"status":        "success",
	}

	return metrics, analyzerOutput, logBuffer.String(), nil, nil
}

// executeWithWrapperPath executes a job using the edge workload wrapper
func executeWithWrapperPath(job *models.Job, cmdPath string, cmdName string, args []string, limits *resources.ResourceLimits, metricsExporter *prometheus.WorkerExporter) (metrics map[string]interface{}, analyzerOutput map[string]interface{}, logs string, cancelResult *CancellationResult, err error) {
	log.Printf("🔧 Wrapper mode enabled for job %s", job.ID)
	
	// Get duration for timeout calculation
	duration := 0
	if job.Parameters != nil {
		if d, ok := job.Parameters["duration"].(float64); ok {
			duration = int(d)
		} else if d, ok := job.Parameters["duration"].(int); ok {
			duration = d
		} else if d, ok := job.Parameters["duration_seconds"].(float64); ok {
			duration = int(d)
		} else if d, ok := job.Parameters["duration_seconds"].(int); ok {
			duration = d
		}
	}
	
	// Determine timeout
	var timeoutDuration time.Duration
	if limits.TimeoutSec > 0 {
		timeoutDuration = time.Duration(limits.TimeoutSec) * time.Second
	} else if duration > 0 {
		timeoutDuration = time.Duration(duration*2 + 60) * time.Second
	} else {
		timeoutDuration = 10 * time.Minute
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()
	
	// Execute with wrapper
	startTime := time.Now()
	result, err := agent.ExecuteWithWrapper(ctx, job, cmdPath, args)
	execDuration := time.Since(startTime).Seconds()
	
	// Build logs
	var logBuffer bytes.Buffer
	logBuffer.WriteString(fmt.Sprintf("=== Wrapper Execution ===\n"))
	logBuffer.WriteString(fmt.Sprintf("Command: %s %s\n", cmdName, strings.Join(args, " ")))
	logBuffer.WriteString(fmt.Sprintf("Duration: %.2f seconds\n", execDuration))
	logBuffer.WriteString(fmt.Sprintf("PID: %d\n", result.PID))
	logBuffer.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))
	logBuffer.WriteString(fmt.Sprintf("Platform SLA: %v (%s)\n", result.PlatformSLA, result.PlatformSLAReason))
	
	if err != nil {
		logBuffer.WriteString(fmt.Sprintf("\n=== ERROR ===\n%v\n", err))
		return nil, nil, logBuffer.String(), nil, fmt.Errorf("wrapper execution failed: %w", err)
	}
	
	// Check exit code
	if result.ExitCode != 0 {
		logBuffer.WriteString(fmt.Sprintf("\n=== NON-ZERO EXIT ===\nProcess exited with code %d\n", result.ExitCode))
		return nil, nil, logBuffer.String(), nil, fmt.Errorf("%s exited with code %d", cmdName, result.ExitCode)
	}
	
	// Success
	logBuffer.WriteString(fmt.Sprintf("\n✓ %s completed via wrapper (%.2f seconds)\n", cmdName, execDuration))
	
	// Build metrics
	metrics = map[string]interface{}{
		"exec_duration":   execDuration,
		"wrapper_enabled": true,
		"platform_sla":    result.PlatformSLA,
		"exit_code":       result.ExitCode,
		"pid":             result.PID,
	}
	
	// Determine output mode
	outputMode := "file"
	if job.Parameters != nil {
		if mode, ok := job.Parameters["output_mode"].(string); ok {
			outputMode = mode
		}
	}
	
	// Generate analyzer output
	analyzerOutput = map[string]interface{}{
		"scenario":      job.Scenario,
		"engine":        cmdName,
		"output_mode":   outputMode,
		"exec_duration": execDuration,
		"status":        "success",
		"wrapper":       true,
		"platform_sla":  result.PlatformSLA,
	}
	
	return metrics, analyzerOutput, logBuffer.String(), nil, nil
}

// getFileSize safely gets the size of a file in bytes
func getFileSize(filePath string) int64 {
	if filePath == "" {
		return 0
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

// executeFFmpegJob executes an FFmpeg transcoding job based on job parameters
// This is kept for backward compatibility and direct FFmpeg usage
func executeFFmpegJob(job *models.Job, client *agent.Client, ffmpegOpt *agent.FFmpegOptimization) (metrics map[string]interface{}, analyzerOutput map[string]interface{}, err error) {
	// Extract parameters from job
	params := job.Parameters
	if params == nil {
		params = make(map[string]interface{})
	}

	// Apply hardware-optimized parameters (job parameters take precedence)
	params = agent.ApplyOptimizationToParameters(params, ffmpegOpt)
	
	log.Printf("Using optimized FFmpeg parameters: encoder=%s, preset=%s", 
		params["codec"], params["preset"])

	// Determine output mode: RTMP streaming or file output
	outputMode := "file" // default
	if mode, ok := params["output_mode"].(string); ok {
		outputMode = mode
	}

	// Get RTMP URL if streaming mode
	rtmpURL := ""
	if outputMode == "rtmp" || outputMode == "stream" {
		if rtmpURLParam, ok := params["rtmp_url"].(string); ok && rtmpURLParam != "" {
			rtmpURL = rtmpURLParam
		} else {
			// Default: construct RTMP URL pointing to master node
			// Get master URL from client (which was configured with --master flag)
			masterURL := client.GetMasterURL()
			masterHost := "localhost"
			
			if masterURL != "" {
				parsedURL, err := url.Parse(masterURL)
				if err == nil && parsedURL.Host != "" {
					// Extract hostname (remove API port)
					host := parsedURL.Host
					if colonIdx := strings.Index(host, ":"); colonIdx > 0 {
						host = host[:colonIdx]
					}
					masterHost = host
				}
			}
			
			streamKey := job.ID
			if key, ok := params["stream_key"].(string); ok && key != "" {
				streamKey = key
			}
			// RTMP server runs on master at port 1935
			rtmpURL = fmt.Sprintf("rtmp://%s:1935/live/%s", masterHost, streamKey)
		}
		log.Printf("RTMP streaming mode enabled: %s", rtmpURL)
		log.Printf("  Master URL source: %s (from --master flag)", client.GetMasterURL())
		log.Printf("  Streaming to master node RTMP server")
	}

	// Get input file (use test pattern if not specified)
	inputFile := filepath.Join(os.TempDir(), "test_input.mp4")
	if input, ok := params["input"].(string); ok && input != "" {
		inputFile = input
	}

	// Get output file (only used in file mode)
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

	// Create test input file only if in file mode and input doesn't exist
	if outputMode != "rtmp" && outputMode != "stream" {
		if _, statErr := os.Stat(inputFile); os.IsNotExist(statErr) {
			log.Printf("Input file %s not found, creating test video...", inputFile)
			if err := createTestVideo(inputFile, duration); err != nil {
				return nil, nil, fmt.Errorf("failed to create test input: %w", err)
			}
		}
		log.Printf("Transcoding: %s -> %s", inputFile, outputFile)
	} else {
		log.Printf("Streaming mode: generating test pattern and streaming to %s", 
			func() string {
				if rtmpURL != "" {
					return rtmpURL
				}
				return "RTMP server"
			}())
	}
	
	log.Printf("  Codec: %s, Bitrate: %s, Preset: %s", codec, bitrate, preset)

	// Verify FFmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Build FFmpeg command based on output mode
	var args []string
	
	if outputMode == "rtmp" || outputMode == "stream" {
		// RTMP Streaming mode - generate test source and stream
		log.Printf("Building RTMP streaming command with hardware optimizations...")
		
		// Get resolution and framerate for test source
		resolution := "1280x720"
		if res, ok := params["resolution"].(string); ok && res != "" {
			resolution = res
		}
		
		fps := 30
		if f, ok := params["fps"].(float64); ok {
			fps = int(f)
		} else if f, ok := params["fps"].(int); ok {
			fps = f
		}
		
		// Calculate buffer size (2x bitrate for streaming)
		bufsize := bitrate
		if strings.HasSuffix(bitrate, "k") {
			bitrateNum := bitrate[:len(bitrate)-1]
			bufsize = fmt.Sprintf("%sk", bitrateNum) // Can multiply by 2 here if needed
		}
		
		// Build streaming command with hardware optimizations
		args = []string{
			"-re", // Read input at native framerate (important for streaming)
			"-f", "lavfi",
			"-i", fmt.Sprintf("testsrc=size=%s:rate=%d", resolution, fps),
			"-f", "lavfi",
			"-i", "sine=frequency=1000:sample_rate=48000",
		}
		
		// Add duration limit if specified (before encoding options)
		if duration > 0 {
			args = append(args, "-t", fmt.Sprintf("%d", duration))
		}
		
		// Video encoding options with hardware optimization
		args = append(args, "-c:v", codec, "-preset", preset)
		
		// Apply hardware-optimized extra flags from ffmpegOpt
		log.Printf("Applying hardware optimization flags to RTMP stream...")
		for key, value := range ffmpegOpt.ExtraFlags {
			switch key {
			case "tune":
				// Apply tune for software encoders
				if codec == "libx264" || codec == "libx265" {
					args = append(args, "-tune", value)
					log.Printf("  Added -tune %s (from hardware optimization)", value)
				}
			case "threads":
				args = append(args, "-threads", value)
				log.Printf("  Added -threads %s (from hardware optimization)", value)
			case "rc", "spatial-aq", "temporal-aq", "bf", "zerolatency":
				// NVENC-specific flags
				if strings.Contains(codec, "nvenc") {
					args = append(args, fmt.Sprintf("-%s", key), value)
					log.Printf("  Added -%s %s (NVENC optimization)", key, value)
				}
			case "aq-mode", "no-sao", "bframes", "rd":
				// HEVC-specific flags
				if codec == "libx265" {
					args = append(args, "-x265-params", fmt.Sprintf("%s=%s", key, value))
					log.Printf("  Added x265 param %s=%s (from hardware optimization)", key, value)
				}
			case "me":
				// Motion estimation for x264
				if codec == "libx264" {
					args = append(args, "-me_method", value)
					log.Printf("  Added -me_method %s (from hardware optimization)", value)
				}
			case "g":
				// GOP size - already handled below
				continue
			}
		}
		
		// Streaming-specific encoding options
		args = append(args,
			"-b:v", bitrate,
			"-maxrate", bitrate,
			"-bufsize", bufsize,
			"-pix_fmt", "yuv420p",
			"-g", fmt.Sprintf("%d", fps*2), // GOP size: 2 seconds
		)
		
		// Audio encoding options
		args = append(args,
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-f", "flv", // FLV container for RTMP
			rtmpURL,
		)
		
		log.Printf("RTMP command built with %d optimization flags applied", len(ffmpegOpt.ExtraFlags))
		
	} else {
		// File transcoding mode
		log.Printf("Building file transcoding command...")
		
		args = []string{
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
	}

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

	// Verify output (different for streaming vs file mode)
	var outputSize int64
	if outputMode == "rtmp" || outputMode == "stream" {
		// For RTMP streaming, we can't verify file output
		// Just log that streaming completed
		log.Printf("✓ RTMP streaming completed: %s (%.2f seconds)", rtmpURL, execDuration)
		outputSize = 0 // Not applicable for streaming
	} else {
		// Verify output file was created
		outputInfo, err := os.Stat(outputFile)
		if err != nil {
			return nil, nil, fmt.Errorf("output file not created: %w", err)
		}
		outputSize = outputInfo.Size()
		log.Printf("✓ Transcoding completed: %s (%.2f MB in %.2f seconds)", 
			outputFile, float64(outputSize)/1024/1024, execDuration)
	}

	// Parse FFmpeg output for metrics
	metrics = parseFFmpegMetrics(stderr.String(), execDuration, outputSize)

	// Generate analyzer output
	analyzerOutput = map[string]interface{}{
		"scenario":            job.Scenario,
		"output_mode":         outputMode,
		"codec":               codec,
		"bitrate":             bitrate,
		"preset":              preset,
		"exec_duration":       execDuration,
		"status":              "success",
		"optimization_reason": ffmpegOpt.Reason,
		"hwaccel":             ffmpegOpt.HWAccel,
	}
	
	if outputMode == "rtmp" || outputMode == "stream" {
		analyzerOutput["rtmp_url"] = rtmpURL
	} else {
		analyzerOutput["input_file"] = inputFile
		analyzerOutput["output_file"] = outputFile
		analyzerOutput["output_size"] = outputSize
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

// getWrapperMetrics returns wrapper metrics in Prometheus format
func getWrapperMetrics() string {
// Try to import from internal/report, fallback to empty if not available
// This will be populated when wrapper is integrated
return `# Wrapper metrics
# Currently available via wrapper execution results
# See /violations endpoint for recent platform SLA violations
`
}

// getWrapperViolations returns recent SLA violations in JSON format
func getWrapperViolations() string {
// Try to get from internal/report, fallback to empty array
return `[]`
}

// toJSON converts a map to JSON string (simple implementation)
func toJSON(m map[string]string) string {
var parts []string
for k, v := range m {
parts = append(parts, fmt.Sprintf(`"%s":"%s"`, k, v))
}
return "{" + strings.Join(parts, ",") + "}"
}
