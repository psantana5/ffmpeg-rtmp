package discover

import (
	"fmt"
	"os"
	"time"
	
	"gopkg.in/yaml.v3"
	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
)

// WatchConfig represents the complete configuration for watch daemon
type WatchConfig struct {
	// Scanning configuration
	ScanInterval string   `yaml:"scan_interval"` // e.g., "10s", "1m"
	TargetCommands []string `yaml:"target_commands"`
	
	// Default resource limits
	DefaultLimits ResourceLimits `yaml:"default_limits"`
	
	// Filtering rules
	Filters FilterRules `yaml:"filters"`
	
	// Command-specific overrides
	Commands map[string]CommandConfig `yaml:"commands"`
}

// ResourceLimits defines cgroup resource limits
type ResourceLimits struct {
	CPUQuota    int `yaml:"cpu_quota"`     // CPU quota percentage (e.g., 200 = 200%)
	CPUWeight   int `yaml:"cpu_weight"`    // CPU weight 1-10000 (default 100)
	MemoryLimit int `yaml:"memory_limit"`  // Memory limit in MB
}

// FilterRules defines process filtering rules
type FilterRules struct {
	AllowedUsers []string `yaml:"allowed_users"`
	BlockedUsers []string `yaml:"blocked_users"`
	AllowedUIDs  []int    `yaml:"allowed_uids"`
	BlockedUIDs  []int    `yaml:"blocked_uids"`
	
	AllowedParents []int `yaml:"allowed_parents"`
	BlockedParents []int `yaml:"blocked_parents"`
	
	MinRuntime string `yaml:"min_runtime"` // e.g., "5s", "1m"
	MaxRuntime string `yaml:"max_runtime"` // e.g., "24h"
	
	AllowedDirs []string `yaml:"allowed_dirs"`
	BlockedDirs []string `yaml:"blocked_dirs"`
}

// CommandConfig defines per-command configuration
type CommandConfig struct {
	Limits  *ResourceLimits `yaml:"limits,omitempty"`
	Filters *CommandFilterRules `yaml:"filters,omitempty"`
}

