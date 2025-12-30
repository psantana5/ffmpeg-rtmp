package agent

import (
	"fmt"
)

// WorkerCapabilities represents the hardware capabilities of a worker node
type WorkerCapabilities struct {
	CPUCores  int    // Number of CPU cores
	GPUType   string // GPU type (e.g., "nvidia-rtx-3080", "none")
	HasNVENC  bool   // Whether NVENC hardware encoding is available
	MemoryGB  int    // Total system memory in GB
	HasQSV    bool   // Intel Quick Sync Video availability
	HasVAAPI  bool   // Video Acceleration API (Linux)
}

// ContentProperties describes the characteristics of the video content
type ContentProperties struct {
	Resolution  string  // e.g., "1920x1080", "3840x2160"
	MotionLevel string  // "low", "medium", "high"
	GrainLevel  string  // "none", "low", "medium", "high"
	Framerate   int     // Frames per second
	IsHDR       bool    // Whether content is HDR
	ColorSpace  string  // e.g., "bt709", "bt2020"
	PixelFormat string  // e.g., "yuv420p", "yuv420p10le"
}

// TranscodingGoal represents the optimization objective
type TranscodingGoal string

const (
	GoalQuality  TranscodingGoal = "quality"  // Prioritize visual quality
	GoalEnergy   TranscodingGoal = "energy"   // Prioritize energy efficiency
	GoalLatency  TranscodingGoal = "latency"  // Prioritize speed/low latency
	GoalBalanced TranscodingGoal = "balanced" // Balance all factors
)

// TranscodingParams contains the optimized FFmpeg parameters
type TranscodingParams struct {
	Encoder      string            `json:"encoder"`       // e.g., "libx265", "hevc_nvenc"
	Preset       string            `json:"preset"`        // Encoder-specific preset
	CRF          int               `json:"crf,omitempty"` // Constant Rate Factor (0 = use bitrate instead)
	Bitrate      string            `json:"bitrate,omitempty"`
	ExtraParams  map[string]string `json:"extra_params"`  // Additional FFmpeg flags
	Threads      int               `json:"threads"`       // Number of encoding threads
	Reasoning    []string          `json:"reasoning"`     // Explanation of choices made
	PixelFormat  string            `json:"pixel_format"`  // Output pixel format
}

// CalculateOptimalFFmpegParams determines the best FFmpeg parameters for given inputs
func CalculateOptimalFFmpegParams(
	worker WorkerCapabilities,
	content ContentProperties,
	goal TranscodingGoal,
) *TranscodingParams {
	params := &TranscodingParams{
		ExtraParams: make(map[string]string),
		Reasoning:   make([]string, 0),
	}

	// Parse resolution for decision making
	is4K := content.Resolution == "3840x2160" || content.Resolution == "4096x2160"

	// Step 1: Encoder Selection
	// Rule: Prefer GPU if available and goal is energy or latency
	// Rule: Prefer CPU encoders for highest quality (especially for 4K and high-motion)
	if goal == GoalEnergy || goal == GoalLatency {
		if worker.HasNVENC {
			// NVENC for energy efficiency and low latency
			if is4K {
				params.Encoder = "hevc_nvenc"
				params.Reasoning = append(params.Reasoning,
					"Selected HEVC NVENC for 4K: hardware acceleration provides energy efficiency and low latency")
			} else {
				params.Encoder = "h264_nvenc"
				params.Reasoning = append(params.Reasoning,
					"Selected H.264 NVENC: hardware acceleration optimizes for energy/latency goal")
			}
		} else if worker.HasQSV {
			params.Encoder = "h264_qsv"
			params.Reasoning = append(params.Reasoning,
				"Selected Intel QSV: hardware acceleration available for energy/latency goal")
		} else {
			// Fallback to CPU with fast preset
			params.Encoder = "libx264"
			params.Reasoning = append(params.Reasoning,
				"Selected libx264: no GPU available, using CPU with fast preset for latency goal")
		}
	} else if goal == GoalQuality {
		// Quality goal: prefer CPU encoders for better quality
		if is4K || content.MotionLevel == "high" {
			// Use HEVC (H.265) for 4K and high-motion for better compression
			params.Encoder = "libx265"
			params.Reasoning = append(params.Reasoning,
				"Selected libx265 for quality goal: CPU encoding provides superior quality for 4K/high-motion content")
		} else {
			// For 1080p and below with quality goal, x264 is sufficient
			params.Encoder = "libx264"
			params.Reasoning = append(params.Reasoning,
				"Selected libx264 for quality goal: excellent quality-to-speed ratio for Full HD content")
		}
	} else { // GoalBalanced
		// Balanced: use GPU if available, otherwise CPU with reasonable preset
		if worker.HasNVENC && is4K {
			params.Encoder = "hevc_nvenc"
			params.Reasoning = append(params.Reasoning,
				"Selected HEVC NVENC for balanced goal: good quality and performance for 4K")
		} else if worker.HasNVENC {
			params.Encoder = "h264_nvenc"
			params.Reasoning = append(params.Reasoning,
				"Selected H.264 NVENC for balanced goal: hardware acceleration with good quality")
		} else if is4K {
			params.Encoder = "libx265"
			params.Reasoning = append(params.Reasoning,
				"Selected libx265 for balanced 4K: better compression efficiency")
		} else {
			params.Encoder = "libx264"
			params.Reasoning = append(params.Reasoning,
				"Selected libx264 for balanced goal: widely compatible with good performance")
		}
	}

	// Step 2: Preset Selection
	selectPreset(params, worker, content, goal)

	// Step 3: CRF/Bitrate Selection
	// Rule: Adjust CRF based on motion and complexity
	selectRateControl(params, content, goal)

	// Step 4: Thread Configuration
	// Rule: Avoid too many threads on low-core systems
	selectThreads(params, worker)

	// Step 5: Pixel Format Selection
	// Rule: Use 10-bit if HDR is detected
	selectPixelFormat(params, content)

	// Step 6: Special Flags
	// Rule: Optimize based on content characteristics
	addSpecialFlags(params, worker, content, goal)

	return params
}

