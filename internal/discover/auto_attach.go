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
	
	// StateConfig for persistence (optional)
	StateConfig *StateConfig
	
	// Reliability configuration (Phase 3)
	EnableRetry   bool // Enable retry for failed attachments
	MaxRetryAttempts int  // Max retry attempts (default: 3)
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
	
	// State management (Phase 3)
	stateManager *StateManager
	
	// Reliability (Phase 3)
	healthCheck     *HealthCheck
	retryQueue      *RetryQueue
	errorClassifier *ErrorClassifier
	
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
	
	service := &AutoAttachService{
		config:          config,
		scanner:         NewScanner(config.TargetCommands),
		attachments:     make(map[int]context.CancelFunc),
		stopCh:          make(chan struct{}),
		logger:          logger,
		healthCheck:     NewHealthCheck(),
		errorClassifier: &ErrorClassifier{},
	}
	
	// Initialize retry queue if enabled
	if config.EnableRetry {
		maxAttempts := config.MaxRetryAttempts
		if maxAttempts <= 0 {
			maxAttempts = 3
		}
		service.retryQueue = NewRetryQueue(maxAttempts, logger)
		logger.Printf("Retry queue enabled (max attempts: %d)", maxAttempts)
	}
	
	// Initialize state manager if configured
	if config.StateConfig != nil {
		service.stateManager = NewStateManager(config.StateConfig)
		
		// Load existing state
		if err := service.stateManager.Load(); err != nil {
			logger.Printf("Warning: failed to load state: %v", err)
		} else {
			logger.Printf("State loaded from %s", config.StateConfig.StatePath)
			
			// Restore statistics from state
			stats := service.stateManager.GetStatistics()
			service.statsMu.Lock()
			service.stats.TotalScans = stats.TotalScans
			service.stats.TotalDiscovered = stats.TotalDiscovered
			service.stats.TotalAttachments = stats.TotalAttachments
			service.statsMu.Unlock()
		}
		
		// Start periodic flush
		go service.stateManager.StartPeriodicFlush()
	}
	
	return service
}

