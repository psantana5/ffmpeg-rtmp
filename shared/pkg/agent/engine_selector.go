package agent

import (
	"fmt"
	"log"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// EngineSelector selects the appropriate transcoding engine for a job
type EngineSelector struct {
	ffmpegEngine    *FFmpegEngine
	gstreamerEngine *GStreamerEngine
	caps            *models.NodeCapabilities
}

// NewEngineSelector creates a new engine selector
func NewEngineSelector(caps *models.NodeCapabilities, nodeType models.NodeType) *EngineSelector {
	return &EngineSelector{
		ffmpegEngine:    NewFFmpegEngine(caps, nodeType),
		gstreamerEngine: NewGStreamerEngine(caps, nodeType),
		caps:            caps,
	}
}

// SelectEngine selects the best engine for a job based on preferences and capabilities
func (s *EngineSelector) SelectEngine(job *models.Job) (Engine, string) {
	// 1. Check job preference for engine
	if job.Parameters != nil {
		if enginePref, ok := job.Parameters["engine"].(string); ok && enginePref != "" {
			return s.selectByPreference(job, enginePref)
		}
	}

	// 2. Auto selection based on scenario and queue type
	return s.autoSelectEngine(job)
}

// selectByPreference selects engine based on explicit preference
func (s *EngineSelector) selectByPreference(job *models.Job, preference string) (Engine, string) {
	engineType := EngineType(preference)

	switch engineType {
	case EngineTypeFFmpeg:
		reason := fmt.Sprintf("Engine explicitly set to FFmpeg via job parameters")
		log.Printf("Engine selection: %s (job %s)", reason, job.ID)
		return s.ffmpegEngine, reason

	case EngineTypeGStreamer:
		// Check if GStreamer is available
		if s.isGStreamerAvailable() {
			reason := fmt.Sprintf("Engine explicitly set to GStreamer via job parameters")
			log.Printf("Engine selection: %s (job %s)", reason, job.ID)
			return s.gstreamerEngine, reason
		}
		// Fallback to FFmpeg if GStreamer not available
		reason := fmt.Sprintf("GStreamer requested but not available, falling back to FFmpeg")
		log.Printf("Engine selection: %s (job %s)", reason, job.ID)
		return s.ffmpegEngine, reason

	case EngineTypeAuto:
		return s.autoSelectEngine(job)

	default:
		// Unknown engine preference, use auto selection
		reason := fmt.Sprintf("Unknown engine preference '%s', using auto selection", preference)
		log.Printf("Engine selection: %s (job %s)", reason, job.ID)
		return s.autoSelectEngine(job)
	}
}

// autoSelectEngine automatically selects the best engine based on job characteristics
func (s *EngineSelector) autoSelectEngine(job *models.Job) (Engine, string) {
	// Determine output mode
	outputMode := "file" // default
	if job.Parameters != nil {
		if mode, ok := job.Parameters["output_mode"].(string); ok {
			outputMode = mode
		}
	}

	// Check queue type
	queueType := job.Queue
	if queueType == "" {
		queueType = "default"
	}

	// Selection logic:
	// 1. LIVE queue → prefer GStreamer (optimized for streaming)
	// 2. FILE/batch workloads → prefer FFmpeg (better for offline processing)
	// 3. RTMP/stream output mode → prefer GStreamer
	// 4. GPU workers with NVENC → consider GStreamer for hardware encoding

	gstreamerAvailable := s.isGStreamerAvailable()

	// LIVE queue - prefer GStreamer if available
	if queueType == "live" {
		if gstreamerAvailable {
			reason := fmt.Sprintf("LIVE queue job - GStreamer preferred for low-latency streaming (scenario: %s)", job.Scenario)
			log.Printf("Engine selection: %s (job %s)", reason, job.ID)
			return s.gstreamerEngine, reason
		}
		reason := fmt.Sprintf("LIVE queue job but GStreamer not available, using FFmpeg (scenario: %s)", job.Scenario)
		log.Printf("Engine selection: %s (job %s)", reason, job.ID)
		return s.ffmpegEngine, reason
	}

	// RTMP/stream output mode - prefer GStreamer if available
	if (outputMode == "rtmp" || outputMode == "stream") && gstreamerAvailable {
		reason := fmt.Sprintf("Streaming output mode - GStreamer preferred for RTMP (scenario: %s)", job.Scenario)
		log.Printf("Engine selection: %s (job %s)", reason, job.ID)
		return s.gstreamerEngine, reason
	}

	// GPU workers with NVENC capabilities - consider GStreamer for hardware encoding
	if s.caps.HasGPU && gstreamerAvailable {
		// Check for NVENC capabilities
		hasNVENC := false
		for _, cap := range s.caps.GPUCapabilities {
			if cap == "nvenc_h264" || cap == "nvenc_h265" {
				hasNVENC = true
				break
			}
		}
		
		if hasNVENC && (outputMode == "rtmp" || outputMode == "stream") {
			reason := fmt.Sprintf("GPU worker with NVENC - GStreamer preferred for hardware-accelerated streaming (GPU: %s, scenario: %s)", s.caps.GPUType, job.Scenario)
			log.Printf("Engine selection: %s (job %s)", reason, job.ID)
			return s.gstreamerEngine, reason
		}
	}

	// Default: FFmpeg for file-based and batch processing
	reason := fmt.Sprintf("File/batch processing - FFmpeg preferred (queue: %s, output_mode: %s, scenario: %s)", queueType, outputMode, job.Scenario)
	log.Printf("Engine selection: %s (job %s)", reason, job.ID)
	return s.ffmpegEngine, reason
}

// isGStreamerAvailable checks if GStreamer is available on the system
func (s *EngineSelector) isGStreamerAvailable() bool {
	// For now, we'll assume GStreamer might not be available
	// In a real implementation, this would check for gst-launch-1.0 binary
	// For this implementation, we'll return true to allow testing
	// In production, add actual binary check:
	// _, err := exec.LookPath("gst-launch-1.0")
	// return err == nil
	
	// For now, return true to allow engine selection to work
	return true
}

// GetAvailableEngines returns a list of available engines
func (s *EngineSelector) GetAvailableEngines() []string {
	engines := []string{"ffmpeg"} // FFmpeg is always available
	
	if s.isGStreamerAvailable() {
		engines = append(engines, "gstreamer")
	}
	
	return engines
}
