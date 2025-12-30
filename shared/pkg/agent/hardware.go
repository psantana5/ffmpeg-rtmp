package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

const (
	// ServerDetectionMinThreads is the minimum CPU threads for server classification
	ServerDetectionMinThreads = 16
	// ServerDetectionMinRAMGB is the minimum RAM in GB for server classification
	ServerDetectionMinRAMGB = 32
)

// DetectHardware detects the hardware capabilities of the current system
func DetectHardware() (*models.NodeCapabilities, error) {
	caps := &models.NodeCapabilities{
		Labels: make(map[string]string),
	}

	// Detect CPU
	cpuThreads, cpuModel := detectCPU()
	caps.CPUThreads = cpuThreads
	caps.CPUModel = cpuModel

	// Detect GPU
	hasGPU, gpuType := detectGPU()
	caps.HasGPU = hasGPU
	caps.GPUType = gpuType

	// Detect RAM
	ramBytes := detectRAM()
	caps.RAMBytes = ramBytes

	// Add OS label
	caps.Labels["os"] = runtime.GOOS
	caps.Labels["arch"] = runtime.GOARCH

	return caps, nil
}

// detectCPU detects CPU information
func detectCPU() (int, string) {
	threads := runtime.NumCPU()
	model := "Unknown"

	switch runtime.GOOS {
	case "linux":
		model = detectLinuxCPU()
	case "darwin":
		model = detectDarwinCPU()
	case "windows":
		model = detectWindowsCPU()
	}

	return threads, model
}

func detectLinuxCPU() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "Unknown"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "model name") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

func detectDarwinCPU() string {
	out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err != nil {
		return "Unknown"
	}
	return strings.TrimSpace(string(out))
}

func detectWindowsCPU() string {
	out, err := exec.Command("wmic", "cpu", "get", "name").Output()
	if err != nil {
		return "Unknown"
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) > 1 {
		return strings.TrimSpace(lines[1])
	}
	return "Unknown"
}

// detectGPU detects NVIDIA GPU
func detectGPU() (bool, string) {
	// Try nvidia-smi
	out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output()
	if err == nil && len(out) > 0 {
		return true, strings.TrimSpace(string(out))
	}

	return false, ""
}

// detectRAM detects system RAM
func detectRAM() uint64 {
	switch runtime.GOOS {
	case "linux":
		return detectLinuxRAM()
	case "darwin":
		return detectDarwinRAM()
	case "windows":
		return detectWindowsRAM()
	}
	return 8 * 1024 * 1024 * 1024 // Default 8GB
}

func detectLinuxRAM() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 8 * 1024 * 1024 * 1024
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "MemTotal") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb * 1024 // Convert KB to bytes
				}
			}
		}
	}
	return 8 * 1024 * 1024 * 1024
}

func detectDarwinRAM() uint64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 8 * 1024 * 1024 * 1024
	}
	bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 8 * 1024 * 1024 * 1024
	}
	return bytes
}

func detectWindowsRAM() uint64 {
	out, err := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory").Output()
	if err != nil {
		return 8 * 1024 * 1024 * 1024
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) > 1 {
		bytes, err := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
		if err == nil {
			return bytes
		}
	}
	return 8 * 1024 * 1024 * 1024
}

// DetectNodeType determines the node type based on hardware characteristics
func DetectNodeType(cpuThreads int, ramBytes uint64) models.NodeType {
	// Check for battery to detect laptop
	if hasLaptopBattery() {
		return models.NodeTypeLaptop
	}

	ramGB := float64(ramBytes) / (1024 * 1024 * 1024)

	// Server: >ServerDetectionMinThreads threads AND >ServerDetectionMinRAMGB GB RAM
	if cpuThreads > ServerDetectionMinThreads && ramGB > ServerDetectionMinRAMGB {
		return models.NodeTypeServer
	}

	// Default to desktop
	return models.NodeTypeDesktop
}

func hasLaptopBattery() bool {
	switch runtime.GOOS {
	case "linux":
		// Check for battery in /sys/class/power_supply
		powerSupplyPath := "/sys/class/power_supply"
		entries, err := os.ReadDir(powerSupplyPath)
		if err != nil {
			return false
		}
		for _, entry := range entries {
			if strings.Contains(strings.ToUpper(entry.Name()), "BAT") {
				return true
			}
		}
	case "darwin":
		// Check using system_profiler
		out, err := exec.Command("system_profiler", "SPPowerDataType").Output()
		if err == nil && strings.Contains(string(out), "Battery") {
			return true
		}
	case "windows":
		// Check using WMIC
		out, err := exec.Command("wmic", "path", "win32_battery", "get", "status").Output()
		if err == nil && strings.Contains(string(out), "Status") {
			return true
		}
	}
	return false
}

// FormatRAM formats RAM bytes to human-readable string
func FormatRAM(bytes uint64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1f GB", gb)
}

// GetRecommendedWorkDir returns a recommended working directory for the agent
func GetRecommendedWorkDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/ffmpeg-agent"
	}
	return filepath.Join(home, ".ffmpeg-agent")
}
