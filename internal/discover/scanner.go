package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Process represents a discovered running process with rich metadata
type Process struct {
	PID            int
	Command        string
	CommandLine    []string
	StartTime      time.Time
	Monitored      bool
	
	// Enhanced metadata (Phase 2)
	UserID         int           // UID of process owner
	Username       string        // Username of process owner
	ParentPID      int           // Parent process ID
	WorkingDir     string        // Current working directory
	ProcessAge     time.Duration // Time since process started
}

// Scanner discovers running FFmpeg/transcoding processes
type Scanner struct {
	targetCommands []string
	trackedPIDs    map[int]bool
	ownPID         int              // Scanner's own PID (to filter out self)
	excludePPIDs   map[int]bool     // Parent PIDs to exclude
	filter         *FilterConfig    // Advanced filtering rules
}

// NewScanner creates a new process scanner
func NewScanner(targetCommands []string) *Scanner {
	if len(targetCommands) == 0 {
		targetCommands = []string{"ffmpeg", "gst-launch-1.0"}
	}
	return &Scanner{
		targetCommands: targetCommands,
		trackedPIDs:    make(map[int]bool),
		ownPID:         os.Getpid(),
		excludePPIDs:   make(map[int]bool),
		filter:         NewFilterConfig(), // Default: allow all
	}
}

// SetFilter sets the filtering rules for the scanner
func (s *Scanner) SetFilter(filter *FilterConfig) {
	s.filter = filter
}

// ExcludeParentPID adds a parent PID to exclude (e.g., watch daemon's own PID)
func (s *Scanner) ExcludeParentPID(ppid int) {
	s.excludePPIDs[ppid] = true
}

// ScanRunningProcesses discovers all matching processes
func (s *Scanner) ScanRunningProcesses() ([]*Process, error) {
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	var processes []*Process

	for _, entry := range entries {
		// Only check numeric directories (PIDs)
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Read command line
		cmdlinePath := filepath.Join(procDir, pidStr, "cmdline")
		cmdlineBytes, err := os.ReadFile(cmdlinePath)
		if err != nil {
			continue // Process may have exited
		}

		// Parse command line (null-separated)
		cmdline := string(cmdlineBytes)
		if cmdline == "" {
			continue
		}

		parts := strings.Split(cmdline, "\x00")
		if len(parts) == 0 {
			continue
		}

		// Check if this is a target command
		command := filepath.Base(parts[0])
		if !s.isTargetCommand(command) {
			continue
		}

		// Filter out our own PID
		if pid == s.ownPID {
			continue
		}

		// Get parent PID and check if we should exclude it
		statPath := filepath.Join(procDir, pidStr, "stat")
		ppid := s.getParentPID(statPath)
		if s.excludePPIDs[ppid] {
			continue
		}

		// Get process start time
		startTime, err := s.getProcessStartTime(statPath)
		if err != nil {
			startTime = time.Time{}
		}
		
		// Calculate process age
		processAge := time.Duration(0)
		if !startTime.IsZero() {
			processAge = time.Since(startTime)
		}
		
		// Get user ID and username
		uid := s.getUserID(filepath.Join(procDir, pidStr))
		username := s.getUsername(uid)
		
		// Get working directory
		workingDir := s.getWorkingDir(filepath.Join(procDir, pidStr, "cwd"))

		proc := &Process{
			PID:         pid,
			Command:     command,
			CommandLine: parts,
			StartTime:   startTime,
			Monitored:   s.trackedPIDs[pid],
			UserID:      uid,
			Username:    username,
			ParentPID:   ppid,
			WorkingDir:  workingDir,
			ProcessAge:  processAge,
		}
		
		// Apply filtering rules
		if !s.filter.ShouldDiscover(proc) {
			continue
		}

		processes = append(processes, proc)
	}

	return processes, nil
}

// MarkAsTracked marks a PID as being monitored
func (s *Scanner) MarkAsTracked(pid int) {
	s.trackedPIDs[pid] = true
}

// UnmarkTracked removes a PID from tracked list
func (s *Scanner) UnmarkTracked(pid int) {
	delete(s.trackedPIDs, pid)
}

// IsTracked checks if a PID is already being monitored
func (s *Scanner) IsTracked(pid int) bool {
	return s.trackedPIDs[pid]
}

// GetNewProcesses returns only untracked processes
func (s *Scanner) GetNewProcesses() ([]*Process, error) {
	allProcesses, err := s.ScanRunningProcesses()
	if err != nil {
		return nil, err
	}

	var newProcesses []*Process
	for _, proc := range allProcesses {
		if !s.trackedPIDs[proc.PID] {
			newProcesses = append(newProcesses, proc)
		}
	}

	return newProcesses, nil
}

// isTargetCommand checks if a command matches our targets
func (s *Scanner) isTargetCommand(cmd string) bool {
	for _, target := range s.targetCommands {
		if cmd == target {
			return true
		}
	}
	return false
}

// getProcessStartTime reads process start time from /proc/[pid]/stat
func (s *Scanner) getProcessStartTime(statPath string) (time.Time, error) {
	data, err := os.ReadFile(statPath)
	if err != nil {
		return time.Time{}, err
	}

	// Parse stat file - starttime is field 22 (1-indexed, 21 in 0-indexed)
	// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags ...
	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return time.Time{}, fmt.Errorf("invalid stat format")
	}

	// starttime is in clock ticks since system boot
	startTicks, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	// Read system boot time from /proc/stat
	bootTime, err := s.getSystemBootTime()
	if err != nil {
		return time.Time{}, err
	}

	// Convert ticks to seconds (usually 100 ticks per second)
	clockTicks := int64(100) // USER_HZ, typically 100
	startSeconds := startTicks / clockTicks

	return bootTime.Add(time.Duration(startSeconds) * time.Second), nil
}

// getSystemBootTime reads system boot time from /proc/stat
func (s *Scanner) getSystemBootTime() (time.Time, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				bootTimestamp, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return time.Time{}, err
				}
				return time.Unix(bootTimestamp, 0), nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("btime not found in /proc/stat")
}

// getParentPID extracts parent PID from /proc/[pid]/stat
func (s *Scanner) getParentPID(statPath string) int {
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0
	}

	// Parse stat file - ppid is field 4 (1-indexed, 3 in 0-indexed)
	// Format: pid (comm) state ppid ...
	fields := strings.Fields(string(data))
	if len(fields) < 4 {
		return 0
	}

	ppid, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0
	}

	return ppid
}

// getUserID extracts the UID of the process owner from /proc/[pid]
func (s *Scanner) getUserID(procPath string) int {
	fileInfo, err := os.Stat(procPath)
	if err != nil {
		return -1
	}
	
	// Get UID from file ownership
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid)
	}
	
	return -1
}

// getUsername converts UID to username
func (s *Scanner) getUsername(uid int) string {
	if uid < 0 {
		return "unknown"
	}
	
	// Read /etc/passwd to map UID to username
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return fmt.Sprintf("uid:%d", uid)
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		fields := strings.Split(line, ":")
		if len(fields) >= 3 {
			username := fields[0]
			uidStr := fields[2]
			
			if uidStr == strconv.Itoa(uid) {
				return username
			}
		}
	}
	
	return fmt.Sprintf("uid:%d", uid)
}

// getWorkingDir reads the current working directory of a process
func (s *Scanner) getWorkingDir(cwdPath string) string {
	target, err := os.Readlink(cwdPath)
	if err != nil {
		return ""
	}
	return target
}
