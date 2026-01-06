package observe

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"os"
	"syscall"
	"time"
)

// Watcher observes PID lifecycle. Nothing else.
type Watcher struct {
	pid       int
	startTime time.Time
}

// New creates a watcher for a PID
func New(pid int) *Watcher {
	return &Watcher{
		pid:       pid,
		startTime: time.Now(),
	}
}

// Exists checks if PID still exists
func (w *Watcher) Exists() bool {
	process, err := os.FindProcess(w.pid)
	if err != nil {
		return false
	}
	
	// Send signal 0 to check existence
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Wait waits for PID to exit (passive observation)
// Returns when process exits
func (w *Watcher) Wait() (int, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		<-ticker.C
		if !w.Exists() {
			// Process exited
			return -1, nil // Can't get exit code for attached process
		}
	}
}

// Duration returns how long we've been observing
func (w *Watcher) Duration() time.Duration {
	return time.Since(w.startTime)
}
