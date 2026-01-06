package agent

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

import (
	"context"
	"fmt"
	"log"
	
	"github.com/psantana5/ffmpeg-rtmp/internal/cgroups"
	"github.com/psantana5/ffmpeg-rtmp/internal/report"
	"github.com/psantana5/ffmpeg-rtmp/internal/wrapper"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// ExecuteWithWrapper executes a job using the edge workload wrapper
// This provides process governance without owning the workload
func ExecuteWithWrapper(ctx context.Context, job *models.Job, command string, args []string) (*report.Result, error) {
	log.Printf("🔧 Using workload wrapper for job %s", job.ID)
	
	// Build constraints from job
	limits := buildWrapperLimits(job)
	
	// Log constraints
	if limits != nil {
		log.Printf("Wrapper constraints:")
		if limits.CPUMax != "" {
			log.Printf("  CPU max: %s", limits.CPUMax)
		}
		if limits.CPUWeight > 0 {
			log.Printf("  CPU weight: %d", limits.CPUWeight)
		}
		if limits.MemoryMax > 0 {
			log.Printf("  Memory max: %d MB", limits.MemoryMax/(1024*1024))
		}
		if limits.IOMax != "" {
			log.Printf("  IO max: %s", limits.IOMax)
		}
	}
	
	// Execute with wrapper
	result, err := wrapper.Run(ctx, job.ID, limits, command, args)
	if err != nil {
		return nil, fmt.Errorf("wrapper execution failed: %w", err)
	}
	
	log.Printf("✓ Wrapper execution complete:")
	log.Printf("  PID: %d", result.PID)
	log.Printf("  Exit Code: %d", result.ExitCode)
	log.Printf("  Duration: %.2fs", result.Duration.Seconds())
	log.Printf("  Platform SLA: %v (%s)", result.PlatformSLA, result.PlatformSLAReason)
	
	return result, nil
}

// buildWrapperLimits converts job parameters to wrapper constraints
func buildWrapperLimits(job *models.Job) *cgroups.Limits {
	// If job has explicit wrapper constraints, use those
	if job.WrapperConstraints != nil {
		limits := &cgroups.Limits{
			CPUMax:    job.WrapperConstraints.CPUMax,
			CPUWeight: job.WrapperConstraints.CPUWeight,
			IOMax:     job.WrapperConstraints.IOMax,
		}
		
		// Convert MB to bytes for memory
		if job.WrapperConstraints.MemoryMaxMB > 0 {
			limits.MemoryMax = job.WrapperConstraints.MemoryMaxMB * 1024 * 1024
		}
		
		return limits
	}
	
	// Otherwise, try to infer from legacy resource_limits
	if job.Parameters == nil {
		return nil
	}
	
	resourceLimits, ok := job.Parameters["resource_limits"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	limits := &cgroups.Limits{}
	
	// CPU percentage to quota conversion
	if maxCPU, ok := resourceLimits["max_cpu_percent"].(float64); ok && maxCPU > 0 {
		// Convert percentage to quota format: "quota period"
		// period = 100000 (100ms), quota = (percent * period) / 100
		period := 100000
		quota := int((maxCPU * float64(period)) / 100)
		limits.CPUMax = fmt.Sprintf("%d %d", quota, period)
		limits.CPUWeight = 100 // default
	}
	
	// Memory limit
	if maxMem, ok := resourceLimits["max_memory_mb"].(float64); ok && maxMem > 0 {
		limits.MemoryMax = int64(maxMem) * 1024 * 1024
	}
	
	return limits
}