// selectPreset chooses the optimal preset based on encoder, hardware, and goal
func selectPreset(params *TranscodingParams, worker WorkerCapabilities, content ContentProperties, goal TranscodingGoal) {
	isNVENC := params.Encoder == "h264_nvenc" || params.Encoder == "hevc_nvenc"
	isHEVC := params.Encoder == "libx265" || params.Encoder == "hevc_nvenc"

	if isNVENC {
		// NVENC presets: p1 (fastest) to p7 (slowest)
		switch goal {
		case GoalLatency:
			params.Preset = "p1"
			params.Reasoning = append(params.Reasoning,
				"Using NVENC preset p1: fastest encoding for latency goal")
		case GoalEnergy:
			params.Preset = "p3"
			params.Reasoning = append(params.Reasoning,
				"Using NVENC preset p3: balanced speed and quality for energy efficiency")
		case GoalQuality:
			params.Preset = "p6"
			params.Reasoning = append(params.Reasoning,
				"Using NVENC preset p6: higher quality encoding with acceptable speed")
		default: // Balanced
			params.Preset = "p4"
			params.Reasoning = append(params.Reasoning,
				"Using NVENC preset p4: balanced quality and performance")
		}
	} else if isHEVC {
		// libx265 presets: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow
		// Rule: Avoid slow presets when CPU has few cores
		switch {
		case worker.CPUCores <= 4:
			// Limited cores: use faster presets
			if goal == GoalQuality {
				params.Preset = "fast"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'fast': limited CPU cores (≤4) require faster preset even for quality goal")
			} else {
				params.Preset = "veryfast"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'veryfast': limited CPU cores (≤4) require fast encoding")
			}
		case worker.CPUCores <= 8:
			// Mid-range cores
			switch goal {
			case GoalQuality:
				params.Preset = "medium"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'medium': balanced quality on mid-range CPU (5-8 cores)")
			case GoalLatency:
				params.Preset = "fast"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'fast': prioritizing speed on mid-range CPU")
			default:
				params.Preset = "fast"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'fast': balanced performance on mid-range CPU")
			}
		default:
			// High-end cores: can afford slower presets
			switch goal {
			case GoalQuality:
				params.Preset = "slow"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'slow': high-end CPU (8+ cores) can afford better quality encoding")
			case GoalLatency:
				params.Preset = "fast"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'fast': prioritizing speed even with high-end CPU")
			default:
				params.Preset = "medium"
				params.Reasoning = append(params.Reasoning,
					"Using preset 'medium': balanced quality on high-end CPU")
			}
		}
	} else {
		// libx264 presets: same as libx265
		switch {
		case worker.CPUCores <= 4:
			if goal == GoalQuality {
				params.Preset = "fast"
			} else {
				params.Preset = "veryfast"
			}
			params.Reasoning = append(params.Reasoning,
				fmt.Sprintf("Using preset '%s': optimized for limited CPU cores", params.Preset))
		case worker.CPUCores <= 8:
			switch goal {
			case GoalQuality:
				params.Preset = "medium"
			case GoalLatency:
				params.Preset = "veryfast"
			default:
				params.Preset = "fast"
			}
			params.Reasoning = append(params.Reasoning,
				fmt.Sprintf("Using preset '%s': balanced for mid-range CPU", params.Preset))
		default:
			switch goal {
			case GoalQuality:
				params.Preset = "slower"
			case GoalLatency:
				params.Preset = "fast"
			default:
				params.Preset = "medium"
			}
			params.Reasoning = append(params.Reasoning,
				fmt.Sprintf("Using preset '%s': optimized for high-end CPU", params.Preset))
		}
	}
}

