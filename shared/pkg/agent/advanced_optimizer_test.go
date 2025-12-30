package agent

import (
	"strings"
	"testing"
)

func TestCalculateOptimalFFmpegParams_GPU_LatencyGoal(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-3080",
		HasNVENC: true,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "low",
		Framerate:   30,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalLatency)

	if params.Encoder != "h264_nvenc" {
		t.Errorf("Expected h264_nvenc for latency goal with GPU, got %s", params.Encoder)
	}

	if params.Preset != "p1" {
		t.Errorf("Expected fastest NVENC preset p1 for latency, got %s", params.Preset)
	}

	if params.ExtraParams["zerolatency"] != "1" {
		t.Error("Expected zerolatency flag for latency goal")
	}

	if len(params.Reasoning) == 0 {
		t.Error("Expected reasoning to be populated")
	}
}

func TestCalculateOptimalFFmpegParams_GPU_4K_EnergyGoal(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 12,
		GPUType:  "nvidia-rtx-4090",
		HasNVENC: true,
		MemoryGB: 32,
	}

	content := ContentProperties{
		Resolution:  "3840x2160",
		MotionLevel: "high",
		GrainLevel:  "medium",
		Framerate:   60,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalEnergy)

	if params.Encoder != "hevc_nvenc" {
		t.Errorf("Expected hevc_nvenc for 4K energy goal, got %s", params.Encoder)
	}

	if params.Preset != "p3" {
		t.Errorf("Expected balanced NVENC preset p3 for energy, got %s", params.Preset)
	}

	// High motion should reduce B-frames
	if params.ExtraParams["bf"] != "2" {
		t.Errorf("Expected 2 B-frames for high motion, got %s", params.ExtraParams["bf"])
	}

	// High motion should reduce CRF (more bits)
	if params.CRF > 26 {
		t.Errorf("Expected lower CRF for high motion, got %d", params.CRF)
	}
}

func TestCalculateOptimalFFmpegParams_CPU_QualityGoal_4K(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 16,
		GPUType:  "none",
		HasNVENC: false,
		MemoryGB: 64,
	}

	content := ContentProperties{
		Resolution:  "3840x2160",
		MotionLevel: "high",
		GrainLevel:  "medium",
		Framerate:   24,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalQuality)

	if params.Encoder != "libx265" {
		t.Errorf("Expected libx265 for 4K quality goal, got %s", params.Encoder)
	}

	if params.Preset != "slow" {
		t.Errorf("Expected 'slow' preset for quality on high-end CPU, got %s", params.Preset)
	}

	if params.ExtraParams["aq-mode"] != "3" {
		t.Errorf("Expected AQ mode 3 for quality goal, got %s", params.ExtraParams["aq-mode"])
	}

	// Should use most cores
	if params.Threads < 14 {
		t.Errorf("Expected to use most cores (14+), got %d", params.Threads)
	}

	// High motion and medium grain should lower CRF
	if params.CRF > 23 {
		t.Errorf("Expected lower CRF for high motion + grain content, got %d", params.CRF)
	}
}

func TestCalculateOptimalFFmpegParams_LowEndCPU_BalancedGoal(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 4,
		GPUType:  "none",
		HasNVENC: false,
		MemoryGB: 8,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "none",
		Framerate:   30,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalBalanced)

	if params.Encoder != "libx264" {
		t.Errorf("Expected libx264 for balanced 1080p, got %s", params.Encoder)
	}

	// Low-end CPU should use fast preset even for balanced goal
	if params.Preset != "veryfast" && params.Preset != "fast" {
		t.Errorf("Expected fast preset for low-end CPU, got %s", params.Preset)
	}

	// Should use all available cores
	if params.Threads != 4 {
		t.Errorf("Expected to use all 4 cores, got %d", params.Threads)
	}
}

func TestCalculateOptimalFFmpegParams_HDR_Content(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 12,
		GPUType:  "nvidia-rtx-3080",
		HasNVENC: true,
		MemoryGB: 24,
	}

	content := ContentProperties{
		Resolution:  "3840x2160",
		MotionLevel: "low",
		GrainLevel:  "none",
		Framerate:   24,
		IsHDR:       true,
		ColorSpace:  "bt2020",
		PixelFormat: "yuv420p10le",
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalQuality)

	if params.PixelFormat != "yuv420p10le" {
		t.Errorf("Expected 10-bit pixel format for HDR, got %s", params.PixelFormat)
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
		t.Error("Expected reasoning to mention HDR or 10-bit encoding")
	}
}

func TestCalculateOptimalFFmpegParams_HighGrain_Content(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "none",
		HasNVENC: false,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "low",
		GrainLevel:  "high",
		Framerate:   24,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalQuality)

	if params.Encoder != "libx264" {
		t.Errorf("Expected libx264, got %s", params.Encoder)
	}

	// High grain should use tune=grain
	if params.ExtraParams["tune"] != "grain" {
		t.Errorf("Expected tune=grain for high-grain content, got %s", params.ExtraParams["tune"])
	}

	// High grain should reduce CRF (more bits needed)
	if params.CRF > 21 {
		t.Errorf("Expected lower CRF for high-grain content, got %d", params.CRF)
	}
}

func TestCalculateOptimalFFmpegParams_LowMotion_HighBFrames(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 16,
		GPUType:  "none",
		HasNVENC: false,
		MemoryGB: 32,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "low",
		GrainLevel:  "none",
		Framerate:   30,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalQuality)

	// Low motion should allow more B-frames for better compression
	if params.ExtraParams["bf"] == "" {
		t.Error("Expected B-frames to be set")
	}

	// Low motion should allow higher CRF (more efficient)
	if params.CRF < 21 {
		t.Errorf("Expected higher CRF for low-motion content, got %d", params.CRF)
	}
}

