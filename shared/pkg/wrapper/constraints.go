package wrapper

// Constraints defines OS-level resource constraints
type Constraints struct {
	// CPU constraints
	CPUQuotaPercent int  // CPU quota as percentage (100 = 1 core, 200 = 2 cores, 0 = unlimited)
	CPUWeight       int  // CPU weight for proportional sharing (1-10000, default 100)
	NicePriority    int  // Nice priority (-20 to 19, default 0)
	
	// Memory constraints
	MemoryLimitMB   int64 // Memory limit in MB (0 = unlimited)
	MemorySwapMB    int64 // Memory + swap limit in MB (0 = no swap)
	
	// IO constraints (cgroup v2 only)
	IOWeightPercent int  // IO weight as percentage (1-100, 0 = no constraint)
	
	// Process constraints
	OOMScoreAdj     int  // OOM killer score adjustment (-1000 to 1000, 0 = default)
}

// DefaultConstraints returns unconstrained defaults
func DefaultConstraints() *Constraints {
	return &Constraints{
		CPUQuotaPercent: 0,    // Unlimited
		CPUWeight:       100,  // Default weight
		NicePriority:    0,    // Normal priority
		MemoryLimitMB:   0,    // Unlimited
		MemorySwapMB:    0,    // No swap limit
		IOWeightPercent: 0,    // No IO constraint
		OOMScoreAdj:     0,    // Default OOM score
	}
}

// LowPriorityConstraints returns constraints for low-priority workloads
func LowPriorityConstraints() *Constraints {
	return &Constraints{
		CPUQuotaPercent: 0,    // Unlimited but deprioritized
		CPUWeight:       50,   // Half the default weight
		NicePriority:    10,   // Lower priority
		MemoryLimitMB:   0,    // Unlimited
		MemorySwapMB:    0,    // No swap
		IOWeightPercent: 50,   // Half IO priority
		OOMScoreAdj:     100,  // More likely to be killed under OOM
	}
}

// HighPriorityConstraints returns constraints for high-priority workloads
func HighPriorityConstraints() *Constraints {
	return &Constraints{
		CPUQuotaPercent: 0,    // Unlimited
		CPUWeight:       200,  // Double the default weight
		NicePriority:    -5,   // Higher priority (requires privilege)
		MemoryLimitMB:   0,    // Unlimited
		MemorySwapMB:    0,    // No swap
		IOWeightPercent: 100,  // Full IO priority
		OOMScoreAdj:     -100, // Less likely to be killed
	}
}

// Validate ensures constraints are within acceptable ranges
func (c *Constraints) Validate() error {
	if c.CPUQuotaPercent < 0 {
		c.CPUQuotaPercent = 0
	}
	
	if c.CPUWeight < 1 {
		c.CPUWeight = 1
	} else if c.CPUWeight > 10000 {
		c.CPUWeight = 10000
	}
	
	if c.NicePriority < -20 {
		c.NicePriority = -20
	} else if c.NicePriority > 19 {
		c.NicePriority = 19
	}
	
	if c.MemoryLimitMB < 0 {
		c.MemoryLimitMB = 0
	}
	
	if c.IOWeightPercent < 0 {
		c.IOWeightPercent = 0
	} else if c.IOWeightPercent > 100 {
		c.IOWeightPercent = 100
	}
	
	if c.OOMScoreAdj < -1000 {
		c.OOMScoreAdj = -1000
	} else if c.OOMScoreAdj > 1000 {
		c.OOMScoreAdj = 1000
	}
	
	return nil
}