// Start begins the auto-attach service
func (s *AutoAttachService) Start(ctx context.Context) error {
	s.logger.Println("Starting auto-attach service...")
	s.logger.Printf("Scan interval: %v", s.config.ScanInterval)
	s.logger.Printf("Target commands: %v", s.config.TargetCommands)
	
	// Exclude watch daemon's own PID from discoveries
	s.scanner.ExcludeParentPID(s.scanner.ownPID)
	
	// Start retry worker if enabled
	if s.retryQueue != nil {
		retryFunc := func(proc *Process) error {
			s.logger.Printf("Retrying attachment to PID %d (%s)", proc.PID, proc.Command)
			s.attachToProcess(proc)
			return nil
		}
		go s.retryQueue.StartRetryWorker(ctx, retryFunc)
		s.logger.Println("Retry worker started")
	}
	
	ticker := time.NewTicker(s.config.ScanInterval)
	defer ticker.Stop()
	
	// Initial scan
	s.logger.Println("Performing initial scan...")
	if err := s.scanAndAttach(); err != nil {
		s.logger.Printf("Initial scan failed: %v", err)
		// Log health status after initial scan failure
		if s.healthCheck != nil {
			status := s.healthCheck.GetStatus()
			s.logger.Printf("Health status: %s", status)
		}
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
				// Log health status on degradation
				if s.healthCheck != nil {
					status := s.healthCheck.GetStatus()
					report := s.healthCheck.GetHealthReport()
					if status != HealthStatusHealthy {
						s.logger.Printf("Health status: %s (scan failures: %d, attach failures: %d)",
							status, report["consecutive_scan_failures"], report["consecutive_attachment_failures"])
					}
				}
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
	
	// Stop state manager if enabled
	if s.stateManager != nil {
		s.stateManager.Stop()
		s.logger.Println("State saved")
	}
	
	close(s.stopCh)
	s.logger.Println("Auto-attach service stopped")
}

// scanAndAttach discovers new processes and attaches to them
func (s *AutoAttachService) scanAndAttach() error {
	scanStart := time.Now()
	
	newProcesses, err := s.scanner.GetNewProcesses()
	if err != nil {
		// Wrap error with context for error classification
		discErr := &DiscoveryError{
			Operation: "scan",
			Timestamp: scanStart,
			Err:       err,
		}
		
		// Classify error and determine if retryable
		errType := s.errorClassifier.Classify(err)
		discErr.Type = errType
		discErr.Retryable = (errType == ErrorTypeTransient || errType == ErrorTypeRateLimit)
		
		// Record failure in health check
		if s.healthCheck != nil {
			s.healthCheck.RecordScanFailure(discErr)
		}
		
		return discErr
	}
	
	scanDuration := time.Since(scanStart)
	
	// Record successful scan in health check
	if s.healthCheck != nil {
		s.healthCheck.RecordScanSuccess()
	}
	
	// Update statistics
	s.statsMu.Lock()
	s.stats.TotalScans++
	s.stats.LastScanDuration = scanDuration
	s.stats.LastScanTime = scanStart
	totalFound := len(newProcesses)
	s.stats.TotalDiscovered += int64(totalFound)
	s.statsMu.Unlock()
	
	// Record scan in state manager
	if s.stateManager != nil {
		s.stateManager.RecordScan()
	}
	
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
	
	// Record discovery in state manager
	if s.stateManager != nil {
		s.stateManager.RecordDiscovery(proc.PID, jobID, proc.Command)
	}
	
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
		// Record attachment in state manager
		if s.stateManager != nil {
			s.stateManager.RecordAttachment(proc.PID)
		}
		
		attachStart := time.Now()
		result, err := wrapper.Attach(ctx, jobID, proc.PID, s.config.DefaultLimits)
		
		// Handle attachment error
		if err != nil && err != context.Canceled {
			// Wrap error with context for classification
			discErr := &DiscoveryError{
				Operation: "attach",
				PID:       proc.PID,
				Timestamp: attachStart,
				Err:       err,
			}
			
			// Classify error and determine if retryable
			errType := s.errorClassifier.Classify(err)
			discErr.Type = errType
			discErr.Retryable = (errType == ErrorTypeTransient || errType == ErrorTypeRateLimit || errType == ErrorTypeResource)
			
			// Record failure in health check
			if s.healthCheck != nil {
				s.healthCheck.RecordAttachFailure(discErr)
			}
			
			// Add to retry queue if retryable and retry is enabled
			if discErr.Retryable && s.retryQueue != nil {
				s.retryQueue.Add(proc, discErr)
				s.logger.Printf("Attachment to PID %d failed (%s), added to retry queue: %v", proc.PID, errType, err)
			} else {
				s.logger.Printf("Attachment to PID %d failed (%s): %v", proc.PID, errType, err)
			}
			
			// Clean up failed attachment
			s.mu.Lock()
			delete(s.attachments, proc.PID)
			s.scanner.UnmarkTracked(proc.PID)
			s.mu.Unlock()
			
			// Remove from state manager
			if s.stateManager != nil {
				s.stateManager.RemoveProcess(proc.PID)
			}
			
			return
		}
		
		// Update statistics for successful attachment
		s.statsMu.Lock()
		s.stats.TotalAttachments++
		s.statsMu.Unlock()
		
		// Record success in health check
		if s.healthCheck != nil {
			s.healthCheck.RecordAttachSuccess()
		}
		
		// Clean up when attachment ends
		s.mu.Lock()
		delete(s.attachments, proc.PID)
		s.scanner.UnmarkTracked(proc.PID)
		s.mu.Unlock()
		
		// Remove from state manager
		if s.stateManager != nil {
			s.stateManager.RemoveProcess(proc.PID)
		}
		
		if result != nil {
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

// GetScanner returns the underlying scanner (for applying filters)
func (s *AutoAttachService) GetScanner() *Scanner {
	return s.scanner
}

// GetHealthStatus returns the current health status and detailed report
func (s *AutoAttachService) GetHealthStatus() (HealthStatus, map[string]interface{}) {
	if s.healthCheck == nil {
		return HealthStatusHealthy, nil
	}
	return s.healthCheck.GetStatus(), s.healthCheck.GetHealthReport()
}

// GetHealthReport returns a detailed health report
func (s *AutoAttachService) GetHealthReport() map[string]interface{} {
	if s.healthCheck == nil {
		return map[string]interface{}{"enabled": false}
	}
	return s.healthCheck.GetHealthReport()
}