// CommandFilterRules defines per-command filtering (subset of full filters)
type CommandFilterRules struct {
	AllowedUsers []string `yaml:"allowed_users"`
	BlockedUsers []string `yaml:"blocked_users"`
	MinRuntime   string   `yaml:"min_runtime"`
	MaxRuntime   string   `yaml:"max_runtime"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*WatchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config WatchConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Set defaults
	if config.ScanInterval == "" {
		config.ScanInterval = "10s"
	}
	
	if len(config.TargetCommands) == 0 {
		config.TargetCommands = []string{"ffmpeg", "gst-launch-1.0"}
	}
	
	if config.DefaultLimits.CPUWeight == 0 {
		config.DefaultLimits.CPUWeight = 100
	}
	
	return &config, nil
}

// ToAttachConfig converts WatchConfig to AttachConfig
func (c *WatchConfig) ToAttachConfig() (*AttachConfig, error) {
	// Parse scan interval
	scanInterval, err := time.ParseDuration(c.ScanInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid scan_interval: %w", err)
	}
	
	// Convert resource limits
	limits := &cgroups.Limits{
		CPUWeight: c.DefaultLimits.CPUWeight,
	}
	
	if c.DefaultLimits.CPUQuota > 0 {
		quota := c.DefaultLimits.CPUQuota * 1000
		limits.CPUMax = fmt.Sprintf("%d 100000", quota)
	}
	
	if c.DefaultLimits.MemoryLimit > 0 {
		limits.MemoryMax = int64(c.DefaultLimits.MemoryLimit) * 1024 * 1024
	}
	
	return &AttachConfig{
		ScanInterval:  scanInterval,
		TargetCommands: c.TargetCommands,
		DefaultLimits: limits,
	}, nil
}

// ToFilterConfig converts FilterRules to FilterConfig
func (r *FilterRules) ToFilterConfig() (*FilterConfig, error) {
	filter := NewFilterConfig()
	
	filter.AllowedUsers = r.AllowedUsers
	filter.BlockedUsers = r.BlockedUsers
	filter.AllowedUIDs = r.AllowedUIDs
	filter.BlockedUIDs = r.BlockedUIDs
	filter.AllowedParents = r.AllowedParents
	filter.BlockedParents = r.BlockedParents
	filter.AllowedDirs = r.AllowedDirs
	filter.BlockedDirs = r.BlockedDirs
	
	// Parse runtime durations
	if r.MinRuntime != "" {
		minRuntime, err := time.ParseDuration(r.MinRuntime)
		if err != nil {
			return nil, fmt.Errorf("invalid min_runtime: %w", err)
		}
		filter.MinRuntime = minRuntime
	}
	
	if r.MaxRuntime != "" {
		maxRuntime, err := time.ParseDuration(r.MaxRuntime)
		if err != nil {
			return nil, fmt.Errorf("invalid max_runtime: %w", err)
		}
		filter.MaxRuntime = maxRuntime
	}
	
	return filter, nil
}

// ApplyCommandFilters adds command-specific filters to FilterConfig
func (c *WatchConfig) ApplyCommandFilters(filter *FilterConfig) error {
	for cmdName, cmdConfig := range c.Commands {
		if cmdConfig.Filters == nil {
			continue
		}
		
		cmdFilter := &CommandFilter{
			AllowedUsers: cmdConfig.Filters.AllowedUsers,
			BlockedUsers: cmdConfig.Filters.BlockedUsers,
		}
		
		// Parse command-specific runtime durations
		if cmdConfig.Filters.MinRuntime != "" {
			minRuntime, err := time.ParseDuration(cmdConfig.Filters.MinRuntime)
			if err != nil {
				return fmt.Errorf("invalid min_runtime for command %s: %w", cmdName, err)
			}
			cmdFilter.MinRuntime = minRuntime
		}
		
		if cmdConfig.Filters.MaxRuntime != "" {
			maxRuntime, err := time.ParseDuration(cmdConfig.Filters.MaxRuntime)
			if err != nil {
				return fmt.Errorf("invalid max_runtime for command %s: %w", cmdName, err)
			}
			cmdFilter.MaxRuntime = maxRuntime
		}
		
		filter.CommandFilters[cmdName] = cmdFilter
	}
	
	return nil
}

// Example configuration as a string
const ExampleConfig = `# FFmpeg Auto-Discovery Watch Daemon Configuration

# How often to scan for new processes
scan_interval: "10s"

# Commands to discover (process names)
target_commands:
  - ffmpeg
  - gst-launch-1.0

# Default resource limits applied to discovered processes
default_limits:
  cpu_quota: 200      # 200% CPU (2 cores)
  cpu_weight: 100     # CPU scheduling weight (1-10000)
  memory_limit: 4096  # 4GB memory limit

# Global filtering rules
filters:
  # User-based filtering
  allowed_users: []     # Empty = allow all users
  blocked_users: []     # Block specific users
  allowed_uids: []      # Allow specific UIDs
  blocked_uids: []      # Block specific UIDs
  
  # Parent process filtering
  allowed_parents: []   # Only discover children of these PIDs
  blocked_parents: []   # Never discover children of these PIDs
  
  # Runtime-based filtering
  min_runtime: "5s"     # Ignore processes younger than 5s
  max_runtime: ""       # No maximum (empty = unlimited)
  
  # Working directory filtering
  allowed_dirs: []      # Only discover processes in these dirs
  blocked_dirs:         # Never discover processes in these dirs
    - /tmp
    - /home/test

# Per-command overrides
commands:
  ffmpeg:
    limits:
      cpu_quota: 300      # FFmpeg gets 3 cores
      memory_limit: 8192  # FFmpeg gets 8GB
    filters:
      allowed_users:
        - ffmpeg          # Only discover FFmpeg processes owned by 'ffmpeg' user
        - video
      min_runtime: "10s"  # Only long-running FFmpeg jobs
  
  gst-launch-1.0:
    limits:
      cpu_quota: 150
      memory_limit: 2048
    filters:
      min_runtime: "30s"  # Only discover GStreamer jobs running >30s
`
