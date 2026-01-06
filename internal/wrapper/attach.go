package wrapper

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"context"
	"fmt"
	"os"
	"syscall"
	
	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
	"github.com/psantana5/ffmpeg-rtmp/internal/observe"
	"github.com/psantana5/ffmpeg-rtmp/internal/report"
)

// Attach attaches to an already-running process.
// CRITICAL: No restart. No signals. Just observe.
func Attach(ctx context.Context, jobID string, pid int, limits *cgroups.Limits) (*report.Result, error) {
	report.Global().IncrAttached()
	
	// Validate PID exists
	if !pidExists(pid) {
		return nil, fmt.Errorf("process %d does not exist", pid)
	}
	
	timing := observe.NewTiming()
	
	// Apply limits (best effort, no errors)
	cgroupPath := applyLimits(jobID, pid, limits)
	
	// Cleanup cgroup on exit (best effort)
	defer func() {
		if cgroupPath != "" {
			mgr := cgroups.New()
			mgr.Delete(cgroupPath)
		}
	}()
	
	// Create watcher
	watcher := observe.New(pid)
	
	// Wait for process to exit (or context cancel)
	done := make(chan struct{})
	
	go func() {
		watcher.Wait()
		close(done)
	}()
	
	// If wrapper crashes here → workload continues ✓
	select {
	case <-ctx.Done():
		// Wrapper told to stop, workload continues
		timing.Complete()
		result := report.NewResult(jobID, pid, -1, timing.Duration(), "attach")
		result.SetPlatformSLA(true, "detached_workload_continues")
		return result, ctx.Err()
		
	case <-done:
		// Process exited naturally
		timing.Complete()
		result := report.NewResult(jobID, pid, -1, timing.Duration(), "attach")
		result.SetPlatformSLA(true, "observed_to_completion")
		return result, nil
	}
}

// pidExists checks if PID exists
func pidExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
