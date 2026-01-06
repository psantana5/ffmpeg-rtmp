package cgroups

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const cgroupRoot = "/sys/fs/cgroup"

// Manager handles cgroup lifecycle only.
// Create. Join. Delete. Nothing else.
type Manager struct {
	version int
}

// New creates a cgroup manager
func New() *Manager {
	return &Manager{
		version: Version(),
	}
}

// Create creates a cgroup directory
// Returns: cgroup path (empty if failed)
func (m *Manager) Create(jobID string) (string, error) {
	if jobID == "" {
		jobID = fmt.Sprintf("unnamed-%d", os.Getpid())
	}
	
	cgroupName := fmt.Sprintf("ffrtmp/%s", jobID)
	
	if m.version == 2 {
		return m.createV2(cgroupName)
	}
	return m.createV1(cgroupName)
}

func (m *Manager) createV2(name string) (string, error) {
	path := filepath.Join(cgroupRoot, name)
	
	if err := os.MkdirAll(path, 0755); err != nil {
		if os.IsPermission(err) {
			return "", nil // not an error, just can't create
		}
		return "", err
	}
	
	return path, nil
}

func (m *Manager) createV1(name string) (string, error) {
	// v1: create under /sys/fs/cgroup/cpu/
	cpuPath := filepath.Join(cgroupRoot, "cpu", name)
	
	if err := os.MkdirAll(cpuPath, 0755); err != nil {
		if os.IsPermission(err) {
			return "", nil
		}
		return "", err
	}
	
	// Also create under memory
	memPath := filepath.Join(cgroupRoot, "memory", name)
	os.MkdirAll(memPath, 0755) // best effort
	
	return cpuPath, nil
}

// Join moves a PID into the cgroup
func (m *Manager) Join(cgroupPath string, pid int) error {
	if cgroupPath == "" {
		return nil // no cgroup, skip
	}
	
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	
	if m.version == 2 {
		return m.joinV2(cgroupPath, pid)
	}
	return m.joinV1(cgroupPath, pid)
}

func (m *Manager) joinV2(path string, pid int) error {
	procsFile := filepath.Join(path, "cgroup.procs")
	return os.WriteFile(procsFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func (m *Manager) joinV1(path string, pid int) error {
	// v1: write to cpu cgroup
	cpuProcs := filepath.Join(path, "cgroup.procs")
	if err := os.WriteFile(cpuProcs, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return err
	}
	
	// Also write to memory cgroup (best effort)
	memPath := strings.Replace(path, "/cpu/", "/memory/", 1)
	memProcs := filepath.Join(memPath, "cgroup.procs")
	os.WriteFile(memProcs, []byte(fmt.Sprintf("%d", pid)), 0644)
	
	return nil
}

// Delete removes the cgroup directory
func (m *Manager) Delete(cgroupPath string) error {
	if cgroupPath == "" {
		return nil
	}
	
	if m.version == 1 {
		// v1: also delete memory cgroup
		memPath := strings.Replace(cgroupPath, "/cpu/", "/memory/", 1)
		os.Remove(memPath) // best effort
	}
	
	return os.Remove(cgroupPath)
}
