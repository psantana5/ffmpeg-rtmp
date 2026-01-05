package resources

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// ResourceLimits defines resource constraints for a job
type ResourceLimits struct {
	MaxCPUPercent int   // CPU limit as percentage (100 = 1 core, 200 = 2 cores)
	MaxMemoryMB   int   // Memory limit in MB
	MaxDiskMB     int   // Disk space limit in MB
	TimeoutSec    int   // Timeout in seconds
}

// DefaultLimits returns sensible default resource limits
func DefaultLimits() *ResourceLimits {
	numCPU := runtime.NumCPU()
	
	return &ResourceLimits{
		MaxCPUPercent: numCPU * 100, // Use all available CPUs by default
		MaxMemoryMB:   2048,          // 2GB default
		MaxDiskMB:     5000,          // 5GB temp space default
		TimeoutSec:    3600,          // 1 hour default timeout
	}
}

// CgroupManager manages cgroup resources for jobs
type CgroupManager struct {
	cgroupRoot    string
	cgroupVersion int // 1 for v1, 2 for v2
}

// NewCgroupManager creates a new cgroup manager
func NewCgroupManager() (*CgroupManager, error) {
	// Detect cgroup version
	version := detectCgroupVersion()
	
	var cgroupRoot string
	if version == 2 {
		cgroupRoot = "/sys/fs/cgroup"
	} else {
		cgroupRoot = "/sys/fs/cgroup"
	}
	
	log.Printf("Detected cgroup v%d (root: %s)", version, cgroupRoot)
	
	return &CgroupManager{
		cgroupRoot:    cgroupRoot,
		cgroupVersion: version,
	}, nil
}

// detectCgroupVersion detects whether system uses cgroup v1 or v2
func detectCgroupVersion() int {
	// Check if cgroup v2 unified hierarchy exists
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return 2
	}
	return 1
}

// CreateCgroup creates a cgroup for a job with specified limits
func (cm *CgroupManager) CreateCgroup(jobID string, limits *ResourceLimits) (string, error) {
	if limits == nil {
		limits = DefaultLimits()
	}
	
	cgroupName := fmt.Sprintf("ffmpeg-job-%s", jobID)
	
	if cm.cgroupVersion == 2 {
		return cm.createCgroupV2(cgroupName, limits)
	}
	return cm.createCgroupV1(cgroupName, limits)
}

// createCgroupV2 creates cgroup for v2 (unified hierarchy)
func (cm *CgroupManager) createCgroupV2(cgroupName string, limits *ResourceLimits) (string, error) {
	cgroupPath := filepath.Join(cm.cgroupRoot, cgroupName)
	
	// Create cgroup directory
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		// If permission denied, cgroups might not be available
		if os.IsPermission(err) {
			log.Printf("WARNING: Cannot create cgroup (permission denied), running without cgroup limits")
			return "", nil
		}
		return "", fmt.Errorf("failed to create cgroup: %w", err)
	}
	
	// Set CPU limit (cpu.max format: "quota period")
	// quota = (percent / 100) * period
	// period = 100000 (100ms in microseconds)
	if limits.MaxCPUPercent > 0 {
		period := 100000
		quota := (limits.MaxCPUPercent * period) / 100
		cpuMax := fmt.Sprintf("%d %d", quota, period)
		
		cpuMaxFile := filepath.Join(cgroupPath, "cpu.max")
		if err := os.WriteFile(cpuMaxFile, []byte(cpuMax), 0644); err != nil {
			log.Printf("WARNING: Failed to set CPU limit: %v", err)
		} else {
			log.Printf("Set CPU limit: %d%% (%s)", limits.MaxCPUPercent, cpuMax)
		}
	}
	
	// Set memory limit
	if limits.MaxMemoryMB > 0 {
		memoryBytes := int64(limits.MaxMemoryMB) * 1024 * 1024
		memoryMaxFile := filepath.Join(cgroupPath, "memory.max")
		
		if err := os.WriteFile(memoryMaxFile, []byte(fmt.Sprintf("%d", memoryBytes)), 0644); err != nil {
			log.Printf("WARNING: Failed to set memory limit: %v", err)
		} else {
			log.Printf("Set memory limit: %d MB", limits.MaxMemoryMB)
		}
	}
	
	return cgroupPath, nil
}

