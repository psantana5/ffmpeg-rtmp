package agent

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// GStreamerEngine implements the Engine interface for GStreamer
type GStreamerEngine struct {
	caps     *models.NodeCapabilities
	nodeType models.NodeType
}

// NewGStreamerEngine creates a new GStreamer engine
func NewGStreamerEngine(caps *models.NodeCapabilities, nodeType models.NodeType) *GStreamerEngine {
	return &GStreamerEngine{
		caps:     caps,
		nodeType: nodeType,
	}
}

// Name returns the engine name
func (e *GStreamerEngine) Name() string {
	return "gstreamer"
}

// Supports checks if GStreamer can handle the job
func (e *GStreamerEngine) Supports(job *models.Job, caps *models.NodeCapabilities) bool {
	// GStreamer is particularly good for live streaming scenarios
	// Support all scenarios but preferred for live/streaming workloads
	return true
}

// BuildCommand generates GStreamer gst-launch-1.0 command arguments
func (e *GStreamerEngine) BuildCommand(job *models.Job, hostURL string) ([]string, error) {
	params := job.Parameters
	if params == nil {
		params = make(map[string]interface{})
	}

	// Get duration parameter (in seconds)
	duration := 0
	if d, ok := params["duration"].(float64); ok {
		duration = int(d)
	} else if d, ok := params["duration"].(int); ok {
		duration = d
	} else if d, ok := params["duration_seconds"].(float64); ok {
		duration = int(d)
	} else if d, ok := params["duration_seconds"].(int); ok {
		duration = d
	}

	// Get RTMP URL
	rtmpURL := ""
	masterHost := "localhost" // Declare here
	if rtmpURLParam, ok := params["rtmp_url"].(string); ok && rtmpURLParam != "" {
		rtmpURL = rtmpURLParam
	} else {
		// Default: construct RTMP URL pointing to master node
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

	// Get input file or use test pattern
	inputFile := ""
	if input, ok := params["input"].(string); ok && input != "" {
		inputFile = input
	}

	// Get encoding parameters
	bitrate := 2000 // kbps (default)
	if b, ok := params["bitrate"].(string); ok && b != "" {
		// Parse bitrate (e.g., "2000k" -> 2000)
		bitrateStr := strings.TrimSuffix(b, "k")
		bitrateStr = strings.TrimSuffix(bitrateStr, "K")
		if _, err := fmt.Sscanf(bitrateStr, "%d", &bitrate); err != nil {
			// Failed to parse, use default
			bitrate = 2000
		}
	}

	// Select encoder based on hardware capabilities
	videoEncoder := e.selectVideoEncoder()
	
	// Build GStreamer pipeline
	var pipeline []string

	// Add -e flag for EOS (End of Stream) signal handling
	// This ensures proper cleanup when duration expires or stream ends
	pipeline = append(pipeline, "-e")

	if inputFile != "" {
		// File input pipeline
		pipeline = append(pipeline,
			"filesrc", fmt.Sprintf("location=%s", inputFile), "!",
			"decodebin", "!",
		)
		
		// Add duration limiter for file inputs
		if duration > 0 {
			//Calculate number of buffers based on framerate assumption (30fps default)
			// This caps the video at specified duration
			numBuffers := duration * 30
			pipeline = append(pipeline,
				"identity", fmt.Sprintf("drop-allocation=true"), "!",
				"videorate", "!",
				"video/x-raw,framerate=30/1", "!",
				"identity", fmt.Sprintf("num-buffers=%d", numBuffers), "!",
			)
			log.Printf("→ File input: limiting to %d buffers (%d seconds at 30fps)", numBuffers, duration)
		} else {
			log.Printf("→ File input: no duration limit, will process entire file")
		}
		
		pipeline = append(pipeline, "videoconvert", "!")
	} else {
		// Test pattern pipeline
		pipeline = append(pipeline,
			"videotestsrc", "pattern=ball",
		)
		
		// Add num-buffers for duration control on test source
		if duration > 0 {
			// Calculate number of buffers at 30fps
			numBuffers := duration * 30
			pipeline = append(pipeline,
				fmt.Sprintf("num-buffers=%d", numBuffers),
			)
		}
		
		pipeline = append(pipeline,
			"!",
			"video/x-raw,format=I420,width=1280,height=720,framerate=30/1", "!",
			"videoconvert", "!",
		)
	}

	// Add video encoding based on hardware
	switch videoEncoder {
	case "nvh264enc", "nvh265enc":
		// NVIDIA NVENC hardware encoding
		pipeline = append(pipeline,
			videoEncoder,
			fmt.Sprintf("bitrate=%d", bitrate),
			"preset=low-latency-hq",
			"rc-mode=cbr",
			"gop-size=60",
			"!",
		)
	case "vaapih264enc":
		// Intel VAAPI hardware encoding
		pipeline = append(pipeline,
			videoEncoder,
			fmt.Sprintf("bitrate=%d", bitrate),
			"rate-control=cbr",
			"keyframe-period=60",
			"!",
		)
	case "qsvh264enc":
		// Intel QSV hardware encoding
		pipeline = append(pipeline,
			videoEncoder,
			fmt.Sprintf("bitrate=%d", bitrate),
			"rate-control=cbr",
			"gop-size=60",
			"!",
		)
	default:
		// Software x264 encoding
		pipeline = append(pipeline,
			"x264enc",
			fmt.Sprintf("bitrate=%d", bitrate),
			"pass=cbr",
			"tune=zerolatency",
			"speed-preset=ultrafast",
			"key-int-max=60",
			"!",
		)
	}

	// Add muxer and RTMP sink with buffer tuning for low latency
	pipeline = append(pipeline,
		"video/x-h264,profile=baseline", "!",
		"h264parse", "!", // Add h264parse for better stream compatibility
		"flvmux", "name=mux", "streamable=true", "!",
		"rtmpsink",
		fmt.Sprintf("location=%s", rtmpURL),
		"sync=false",
		"async=false",
		"max-lateness=-1", // Allow late buffers (changed from 0)
		"qos=false", // Disable QoS to prevent early termination
	)

	return pipeline, nil
}

// selectVideoEncoder selects the best video encoder based on hardware capabilities
func (e *GStreamerEngine) selectVideoEncoder() string {
	if e.caps.HasGPU {
		gpuType := strings.ToLower(e.caps.GPUType)
		
		// NVIDIA GPU - use NVENC
		if strings.Contains(gpuType, "nvidia") || strings.Contains(gpuType, "geforce") || strings.Contains(gpuType, "quadro") || strings.Contains(gpuType, "tesla") {
			// Check for H.265 support in capabilities
			for _, cap := range e.caps.GPUCapabilities {
				if strings.Contains(cap, "nvenc_h265") || strings.Contains(cap, "hevc") {
					return "nvh265enc"
				}
			}
			return "nvh264enc"
		}
		
		// Intel GPU - try VAAPI or QSV
		if strings.Contains(gpuType, "intel") {
			// Prefer QSV if available
			for _, cap := range e.caps.GPUCapabilities {
				if strings.Contains(cap, "qsv") {
					return "qsvh264enc"
				}
			}
			return "vaapih264enc"
		}
	}

	// Default to software encoding
	return "x264enc"
}
