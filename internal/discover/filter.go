package discover

import (
	"time"
)

// FilterConfig defines rules for filtering discovered processes
type FilterConfig struct {
	// User-based filtering
	AllowedUsers   []string      // Whitelist of usernames (empty = allow all)
	BlockedUsers   []string      // Blacklist of usernames
	AllowedUIDs    []int         // Whitelist of UIDs (empty = allow all)
	BlockedUIDs    []int         // Blacklist of UIDs
	
	// Parent-based filtering
	AllowedParents []int         // Whitelist of parent PIDs (empty = allow all)
	BlockedParents []int         // Blacklist of parent PIDs
	
	// Runtime-based filtering
	MinRuntime     time.Duration // Minimum process age (0 = no minimum)
	MaxRuntime     time.Duration // Maximum process age (0 = no maximum)
	
	// Working directory filtering
	AllowedDirs    []string      // Whitelist of working directories (empty = allow all)
	BlockedDirs    []string      // Blacklist of working directories
	
	// Command-specific filtering
	CommandFilters map[string]*CommandFilter // Per-command filter overrides
}

// CommandFilter defines per-command filtering rules
type CommandFilter struct {
	AllowedUsers []string
	BlockedUsers []string
	MinRuntime   time.Duration
	MaxRuntime   time.Duration
}

// NewFilterConfig creates a default filter config (allow everything)
func NewFilterConfig() *FilterConfig {
	return &FilterConfig{
		AllowedUsers:   []string{},
		BlockedUsers:   []string{},
		AllowedUIDs:    []int{},
		BlockedUIDs:    []int{},
		AllowedParents: []int{},
		BlockedParents: []int{},
		MinRuntime:     0,
		MaxRuntime:     0,
		AllowedDirs:    []string{},
		BlockedDirs:    []string{},
		CommandFilters: make(map[string]*CommandFilter),
	}
}

// ShouldDiscover determines if a process should be discovered based on filters
func (f *FilterConfig) ShouldDiscover(proc *Process) bool {
	// Check command-specific filters first
	if cmdFilter, ok := f.CommandFilters[proc.Command]; ok {
		if !f.checkUserFilter(proc, cmdFilter.AllowedUsers, cmdFilter.BlockedUsers) {
			return false
		}
		if !f.checkRuntimeFilter(proc, cmdFilter.MinRuntime, cmdFilter.MaxRuntime) {
			return false
		}
	}
	
	// Check global user filters
	if !f.checkUserFilter(proc, f.AllowedUsers, f.BlockedUsers) {
		return false
	}
	
	// Check UID filters
	if !f.checkUIDFilter(proc) {
		return false
	}
	
	// Check parent PID filters
	if !f.checkParentFilter(proc) {
		return false
	}
	
	// Check runtime filters
	if !f.checkRuntimeFilter(proc, f.MinRuntime, f.MaxRuntime) {
		return false
	}
	
	// Check working directory filters
	if !f.checkDirFilter(proc) {
		return false
	}
	
	return true
}

// checkUserFilter checks username whitelist/blacklist
func (f *FilterConfig) checkUserFilter(proc *Process, allowed, blocked []string) bool {
	// If whitelist exists and user not in it, reject
	if len(allowed) > 0 {
		found := false
		for _, user := range allowed {
			if user == proc.Username {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// If blacklist exists and user in it, reject
	if len(blocked) > 0 {
		for _, user := range blocked {
			if user == proc.Username {
				return false
			}
		}
	}
	
	return true
}

// checkUIDFilter checks UID whitelist/blacklist
func (f *FilterConfig) checkUIDFilter(proc *Process) bool {
	// If whitelist exists and UID not in it, reject
	if len(f.AllowedUIDs) > 0 {
		found := false
		for _, uid := range f.AllowedUIDs {
			if uid == proc.UserID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// If blacklist exists and UID in it, reject
	if len(f.BlockedUIDs) > 0 {
		for _, uid := range f.BlockedUIDs {
			if uid == proc.UserID {
				return false
			}
		}
	}
	
	return true
}

// checkParentFilter checks parent PID whitelist/blacklist
func (f *FilterConfig) checkParentFilter(proc *Process) bool {
	// If whitelist exists and parent not in it, reject
	if len(f.AllowedParents) > 0 {
		found := false
		for _, ppid := range f.AllowedParents {
			if ppid == proc.ParentPID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// If blacklist exists and parent in it, reject
	if len(f.BlockedParents) > 0 {
		for _, ppid := range f.BlockedParents {
			if ppid == proc.ParentPID {
				return false
			}
		}
	}
	
	return true
}

// checkRuntimeFilter checks minimum and maximum runtime
func (f *FilterConfig) checkRuntimeFilter(proc *Process, minRuntime, maxRuntime time.Duration) bool {
	// Check minimum runtime
	if minRuntime > 0 && proc.ProcessAge < minRuntime {
		return false
	}
	
	// Check maximum runtime
	if maxRuntime > 0 && proc.ProcessAge > maxRuntime {
		return false
	}
	
	return true
}

// checkDirFilter checks working directory whitelist/blacklist
func (f *FilterConfig) checkDirFilter(proc *Process) bool {
	if proc.WorkingDir == "" {
		// If we can't determine working dir, allow by default
		return true
	}
	
	// If whitelist exists and dir not in it, reject
	if len(f.AllowedDirs) > 0 {
		found := false
		for _, dir := range f.AllowedDirs {
			if dir == proc.WorkingDir {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// If blacklist exists and dir in it, reject
	if len(f.BlockedDirs) > 0 {
		for _, dir := range f.BlockedDirs {
			if dir == proc.WorkingDir {
				return false
			}
		}
	}
	
	return true
}