// createCgroupV1 creates cgroup for v1 (separate hierarchies)
func (cm *CgroupManager) createCgroupV1(cgroupName string, limits *ResourceLimits) (string, error) {
	// For v1, we need separate paths for cpu and memory
	cpuPath := filepath.Join(cm.cgroupRoot, "cpu", cgroupName)
	memoryPath := filepath.Join(cm.cgroupRoot, "memory", cgroupName)
	
	// Create CPU cgroup
	if err := os.MkdirAll(cpuPath, 0755); err != nil {
		if os.IsPermission(err) {
			log.Printf("WARNING: Cannot create cgroup (permission denied), running without cgroup limits")
			return "", nil
		}
		return "", fmt.Errorf("failed to create CPU cgroup: %w", err)
	}
	
	// Create memory cgroup
	if err := os.MkdirAll(memoryPath, 0755); err != nil {
		log.Printf("WARNING: Failed to create memory cgroup: %v", err)
	}
	
	// Set CPU limit (cfs_quota_us / cfs_period_us = CPU fraction)
	if limits.MaxCPUPercent > 0 {
		period := 100000 // 100ms default period
		quota := (limits.MaxCPUPercent * period) / 100
		
		quotaFile := filepath.Join(cpuPath, "cpu.cfs_quota_us")
		periodFile := filepath.Join(cpuPath, "cpu.cfs_period_us")
		
		if err := os.WriteFile(periodFile, []byte(fmt.Sprintf("%d", period)), 0644); err != nil {
			log.Printf("WARNING: Failed to set CPU period: %v", err)
		}
		
		if err := os.WriteFile(quotaFile, []byte(fmt.Sprintf("%d", quota)), 0644); err != nil {
			log.Printf("WARNING: Failed to set CPU quota: %v", err)
		} else {
			log.Printf("Set CPU limit: %d%% (quota=%d, period=%d)", limits.MaxCPUPercent, quota, period)
		}
	}
	
	// Set memory limit
	if limits.MaxMemoryMB > 0 {
		memoryBytes := int64(limits.MaxMemoryMB) * 1024 * 1024
		limitFile := filepath.Join(memoryPath, "memory.limit_in_bytes")
		
		if err := os.WriteFile(limitFile, []byte(fmt.Sprintf("%d", memoryBytes)), 0644); err != nil {
			log.Printf("WARNING: Failed to set memory limit: %v", err)
		} else {
			log.Printf("Set memory limit: %d MB", limits.MaxMemoryMB)
		}
	}
	
	return cpuPath, nil
}

// AddProcessToCgroup adds a process to the cgroup
func (cm *CgroupManager) AddProcessToCgroup(cgroupPath string, pid int) error {
	if cgroupPath == "" {
		// Cgroup not created (likely due to permissions), skip
		return nil
	}
	
	if cm.cgroupVersion == 2 {
		procsFile := filepath.Join(cgroupPath, "cgroup.procs")
		return os.WriteFile(procsFile, []byte(fmt.Sprintf("%d", pid)), 0644)
	}
	
	// For v1, add to both cpu and memory cgroups
	cpuProcs := filepath.Join(cgroupPath, "cgroup.procs")
	memoryPath := strings.Replace(cgroupPath, "/cpu/", "/memory/", 1)
	memoryProcs := filepath.Join(memoryPath, "cgroup.procs")
	
	if err := os.WriteFile(cpuProcs, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to add process to CPU cgroup: %w", err)
	}
	
	if err := os.WriteFile(memoryProcs, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		log.Printf("WARNING: Failed to add process to memory cgroup: %v", err)
	}
	
	return nil
}

// RemoveCgroup removes a cgroup after job completion
func (cm *CgroupManager) RemoveCgroup(cgroupPath string) error {
	if cgroupPath == "" {
		return nil
	}
	
	// For v1, also remove memory cgroup
	if cm.cgroupVersion == 1 {
		memoryPath := strings.Replace(cgroupPath, "/cpu/", "/memory/", 1)
		if err := os.Remove(memoryPath); err != nil && !os.IsNotExist(err) {
			log.Printf("WARNING: Failed to remove memory cgroup: %v", err)
		}
	}
	
	return os.Remove(cgroupPath)
}

// DiskSpaceInfo contains disk space information
type DiskSpaceInfo struct {
	TotalMB     uint64
	AvailableMB uint64
	UsedMB      uint64
	UsedPercent float64
}

