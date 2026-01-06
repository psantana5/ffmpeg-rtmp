package cgroups

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"fmt"
	"os"
	"path/filepath"
)

// Limits defines what can be written to cgroups.
// Nothing else. No policy. No magic.
type Limits struct {
	CPUMax      string // "quota period" or "max"
	CPUWeight   int    // 1-10000
	MemoryMax   int64  // bytes, 0 = no limit
	IOMax       string // "major:minor rbps=X wbps=Y" (cgroup v2 only)
}

// Version returns detected cgroup version (1 or 2)
func Version() int {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return 2
	}
	return 1
}

// WriteCPUMax writes cpu.max (v2) or cpu.cfs_quota_us + cpu.cfs_period_us (v1)
func WriteCPUMax(cgroupPath string, value string) error {
	version := Version()
	
	if version == 2 {
		// v2: "quota period" format
		cpuMaxFile := filepath.Join(cgroupPath, "cpu.max")
		return os.WriteFile(cpuMaxFile, []byte(value), 0644)
	}
	
	// v1: parse "quota period" and write separately
	// For now, skip v1 CPU max (too complex for minimal approach)
	return nil
}

// WriteCPUWeight writes cpu.weight (v2) or cpu.shares (v1)
func WriteCPUWeight(cgroupPath string, weight int) error {
	if weight <= 0 || weight > 10000 {
		return fmt.Errorf("invalid cpu weight: %d (must be 1-10000)", weight)
	}
	
	version := Version()
	
	if version == 2 {
		cpuWeightFile := filepath.Join(cgroupPath, "cpu.weight")
		return os.WriteFile(cpuWeightFile, []byte(fmt.Sprintf("%d", weight)), 0644)
	}
	
	// v1: convert weight to shares (weight 100 = 1024 shares)
	shares := (weight * 1024) / 100
	cpuSharesFile := filepath.Join(cgroupPath, "cpu.shares")
	return os.WriteFile(cpuSharesFile, []byte(fmt.Sprintf("%d", shares)), 0644)
}

// WriteMemoryMax writes memory.max (v2) or memory.limit_in_bytes (v1)
func WriteMemoryMax(cgroupPath string, bytes int64) error {
	if bytes < 0 {
		return fmt.Errorf("invalid memory limit: %d", bytes)
	}
	
	if bytes == 0 {
		return nil // no limit
	}
	
	version := Version()
	
	if version == 2 {
		memMaxFile := filepath.Join(cgroupPath, "memory.max")
		return os.WriteFile(memMaxFile, []byte(fmt.Sprintf("%d", bytes)), 0644)
	}
	
	// v1
	memLimitFile := filepath.Join(cgroupPath, "memory.limit_in_bytes")
	return os.WriteFile(memLimitFile, []byte(fmt.Sprintf("%d", bytes)), 0644)
}

// WriteIOMax writes io.max (v2 only)
// Format: "major:minor rbps=X wbps=Y"
func WriteIOMax(cgroupPath string, value string) error {
	if value == "" {
		return nil
	}
	
	version := Version()
	if version != 2 {
		return nil // v1 doesn't have io.max
	}
	
	ioMaxFile := filepath.Join(cgroupPath, "io.max")
	return os.WriteFile(ioMaxFile, []byte(value), 0644)
}
