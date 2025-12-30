package agent

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// FFmpegEngine implements the Engine interface for FFmpeg
type FFmpegEngine struct {
	caps       *models.NodeCapabilities
	nodeType   models.NodeType
	optimizer  *FFmpegOptimization
}

// NewFFmpegEngine creates a new FFmpeg engine
func NewFFmpegEngine(caps *models.NodeCapabilities, nodeType models.NodeType) *FFmpegEngine {
	optimizer := OptimizeFFmpegParameters(caps, nodeType)
	return &FFmpegEngine{
		caps:      caps,
		nodeType:  nodeType,
		optimizer: optimizer,
	}
}

// Name returns the engine name
func (e *FFmpegEngine) Name() string {
	return "ffmpeg"
}

// Supports checks if FFmpeg can handle the job
func (e *FFmpegEngine) Supports(job *models.Job, caps *models.NodeCapabilities) bool {
	// FFmpeg supports all scenarios
	return true
}

// BuildCommand generates FFmpeg command arguments
func (e *FFmpegEngine) BuildCommand(job *models.Job, hostURL string) ([]string, error) {
	params := job.Parameters
	if params == nil {
		params = make(map[string]interface{})
	}

	// Apply hardware-optimized parameters (job parameters take precedence)
	params = ApplyOptimizationToParameters(params, e.optimizer)

	// Determine output mode: RTMP streaming or file output
	outputMode := "file" // default
	if mode, ok := params["output_mode"].(string); ok {
		outputMode = mode
	}

	// Get RTMP URL if streaming mode
	rtmpURL := ""
	if outputMode == "rtmp" || outputMode == "stream" {
		if rtmpURLParam, ok := params["rtmp_url"].(string); ok && rtmpURLParam != "" {
			rtmpURL = rtmpURLParam
		} else {
			// Default: construct RTMP URL pointing to master node
			masterHost := "localhost"
			if hostURL != "" {
				parsedURL, err := url.Parse(hostURL)
				if err == nil && parsedURL.Host != "" {
					// Extract hostname (remove API port)
					host := parsedURL.Host
					if colonIdx := strings.Index(host, ":"); colonIdx > 0 {
						host = host[:colonIdx]
					}
					masterHost = host
				}
			}

			streamKey := job.ID
			if key, ok := params["stream_key"].(string); ok && key != "" {
				streamKey = key
			}
			// RTMP server runs on master at port 1935
			rtmpURL = fmt.Sprintf("rtmp://%s:1935/live/%s", masterHost, streamKey)
		}
	}

	// Get input file (use test pattern if not specified)
	inputFile := "/tmp/test_input.mp4"
	if input, ok := params["input"].(string); ok && input != "" {
		inputFile = input
	}

	// Get output file (only used in file mode)
	outputFile := fmt.Sprintf("/tmp/job_%s_output.mp4", job.ID)
	if output, ok := params["output"].(string); ok && output != "" {
		outputFile = output
	}

	// Get transcode parameters with defaults
	bitrate := "2000k"
	if b, ok := params["bitrate"].(string); ok && b != "" {
		bitrate = b
	}

	codec := "libx264"
	if c, ok := params["codec"].(string); ok && c != "" {
		// Map common codec names
		switch c {
		case "h264":
			codec = "libx264"
		case "h265", "hevc":
			codec = "libx265"
		case "vp9":
			codec = "libvpx-vp9"
		default:
			codec = c
		}
	}

	preset := "medium"
	if p, ok := params["preset"].(string); ok && p != "" {
		preset = p
	}

	duration := 0
	if d, ok := params["duration"].(float64); ok {
		duration = int(d)
	} else if d, ok := params["duration"].(int); ok {
		duration = d
	}

	// Build FFmpeg command based on output mode
	var args []string

	if outputMode == "rtmp" || outputMode == "stream" {
		// RTMP Streaming mode - generate test source and stream
		// Get resolution and framerate for test source
		resolution := "1280x720"
		if res, ok := params["resolution"].(string); ok && res != "" {
			resolution = res
		}

		fps := 30
		if f, ok := params["fps"].(float64); ok {
			fps = int(f)
		} else if f, ok := params["fps"].(int); ok {
			fps = f
		}

		// Calculate buffer size
		bufsize := bitrate
		if strings.HasSuffix(bitrate, "k") {
			bitrateNum := bitrate[:len(bitrate)-1]
			bufsize = fmt.Sprintf("%sk", bitrateNum)
		}

		// Build streaming command with hardware optimizations
		args = []string{
			"-re", // Read input at native framerate (important for streaming)
			"-f", "lavfi",
			"-i", fmt.Sprintf("testsrc=size=%s:rate=%d", resolution, fps),
			"-f", "lavfi",
			"-i", "sine=frequency=1000:sample_rate=48000",
		}

		// Add duration limit if specified (before encoding options)
		if duration > 0 {
			args = append(args, "-t", fmt.Sprintf("%d", duration))
		}

		// Video encoding options with hardware optimization
		args = append(args, "-c:v", codec, "-preset", preset)

		// Apply hardware-optimized extra flags
		for key, value := range e.optimizer.ExtraFlags {
			switch key {
			case "tune":
				// Apply tune for software encoders
				if codec == "libx264" || codec == "libx265" {
					args = append(args, "-tune", value)
				}
			case "threads":
				args = append(args, "-threads", value)
			case "rc", "spatial-aq", "temporal-aq", "bf", "zerolatency":
				// NVENC-specific flags
				if strings.Contains(codec, "nvenc") {
					args = append(args, fmt.Sprintf("-%s", key), value)
				}
			}
		}

		// Streaming-specific encoding options
		args = append(args,
			"-b:v", bitrate,
			"-maxrate", bitrate,
			"-bufsize", bufsize,
			"-pix_fmt", "yuv420p",
			"-g", fmt.Sprintf("%d", fps*2), // GOP size: 2 seconds
		)

		// Audio encoding options
		args = append(args,
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-f", "flv", // FLV container for RTMP
			rtmpURL,
		)

	} else {
		// File transcoding mode
		args = []string{
			"-i", inputFile,
			"-c:v", codec,
			"-b:v", bitrate,
			"-preset", preset,
			"-y", // Overwrite output
		}

		// Add duration limit if specified
		if duration > 0 {
			args = append([]string{"-t", fmt.Sprintf("%d", duration)}, args...)
		}

		args = append(args, outputFile)
	}

	return args, nil
}
