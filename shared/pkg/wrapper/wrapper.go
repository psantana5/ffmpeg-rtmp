package wrapper

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Wrapper wraps an existing workload with OS-level governance
// CRITICAL: The wrapper NEVER owns the workload
type Wrapper struct {
	metadata  *WorkloadMetadata
	constraints *Constraints
	cgroup    *CgroupManager
	pid       int
	cmd       *exec.Cmd // Only set in Run mode
	attached  bool      // True if we attached to existing process
	cgroupPath string
	
	// Lifecycle tracking
	startTime time.Time
	events    []LifecycleEvent
	exitCode  int
	exitReason ExitReason
}

// New creates a new wrapper instance
func New(metadata *WorkloadMetadata, constraints *Constraints) *Wrapper {
	if metadata == nil {
		metadata = &WorkloadMetadata{
			JobID:  "unknown",
			Intent: IntentProduction,
		}
	}
	
	if constraints == nil {
		constraints = DefaultConstraints()
	}
	
	metadata.Validate()
	constraints.Validate()
	
	cgroup := NewCgroupManager("ffrtmp-wrapper")
	
	return &Wrapper{
		metadata:    metadata,
		constraints: constraints,
		cgroup:      cgroup,
		events:      []LifecycleEvent{},
	}
}

// Run spawns a new workload process with constraints applied
// The workload will continue running even if wrapper crashes
func (w *Wrapper) Run(ctx context.Context, command string, args ...string) error {
	log.Printf("[wrapper] RUN mode: %s %v", command, args)
	log.Printf("[wrapper] Job: %s, Intent: %s, SLA: %v", 
		w.metadata.JobID, w.metadata.Intent, w.metadata.SLAEligible)
	
	w.attached = false
	w.startTime = time.Now()
	
	// Emit starting event
	w.emitEvent(StateStarting, "Spawning workload process")
	
	// Create command
	cmd := exec.CommandContext(ctx, command, args...)
	
	// Set process group to ensure workload survives wrapper crash
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // New process group
		Pgid:    0,    // Process becomes its own group leader
	}
	
	// Forward stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start the process
	if err := cmd.Start(); err != nil {
		w.emitEvent(StateFailed, fmt.Sprintf("Failed to start: %v", err))
		return fmt.Errorf("failed to start workload: %w", err)
	}
	
	w.cmd = cmd
	w.pid = cmd.Process.Pid
	
	log.Printf("[wrapper] Started PID %d", w.pid)
	w.emitEvent(StateRunning, fmt.Sprintf("PID %d started", w.pid))
	
	// Apply constraints AFTER process started
	if err := w.applyConstraints(); err != nil {
		log.Printf("[wrapper] WARNING: Failed to apply some constraints: %v", err)
	}
	
	// Wait for completion in background
	return w.wait(ctx)
}

// Attach attaches to an already-running process
// CRITICAL: Does NOT restart or modify execution flow
func (w *Wrapper) Attach(ctx context.Context, pid int) error {
	log.Printf("[wrapper] ATTACH mode: PID %d", pid)
	log.Printf("[wrapper] Job: %s, Intent: %s, SLA: %v", 
		w.metadata.JobID, w.metadata.Intent, w.metadata.SLAEligible)
	
	// Verify process exists
	if !processExists(pid) {
		return fmt.Errorf("process %d does not exist", pid)
	}
	
	w.attached = true
	w.pid = pid
	w.startTime = time.Now()
	
	log.Printf("[wrapper] Attached to existing PID %d", pid)
	w.emitEvent(StateRunning, fmt.Sprintf("Attached to PID %d", pid))
	
	// Apply constraints to existing process
	if err := w.applyConstraints(); err != nil {
		log.Printf("[wrapper] WARNING: Failed to apply some constraints: %v", err)
	}
	
	// Monitor the process (passive observation)
	return w.monitorAttached(ctx)
}

// applyConstraints applies OS-level constraints to the process
func (w *Wrapper) applyConstraints() error {
	log.Printf("[wrapper] Applying constraints to PID %d", w.pid)
	
	// Create or join cgroup
	cgroupPath, err := w.cgroup.CreateOrJoinCgroup(w.metadata.JobID, w.constraints)
	if err != nil {
		return fmt.Errorf("failed to create cgroup: %w", err)
	}
	w.cgroupPath = cgroupPath
	
	// Attach process to cgroup
	if cgroupPath != "" {
		if err := w.cgroup.AttachProcess(cgroupPath, w.pid); err != nil {
			log.Printf("[wrapper] WARNING: Failed to attach to cgroup: %v", err)
		}
	}
	
	// Apply nice priority (always works, even without cgroups)
	if w.constraints.NicePriority != 0 {
		if err := ApplyNicePriority(w.pid, w.constraints.NicePriority); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set nice priority: %v", err)
		}
	}
	
	// Apply OOM score adjustment
	if w.constraints.OOMScoreAdj != 0 {
		if err := ApplyOOMScoreAdj(w.pid, w.constraints.OOMScoreAdj); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set OOM score: %v", err)
		}
	}
	
	return nil
}

