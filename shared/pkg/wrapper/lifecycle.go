package wrapper

import (
	"fmt"
	"syscall"
	"time"
)

// LifecycleState represents the workload's lifecycle state
type LifecycleState string

const (
	StateUnknown   LifecycleState = "unknown"
	StateStarting  LifecycleState = "starting"
	StateRunning   LifecycleState = "running"
	StateCompleted LifecycleState = "completed"
	StateFailed    LifecycleState = "failed"
	StateKilled    LifecycleState = "killed"
)

// ExitReason describes why a workload terminated
type ExitReason string

const (
	ExitReasonSuccess       ExitReason = "success"           // Exit code 0
	ExitReasonError         ExitReason = "error"             // Exit code != 0
	ExitReasonSignal        ExitReason = "signal"            // Killed by signal
	ExitReasonTimeout       ExitReason = "timeout"           // Wrapper timeout
	ExitReasonOOM           ExitReason = "oom"               // Out of memory killed
	ExitReasonCgroup        ExitReason = "cgroup_limit"      // Cgroup limit exceeded
	ExitReasonPolicy        ExitReason = "policy_violation"  // Policy enforcement
	ExitReasonUnknown       ExitReason = "unknown"
)

// LifecycleEvent represents a lifecycle state change
type LifecycleEvent struct {
	PID        int            `json:"pid"`
	State      LifecycleState `json:"state"`
	Timestamp  time.Time      `json:"timestamp"`
	ExitCode   int            `json:"exit_code,omitempty"`
	ExitReason ExitReason     `json:"exit_reason,omitempty"`
	Signal     string         `json:"signal,omitempty"`
	Message    string         `json:"message,omitempty"`
}

// DetermineExitReason analyzes process exit to determine the reason
func DetermineExitReason(exitCode int, waitStatus syscall.WaitStatus) ExitReason {
	if waitStatus.Exited() {
		if exitCode == 0 {
			return ExitReasonSuccess
		}
		
		// Check for OOM killer (exit code 137 or 143)
		if exitCode == 137 || exitCode == 143 {
			// Additional check: read /proc/[pid]/oom_score_adj if possible
			return ExitReasonOOM
		}
		
		return ExitReasonError
	}
	
	if waitStatus.Signaled() {
		signal := waitStatus.Signal()
		
		switch signal {
		case syscall.SIGKILL:
			// Could be OOM killer or manual kill
			return ExitReasonSignal
		case syscall.SIGTERM, syscall.SIGINT:
			return ExitReasonSignal
		case syscall.SIGXCPU:
			return ExitReasonCgroup // CPU limit exceeded
		default:
			return ExitReasonSignal
		}
	}
	
	return ExitReasonUnknown
}

// SignalName returns the signal name for a signal number
func SignalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGHUP:
		return "SIGHUP"
	case syscall.SIGQUIT:
		return "SIGQUIT"
	case syscall.SIGABRT:
		return "SIGABRT"
	case syscall.SIGSEGV:
		return "SIGSEGV"
	case syscall.SIGPIPE:
		return "SIGPIPE"
	case syscall.SIGXCPU:
		return "SIGXCPU"
	case syscall.SIGXFSZ:
		return "SIGXFSZ"
	default:
		return fmt.Sprintf("SIG%d", sig)
	}
}

// IsSuccess returns true if the exit represents success
func (r ExitReason) IsSuccess() bool {
	return r == ExitReasonSuccess
}

// IsPlatformIssue returns true if the exit was due to platform/wrapper behavior
func (r ExitReason) IsPlatformIssue() bool {
	// These are NOT platform issues - workload itself failed
	if r == ExitReasonSuccess || r == ExitReasonError || r == ExitReasonSignal {
		return false
	}
	
	// These ARE platform issues - resource/policy enforcement
	return r == ExitReasonTimeout || r == ExitReasonOOM || 
	       r == ExitReasonCgroup || r == ExitReasonPolicy
}
