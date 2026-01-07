package discover

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
	"github.com/psantana5/ffmpeg-rtmp/internal/wrapper"
)

// AttachConfig configures the auto-attach service
type AttachConfig struct {
	// ScanInterval is how often to scan for new processes
	ScanInterval time.Duration
	
	// TargetCommands are the commands to look for (e.g., "ffmpeg", "gst-launch-1.0")
	TargetCommands []string
	
	// DefaultLimits are resource limits to apply to discovered processes
	DefaultLimits *cgroups.Limits
	
	// OnAttach is called when a process is attached
	OnAttach func(pid int, jobID string)
	
	// OnDetach is called when a monitored process exits
	OnDetach func(pid int, jobID string)
	
	// Logger for output (optional)
	Logger *log.Logger
}

// AutoAttachService automatically discovers and attaches to running processes
type AutoAttachService struct {
	config  *AttachConfig
	scanner *Scanner
	
	// Track active attachments
	attachments map[int]context.CancelFunc
	mu          sync.Mutex
	
	// Statistics
	stats struct {
		TotalScans       int64
		TotalDiscovered  int64
		TotalAttachments int64
		LastScanDuration time.Duration
		LastScanTime     time.Time
	}
	statsMu sync.RWMutex
	
	// Control channels
	stopCh chan struct{}
	logger *log.Logger
}

// NewAutoAttachService creates a new auto-attach service
func NewAutoAttachService(config *AttachConfig) *AutoAttachService {
	if config.ScanInterval == 0 {
		config.ScanInterval = 10 * time.Second
	}
	
	if config.DefaultLimits == nil {
		config.DefaultLimits = &cgroups.Limits{
			CPUWeight: 100,
		}
	}
	
	logger := config.Logger
	if logger == nil {
		logger = log.New(log.Writer(), "[auto-attach] ", log.LstdFlags)
	}
	
	return &AutoAttachService{
		config:      config,
		scanner:     NewScanner(config.TargetCommands),
		attachments: make(map[int]context.CancelFunc),
		stopCh:      make(chan struct{}),
		logger:      logger,
	}
}

// Start begins the auto-attach service
func (s *AutoAttachService) Start(ctx context.Context) error {
	s.logger.Println("Starting auto-attach service...")
	s.logger.Printf("Scan interval: %v", s.config.ScanInterval)
	s.logger.Printf("Target commands: %v", s.config.TargetCommands)
	
	// Exclude watch daemon's own PID from discoveries
	s.scanner.ExcludeParentPID(s.scanner.ownPID)
	
	ticker := time.NewTicker(s.config.ScanInterval)
	defer ticker.Stop()
	
	// Initial scan
	s.logger.Println("Performing initial scan...")
	if err := s.scanAndAttach(); err != nil {
		s.logger.Printf("Initial scan failed: %v", err)
	}
	
	for {
		select {
		case <-ctx.Done():
			s.logger.Println("Context canceled, stopping auto-attach service")
			s.Stop()
			return ctx.Err()
		case <-s.stopCh:
			s.logger.Println("Stop signal received")
			return nil
		case <-ticker.C:
			s.logger.Println("Scanning for processes...")
			if err := s.scanAndAttach(); err != nil {
				s.logger.Printf("Scan failed: %v", err)
			}
		}
	}
}

// Stop stops the auto-attach service
func (s *AutoAttachService) Stop() {
	s.logger.Println("Stopping auto-attach service...")
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Cancel all active attachments
	for pid, cancel := range s.attachments {
		s.logger.Printf("Detaching from PID %d", pid)
		cancel()
	}
	
	close(s.stopCh)
	s.logger.Println("Auto-attach service stopped")
}

// scanAndAttach discovers new processes and attaches to them
func (s *AutoAttachService) scanAndAttach() error {
	scanStart := time.Now()
	
	newProcesses, err := s.scanner.GetNewProcesses()
	if err != nil {
		return fmt.Errorf("failed to scan processes: %w", err)
	}
	
	scanDuration := time.Since(scanStart)
	
	// Update statistics
	s.statsMu.Lock()
	s.stats.TotalScans++
	s.stats.LastScanDuration = scanDuration
	s.stats.LastScanTime = scanStart
	totalFound := len(newProcesses)
	s.stats.TotalDiscovered += int64(totalFound)
	s.statsMu.Unlock()
	
	// Get current tracked count
	s.mu.Lock()
	trackedCount := len(s.attachments)
	s.mu.Unlock()
	
	// Log scan results with statistics
	if len(newProcesses) > 0 {
		s.logger.Printf("Discovered %d new process(es)", len(newProcesses))
		s.logger.Printf("Scan complete: new=%d tracked=%d duration=%v",
			len(newProcesses), trackedCount, scanDuration)
	}
	
	for _, proc := range newProcesses {
		s.attachToProcess(proc)
	}
	
	return nil
}

// attachToProcess attaches to a specific process
func (s *AutoAttachService) attachToProcess(proc *Process) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Generate job ID for this process
	jobID := fmt.Sprintf("auto-%s-%d", proc.Command, proc.PID)
	
	s.logger.Printf("Attaching to PID %d (%s) as job %s", proc.PID, proc.Command, jobID)
	
	// Mark as tracked
	s.scanner.MarkAsTracked(proc.PID)
	
	// Create context for this attachment
	ctx, cancel := context.WithCancel(context.Background())
	s.attachments[proc.PID] = cancel
	
	// Call OnAttach callback
	if s.config.OnAttach != nil {
		s.config.OnAttach(proc.PID, jobID)
	}
	
	// Attach in background
	go func() {
		result, err := wrapper.Attach(ctx, jobID, proc.PID, s.config.DefaultLimits)
		
		// Update statistics
		s.statsMu.Lock()
		s.stats.TotalAttachments++
		s.statsMu.Unlock()
		
		// Clean up when attachment ends
		s.mu.Lock()
		delete(s.attachments, proc.PID)
		s.scanner.UnmarkTracked(proc.PID)
		s.mu.Unlock()
		
		if err != nil && err != context.Canceled {
			s.logger.Printf("Attachment to PID %d failed: %v", proc.PID, err)
		} else if result != nil {
			s.logger.Printf("Process %d exited (job %s, duration: %.2fs)", 
				proc.PID, result.JobID, result.Duration.Seconds())
		}
		
		// Call OnDetach callback
		if s.config.OnDetach != nil {
			s.config.OnDetach(proc.PID, jobID)
		}
	}()
}

// GetActiveAttachments returns the count of currently monitored processes
func (s *AutoAttachService) GetActiveAttachments() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.attachments)
}

// GetTrackedPIDs returns a list of currently tracked PIDs
func (s *AutoAttachService) GetTrackedPIDs() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	pids := make([]int, 0, len(s.attachments))
	for pid := range s.attachments {
		pids = append(pids, pid)
	}
	return pids
}

// Stats represents service statistics
type Stats struct {
	TotalScans       int64
	TotalDiscovered  int64
	TotalAttachments int64
	ActiveAttachments int
	LastScanDuration time.Duration
	LastScanTime     time.Time
}

// GetStats returns current service statistics
func (s *AutoAttachService) GetStats() Stats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	
	s.mu.Lock()
	activeCount := len(s.attachments)
	s.mu.Unlock()
	
	return Stats{
		TotalScans:       s.stats.TotalScans,
		TotalDiscovered:  s.stats.TotalDiscovered,
		TotalAttachments: s.stats.TotalAttachments,
		ActiveAttachments: activeCount,
		LastScanDuration: s.stats.LastScanDuration,
		LastScanTime:     s.stats.LastScanTime,
	}
}
