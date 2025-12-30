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

// FFmpegParams is an alias for TranscodingParams for clearer API naming
// This provides better semantic clarity when building FFmpeg commands
type FFmpegParams = TranscodingParams

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

// RTMPConfig contains configuration for RTMP streaming
type RTMPConfig struct {
	MasterHost string // Master node hostname or IP (e.g., "10.0.1.5", "master.local")
	StreamKey  string // Unique stream identifier
	InputFile  string // Input video file path
}

// BuildFFmpegCommand constructs a complete FFmpeg command for RTMP streaming
// with all hardware optimizations applied. The command is validated for the
// chosen codec to ensure all flags are compatible.
//
// Parameters:
//   - params: Optimized FFmpeg parameters from CalculateOptimalFFmpegParams
//   - config: RTMP streaming configuration
//
// Returns:
//   - Complete FFmpeg command as a slice of arguments (ready for exec.Command)
//   - Error if validation fails or configuration is invalid
func BuildFFmpegCommand(params *FFmpegParams, config RTMPConfig) ([]string, error) {
	// Validate required configuration
	if config.MasterHost == "" {
		return nil, fmt.Errorf("master host is required (do not use localhost)")
	}
	if config.StreamKey == "" {
		return nil, fmt.Errorf("stream key is required")
	}
	if config.InputFile == "" {
		return nil, fmt.Errorf("input file is required")
	}

	// Build RTMP URL - RTMP server runs on master node at port 1935
	rtmpURL := fmt.Sprintf("rtmp://%s:1935/live/%s", config.MasterHost, config.StreamKey)

	// Start building command arguments
	args := []string{
		"-re", // Read input at native frame rate (important for streaming)
		"-i", config.InputFile,
	}

	// Add codec selection
	// Note: FFmpeg uses -c:v for video codec
	args = append(args, "-c:v", params.Encoder)

	// Add preset
	args = append(args, "-preset", params.Preset)

	// Add pixel format if specified
	if params.PixelFormat != "" {
		args = append(args, "-pix_fmt", params.PixelFormat)
	}

	// Add rate control: CRF or bitrate
	// CRF is preferred for quality-based encoding
	if params.CRF > 0 {
		// Validate CRF flag based on encoder type
		if err := validateCRFForEncoder(params.Encoder, params.CRF); err != nil {
			return nil, err
		}
		args = append(args, "-crf", fmt.Sprintf("%d", params.CRF))
	} else if params.Bitrate != "" {
		args = append(args, "-b:v", params.Bitrate)
	}

	// Add thread count if specified
	if params.Threads > 0 {
		args = append(args, "-threads", fmt.Sprintf("%d", params.Threads))
	}

	// Add encoder-specific extra parameters
	// These are validated based on the encoder type
	if err := addEncoderSpecificFlags(&args, params); err != nil {
		return nil, err
	}

	// Add output format and destination
	args = append(args, "-f", "flv", rtmpURL)

	return args, nil
}

// validateCRFForEncoder ensures CRF values are appropriate for the encoder
func validateCRFForEncoder(encoder string, crf int) error {
	switch encoder {
	case "libx264", "h264_nvenc":
		// H.264 CRF range: 0-51 (typical: 18-28)
		if crf < 0 || crf > 51 {
			return fmt.Errorf("CRF %d out of range for %s (valid: 0-51)", crf, encoder)
		}
	case "libx265", "hevc_nvenc":
		// H.265 CRF range: 0-51 (typical: 20-32 due to better compression)
		if crf < 0 || crf > 51 {
			return fmt.Errorf("CRF %d out of range for %s (valid: 0-51)", crf, encoder)
		}
	case "h264_qsv", "hevc_qsv":
		// QSV uses global_quality instead, but we still validate range
		if crf < 0 || crf > 51 {
			return fmt.Errorf("CRF %d out of range for %s (valid: 0-51)", crf, encoder)
		}
	default:
		// For other encoders, allow any positive value
		if crf < 0 {
			return fmt.Errorf("CRF must be non-negative")
		}
	}
	return nil
}

