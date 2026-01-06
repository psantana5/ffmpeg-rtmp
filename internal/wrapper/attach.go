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
	applyLimits(jobID, pid, limits)
	
	// Create watcher
	watcher := observe.New(pid)
	
	// Passive observation only
	// If wrapper crashes here → workload continues ✓
	go func() {
		<-ctx.Done()
	}()
	
	// Wait for process to exit (or context cancel)
	select {
	case <-ctx.Done():
		// Wrapper told to stop, workload continues
		timing.Complete()
		result := report.NewResult(jobID, pid, -1, timing.Duration(), "attach")
		result.SetPlatformSLA(true, "detached_workload_continues")
		return result, ctx.Err()
		
	default:
		// Wait for process exit
		watcher.Wait()
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