// CheckDiskSpace checks available disk space for a path
func CheckDiskSpace(path string) (*DiskSpaceInfo, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("failed to check disk space: %w", err)
	}
	
	// Calculate sizes in MB
	totalMB := (stat.Blocks * uint64(stat.Bsize)) / (1024 * 1024)
	availableMB := (stat.Bavail * uint64(stat.Bsize)) / (1024 * 1024)
	usedMB := totalMB - availableMB
	usedPercent := (float64(usedMB) / float64(totalMB)) * 100
	
	return &DiskSpaceInfo{
		TotalMB:     totalMB,
		AvailableMB: availableMB,
		UsedMB:      usedMB,
		UsedPercent: usedPercent,
	}, nil
}

// EnsureSufficientDiskSpace checks if sufficient disk space is available
func EnsureSufficientDiskSpace(path string, requiredMB int) error {
	info, err := CheckDiskSpace(path)
	if err != nil {
		return err
	}
	
	if info.AvailableMB < uint64(requiredMB) {
		return fmt.Errorf("insufficient disk space: need %d MB, available %d MB (%.1f%% used)",
			requiredMB, info.AvailableMB, info.UsedPercent)
	}
	
	if info.UsedPercent > 90 {
		log.Printf("WARNING: Disk usage is high: %.1f%% (available: %d MB)", 
			info.UsedPercent, info.AvailableMB)
	}
	
	return nil
}

// SetProcessPriority sets the nice value for a process (lower priority)
func SetProcessPriority(pid int, niceness int) error {
	// niceness range: -20 (highest priority) to 19 (lowest priority)
	// For transcoding jobs, we typically want 10-15 (lower than normal)
	if niceness < -20 {
		niceness = -20
	}
	if niceness > 19 {
		niceness = 19
	}
	
	return syscall.Setpriority(syscall.PRIO_PROCESS, pid, niceness)
}

// GetProcessResourceUsage gets CPU and memory usage for a process
func GetProcessResourceUsage(pid int) (cpuPercent float64, memoryMB int64, err error) {
	// Read /proc/[pid]/stat for CPU times
	statFile := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statFile)
	if err != nil {
		return 0, 0, err
	}
	
	// Parse stat file (see proc(5) man page)
	fields := strings.Fields(string(data))
	if len(fields) < 24 {
		return 0, 0, fmt.Errorf("invalid stat file format")
	}
	
	// Memory is in field 23 (RSS in pages)
	rss, _ := strconv.ParseInt(fields[23], 10, 64)
	pageSize := int64(os.Getpagesize())
	memoryMB = (rss * pageSize) / (1024 * 1024)
	
	// CPU percentage would require tracking over time, simplified here
	cpuPercent = 0 // Would need baseline measurement
	
	return cpuPercent, memoryMB, nil
}

// KillProcessGroup kills a process and all its children
func KillProcessGroup(pid int) error {
	// Send SIGTERM to the process group
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return fmt.Errorf("failed to get process group: %w", err)
	}
	
	// Kill the entire process group (negative PID)
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process group: %w", err)
	}
	
	// Wait a bit for graceful termination
	time.Sleep(2 * time.Second)
	
	// Force kill if still running
	if processExists(pid) {
		log.Printf("Process %d didn't terminate gracefully, sending SIGKILL", pid)
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to send SIGKILL to process group: %w", err)
		}
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

// MonitorProcess monitors a process for resource limits and timeout
func MonitorProcess(cmd *exec.Cmd, limits *ResourceLimits, doneChan chan struct{}) {
	if cmd.Process == nil {
		return
	}
	
	pid := cmd.Process.Pid
	startTime := time.Now()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-doneChan:
			return
		case <-ticker.C:
			// Check if process still exists
			if !processExists(pid) {
				return
			}
			
			// Check timeout
			if limits.TimeoutSec > 0 {
				elapsed := time.Since(startTime).Seconds()
				if elapsed > float64(limits.TimeoutSec) {
					log.Printf("Process %d exceeded timeout (%d seconds), killing...", pid, limits.TimeoutSec)
					if err := KillProcessGroup(pid); err != nil {
						log.Printf("Error killing process group: %v", err)
					}
					return
				}
			}
			
			// Check memory usage (optional logging)
			_, memMB, err := GetProcessResourceUsage(pid)
			if err == nil && limits.MaxMemoryMB > 0 {
				if memMB > int64(limits.MaxMemoryMB) {
					log.Printf("WARNING: Process %d using %d MB (limit: %d MB)", 
						pid, memMB, limits.MaxMemoryMB)
				}
			}
		}
	}
}
