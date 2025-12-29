package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/agent"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func main() {
	masterURL := flag.String("master", "http://localhost:8080", "Master node URL")
	register := flag.Bool("register", false, "Register with master node")
	pollInterval := flag.Duration("poll-interval", 10*time.Second, "Job polling interval")
	heartbeatInterval := flag.Duration("heartbeat-interval", 30*time.Second, "Heartbeat interval")
	allowMasterAsWorker := flag.Bool("allow-master-as-worker", false, "Allow registering master node as worker (development mode)")
	flag.Parse()

	log.Println("Starting FFmpeg RTMP Distributed Compute Agent")
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

	// Create client
	client := agent.NewClient(*masterURL)

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

		// Execute job
		result := executeJob(job, client.GetNodeID())

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
func executeJob(job *models.Job, nodeID string) *models.JobResult {
	log.Printf("Executing job %s...", job.ID)

	// For now, just simulate job execution
	// In a real implementation, this would:
	// 1. Run recommended_test.py with appropriate parameters
	// 2. Execute FFmpeg with optimal parameters
	// 3. Collect metrics from exporters
	// 4. Run analyzer to generate output
	time.Sleep(5 * time.Second)

	result := &models.JobResult{
		JobID:       job.ID,
		NodeID:      nodeID,
		Status:      models.JobStatusCompleted,
		CompletedAt: time.Now(),
		Metrics: map[string]interface{}{
			"cpu_usage":    75.5,
			"memory_usage": 2048,
			"duration":     5.0,
		},
		AnalyzerOutput: map[string]interface{}{
			"scenario":       job.Scenario,
			"recommendation": "optimal",
		},
	}

	log.Printf("Job %s completed successfully", job.ID)
	return result
}
