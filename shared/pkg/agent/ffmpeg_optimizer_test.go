package agent

import (
	"testing"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func TestOptimizeFFmpegParameters_WithGPU(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 8,
		CPUModel:   "Intel Core i7",
		HasGPU:     true,
		GPUType:    "NVIDIA GeForce RTX 3080",
		RAMTotalBytes:   16 * 1024 * 1024 * 1024, // 16GB
		Labels:     make(map[string]string),
	}

	opt := OptimizeFFmpegParameters(caps, models.NodeTypeDesktop)

	if opt.Encoder != "h264_nvenc" {
		t.Errorf("Expected encoder 'h264_nvenc', got '%s'", opt.Encoder)
	}

	if opt.HWAccel != "nvenc" {
		t.Errorf("Expected hwaccel 'nvenc', got '%s'", opt.HWAccel)
	}

	if opt.Preset != "medium" {
		t.Errorf("Expected preset 'medium' for NVENC, got '%s'", opt.Preset)
	}

	if opt.Reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestOptimizeFFmpegParameters_HighEndCPU(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 24,
		CPUModel:   "AMD Ryzen 9 5950X",
		HasGPU:     false,
		RAMTotalBytes:   64 * 1024 * 1024 * 1024, // 64GB
		Labels:     make(map[string]string),
	}

	opt := OptimizeFFmpegParameters(caps, models.NodeTypeServer)

	if opt.Encoder != "libx264" {
		t.Errorf("Expected encoder 'libx264', got '%s'", opt.Encoder)
	}

	if opt.Preset != "fast" {
		t.Errorf("Expected preset 'fast' for high-end CPU, got '%s'", opt.Preset)
	}

	if opt.HWAccel != "none" {
		t.Errorf("Expected hwaccel 'none' without GPU, got '%s'", opt.HWAccel)
	}
}

func TestOptimizeFFmpegParameters_MidRangeCPU(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 10,
		CPUModel:   "Intel Core i5",
		HasGPU:     false,
		RAMTotalBytes:   16 * 1024 * 1024 * 1024, // 16GB
		Labels:     make(map[string]string),
	}

	opt := OptimizeFFmpegParameters(caps, models.NodeTypeDesktop)

	if opt.Encoder != "libx264" {
		t.Errorf("Expected encoder 'libx264', got '%s'", opt.Encoder)
	}

	if opt.Preset != "fast" {
		t.Errorf("Expected preset 'fast' for mid-range CPU, got '%s'", opt.Preset)
	}
}

func TestOptimizeFFmpegParameters_LowEndCPU(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 4,
		CPUModel:   "Intel Core i3",
		HasGPU:     false,
		RAMTotalBytes:   8 * 1024 * 1024 * 1024, // 8GB
		Labels:     make(map[string]string),
	}

	opt := OptimizeFFmpegParameters(caps, models.NodeTypeDesktop)

	if opt.Encoder != "libx264" {
		t.Errorf("Expected encoder 'libx264', got '%s'", opt.Encoder)
	}

	if opt.Preset != "veryfast" {
		t.Errorf("Expected preset 'veryfast' for low-end CPU, got '%s'", opt.Preset)
	}
}

func TestOptimizeFFmpegParameters_VeryLowEndCPU(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 2,
		CPUModel:   "Intel Celeron",
		HasGPU:     false,
		RAMTotalBytes:   4 * 1024 * 1024 * 1024, // 4GB
		Labels:     make(map[string]string),
	}

	opt := OptimizeFFmpegParameters(caps, models.NodeTypeDesktop)

	if opt.Encoder != "libx264" {
		t.Errorf("Expected encoder 'libx264', got '%s'", opt.Encoder)
	}

	if opt.Preset != "ultrafast" {
		t.Errorf("Expected preset 'ultrafast' for very low-end CPU, got '%s'", opt.Preset)
	}
}

func TestOptimizeFFmpegParameters_Laptop(t *testing.T) {
	caps := &models.NodeCapabilities{
		CPUThreads: 8,
		CPUModel:   "Intel Core i7 Mobile",
		HasGPU:     false,
		RAMTotalBytes:   16 * 1024 * 1024 * 1024, // 16GB
		Labels:     make(map[string]string),
	}

	opt := OptimizeFFmpegParameters(caps, models.NodeTypeLaptop)

	// Should use faster preset for thermal/battery efficiency
	if opt.Preset != "veryfast" {
		t.Errorf("Expected preset 'veryfast' for laptop, got '%s'", opt.Preset)
	}

	// Check reason mentions laptop optimization
	if opt.Reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestApplyOptimizationToParameters_EmptyParams(t *testing.T) {
	opt := &FFmpegOptimization{
		Encoder:    "h264_nvenc",
		Preset:     "medium",
		HWAccel:    "nvenc",
		ExtraFlags: map[string]string{"rc": "vbr"},
	}

	params := make(map[string]interface{})
	result := ApplyOptimizationToParameters(params, opt)

	if result["codec"] != "h264_nvenc" {
		t.Errorf("Expected codec 'h264_nvenc', got '%v'", result["codec"])
	}

	if result["preset"] != "medium" {
		t.Errorf("Expected preset 'medium', got '%v'", result["preset"])
	}

	if result["hwaccel"] != "nvenc" {
		t.Errorf("Expected hwaccel 'nvenc', got '%v'", result["hwaccel"])
	}

	if result["rc"] != "vbr" {
		t.Errorf("Expected rc 'vbr', got '%v'", result["rc"])
	}
}

func TestApplyOptimizationToParameters_WithExistingParams(t *testing.T) {
	opt := &FFmpegOptimization{
		Encoder:    "h264_nvenc",
		Preset:     "medium",
		HWAccel:    "nvenc",
		ExtraFlags: map[string]string{"rc": "vbr"},
	}

	// User specified custom codec and preset - should not be overridden
	params := map[string]interface{}{
		"codec":  "libx265",
		"preset": "slow",
	}

	result := ApplyOptimizationToParameters(params, opt)

	// User parameters should take precedence
	if result["codec"] != "libx265" {
		t.Errorf("Expected user codec 'libx265' to be preserved, got '%v'", result["codec"])
	}

	if result["preset"] != "slow" {
		t.Errorf("Expected user preset 'slow' to be preserved, got '%v'", result["preset"])
	}

	// Optimization parameters not specified by user should still be applied
	if result["hwaccel"] != "nvenc" {
		t.Errorf("Expected hwaccel 'nvenc', got '%v'", result["hwaccel"])
	}
}

func TestApplyOptimizationToParameters_NilParams(t *testing.T) {
	opt := &FFmpegOptimization{
		Encoder:    "libx264",
		Preset:     "fast",
		HWAccel:    "none",
		ExtraFlags: make(map[string]string),
	}

	result := ApplyOptimizationToParameters(nil, opt)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result["codec"] != "libx264" {
		t.Errorf("Expected codec 'libx264', got '%v'", result["codec"])
	}
}
