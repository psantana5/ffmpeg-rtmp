package wrapper

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// CgroupManager manages cgroup resources for wrapped workloads
type CgroupManager struct {
	cgroupRoot    string
	cgroupVersion int    // 1 for v1, 2 for v2
	namespace     string // Cgroup namespace (e.g., "ffrtmp-wrapper")
	available     bool   // Whether cgroups are available
}

// NewCgroupManager creates a new cgroup manager for the wrapper
func NewCgroupManager(namespace string) *CgroupManager {
	version := detectCgroupVersion()
	available := checkCgroupAvailable()
	
	cgroupRoot := "/sys/fs/cgroup"
	
	if !available {
		log.Printf("[wrapper] cgroups not available - will degrade gracefully")
	} else {
		log.Printf("[wrapper] cgroup v%d detected (namespace: %s)", version, namespace)
	}
	
	return &CgroupManager{
		cgroupRoot:    cgroupRoot,
		cgroupVersion: version,
		namespace:     namespace,
		available:     available,
	}
}

// detectCgroupVersion detects whether system uses cgroup v1 or v2
func detectCgroupVersion() int {
	// Check if cgroup v2 unified hierarchy exists
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return 2
	}
	return 1
}

// checkCgroupAvailable checks if cgroups are available and writable
func checkCgroupAvailable() bool {
	// Try to write to cgroup to verify permissions
	testPath := "/sys/fs/cgroup"
	
	// Check if cgroup root exists
	if _, err := os.Stat(testPath); err != nil {
		return false
	}
	
	// Check if we can read it
	if _, err := os.ReadDir(testPath); err != nil {
		return false
	}
	
	return true
}

// CreateOrJoinCgroup creates a new cgroup or joins an existing one
// Returns cgroup path (empty string if cgroups not available)
func (cm *CgroupManager) CreateOrJoinCgroup(workloadID string, constraints *Constraints) (string, error) {
	if !cm.available {
		log.Printf("[wrapper] cgroups not available - skipping cgroup creation")
		return "", nil
	}
	
	if constraints == nil {
		constraints = DefaultConstraints()
	}
	
	if err := constraints.Validate(); err != nil {
		return "", fmt.Errorf("invalid constraints: %w", err)
	}
	
	cgroupName := fmt.Sprintf("%s-%s", cm.namespace, workloadID)
	
	if cm.cgroupVersion == 2 {
		return cm.createCgroupV2(cgroupName, constraints)
	}
	return cm.createCgroupV1(cgroupName, constraints)
}