// wait waits for process completion (Run mode)
func (w *Wrapper) wait(ctx context.Context) error {
	if w.cmd == nil {
		return fmt.Errorf("no command to wait for")
	}
	
	// Wait for process to complete
	err := w.cmd.Wait()
	
	// Determine exit code and reason
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			w.exitCode = exitErr.ExitCode()
			
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				w.exitReason = DetermineExitReason(w.exitCode, status)
				
				if status.Signaled() {
					signal := status.Signal()
					w.emitEvent(StateKilled, fmt.Sprintf("Killed by %s", SignalName(signal)))
				} else {
					w.emitEvent(StateFailed, fmt.Sprintf("Exited with code %d", w.exitCode))
				}
			}
		} else {
			w.exitCode = 1
			w.exitReason = ExitReasonError
			w.emitEvent(StateFailed, fmt.Sprintf("Wait error: %v", err))
		}
	} else {
		w.exitCode = 0
		w.exitReason = ExitReasonSuccess
		w.emitEvent(StateCompleted, "Completed successfully")
	}
	
	// Cleanup
	w.cleanup()
	
	log.Printf("[wrapper] Process exited: code=%d, reason=%s, duration=%.1fs",
		w.exitCode, w.exitReason, time.Since(w.startTime).Seconds())
	
	return nil
}

// monitorAttached monitors an attached process (Attach mode)
func (w *Wrapper) monitorAttached(ctx context.Context) error {
	// Passive monitoring - just wait for process to exit
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("[wrapper] Context cancelled, stopping monitoring")
			return ctx.Err()
			
		case <-ticker.C:
			if !processExists(w.pid) {
				// Process exited - try to determine reason
				// (We can't get exit code for attached processes easily)
				w.exitCode = -1
				w.exitReason = ExitReasonUnknown
				w.emitEvent(StateCompleted, "Process exited (attached mode)")
				
				// Cleanup
				w.cleanup()
				
				log.Printf("[wrapper] Attached process %d exited after %.1fs",
					w.pid, time.Since(w.startTime).Seconds())
				
				return nil
			}
		}
	}
}

// cleanup removes cgroup and does final cleanup
func (w *Wrapper) cleanup() {
	log.Printf("[wrapper] Cleaning up...")
	
	// Remove cgroup
	if w.cgroupPath != "" {
		if err := w.cgroup.RemoveCgroup(w.cgroupPath); err != nil {
			log.Printf("[wrapper] WARNING: Failed to remove cgroup: %v", err)
		}
	}
}

// emitEvent records a lifecycle event
func (w *Wrapper) emitEvent(state LifecycleState, message string) {
	event := LifecycleEvent{
		PID:       w.pid,
		State:     state,
		Timestamp: time.Now(),
		Message:   message,
		ExitCode:  w.exitCode,
		ExitReason: w.exitReason,
	}
	
	w.events = append(w.events, event)
}

// GetEvents returns all lifecycle events
func (w *Wrapper) GetEvents() []LifecycleEvent {
	return w.events
}

// GetExitCode returns the exit code (if available)
func (w *Wrapper) GetExitCode() int {
	return w.exitCode
}

// GetExitReason returns the exit reason
func (w *Wrapper) GetExitReason() ExitReason {
	return w.exitReason
}

// GetDuration returns how long the workload ran
func (w *Wrapper) GetDuration() time.Duration {
	return time.Since(w.startTime)
}

// WriteReport writes a summary report to the given writer
func (w *Wrapper) WriteReport(out io.Writer) error {
	fmt.Fprintf(out, "=== Workload Wrapper Report ===\n")
	fmt.Fprintf(out, "Job ID: %s\n", w.metadata.JobID)
	fmt.Fprintf(out, "Intent: %s\n", w.metadata.Intent)
	fmt.Fprintf(out, "SLA Eligible: %v\n", w.metadata.SLAEligible)
	fmt.Fprintf(out, "Mode: %s\n", map[bool]string{true: "attach", false: "run"}[w.attached])
	fmt.Fprintf(out, "PID: %d\n", w.pid)
	fmt.Fprintf(out, "Duration: %.2fs\n", w.GetDuration().Seconds())
	fmt.Fprintf(out, "Exit Code: %d\n", w.exitCode)
	fmt.Fprintf(out, "Exit Reason: %s\n", w.exitReason)
	fmt.Fprintf(out, "\nLifecycle Events:\n")
	for _, event := range w.events {
		fmt.Fprintf(out, "  [%s] %s: %s\n", 
			event.Timestamp.Format("15:04:05"), event.State, event.Message)
	}
	
	return nil
}

// processExists checks if a process exists
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