// selectRateControl chooses between CRF and bitrate, and sets the value
func selectRateControl(params *TranscodingParams, content ContentProperties, goal TranscodingGoal) {
	isNVENC := params.Encoder == "h264_nvenc" || params.Encoder == "hevc_nvenc"
	isHEVC := params.Encoder == "libx265" || params.Encoder == "hevc_nvenc"

	// Base CRF values (lower = higher quality)
	baseCRF := 23 // Standard starting point

	if goal == GoalQuality {
		baseCRF = 20 // Higher quality
	} else if goal == GoalLatency || goal == GoalEnergy {
		baseCRF = 26 // Lower quality for speed
	}

	// Adjust for content complexity
	// Rule: Adjust CRF based on motion and complexity
	switch content.MotionLevel {
	case "high":
		baseCRF -= 2 // More bits for high motion
		params.Reasoning = append(params.Reasoning,
			"Reduced CRF by 2 for high-motion content to preserve detail")
	case "low":
		baseCRF += 2 // Can use higher CRF for static content
		params.Reasoning = append(params.Reasoning,
			"Increased CRF by 2 for low-motion content (more efficient)")
	}

	// Adjust for grain
	switch content.GrainLevel {
	case "high":
		baseCRF -= 2 // More bits needed for grain
		params.Reasoning = append(params.Reasoning,
			"Reduced CRF by 2 for high-grain content to preserve texture")
	case "medium":
		baseCRF -= 1
		params.Reasoning = append(params.Reasoning,
			"Reduced CRF by 1 for medium-grain content")
	}

	// HEVC generally needs higher CRF (more efficient)
	if isHEVC && !isNVENC {
		baseCRF += 5
		params.Reasoning = append(params.Reasoning,
			"Adjusted CRF +5 for HEVC (equivalent quality to H.264 at higher CRF)")
	}

	params.CRF = baseCRF
	params.Reasoning = append(params.Reasoning,
		fmt.Sprintf("Final CRF value: %d (constant quality mode)", baseCRF))
}

// selectThreads determines the optimal thread count
func selectThreads(params *TranscodingParams, worker WorkerCapabilities) {
	isNVENC := params.Encoder == "h264_nvenc" || params.Encoder == "hevc_nvenc"

	if isNVENC {
		// GPU encoding doesn't benefit much from many CPU threads
		params.Threads = 4
		params.Reasoning = append(params.Reasoning,
			"Using 4 threads: GPU encoding has minimal CPU thread requirements")
	} else {
		// Rule: Use most available cores, but leave some for system
		if worker.CPUCores <= 4 {
			params.Threads = worker.CPUCores
		} else if worker.CPUCores <= 8 {
			params.Threads = worker.CPUCores - 1
		} else {
			params.Threads = worker.CPUCores - 2
		}
		params.Reasoning = append(params.Reasoning,
			fmt.Sprintf("Using %d threads: optimized for %d CPU cores (leaving headroom for system)",
				params.Threads, worker.CPUCores))
	}
}

// selectPixelFormat chooses the output pixel format
func selectPixelFormat(params *TranscodingParams, content ContentProperties) {
	// Rule: Use 10-bit if HDR is detected
	if content.IsHDR {
		if content.PixelFormat == "yuv420p10le" || content.ColorSpace == "bt2020" {
			params.PixelFormat = "yuv420p10le"
			params.Reasoning = append(params.Reasoning,
				"Using 10-bit pixel format (yuv420p10le) for HDR content to preserve color depth")
		} else {
			params.PixelFormat = "yuv420p10le"
			params.Reasoning = append(params.Reasoning,
				"Using 10-bit pixel format (yuv420p10le) for HDR content")
		}
	} else {
		params.PixelFormat = "yuv420p"
		params.Reasoning = append(params.Reasoning,
			"Using 8-bit pixel format (yuv420p) for SDR content (standard)")
	}
}