// createCgroupV2 creates or updates cgroup for v2 (unified hierarchy)
func (cm *CgroupManager) createCgroupV2(cgroupName string, constraints *Constraints) (string, error) {
	cgroupPath := filepath.Join(cm.cgroupRoot, cgroupName)
	
	// Create cgroup directory
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		if os.IsPermission(err) {
			log.Printf("[wrapper] WARNING: Cannot create cgroup (permission denied)")
			return "", nil
		}
		return "", fmt.Errorf("failed to create cgroup: %w", err)
	}
	
	log.Printf("[wrapper] Created cgroup: %s", cgroupPath)
	
	// Apply CPU quota
	if constraints.CPUQuotaPercent > 0 {
		period := 100000
		quota := (constraints.CPUQuotaPercent * period) / 100
		cpuMax := fmt.Sprintf("%d %d", quota, period)
		
		cpuMaxFile := filepath.Join(cgroupPath, "cpu.max")
		if err := os.WriteFile(cpuMaxFile, []byte(cpuMax), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set CPU quota: %v", err)
		} else {
			log.Printf("[wrapper] Applied CPU quota: %d%%", constraints.CPUQuotaPercent)
		}
	}
	
	// Apply CPU weight
	if constraints.CPUWeight > 0 && constraints.CPUWeight != 100 {
		cpuWeightFile := filepath.Join(cgroupPath, "cpu.weight")
		if err := os.WriteFile(cpuWeightFile, []byte(fmt.Sprintf("%d", constraints.CPUWeight)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set CPU weight: %v", err)
		} else {
			log.Printf("[wrapper] Applied CPU weight: %d", constraints.CPUWeight)
		}
	}
	
	// Apply memory limit
	if constraints.MemoryLimitMB > 0 {
		memoryBytes := constraints.MemoryLimitMB * 1024 * 1024
		memoryMaxFile := filepath.Join(cgroupPath, "memory.max")
		
		if err := os.WriteFile(memoryMaxFile, []byte(fmt.Sprintf("%d", memoryBytes)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set memory limit: %v", err)
		} else {
			log.Printf("[wrapper] Applied memory limit: %d MB", constraints.MemoryLimitMB)
		}
	}
	
	// Apply IO weight (if supported)
	if constraints.IOWeightPercent > 0 {
		// Convert percentage to io.weight value (1-10000)
		ioWeight := constraints.IOWeightPercent * 100
		ioWeightFile := filepath.Join(cgroupPath, "io.weight")
		
		if err := os.WriteFile(ioWeightFile, []byte(fmt.Sprintf("default %d", ioWeight)), 0644); err != nil {
			// IO weight might not be supported
			log.Printf("[wrapper] INFO: io.weight not supported or failed: %v", err)
		} else {
			log.Printf("[wrapper] Applied IO weight: %d%%", constraints.IOWeightPercent)
		}
	}
	
	return cgroupPath, nil
}

// createCgroupV1 creates cgroup for v1 (separate hierarchies)
func (cm *CgroupManager) createCgroupV1(cgroupName string, constraints *Constraints) (string, error) {
	// For v1, we need separate paths for cpu and memory
	cpuPath := filepath.Join(cm.cgroupRoot, "cpu", cgroupName)
	memoryPath := filepath.Join(cm.cgroupRoot, "memory", cgroupName)
	
	// Create CPU cgroup
	if err := os.MkdirAll(cpuPath, 0755); err != nil {
		if os.IsPermission(err) {
			log.Printf("[wrapper] WARNING: Cannot create cgroup (permission denied)")
			return "", nil
		}
		return "", fmt.Errorf("failed to create CPU cgroup: %w", err)
	}
	
	// Create memory cgroup
	if err := os.MkdirAll(memoryPath, 0755); err != nil {
		log.Printf("[wrapper] WARNING: Failed to create memory cgroup: %v", err)
	}
	
	log.Printf("[wrapper] Created cgroup v1: %s", cpuPath)
	
	// Apply CPU quota
	if constraints.CPUQuotaPercent > 0 {
		period := 100000
		quota := (constraints.CPUQuotaPercent * period) / 100
		
		quotaFile := filepath.Join(cpuPath, "cpu.cfs_quota_us")
		periodFile := filepath.Join(cpuPath, "cpu.cfs_period_us")
		
		if err := os.WriteFile(periodFile, []byte(fmt.Sprintf("%d", period)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set CPU period: %v", err)
		}
		
		if err := os.WriteFile(quotaFile, []byte(fmt.Sprintf("%d", quota)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set CPU quota: %v", err)
		} else {
			log.Printf("[wrapper] Applied CPU quota: %d%%", constraints.CPUQuotaPercent)
		}
	}
	
	// Apply CPU shares (weight)
	if constraints.CPUWeight > 0 && constraints.CPUWeight != 100 {
		// Convert weight (1-10000) to shares (2-262144)
		// Default weight 100 = 1024 shares
		shares := (constraints.CPUWeight * 1024) / 100
		sharesFile := filepath.Join(cpuPath, "cpu.shares")
		
		if err := os.WriteFile(sharesFile, []byte(fmt.Sprintf("%d", shares)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set CPU shares: %v", err)
		} else {
			log.Printf("[wrapper] Applied CPU shares: %d", shares)
		}
	}
	
	// Apply memory limit
	if constraints.MemoryLimitMB > 0 {
		memoryBytes := constraints.MemoryLimitMB * 1024 * 1024
		limitFile := filepath.Join(memoryPath, "memory.limit_in_bytes")
		
		if err := os.WriteFile(limitFile, []byte(fmt.Sprintf("%d", memoryBytes)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to set memory limit: %v", err)
		} else {
			log.Printf("[wrapper] Applied memory limit: %d MB", constraints.MemoryLimitMB)
		}
	}
	
	return cpuPath, nil
}

// AttachProcess attaches a process to the cgroup
func (cm *CgroupManager) AttachProcess(cgroupPath string, pid int) error {
	if cgroupPath == "" || !cm.available {
		// Cgroups not available - skip
		return nil
	}
	
	log.Printf("[wrapper] Attaching PID %d to cgroup", pid)
	
	if cm.cgroupVersion == 2 {
		procsFile := filepath.Join(cgroupPath, "cgroup.procs")
		if err := os.WriteFile(procsFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			return fmt.Errorf("failed to attach process to cgroup: %w", err)
		}
	} else {
		// For v1, attach to both cpu and memory cgroups
		cpuProcs := filepath.Join(cgroupPath, "cgroup.procs")
		memoryPath := strings.Replace(cgroupPath, "/cpu/", "/memory/", 1)
		memoryProcs := filepath.Join(memoryPath, "cgroup.procs")
		
		if err := os.WriteFile(cpuProcs, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			return fmt.Errorf("failed to attach process to CPU cgroup: %w", err)
		}
		
		if err := os.WriteFile(memoryProcs, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			log.Printf("[wrapper] WARNING: Failed to attach process to memory cgroup: %v", err)
		}
	}
	
	log.Printf("[wrapper] PID %d attached to cgroup successfully", pid)
	return nil
}

// RemoveCgroup removes the cgroup (cleanup)
func (cm *CgroupManager) RemoveCgroup(cgroupPath string) error {
	if cgroupPath == "" || !cm.available {
		return nil
	}
	
	log.Printf("[wrapper] Removing cgroup: %s", cgroupPath)
	
	// For v1, also remove memory cgroup
	if cm.cgroupVersion == 1 {
		memoryPath := strings.Replace(cgroupPath, "/cpu/", "/memory/", 1)
		if err := os.Remove(memoryPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[wrapper] WARNING: Failed to remove memory cgroup: %v", err)
		}
	}
	
	if err := os.Remove(cgroupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cgroup: %w", err)
	}
	
	return nil
}

// ApplyNicePriority applies nice priority to a process
// This is a fallback when cgroups are not available or for fine-grained control
func ApplyNicePriority(pid int, niceness int) error {
	// Validate niceness range
	if niceness < -20 {
		niceness = -20
	}
	if niceness > 19 {
		niceness = 19
	}
	
	if niceness == 0 {
		// No change needed
		return nil
	}
	
	log.Printf("[wrapper] Applying nice priority %d to PID %d", niceness, pid)
	
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, niceness); err != nil {
		// Negative nice values require privilege
		if niceness < 0 && os.Geteuid() != 0 {
			log.Printf("[wrapper] WARNING: Cannot set negative nice (requires root), using 0")
			return nil
		}
		return fmt.Errorf("failed to set process priority: %w", err)
	}
	
	return nil
}

// ApplyOOMScoreAdj applies OOM score adjustment to a process
func ApplyOOMScoreAdj(pid int, score int) error {
	if score == 0 {
		return nil
	}
	
	// Validate range
	if score < -1000 {
		score = -1000
	}
	if score > 1000 {
		score = 1000
	}
	
	oomScoreFile := fmt.Sprintf("/proc/%d/oom_score_adj", pid)
	
	if err := os.WriteFile(oomScoreFile, []byte(fmt.Sprintf("%d", score)), 0644); err != nil {
		// Negative values require privilege
		if score < 0 && os.Geteuid() != 0 {
			log.Printf("[wrapper] WARNING: Cannot set negative OOM score (requires root)")
			return nil
		}
		return fmt.Errorf("failed to set OOM score: %w", err)
	}
	
	log.Printf("[wrapper] Applied OOM score adjustment: %d to PID %d", score, pid)
	return nil
}