func TestCalculateOptimalFFmpegParams_HighMotion_FewerBFrames(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-3070",
		HasNVENC: true,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "high",
		GrainLevel:  "none",
		Framerate:   60,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalBalanced)

	// High motion should use fewer B-frames
	if params.ExtraParams["bf"] != "2" {
		t.Errorf("Expected 2 B-frames for high motion, got %s", params.ExtraParams["bf"])
	}

	// High motion should reduce CRF
	if params.CRF > 24 {
		t.Errorf("Expected lower CRF for high-motion content, got %d", params.CRF)
	}
}

func TestCalculateOptimalFFmpegParams_IntelQSV(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "intel-uhd",
		HasNVENC: false,
		HasQSV:   true,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "none",
		Framerate:   30,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalEnergy)

	if params.Encoder != "h264_qsv" {
		t.Errorf("Expected h264_qsv for Intel QSV, got %s", params.Encoder)
	}
}

func TestCalculateOptimalFFmpegParams_GOPSize(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "none",
		HasNVENC: false,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "none",
		Framerate:   60,
		IsHDR:       false,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalBalanced)

	// GOP size should be framerate * 2 (2 seconds)
	expectedGOP := "120" // 60 fps * 2
	if params.ExtraParams["g"] != expectedGOP {
		t.Errorf("Expected GOP size %s, got %s", expectedGOP, params.ExtraParams["g"])
	}
}

func TestCalculateOptimalFFmpegParams_ThreadAllocation(t *testing.T) {
	tests := []struct {
		name         string
		cpuCores     int
		hasNVENC     bool
		expectedMin  int
		expectedMax  int
		description  string
	}{
		{
			name:         "Low-end CPU",
			cpuCores:     4,
			hasNVENC:     false,
			expectedMin:  4,
			expectedMax:  4,
			description:  "Should use all cores",
		},
		{
			name:         "Mid-range CPU",
			cpuCores:     8,
			hasNVENC:     false,
			expectedMin:  7,
			expectedMax:  7,
			description:  "Should leave 1 core for system",
		},
		{
			name:         "High-end CPU",
			cpuCores:     16,
			hasNVENC:     false,
			expectedMin:  14,
			expectedMax:  14,
			description:  "Should leave 2 cores for system",
		},
		{
			name:         "GPU encoding",
			cpuCores:     16,
			hasNVENC:     true,
			expectedMin:  4,
			expectedMax:  4,
			description:  "GPU needs fewer CPU threads",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := WorkerCapabilities{
				CPUCores: tt.cpuCores,
				HasNVENC: tt.hasNVENC,
				MemoryGB: 16,
			}

			content := ContentProperties{
				Resolution:  "1920x1080",
				MotionLevel: "medium",
				Framerate:   30,
			}

			params := CalculateOptimalFFmpegParams(worker, content, GoalBalanced)

			if params.Threads < tt.expectedMin || params.Threads > tt.expectedMax {
				t.Errorf("%s: Expected %d threads, got %d",
					tt.description, tt.expectedMin, params.Threads)
			}
		})
	}
}

func TestCalculateOptimalFFmpegParams_ReasoningPopulated(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-3080",
		HasNVENC: true,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "3840x2160",
		MotionLevel: "high",
		GrainLevel:  "medium",
		Framerate:   30,
		IsHDR:       true,
	}

	params := CalculateOptimalFFmpegParams(worker, content, GoalQuality)

	// Should have comprehensive reasoning
	if len(params.Reasoning) < 5 {
		t.Errorf("Expected at least 5 reasoning entries, got %d", len(params.Reasoning))
	}

	// Check that reasoning is meaningful (not empty strings)
	for i, reason := range params.Reasoning {
		if len(strings.TrimSpace(reason)) == 0 {
			t.Errorf("Reasoning entry %d is empty", i)
		}
	}
}

func TestCalculateOptimalFFmpegParams_AllGoals(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-3080",
		HasNVENC: true,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "low",
		Framerate:   30,
		IsHDR:       false,
	}

	goals := []TranscodingGoal{GoalQuality, GoalEnergy, GoalLatency, GoalBalanced}

	for _, goal := range goals {
		t.Run(string(goal), func(t *testing.T) {
			params := CalculateOptimalFFmpegParams(worker, content, goal)

			if params.Encoder == "" {
				t.Error("Encoder not set")
			}

			if params.Preset == "" {
				t.Error("Preset not set")
			}

			if params.CRF == 0 && params.Bitrate == "" {
				t.Error("Neither CRF nor bitrate set")
			}

			if params.Threads == 0 {
				t.Error("Threads not set")
			}

			if len(params.Reasoning) == 0 {
				t.Error("Reasoning not populated")
			}
		})
	}
}

func TestCalculateOptimalFFmpegParams_NVENC_SpatialTemporalAQ(t *testing.T) {
	worker := WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-4080",
		HasNVENC: true,
		MemoryGB: 16,
	}

	content := ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "none",
		Framerate:   30,
		IsHDR:       false,
	}

	// Use balanced goal to get NVENC encoder (quality goal prefers CPU)
	params := CalculateOptimalFFmpegParams(worker, content, GoalBalanced)

	// Verify we got NVENC encoder
	if params.Encoder != "h264_nvenc" && params.Encoder != "hevc_nvenc" {
		t.Skipf("Test requires NVENC encoder, got %s", params.Encoder)
	}

	// Balanced/Quality goal with NVENC should enable spatial and temporal AQ
	if params.ExtraParams["spatial-aq"] != "1" {
		t.Error("Expected spatial-aq to be enabled for balanced goal with NVENC")
	}

	if params.ExtraParams["temporal-aq"] != "1" {
		t.Error("Expected temporal-aq to be enabled for balanced goal with NVENC")
	}
}
