package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
	"github.com/psantana5/ffmpeg-rtmp/internal/wrapper"
	"github.com/spf13/cobra"
)

var (
	// Metadata
	jobID       string
	slaEligible bool
	
	// Constraints
	cpuMax      string
	cpuQuota    int
	cpuWeight   int
	memoryMax   int64
	memoryLimit int
	ioMax       string
	niceValue   int
	
	// Run mode
	workDir string
	
	// Attach mode
	attachPID int
	
	// Output
	jsonOutput bool
)

var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Spawn workload with governance (non-owning)",
	Long: `Run mode spawns a workload in its own process group.
The workload continues running even if wrapper crashes.

This is governance, not execution.

Example:
  ffrtmp run --job-id job-001 -- ffmpeg -i input.mp4 output.mp4
  ffrtmp run --job-id transcode-001 --sla-eligible --cpu-quota 200 --memory-limit 4096 -- ffmpeg -i input.mp4 -c:v h264_nvenc output.mp4`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWorkload,
}

var attachCmd = &cobra.Command{
	Use:   "attach --pid <PID>",
	Short: "Attach to running process (passive observation)",
	Long: `Attach mode attaches to an already-running process.
CRITICAL for edge nodes receiving signals/streams from clients.

No restart. No signals. Just observe.

Example:
  ffrtmp attach --pid 12345 --job-id job-001
  ffrtmp attach --pid 12345 --job-id existing-job-042 --cpu-weight 150 --nice -5`,
	RunE: attachToWorkload,
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(attachCmd)
	
	// Common flags
	for _, cmd := range []*cobra.Command{runCmd, attachCmd} {
		cmd.Flags().StringVar(&jobID, "job-id", "", "Job identifier")
		cmd.Flags().BoolVar(&slaEligible, "sla-eligible", false, "Mark job as SLA-eligible")
		cmd.Flags().StringVar(&cpuMax, "cpu-max", "", "CPU max (quota period format, e.g. '200000 100000')")
		cmd.Flags().IntVar(&cpuQuota, "cpu-quota", 0, "CPU quota percentage (0-1600, where 100=1 core)")
		cmd.Flags().IntVar(&cpuWeight, "cpu-weight", 100, "CPU weight (1-10000)")
		cmd.Flags().Int64Var(&memoryMax, "memory-max", 0, "Memory limit in bytes (0=unlimited)")
		cmd.Flags().IntVar(&memoryLimit, "memory-limit", 0, "Memory limit in MB (0=unlimited)")
		cmd.Flags().StringVar(&ioMax, "io-max", "", "IO max (major:minor rbps=X wbps=Y)")
		cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	}
	
	// Run mode flags
	runCmd.Flags().StringVar(&workDir, "workdir", "", "Working directory")
	
	// Attach mode flags
	attachCmd.Flags().IntVar(&attachPID, "pid", 0, "PID to attach to")
	attachCmd.Flags().IntVar(&niceValue, "nice", 0, "Process nice value (-20 to 19)")
	attachCmd.MarkFlagRequired("pid")
}

func runWorkload(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}
	
	command := args[0]
	cmdArgs := args[1:]
	
	// Convert cpu-quota to cpu-max format if provided
	cpuMaxValue := cpuMax
	if cpuQuota > 0 {
		// Convert percentage to quota/period (100% = 100000/100000, 200% = 200000/100000)
		quota := cpuQuota * 1000
		cpuMaxValue = fmt.Sprintf("%d 100000", quota)
	}
	
	// Convert memory-limit (MB) to memory-max (bytes) if provided
	memoryMaxValue := memoryMax
	if memoryLimit > 0 {
		memoryMaxValue = int64(memoryLimit) * 1024 * 1024
	}
	
	// Build limits
	limits := &cgroups.Limits{
		CPUMax:      cpuMaxValue,
		CPUWeight:   cpuWeight,
		MemoryMax:   memoryMaxValue,
		IOMax:       ioMax,
	}
	
	// Setup context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nWrapper exiting (workload continues)...\n")
		cancel()
	}()
	
	// Run workload
	result, err := wrapper.Run(ctx, jobID, limits, command, cmdArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	
	// Output result
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	
	fmt.Printf("Job: %s\n", result.JobID)
	fmt.Printf("PID: %d\n", result.PID)
	fmt.Printf("Exit Code: %d\n", result.ExitCode)
	fmt.Printf("Duration: %.2fs\n", result.Duration.Seconds())
	fmt.Printf("Platform SLA: %v (%s)\n", result.PlatformSLA, result.PlatformSLAReason)
	
	return nil
}

func attachToWorkload(cmd *cobra.Command, args []string) error {
	if attachPID <= 0 {
		return fmt.Errorf("invalid PID: %d", attachPID)
	}
	
	// Convert cpu-quota to cpu-max format if provided
	cpuMaxValue := cpuMax
	if cpuQuota > 0 {
		quota := cpuQuota * 1000
		cpuMaxValue = fmt.Sprintf("%d 100000", quota)
	}
	
	// Convert memory-limit (MB) to memory-max (bytes) if provided
	memoryMaxValue := memoryMax
	if memoryLimit > 0 {
		memoryMaxValue = int64(memoryLimit) * 1024 * 1024
	}
	
	// Build limits
	limits := &cgroups.Limits{
		CPUMax:      cpuMaxValue,
		CPUWeight:   cpuWeight,
		MemoryMax:   memoryMaxValue,
		IOMax:       ioMax,
	}
	
	// Apply nice value if specified
	if niceValue != 0 {
		if err := syscall.Setpriority(syscall.PRIO_PROCESS, attachPID, niceValue); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set nice value: %v\n", err)
		}
	}
	
	// Setup context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nDetaching (workload continues)...\n")
		cancel()
	}()
	
	// Attach to workload
	fmt.Printf("Attaching to PID %d...\n", attachPID)
	result, err := wrapper.Attach(ctx, jobID, attachPID, limits)
	if err != nil && err != context.Canceled {
		return fmt.Errorf("failed to attach: %w", err)
	}
	
	// Output result
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	
	fmt.Printf("Job: %s\n", result.JobID)
	fmt.Printf("PID: %d\n", result.PID)
	fmt.Printf("Duration: %.2fs\n", result.Duration.Seconds())
	fmt.Printf("Platform SLA: %v (%s)\n", result.PlatformSLA, result.PlatformSLAReason)
	
	return nil
}
