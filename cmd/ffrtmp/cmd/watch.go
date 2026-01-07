package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
	"github.com/psantana5/ffmpeg-rtmp/internal/discover"
	"github.com/spf13/cobra"
)

var (
	scanInterval   time.Duration
	targetCommands []string
	daemonCPUQuota int
	daemonCPUWeight int
	daemonMemLimit int
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Automatically discover and attach to running FFmpeg processes",
	Long: `Watch mode continuously scans for running FFmpeg/transcoding processes
and automatically attaches resource governance to them.

CRITICAL for production environments where processes may start outside
of the wrapper's control (client-initiated streams, external triggers, etc.).

The daemon runs in the background and automatically applies resource limits
to any discovered processes matching the target commands.

Example:
  ffrtmp watch
  ffrtmp watch --scan-interval 5s --cpu-quota 150 --memory-limit 2048
  ffrtmp watch --target ffmpeg --target gst-launch-1.0`,
	RunE: runWatchDaemon,
}

func init() {
	rootCmd.AddCommand(watchCmd)
	
	watchCmd.Flags().DurationVar(&scanInterval, "scan-interval", 10*time.Second, "How often to scan for new processes")
	watchCmd.Flags().StringSliceVar(&targetCommands, "target", []string{"ffmpeg", "gst-launch-1.0"}, "Target commands to discover")
	watchCmd.Flags().IntVar(&daemonCPUQuota, "cpu-quota", 0, "Default CPU quota for discovered processes (0=unlimited)")
	watchCmd.Flags().IntVar(&daemonCPUWeight, "cpu-weight", 100, "Default CPU weight for discovered processes")
	watchCmd.Flags().IntVar(&daemonMemLimit, "memory-limit", 0, "Default memory limit in MB for discovered processes (0=unlimited)")
}

func runWatchDaemon(cmd *cobra.Command, args []string) error {
	fmt.Printf("╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ FFmpeg Auto-Attach Daemon                                      ║\n")
	fmt.Printf("╠════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║ Scan Interval: %-47s ║\n", scanInterval)
	fmt.Printf("║ Target Commands: %-45s ║\n", fmt.Sprintf("%v", targetCommands))
	fmt.Printf("║ Default CPU Quota: %-43d ║\n", daemonCPUQuota)
	fmt.Printf("║ Default CPU Weight: %-42d ║\n", daemonCPUWeight)
	fmt.Printf("║ Default Memory Limit: %-38d MB ║\n", daemonMemLimit)
	fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n")
	
	// Build default limits
	limits := &cgroups.Limits{
		CPUWeight: daemonCPUWeight,
	}
	
	if daemonCPUQuota > 0 {
		quota := daemonCPUQuota * 1000
		limits.CPUMax = fmt.Sprintf("%d 100000", quota)
	}
	
	if daemonMemLimit > 0 {
		limits.MemoryMax = int64(daemonMemLimit) * 1024 * 1024
	}
	
	// Create logger
	logger := log.New(os.Stdout, "[watch] ", log.LstdFlags)
	
	// Configure auto-attach service
	config := &discover.AttachConfig{
		ScanInterval:   scanInterval,
		TargetCommands: targetCommands,
		DefaultLimits:  limits,
		Logger:         logger,
		OnAttach: func(pid int, jobID string) {
			logger.Printf("✓ Attached to PID %d (job: %s)", pid, jobID)
		},
		OnDetach: func(pid int, jobID string) {
			logger.Printf("⊗ Detached from PID %d (job: %s)", pid, jobID)
		},
	}
	
	// Create service
	service := discover.NewAutoAttachService(config)
	
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		logger.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()
	
	// Start service
	logger.Println("Service started. Press Ctrl+C to stop.")
	fmt.Println()
	
	if err := service.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("service error: %w", err)
	}
	
	logger.Println("Service stopped gracefully")
	return nil
}
