package agent

import (
	"strings"
	"testing"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func TestFFmpegEngine_BuildCommand_FileMode(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads:    8,
		CPUModel:      "Test CPU",
		HasGPU:        false,
		RAMTotalBytes: 16 * 1024 * 1024 * 1024,
	}

	engine := NewFFmpegEngine(caps, models.NodeTypeDesktop)

	job := &models.Job{
		ID:       "test-job-1",
		Scenario: "file-transcode",
		Parameters: map[string]interface{}{
			"output_mode": "file",
			"bitrate":     "5000k",
			"duration":    30,
		},
	}

	args, err := engine.BuildCommand(job, "http://localhost:8080")
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}

	// Check that basic FFmpeg options are present
	argsStr := strings.Join(args, " ")
	
	if !strings.Contains(argsStr, "-i") {
		t.Error("Command should contain input flag -i")
	}
	
	if !strings.Contains(argsStr, "-c:v") {
		t.Error("Command should contain video codec flag -c:v")
	}
	
	if !strings.Contains(argsStr, "5000k") {
		t.Error("Command should contain specified bitrate 5000k")
	}
	
	if !strings.Contains(argsStr, "-y") {
		t.Error("Command should contain overwrite flag -y for file mode")
	}
}

func TestFFmpegEngine_BuildCommand_RTMPMode(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads:    8,
		CPUModel:      "Test CPU",
		HasGPU:        true,
		GPUType:       "NVIDIA GeForce RTX 3080",
		GPUCapabilities: []string{"nvenc_h264"},
		RAMTotalBytes: 16 * 1024 * 1024 * 1024,
	}

	engine := NewFFmpegEngine(caps, models.NodeTypeDesktop)

	job := &models.Job{
		ID:       "test-job-2",
		Scenario: "live-stream",
		Parameters: map[string]interface{}{
			"output_mode": "rtmp",
			"bitrate":     "3000k",
			"resolution":  "1920x1080",
			"fps":         30,
		},
	}

	args, err := engine.BuildCommand(job, "http://localhost:8080")
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}

	argsStr := strings.Join(args, " ")
	
	// Check for streaming-specific options
	if !strings.Contains(argsStr, "-re") {
		t.Error("Command should contain -re flag for real-time streaming")
	}
	
	if !strings.Contains(argsStr, "rtmp://") {
		t.Error("Command should contain RTMP URL")
	}
	
	if !strings.Contains(argsStr, "-f") || !strings.Contains(argsStr, "flv") {
		t.Error("Command should specify FLV format for RTMP")
	}
	
	if !strings.Contains(argsStr, "3000k") {
		t.Error("Command should contain specified bitrate 3000k")
	}
}

func TestFFmpegEngine_BuildCommand_HardwareAcceleration(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads:      8,
		CPUModel:        "Test CPU",
		HasGPU:          true,
		GPUType:         "NVIDIA GeForce RTX 3080",
		GPUCapabilities: []string{"nvenc_h264", "nvenc_h265"},
		RAMTotalBytes:   16 * 1024 * 1024 * 1024,
	}

	engine := NewFFmpegEngine(caps, models.NodeTypeDesktop)

	job := &models.Job{
		ID:       "test-job-3",
		Scenario: "hw-accel-test",
		Parameters: map[string]interface{}{
			"output_mode": "rtmp",
		},
	}

	args, err := engine.BuildCommand(job, "http://localhost:8080")
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}

	argsStr := strings.Join(args, " ")
	
	// Should use NVENC encoder due to GPU capabilities
	if !strings.Contains(argsStr, "nvenc") {
		t.Error("Command should use NVENC hardware encoder for GPU-enabled workers")
	}
}

func TestGStreamerEngine_BuildCommand(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads:    8,
		CPUModel:      "Test CPU",
		HasGPU:        false,
		RAMTotalBytes: 16 * 1024 * 1024 * 1024,
	}

	engine := NewGStreamerEngine(caps, models.NodeTypeDesktop)

	job := &models.Job{
		ID:       "test-gst-1",
		Scenario: "gstreamer-stream",
		Parameters: map[string]interface{}{
			"bitrate": "4000k",
		},
	}

	args, err := engine.BuildCommand(job, "http://localhost:8080")
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}

	argsStr := strings.Join(args, " ")
	
	// Check for GStreamer-specific elements
	if !strings.Contains(argsStr, "-q") {
		t.Error("Command should contain -q flag for quiet mode")
	}
	
	if !strings.Contains(argsStr, "rtmpsink") {
		t.Error("Command should contain rtmpsink element")
	}
	
	if !strings.Contains(argsStr, "flvmux") {
		t.Error("Command should contain flvmux element")
	}
	
	if !strings.Contains(argsStr, "rtmp://") {
		t.Error("Command should contain RTMP URL")
	}
}

func TestGStreamerEngine_BuildCommand_HardwareAcceleration(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads:      8,
		CPUModel:        "Test CPU",
		HasGPU:          true,
		GPUType:         "NVIDIA GeForce RTX 3080",
		GPUCapabilities: []string{"nvenc_h264", "nvenc_h265"},
		RAMTotalBytes:   16 * 1024 * 1024 * 1024,
	}

	engine := NewGStreamerEngine(caps, models.NodeTypeDesktop)

	job := &models.Job{
		ID:       "test-gst-2",
		Scenario: "gstreamer-hw-stream",
	}

	args, err := engine.BuildCommand(job, "http://localhost:8080")
	if err != nil {
		t.Fatalf("BuildCommand() error = %v", err)
	}

	argsStr := strings.Join(args, " ")
	
	// Should use NVENC encoder due to GPU capabilities
	if !strings.Contains(argsStr, "nvh264enc") && !strings.Contains(argsStr, "nvh265enc") {
		t.Error("Command should use NVENC hardware encoder (nvh264enc or nvh265enc) for NVIDIA GPU")
	}
}

func TestGStreamerEngine_SelectVideoEncoder(t *testing.T) {
	tests := []struct {
		name             string
		caps             *models.NodeCapabilities
		expectedContains string
	}{
		{
			name: "NVIDIA GPU",
			caps: &models.NodeCapabilities{
				HasGPU:          true,
				GPUType:         "NVIDIA GeForce RTX 3080",
				GPUCapabilities: []string{"nvenc_h264"},
			},
			expectedContains: "nvh264enc",
		},
		{
			name: "Intel GPU",
			caps: &models.NodeCapabilities{
				HasGPU:  true,
				GPUType: "Intel UHD Graphics 630",
			},
			expectedContains: "vaapi",
		},
		{
			name: "No GPU",
			caps: &models.NodeCapabilities{
				HasGPU: false,
			},
			expectedContains: "x264enc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewGStreamerEngine(tt.caps, models.NodeTypeDesktop)
			encoder := engine.selectVideoEncoder()
			
			if !strings.Contains(encoder, tt.expectedContains) {
				t.Errorf("selectVideoEncoder() = %v, want to contain %v", encoder, tt.expectedContains)
			}
		})
	}
}
