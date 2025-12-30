package agent

import (
	"testing"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func TestEngineSelector_SelectEngine(t *testing.T) {
	// Create test capabilities
	caps := &models.NodeCapabilities{
		CPUThreads:      8,
		CPUModel:        "Test CPU",
		HasGPU:          true,
		GPUType:         "NVIDIA GeForce RTX 3080",
		GPUCapabilities: []string{"nvenc_h264", "nvenc_h265"},
		RAMTotalBytes:   16 * 1024 * 1024 * 1024,
	}

	selector := NewEngineSelector(caps, models.NodeTypeDesktop)

	tests := []struct {
		name           string
		job            *models.Job
		expectedEngine string
	}{
		{
			name: "Explicit FFmpeg preference",
			job: &models.Job{
				ID:       "test-1",
				Scenario: "test",
				Parameters: map[string]interface{}{
					"engine": "ffmpeg",
				},
				Queue: "default",
			},
			expectedEngine: "ffmpeg",
		},
		{
			name: "Explicit GStreamer preference",
			job: &models.Job{
				ID:       "test-2",
				Scenario: "test",
				Parameters: map[string]interface{}{
					"engine": "gstreamer",
				},
				Queue: "default",
			},
			expectedEngine: "gstreamer",
		},
		{
			name: "Auto selection for LIVE queue",
			job: &models.Job{
				ID:       "test-3",
				Scenario: "live-stream",
				Queue:    "live",
			},
			expectedEngine: "gstreamer",
		},
		{
			name: "Auto selection for batch queue",
			job: &models.Job{
				ID:       "test-4",
				Scenario: "batch-transcode",
				Queue:    "batch",
			},
			expectedEngine: "ffmpeg",
		},
		{
			name: "Auto selection for RTMP output mode",
			job: &models.Job{
				ID:       "test-5",
				Scenario: "stream",
				Parameters: map[string]interface{}{
					"output_mode": "rtmp",
				},
				Queue: "default",
			},
			expectedEngine: "gstreamer",
		},
		{
			name: "Auto selection for file output mode",
			job: &models.Job{
				ID:       "test-6",
				Scenario: "transcode",
				Parameters: map[string]interface{}{
					"output_mode": "file",
				},
				Queue: "default",
			},
			expectedEngine: "ffmpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, reason := selector.SelectEngine(tt.job)
			if engine.Name() != tt.expectedEngine {
				t.Errorf("SelectEngine() engine = %v, want %v (reason: %s)", engine.Name(), tt.expectedEngine, reason)
			}
		})
	}
}

func TestEngineSelector_GetAvailableEngines(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads:    4,
		HasGPU:        false,
		RAMTotalBytes: 8 * 1024 * 1024 * 1024,
	}

	selector := NewEngineSelector(caps, models.NodeTypeDesktop)
	engines := selector.GetAvailableEngines()

	if len(engines) < 1 {
		t.Error("Expected at least one available engine")
	}

	// FFmpeg should always be available
	hasFFmpeg := false
	for _, e := range engines {
		if e == "ffmpeg" {
			hasFFmpeg = true
		}
	}

	if !hasFFmpeg {
		t.Error("FFmpeg should always be available")
	}
}

func TestFFmpegEngine_Name(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 4,
		HasGPU:     false,
	}

	engine := NewFFmpegEngine(caps, models.NodeTypeDesktop)
	if engine.Name() != "ffmpeg" {
		t.Errorf("FFmpegEngine.Name() = %v, want %v", engine.Name(), "ffmpeg")
	}
}

func TestFFmpegEngine_Supports(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 4,
		HasGPU:     false,
	}

	engine := NewFFmpegEngine(caps, models.NodeTypeDesktop)
	job := &models.Job{
		ID:       "test",
		Scenario: "test",
	}

	if !engine.Supports(job, caps) {
		t.Error("FFmpegEngine should support all jobs")
	}
}

func TestGStreamerEngine_Name(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 4,
		HasGPU:     false,
	}

	engine := NewGStreamerEngine(caps, models.NodeTypeDesktop)
	if engine.Name() != "gstreamer" {
		t.Errorf("GStreamerEngine.Name() = %v, want %v", engine.Name(), "gstreamer")
	}
}

func TestGStreamerEngine_Supports(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 4,
		HasGPU:     false,
	}

	engine := NewGStreamerEngine(caps, models.NodeTypeDesktop)
	job := &models.Job{
		ID:       "test",
		Scenario: "test",
	}

	if !engine.Supports(job, caps) {
		t.Error("GStreamerEngine should support all jobs")
	}
}
