package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/psantana5/ffmpeg-rtmp/pkg/wrapper"
	"github.com/spf13/cobra"
)

var (
	// Common flags for run/attach
	jobID       string
	slaEligible bool
	intent      string
	
	// Constraint flags
	cpuQuota    int
	cpuWeight   int
	nicePriority int
	memoryLimit int64
	ioWeight    int
	oomScore    int
	
	// Run mode specific
	workDir string
	
	// Attach mode specific
	attachPID int
	
	// Output
	jsonOutput bool
)

var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Wrap and run a workload with OS-level constraints",
	Long: `Run mode spawns a new workload process with OS-level governance applied.

The workload process is started in its own process group and will continue
running even if the wrapper crashes. This is a non-owning wrapper - it only
governs HOW the OS executes the workload.

Example:
  ffrtmp run --job-id job123 --sla-eligible --cpu-quota 200 -- ffmpeg -i input.mp4 output.mp4
  ffrtmp run --intent test --nice 10 -- ./my-benchmark.sh
  ffrtmp run --memory-limit 2048 -- python train.py`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWorkload,
}

var attachCmd = &cobra.Command{
	Use:   "attach --pid <PID>",
	Short: "Attach to an already-running process",
	Long: `Attach mode attaches to a process that is ALREADY running and applies
OS-level governance. This is CRITICAL for adoption in edge environments where
workloads already exist and cannot be restarted.

The wrapper:
- Moves the PID into a managed cgroup
- Applies resource constraints
- Starts passive observation
- Does NOT restart or modify execution flow
- Does NOT inject code

Example:
  ffrtmp attach --pid 12345 --job-id job123 --sla-eligible
  ffrtmp attach --pid 12345 --cpu-weight 50 --nice 10
  ffrtmp attach --pid 12345 --memory-limit 4096`,
	RunE: attachToWorkload,
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(attachCmd)
	
	// Common flags
	for _, cmd := range []*cobra.Command{runCmd, attachCmd} {
		cmd.Flags().StringVar(&jobID, "job-id", "", "Job identifier")
		cmd.Flags().BoolVar(&slaEligible, "sla-eligible", false, "Mark workload as SLA-eligible")
		cmd.Flags().StringVar(&intent, "intent", "production", "Workload intent (production|test|experiment|soak)")
		
		// Constraint flags
		cmd.Flags().IntVar(&cpuQuota, "cpu-quota", 0, "CPU quota percentage (100=1 core, 200=2 cores, 0=unlimited)")
		cmd.Flags().IntVar(&cpuWeight, "cpu-weight", 100, "CPU weight for proportional sharing (1-10000)")
		cmd.Flags().IntVar(&nicePriority, "nice", 0, "Process nice priority (-20 to 19)")
		cmd.Flags().Int64Var(&memoryLimit, "memory-limit", 0, "Memory limit in MB (0=unlimited)")
		cmd.Flags().IntVar(&ioWeight, "io-weight", 0, "IO weight percentage (0-100, 0=no constraint)")
		cmd.Flags().IntVar(&oomScore, "oom-score", 0, "OOM score adjustment (-1000 to 1000)")
		
		// Output
		cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output report as JSON")
	}
	
	// Run mode specific flags
	runCmd.Flags().StringVar(&workDir, "workdir", "", "Working directory for the workload")
	
	// Attach mode specific flags
	attachCmd.Flags().IntVar(&attachPID, "pid", 0, "PID of the process to attach to")
	attachCmd.MarkFlagRequired("pid")
}

func runWorkload(cmd *cobra.Command, args []string) error {
	// Parse command and args
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}
	
	command := args[0]
	cmdArgs := args[1:]
	
	// Build metadata
	metadata := &wrapper.WorkloadMetadata{
		JobID:       jobID,
		SLAEligible: slaEligible,
		Intent:      intent,
	}
	
	// Build constraints
	constraints := &wrapper.Constraints{
		CPUQuotaPercent: cpuQuota,
		CPUWeight:       cpuWeight,
		NicePriority:    nicePriority,
		MemoryLimitMB:   memoryLimit,
		IOWeightPercent: ioWeight,
		OOMScoreAdj:     oomScore,
	}
	
	// Create wrapper
	w := wrapper.New(metadata, constraints)
	
	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\n[wrapper] Received signal, wrapper exiting (workload continues)...\n")
		cancel()
	}()
	
	// Run the workload
	if err := w.Run(ctx, command, cmdArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Error running workload: %v\n", err)
	}
	
	// Output report
	if jsonOutput {
		return outputJSON(w)
	}
	return w.WriteReport(os.Stdout)
}

func attachToWorkload(cmd *cobra.Command, args []string) error {
	if attachPID <= 0 {
		return fmt.Errorf("invalid PID: %d", attachPID)
	}
	
	// Build metadata
	metadata := &wrapper.WorkloadMetadata{
		JobID:       jobID,
		SLAEligible: slaEligible,
		Intent:      intent,
	}
	
	// Build constraints
	constraints := &wrapper.Constraints{
		CPUQuotaPercent: cpuQuota,
		CPUWeight:       cpuWeight,
		NicePriority:    nicePriority,
		MemoryLimitMB:   memoryLimit,
		IOWeightPercent: ioWeight,
		OOMScoreAdj:     oomScore,
	}
	
	// Create wrapper
	w := wrapper.New(metadata, constraints)
	
	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\n[wrapper] Received signal, detaching from workload...\n")
		cancel()
	}()
	
	// Attach to the workload
	fmt.Printf("[wrapper] Attaching to PID %d...\n", attachPID)
	if err := w.Attach(ctx, attachPID); err != nil {
		return fmt.Errorf("failed to attach: %w", err)
	}
	
	// Output report
	if jsonOutput {
		return outputJSON(w)
	}
	return w.WriteReport(os.Stdout)
}

func outputJSON(w *wrapper.Wrapper) error {
	report := map[string]interface{}{
		"job_id":       w.GetEvents()[0].PID, // Simplified
		"exit_code":    w.GetExitCode(),
		"exit_reason":  string(w.GetExitReason()),
		"duration_sec": w.GetDuration().Seconds(),
		"events":       w.GetEvents(),
	}
	
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
