package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProcessState represents the persisted state of a discovered process
type ProcessState struct {
	PID           int       `json:"pid"`
	JobID         string    `json:"job_id"`
	Command       string    `json:"command"`
	DiscoveredAt  time.Time `json:"discovered_at"`
	AttachedAt    time.Time `json:"attached_at,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// DaemonState represents the complete state of the watch daemon
type DaemonState struct {
	Version      string                  `json:"version"`
	LastScanAt   time.Time               `json:"last_scan_at"`
	Processes    map[int]*ProcessState   `json:"processes"`
	Statistics   StateStatistics         `json:"statistics"`
}

// StateStatistics contains aggregate statistics
type StateStatistics struct {
	TotalScans       int64 `json:"total_scans"`
	TotalDiscovered  int64 `json:"total_discovered"`
	TotalAttachments int64 `json:"total_attachments"`
}

// StateManager handles persistence of daemon state
type StateManager struct {
	statePath string
	mu        sync.RWMutex
	state     *DaemonState
	
	// Configuration
	flushInterval time.Duration
	enableSync    bool
	
	// Channels for async operations
	flushChan chan struct{}
	stopChan  chan struct{}
}

// StateConfig configures the state manager
type StateConfig struct {
	StatePath     string        // Path to state file
	FlushInterval time.Duration // How often to flush to disk
	EnableSync    bool          // Use fsync for durability
}

// NewStateManager creates a new state manager
func NewStateManager(config *StateConfig) *StateManager {
	if config.StatePath == "" {
		config.StatePath = "/var/lib/ffrtmp/watch-state.json"
	}
	
	if config.FlushInterval == 0 {
		config.FlushInterval = 30 * time.Second
	}
	
	return &StateManager{
		statePath:     config.StatePath,
		flushInterval: config.FlushInterval,
		enableSync:    config.EnableSync,
		state: &DaemonState{
			Version:   "1.0",
			Processes: make(map[int]*ProcessState),
		},
		flushChan: make(chan struct{}, 1),
		stopChan:  make(chan struct{}),
	}
}

// Load loads state from disk
func (sm *StateManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Check if state file exists
	if _, err := os.Stat(sm.statePath); os.IsNotExist(err) {
		// No existing state, start fresh
		return nil
	}
	
	data, err := os.ReadFile(sm.statePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}
	
	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}
	
	// Validate and clean stale PIDs
	validProcesses := make(map[int]*ProcessState)
	staleCount := 0
	
	for pid, proc := range state.Processes {
		if pidExists(pid) {
			validProcesses[pid] = proc
		} else {
			staleCount++
		}
	}
	
	state.Processes = validProcesses
	sm.state = &state
	
	if staleCount > 0 {
		// Log stale PID cleanup but don't fail
		fmt.Printf("[state] Cleaned %d stale PIDs from state\n", staleCount)
	}
	
	return nil
}

// Save saves state to disk atomically
func (sm *StateManager) Save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Ensure directory exists
	dir := filepath.Dir(sm.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	
	// Marshal state to JSON
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	// Write to temporary file first (atomic operation)
	tempPath := sm.statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	
	// Sync to disk if enabled
	if sm.enableSync {
		f, err := os.OpenFile(tempPath, os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to open temp file for sync: %w", err)
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return fmt.Errorf("failed to sync temp file: %w", err)
		}
		f.Close()
	}
	
	// Atomic rename
	if err := os.Rename(tempPath, sm.statePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	
	return nil
}

// RecordDiscovery records a newly discovered process
func (sm *StateManager) RecordDiscovery(pid int, jobID, command string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	
	sm.state.Processes[pid] = &ProcessState{
		PID:          pid,
		JobID:        jobID,
		Command:      command,
		DiscoveredAt: now,
		LastSeenAt:   now,
	}
	
	sm.state.Statistics.TotalDiscovered++
	
	// Trigger async flush
	select {
	case sm.flushChan <- struct{}{}:
	default:
	}
}

// RecordAttachment records successful attachment to a process
func (sm *StateManager) RecordAttachment(pid int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if proc, ok := sm.state.Processes[pid]; ok {
		proc.AttachedAt = time.Now()
		proc.LastSeenAt = time.Now()
		sm.state.Statistics.TotalAttachments++
	}
	
	// Trigger async flush
	select {
	case sm.flushChan <- struct{}{}:
	default:
	}
}

// RecordScan records a scan event
func (sm *StateManager) RecordScan() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.state.LastScanAt = time.Now()
	sm.state.Statistics.TotalScans++
}

// RemoveProcess removes a process from state (when it exits)
func (sm *StateManager) RemoveProcess(pid int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.state.Processes, pid)
	
	// Trigger async flush
	select {
	case sm.flushChan <- struct{}{}:
	default:
	}
}

// GetTrackedPIDs returns currently tracked PIDs
func (sm *StateManager) GetTrackedPIDs() []int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	pids := make([]int, 0, len(sm.state.Processes))
	for pid := range sm.state.Processes {
		pids = append(pids, pid)
	}
	
	return pids
}

// GetProcessState returns state for a specific process
func (sm *StateManager) GetProcessState(pid int) (*ProcessState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	proc, ok := sm.state.Processes[pid]
	return proc, ok
}

// GetStatistics returns aggregate statistics
func (sm *StateManager) GetStatistics() StateStatistics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return sm.state.Statistics
}

// StartPeriodicFlush starts periodic state flushing
func (sm *StateManager) StartPeriodicFlush() {
	ticker := time.NewTicker(sm.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := sm.Save(); err != nil {
				fmt.Printf("[state] Failed to flush state: %v\n", err)
			}
			
		case <-sm.flushChan:
			// Immediate flush requested
			if err := sm.Save(); err != nil {
				fmt.Printf("[state] Failed to flush state: %v\n", err)
			}
			
		case <-sm.stopChan:
			// Final flush on shutdown
			if err := sm.Save(); err != nil {
				fmt.Printf("[state] Failed to save final state: %v\n", err)
			}
			return
		}
	}
}

// Stop stops the state manager and performs final flush
func (sm *StateManager) Stop() {
	close(sm.stopChan)
}

// pidExists is a helper to check if PID still exists
func pidExists(pid int) bool {
	// Check if /proc/[pid] exists
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}