// addSpecialFlags adds encoder-specific optimization flags
func addSpecialFlags(params *TranscodingParams, worker WorkerCapabilities, content ContentProperties, goal TranscodingGoal) {
	isNVENC := params.Encoder == "h264_nvenc" || params.Encoder == "hevc_nvenc"
	isHEVC := params.Encoder == "libx265" || params.Encoder == "hevc_nvenc"

	if isNVENC {
		// NVENC-specific flags
		params.ExtraParams["rc"] = "vbr"
		params.Reasoning = append(params.Reasoning,
			"Using VBR rate control for NVENC: better quality than CBR")

		// Adaptive quantization for better quality
		if goal == GoalQuality || goal == GoalBalanced {
			params.ExtraParams["spatial-aq"] = "1"
			params.ExtraParams["temporal-aq"] = "1"
			params.Reasoning = append(params.Reasoning,
				"Enabled spatial and temporal AQ for NVENC: improves perceptual quality")
		}

		// B-frames based on content motion
		switch content.MotionLevel {
		case "high":
			params.ExtraParams["bf"] = "2"
			params.Reasoning = append(params.Reasoning,
				"Using 2 B-frames for high-motion: balances compression and encoding speed")
		case "medium":
			params.ExtraParams["bf"] = "3"
			params.Reasoning = append(params.Reasoning,
				"Using 3 B-frames for medium-motion: good compression efficiency")
		default:
			params.ExtraParams["bf"] = "4"
			params.Reasoning = append(params.Reasoning,
				"Using 4 B-frames for low-motion: maximizes compression efficiency")
		}

		// Low-latency mode for latency goal
		if goal == GoalLatency {
			params.ExtraParams["zerolatency"] = "1"
			params.Reasoning = append(params.Reasoning,
				"Enabled zero-latency mode for NVENC: minimizes encoding delay")
		}

	} else if isHEVC {
		// libx265-specific flags
		
		// Adaptive quantization mode
		if goal == GoalQuality {
			params.ExtraParams["aq-mode"] = "3"
			params.Reasoning = append(params.Reasoning,
				"Using AQ mode 3: auto-variance AQ for best perceptual quality")
		} else {
			params.ExtraParams["aq-mode"] = "2"
			params.Reasoning = append(params.Reasoning,
				"Using AQ mode 2: variance AQ for balanced quality")
		}

		// SAO (Sample Adaptive Offset) - disable for low-latency
		if goal == GoalLatency {
			params.ExtraParams["no-sao"] = "1"
			params.Reasoning = append(params.Reasoning,
				"Disabled SAO filter: reduces encoding time for latency goal")
		}

		// B-frames based on content
		switch content.MotionLevel {
		case "high":
			params.ExtraParams["bframes"] = "3"
			params.Reasoning = append(params.Reasoning,
				"Using 3 B-frames for high-motion HEVC: balances quality and speed")
		case "medium":
			params.ExtraParams["bframes"] = "4"
			params.Reasoning = append(params.Reasoning,
				"Using 4 B-frames for medium-motion HEVC: good compression")
		default:
			params.ExtraParams["bframes"] = "8"
			params.Reasoning = append(params.Reasoning,
				"Using 8 B-frames for low-motion HEVC: maximum compression efficiency")
		}

		// RD optimization
		if goal == GoalQuality {
			params.ExtraParams["rd"] = "6"
			params.Reasoning = append(params.Reasoning,
				"Using RD level 6: thorough rate-distortion optimization for quality")
		}

	} else {
		// libx264-specific flags
		
		// Tune option based on content
		if content.GrainLevel == "high" {
			params.ExtraParams["tune"] = "grain"
			params.Reasoning = append(params.Reasoning,
				"Using tune=grain: optimized for grainy content")
		} else if goal == GoalLatency {
			params.ExtraParams["tune"] = "zerolatency"
			params.Reasoning = append(params.Reasoning,
				"Using tune=zerolatency: optimized for low-latency streaming")
		} else {
			params.ExtraParams["tune"] = "film"
			params.Reasoning = append(params.Reasoning,
				"Using tune=film: optimized for general film content")
		}

		// B-frames
		switch content.MotionLevel {
		case "high":
			params.ExtraParams["bf"] = "2"
			params.Reasoning = append(params.Reasoning,
				"Using 2 B-frames for high-motion: reduces compression artifacts")
		default:
			params.ExtraParams["bf"] = "3"
			params.Reasoning = append(params.Reasoning,
				"Using 3 B-frames: standard for good compression")
		}

		// Motion estimation
		if goal == GoalQuality && worker.CPUCores > 4 {
			params.ExtraParams["me"] = "umh"
			params.Reasoning = append(params.Reasoning,
				"Using UMH motion estimation: better quality with sufficient CPU")
		}
	}

	// Common flags for all encoders
	
	// GOP size based on framerate
	gopSize := content.Framerate * 2 // 2 seconds
	params.ExtraParams["g"] = fmt.Sprintf("%d", gopSize)
	params.Reasoning = append(params.Reasoning,
		fmt.Sprintf("GOP size set to %d: 2 seconds for good seek performance", gopSize))
}
