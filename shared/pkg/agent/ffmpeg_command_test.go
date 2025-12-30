package agent

import (
	"strings"
	"testing"
)

func TestBuildFFmpegCommand_Basic(t *testing.T) {
	params := &FFmpegParams{
		Encoder:     "libx264",
		Preset:      "fast",
		CRF:         23,
		Threads:     4,
		PixelFormat: "yuv420p",
		ExtraParams: map[string]string{
			"tune": "film",
			"bf":   "3",
			"g":    "60",
		},
	}

	config := RTMPConfig{
		MasterHost: "10.0.1.5",
		StreamKey:  "test-stream-123",
		InputFile:  "/path/to/input.mp4",
	}

	args, err := BuildFFmpegCommand(params, config)
	if err != nil {
		t.Fatalf("BuildFFmpegCommand failed: %v", err)
	}

	// Verify required elements
	if !containsArg(args, "-re") {
		t.Error("Expected -re flag for streaming")
	}

	if !containsArg(args, "-i") || !containsArg(args, "/path/to/input.mp4") {
		t.Error("Expected input file specification")
	}

	if !containsArg(args, "-c:v") || !containsArg(args, "libx264") {
		t.Error("Expected encoder specification")
	}

	if !containsArg(args, "-preset") || !containsArg(args, "fast") {
		t.Error("Expected preset specification")
	}

	if !containsArg(args, "-crf") || !containsArg(args, "23") {
		t.Error("Expected CRF specification")
	}

	if !containsArg(args, "-pix_fmt") || !containsArg(args, "yuv420p") {
		t.Error("Expected pixel format specification")
	}

	// Verify RTMP URL
	expectedRTMP := "rtmp://10.0.1.5:1935/live/test-stream-123"
	if !containsArg(args, expectedRTMP) {
		t.Errorf("Expected RTMP URL %s in command", expectedRTMP)
	}

	// Verify FLV format for RTMP
	if !containsArg(args, "-f") || !containsArg(args, "flv") {
		t.Error("Expected FLV format for RTMP streaming")
	}
}

func TestBuildFFmpegCommand_NVENC(t *testing.T) {
	params := &FFmpegParams{
		Encoder:     "h264_nvenc",
		Preset:      "p4",
		CRF:         23,
		Threads:     4,
		PixelFormat: "yuv420p",
		ExtraParams: map[string]string{
			"rc":          "vbr",
			"spatial-aq":  "1",
			"temporal-aq": "1",
			"bf":          "3",
			"zerolatency": "1",
			"g":           "60",
		},
	}

	config := RTMPConfig{
		MasterHost: "master.example.com",
		StreamKey:  "nvenc-stream",
		InputFile:  "/videos/test.mp4",
	}

	args, err := BuildFFmpegCommand(params, config)
	if err != nil {
		t.Fatalf("BuildFFmpegCommand failed: %v", err)
	}

	// Verify encoder
	if !containsArg(args, "h264_nvenc") {
		t.Error("Expected h264_nvenc encoder")
	}

	// Verify NVENC-specific flags are included
	if !containsArg(args, "-rc") || !containsArg(args, "vbr") {
		t.Error("Expected NVENC rate control flag")
	}

	if !containsArg(args, "-spatial-aq") || !containsArg(args, "1") {
		t.Error("Expected spatial AQ flag for NVENC")
	}

	if !containsArg(args, "-temporal-aq") {
		t.Error("Expected temporal AQ flag for NVENC")
	}

	if !containsArg(args, "-zerolatency") {
		t.Error("Expected zerolatency flag for NVENC")
	}

	// Verify tune is NOT included (not supported by NVENC)
	if containsArg(args, "-tune") {
		t.Error("Did not expect tune flag for NVENC encoder")
	}

	// Verify RTMP URL
	expectedRTMP := "rtmp://master.example.com:1935/live/nvenc-stream"
	if !containsArg(args, expectedRTMP) {
		t.Errorf("Expected RTMP URL %s", expectedRTMP)
	}
}

