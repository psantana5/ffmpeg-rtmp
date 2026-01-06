package wrapper

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	
	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
	"github.com/psantana5/ffmpeg-rtmp/internal/observe"
	"github.com/psantana5/ffmpeg-rtmp/internal/report"
)

// Run spawns a workload. Process survives wrapper crash.
func Run(ctx context.Context, jobID string, limits *cgroups.Limits, command string, args []string) (*report.Result, error) {
	report.Global().IncrStarted()
	
	timing := observe.NewTiming()
	
	// Create command
	cmd := exec.CommandContext(ctx, command, args...)
	
	// CRITICAL: Set process group so workload is independent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // New process group
		Pgid:    0,    // Process becomes its own group leader
	}
	
	// Forward stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start: %w", err)
	}
	
	pid := cmd.Process.Pid
	
	// Apply limits (best effort)
	cgroupPath := applyLimits(jobID, pid, limits)
	
	// Cleanup cgroup on exit
	defer func() {
		if cgroupPath != "" {
			mgr := cgroups.New()
			mgr.Delete(cgroupPath)
		}
	}()
	
	// Wait for completion
	err := cmd.Wait()
	timing.Complete()
	
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			report.Global().IncrFailed()
		}
	} else {
		report.Global().IncrCompleted()
	}
	
	result := report.NewResult(jobID, pid, exitCode, timing.Duration(), "run")
	
	// Calculate SLA ONCE
	calculatePlatformSLA(result, exitCode)
	
	return result, nil
}

// applyLimits applies cgroup limits (best effort)
// Returns cgroup path for cleanup
func applyLimits(jobID string, pid int, limits *cgroups.Limits) string {
	if limits == nil {
		return ""
	}
	
	mgr := cgroups.New()
	cgroupPath, err := mgr.Create(jobID)
	if err != nil || cgroupPath == "" {
		return "" // Can't create cgroup, continue anyway
	}
	
	// Join cgroup
	if err := mgr.Join(cgroupPath, pid); err != nil {
		return "" // Failed to join, skip limits
	}
	
	// Apply limits (all best effort)
	if limits.CPUMax != "" {
		cgroups.WriteCPUMax(cgroupPath, limits.CPUMax)
	}
	if limits.CPUWeight > 0 {
		cgroups.WriteCPUWeight(cgroupPath, limits.CPUWeight)
	}
	if limits.MemoryMax > 0 {
		cgroups.WriteMemoryMax(cgroupPath, limits.MemoryMax)
	}
	if limits.IOMax != "" {
		cgroups.WriteIOMax(cgroupPath, limits.IOMax)
	}
	
	return cgroupPath
}

// calculatePlatformSLA determines if platform behaved correctly
// Call this ONCE at job completion
func calculatePlatformSLA(result *report.Result, exitCode int) {
	// Simple rule: if we got here, platform did its job
	// Workload success/failure is separate concern
	
	if exitCode == 0 {
		result.SetPlatformSLA(true, "completed_successfully")
	} else {
		// Workload failed, but did platform fail?
		// For now: platform succeeded in executing the workload
		result.SetPlatformSLA(true, "workload_failed_platform_ok")
	}
}
