package agent

import (
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// FFmpegOptimization contains recommended FFmpeg parameters for optimal performance
type FFmpegOptimization struct {
	Encoder    string            // e.g., "libx264", "h264_nvenc", "libx265"
	Preset     string            // e.g., "ultrafast", "veryfast", "fast", "medium"
	HWAccel    string            // Hardware acceleration method, e.g., "nvenc", "qsv", "vaapi"
	ExtraFlags map[string]string // Additional FFmpeg flags for optimization
	Reason     string            // Explanation of why these parameters were chosen
}

// OptimizeFFmpegParameters determines optimal FFmpeg parameters based on hardware capabilities
func OptimizeFFmpegParameters(caps *models.NodeCapabilities, nodeType models.NodeType) *FFmpegOptimization {
	opt := &FFmpegOptimization{
		ExtraFlags: make(map[string]string),
	}

	// 1. Encoder selection: Prioritize GPU hardware acceleration
	if caps.HasGPU {
		// NVIDIA GPU detected - use NVENC hardware encoder
		opt.Encoder = "h264_nvenc"
		opt.HWAccel = "nvenc"
		opt.Preset = "medium" // NVENC presets: fast, medium, slow
		opt.Reason = "NVIDIA GPU detected - using hardware-accelerated NVENC encoder for better performance"
		
		// NVENC-specific optimizations
		opt.ExtraFlags["rc"] = "vbr" // Variable bitrate for better quality
		opt.ExtraFlags["zerolatency"] = "1" // Low latency for streaming
	} else {
		// Software encoding with CPU
		opt.Encoder = "libx264"
		opt.HWAccel = "none"
		
		// Preset selection based on CPU threads
		switch {
		case caps.CPUThreads >= 16:
			// High-end CPU: Use faster preset for better quality
			opt.Preset = "fast"
			opt.Reason = "High-end CPU (16+ threads) - using 'fast' preset for balanced quality/performance"
		case caps.CPUThreads >= 8:
			// Mid-range CPU: Balance between speed and quality
			opt.Preset = "fast"
			opt.Reason = "Mid-range CPU (8-15 threads) - using 'fast' preset"
		case caps.CPUThreads >= 4:
			// Lower-end CPU: Prioritize encoding speed
			opt.Preset = "veryfast"
			opt.Reason = "Lower-end CPU (4-7 threads) - using 'veryfast' preset for lighter load"
		default:
			// Very limited CPU: Maximum speed
			opt.Preset = "ultrafast"
			opt.Reason = "Limited CPU (< 4 threads) - using 'ultrafast' preset to avoid overload"
		}
		
		// Software encoding optimizations
		opt.ExtraFlags["tune"] = "zerolatency" // Optimize for streaming
		opt.ExtraFlags["threads"] = "0"        // Auto thread selection
	}

	// 2. Additional optimizations based on node type
	switch nodeType {
	case models.NodeTypeLaptop:
		// Laptop: Prioritize efficiency over quality to reduce thermal/battery impact
		if opt.HWAccel == "none" {
			// For software encoding on laptops, use faster preset
			if opt.Preset == "fast" {
				opt.Preset = "veryfast"
			}
		}
		opt.Reason += " (Laptop: optimized for thermal/battery efficiency)"
		
	case models.NodeTypeServer:
		// Server: Can use higher quality settings
		if opt.HWAccel == "none" && caps.CPUThreads >= 16 {
			// Servers can afford slightly slower presets for better quality
			if opt.Preset == "veryfast" {
				opt.Preset = "fast"
			}
		}
		opt.Reason += " (Server: optimized for sustained workloads)"
		
	case models.NodeTypeDesktop:
		// Desktop: Default balanced settings already applied
		opt.Reason += " (Desktop: balanced performance)"
	}

	return opt
}

// ApplyOptimizationToParameters merges optimization recommendations into job parameters
// Job-specific parameters take precedence over optimizations
func ApplyOptimizationToParameters(params map[string]interface{}, opt *FFmpegOptimization) map[string]interface{} {
	if params == nil {
		params = make(map[string]interface{})
	}

	// Apply encoder if not already specified
	if _, exists := params["codec"]; !exists {
		params["codec"] = opt.Encoder
	}

	// Apply preset if not already specified
	if _, exists := params["preset"]; !exists {
		params["preset"] = opt.Preset
	}

	// Apply hardware acceleration flag if not already specified
	if opt.HWAccel != "none" {
		if _, exists := params["hwaccel"]; !exists {
			params["hwaccel"] = opt.HWAccel
		}
	}

	// Apply extra flags if not already specified
	for key, value := range opt.ExtraFlags {
		if _, exists := params[key]; !exists {
			params[key] = value
		}
	}

	return params
}