func TestBuildFFmpegCommand_HEVC(t *testing.T) {
	params := &FFmpegParams{
		Encoder:     "libx265",
		Preset:      "medium",
		CRF:         28,
		Threads:     12,
		PixelFormat: "yuv420p10le",
		ExtraParams: map[string]string{
			"aq-mode":  "3",
			"bframes":  "4",
			"rd":       "6",
			"no-sao":   "1",
			"g":        "120",
		},
	}

	config := RTMPConfig{
		MasterHost: "192.168.1.100",
		StreamKey:  "hevc-hdr-stream",
		InputFile:  "/content/4k-hdr.mp4",
	}

	args, err := BuildFFmpegCommand(params, config)
	if err != nil {
		t.Fatalf("BuildFFmpegCommand failed: %v", err)
	}

	// Verify encoder
	if !containsArg(args, "libx265") {
		t.Error("Expected libx265 encoder")
	}

	// Verify 10-bit pixel format
	if !containsArg(args, "yuv420p10le") {
		t.Error("Expected 10-bit pixel format for HDR")
	}

	// Verify x265-params are used for AQ mode
	hasX265Params := false
	for _, arg := range args {
		if strings.Contains(arg, "aq-mode") || strings.Contains(arg, "x265-params") {
			hasX265Params = true
			break
		}
	}
	if !hasX265Params {
		t.Error("Expected x265-params for AQ mode")
	}

	// Verify RTMP URL with IP address
	expectedRTMP := "rtmp://192.168.1.100:1935/live/hevc-hdr-stream"
	if !containsArg(args, expectedRTMP) {
		t.Errorf("Expected RTMP URL %s", expectedRTMP)
	}
}

func TestBuildFFmpegCommand_MissingMasterHost(t *testing.T) {
	params := &FFmpegParams{
		Encoder: "libx264",
		Preset:  "fast",
		CRF:     23,
	}

	config := RTMPConfig{
		MasterHost: "", // Missing!
		StreamKey:  "test-stream",
		InputFile:  "/input.mp4",
	}

	_, err := BuildFFmpegCommand(params, config)
	if err == nil {
		t.Error("Expected error for missing master host")
	}

	if !strings.Contains(err.Error(), "master host is required") {
		t.Errorf("Expected error about master host, got: %v", err)
	}
}

func TestBuildFFmpegCommand_MissingStreamKey(t *testing.T) {
	params := &FFmpegParams{
		Encoder: "libx264",
		Preset:  "fast",
		CRF:     23,
	}

	config := RTMPConfig{
		MasterHost: "10.0.1.5",
		StreamKey:  "", // Missing!
		InputFile:  "/input.mp4",
	}

	_, err := BuildFFmpegCommand(params, config)
	if err == nil {
		t.Error("Expected error for missing stream key")
	}

	if !strings.Contains(err.Error(), "stream key is required") {
		t.Errorf("Expected error about stream key, got: %v", err)
	}
}

func TestBuildFFmpegCommand_MissingInputFile(t *testing.T) {
	params := &FFmpegParams{
		Encoder: "libx264",
		Preset:  "fast",
		CRF:     23,
	}

	config := RTMPConfig{
		MasterHost: "10.0.1.5",
		StreamKey:  "test-stream",
		InputFile:  "", // Missing!
	}

	_, err := BuildFFmpegCommand(params, config)
	if err == nil {
		t.Error("Expected error for missing input file")
	}

	if !strings.Contains(err.Error(), "input file is required") {
		t.Errorf("Expected error about input file, got: %v", err)
	}
}

func TestValidateCRFForEncoder(t *testing.T) {
	tests := []struct {
		name      string
		encoder   string
		crf       int
		expectErr bool
	}{
		{"libx264 valid low", "libx264", 18, false},
		{"libx264 valid mid", "libx264", 23, false},
		{"libx264 valid high", "libx264", 28, false},
		{"libx264 min", "libx264", 0, false},
		{"libx264 max", "libx264", 51, false},
		{"libx264 too high", "libx264", 52, true},
		{"libx264 negative", "libx264", -1, true},

		{"libx265 valid", "libx265", 28, false},
		{"libx265 max", "libx265", 51, false},
		{"libx265 too high", "libx265", 52, true},

		{"h264_nvenc valid", "h264_nvenc", 23, false},
		{"h264_nvenc out of range", "h264_nvenc", 55, true},

		{"hevc_nvenc valid", "hevc_nvenc", 28, false},
		{"hevc_nvenc out of range", "hevc_nvenc", -5, true},

		{"h264_qsv valid", "h264_qsv", 25, false},
		{"h264_qsv out of range", "h264_qsv", 60, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCRFForEncoder(tt.encoder, tt.crf)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for encoder %s with CRF %d", tt.encoder, tt.crf)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Did not expect error for encoder %s with CRF %d: %v", tt.encoder, tt.crf, err)
			}
		})
	}
}