// addEncoderSpecificFlags adds codec-specific flags to the FFmpeg command
// and validates they are compatible with the chosen encoder
func addEncoderSpecificFlags(args *[]string, params *FFmpegParams) error {
	isNVENC := params.Encoder == "h264_nvenc" || params.Encoder == "hevc_nvenc"
	isQSV := params.Encoder == "h264_qsv" || params.Encoder == "hevc_qsv"
	isX264 := params.Encoder == "libx264"
	isX265 := params.Encoder == "libx265"

	for key, value := range params.ExtraParams {
		// Validate and transform flags based on encoder type
		switch key {
		case "tune":
			// tune is valid for libx264/libx265 but not NVENC/QSV
			if isNVENC || isQSV {
				// Skip tune for hardware encoders
				continue
			}
			*args = append(*args, "-tune", value)

		case "rc":
			// Rate control mode - NVENC specific
			if !isNVENC {
				// Skip rc for non-NVENC encoders
				continue
			}
			*args = append(*args, "-rc", value)

		case "spatial-aq", "temporal-aq":
			// Adaptive quantization - NVENC specific
			if !isNVENC {
				continue
			}
			*args = append(*args, "-"+key, value)

		case "zerolatency":
			// Low latency mode - NVENC specific
			if !isNVENC {
				continue
			}
			*args = append(*args, "-zerolatency", value)

		case "bf", "bframes":
			// B-frames: different flags for different encoders
			if isNVENC {
				*args = append(*args, "-bf", value)
			} else if isX264 || isX265 {
				*args = append(*args, "-bf", value)
			} else if isQSV {
				*args = append(*args, "-bf", value)
			}

		case "aq-mode":
			// AQ mode - libx265 specific
			if !isX265 {
				continue
			}
			*args = append(*args, "-x265-params", fmt.Sprintf("aq-mode=%s", value))

		case "no-sao":
			// SAO filter - libx265 specific
			if !isX265 {
				continue
			}
			if value == "1" {
				*args = append(*args, "-x265-params", "no-sao=1")
			}

		case "rd":
			// Rate-distortion optimization - libx265 specific
			if !isX265 {
				continue
			}
			*args = append(*args, "-x265-params", fmt.Sprintf("rd=%s", value))

		case "me":
			// Motion estimation - libx264 specific
			if !isX264 {
				continue
			}
			*args = append(*args, "-me_method", value)

		case "g":
			// GOP size - universal
			*args = append(*args, "-g", value)

		case "threads":
			// Thread count - already handled separately, skip here
			continue

		default:
			// Unknown parameter - log warning but don't fail
			// This allows for future extensibility
			continue
		}
	}

	return nil
}

// GetRTMPURLFromMasterURL extracts the hostname from a master API URL
// and constructs the RTMP streaming URL
//
// Example:
//   - Input: "https://10.0.1.5:8080" -> Output: "10.0.1.5"
//   - Input: "http://master.local:8080" -> Output: "master.local"
//   - Input: "https://master.example.com:443/api" -> Output: "master.example.com"
func GetRTMPURLFromMasterURL(masterURL string) (string, error) {
	if masterURL == "" {
		return "", fmt.Errorf("master URL cannot be empty")
	}

	// Check for localhost - this is an error as workers must stream to master
	if masterURL == "localhost" || masterURL == "127.0.0.1" || masterURL == "::1" {
		return "", fmt.Errorf("cannot use localhost for RTMP streaming - must specify master host")
	}

	// Try to parse as URL
	if len(masterURL) > 4 && (masterURL[:4] == "http" || masterURL[:5] == "https") {
		// Parse full URL
		var host string

		// Find the host:port part after ://
		schemeEnd := 0
		if idx := len("http://"); len(masterURL) > idx && masterURL[:idx] == "http://" {
			schemeEnd = idx
		} else if idx := len("https://"); len(masterURL) > idx && masterURL[:idx] == "https://" {
			schemeEnd = idx
		}

		if schemeEnd == 0 {
			return "", fmt.Errorf("invalid URL scheme")
		}

		// Extract everything after scheme
		remaining := masterURL[schemeEnd:]

		// Find end of host:port (either / or end of string)
		endIdx := len(remaining)
		if idx := 0; idx < len(remaining) {
			for i, ch := range remaining {
				if ch == '/' {
					endIdx = i
					break
				}
			}
		}

		hostPort := remaining[:endIdx]

		// Split host and port
		colonIdx := -1
		for i := len(hostPort) - 1; i >= 0; i-- {
			if hostPort[i] == ':' {
				colonIdx = i
				break
			}
		}

		if colonIdx > 0 {
			host = hostPort[:colonIdx]
		} else {
			host = hostPort
		}

		if host == "" {
			return "", fmt.Errorf("could not extract host from URL")
		}

		// Validate extracted host is not localhost
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return "", fmt.Errorf("cannot use localhost for RTMP streaming - must specify master host")
		}

		return host, nil
	}

	// Not a URL, treat as hostname directly
	// Remove port if present
	colonIdx := -1
	for i := len(masterURL) - 1; i >= 0; i-- {
		if masterURL[i] == ':' {
			colonIdx = i
			break
		}
	}

	if colonIdx > 0 {
		return masterURL[:colonIdx], nil
	}

	return masterURL, nil
}