func TestGetRTMPURLFromMasterURL(t *testing.T) {
	tests := []struct {
		name        string
		masterURL   string
		expected    string
		expectError bool
	}{
		{
			name:        "HTTPS with port",
			masterURL:   "https://10.0.1.5:8080",
			expected:    "10.0.1.5",
			expectError: false,
		},
		{
			name:        "HTTP with port",
			masterURL:   "http://master.local:8080",
			expected:    "master.local",
			expectError: false,
		},
		{
			name:        "HTTPS with path",
			masterURL:   "https://master.example.com:443/api",
			expected:    "master.example.com",
			expectError: false,
		},
		{
			name:        "Hostname only",
			masterURL:   "master-node.local",
			expected:    "master-node.local",
			expectError: false,
		},
		{
			name:        "Hostname with port",
			masterURL:   "master-node:8080",
			expected:    "master-node",
			expectError: false,
		},
		{
			name:        "IP address",
			masterURL:   "192.168.1.100",
			expected:    "192.168.1.100",
			expectError: false,
		},
		{
			name:        "IP with port",
			masterURL:   "192.168.1.100:8080",
			expected:    "192.168.1.100",
			expectError: false,
		},
		{
			name:        "Empty string",
			masterURL:   "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Localhost rejected",
			masterURL:   "localhost",
			expected:    "",
			expectError: true,
		},
		{
			name:        "127.0.0.1 rejected",
			masterURL:   "127.0.0.1",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Localhost URL rejected",
			masterURL:   "https://localhost:8080",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetRTMPURLFromMasterURL(tt.masterURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %s", tt.masterURL)
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect error for input %s: %v", tt.masterURL, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestBuildFFmpegCommand_CompleteIntegration(t *testing.T) {
	// Simulate a complete workflow: hardware detection -> optimization -> command generation
	
	// Step 1: Define worker capabilities (from exporters)
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-3080",
		HasNVENC: true,
		MemoryGB: 16,
	}

	// Step 2: Define content properties
	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "high",
		GrainLevel:  "low",
		Framerate:   60,
		IsHDR:       false,
		ColorSpace:  "bt709",
		PixelFormat: "yuv420p",
	}

	// Step 3: Calculate optimal parameters
	params := CalculateOptimalFFmpegParams(worker, content, GoalBalanced)

	// Verify params were generated
	if params.Encoder == "" {
		t.Fatal("Encoder not set by CalculateOptimalFFmpegParams")
	}

	// Step 4: Build FFmpeg command
	config := RTMPConfig{
		MasterHost: "10.0.1.5",
		StreamKey:  "integration-test-stream",
		InputFile:  "/videos/test-high-motion.mp4",
	}

	args, err := BuildFFmpegCommand(params, config)
	if err != nil {
		t.Fatalf("Failed to build FFmpeg command: %v", err)
	}

	// Verify complete command structure
	if len(args) == 0 {
		t.Fatal("Generated command is empty")
	}

	// Verify key components are present
	if !containsArg(args, "-re") {
		t.Error("Missing -re flag")
	}

	if !containsArg(args, "-i") {
		t.Error("Missing input specification")
	}

	if !containsArg(args, "-c:v") {
		t.Error("Missing codec specification")
	}

	if !containsArg(args, "rtmp://10.0.1.5:1935/live/integration-test-stream") {
		t.Error("Missing or incorrect RTMP URL")
	}

	// Verify reasoning was generated
	if len(params.Reasoning) == 0 {
		t.Error("No reasoning provided for optimization decisions")
	}

	// Print command for manual verification (useful for debugging)
	t.Logf("Generated command: ffmpeg %s", strings.Join(args, " "))
	t.Logf("Reasoning: %v", params.Reasoning)
}

func TestBuildFFmpegCommand_HDR_10bit(t *testing.T) {
	// Test complete workflow with HDR content
	worker := WorkerCapabilities{
		CPUCores: 16,
		GPUType:  "nvidia-rtx-4090",
		HasNVENC: true,
		MemoryGB: 32,
	}

	content := ContentProperties{
		Resolution:  "3840x2160",
		MotionLevel: "medium",
		GrainLevel:  "low",
		Framerate:   24,
		IsHDR:       true,
		ColorSpace:  "bt2020",
		PixelFormat: "yuv420p10le",
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalQuality)

	// Verify 10-bit pixel format was selected
	if params.PixelFormat != "yuv420p10le" {
		t.Errorf("Expected 10-bit pixel format for HDR, got %s", params.PixelFormat)
	}

	config := RTMPConfig{
		MasterHost: "master.local",
		StreamKey:  "hdr-4k-stream",
		InputFile:  "/content/hdr-demo.mp4",
	}

	args, err := BuildFFmpegCommand(params, config)
	if err != nil {
		t.Fatalf("Failed to build HDR command: %v", err)
	}

	// Verify 10-bit pixel format in command
	if !containsArg(args, "yuv420p10le") {
		t.Error("10-bit pixel format not in command")
	}

	// Check reasoning mentions HDR
	hasHDRReasoning := false
	for _, reason := range params.Reasoning {
		if strings.Contains(strings.ToLower(reason), "hdr") ||
			strings.Contains(strings.ToLower(reason), "10-bit") {
			hasHDRReasoning = true
			break
		}
	}
	if !hasHDRReasoning {
		t.Error("Reasoning should mention HDR or 10-bit encoding")
	}
}

// Helper function to check if an argument exists in the command
func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}
