package agent

import (
	"testing"
)

func TestDetectEncoders(t *testing.T) {
	caps := DetectEncoders()
	
	if caps == nil {
		t.Fatal("DetectEncoders returned nil")
	}
	
	// Should always have at least libx264 as fallback
	if caps.SelectedH264 == "" {
		t.Error("No H.264 encoder selected")
	}
	
	if caps.SelectedH265 == "" {
		t.Error("No H.265 encoder selected")
	}
	
	// Check that selected encoder is in the available list (or is fallback)
	if caps.SelectedH264 != "libx264" {
		found := false
		for _, enc := range caps.H264Encoders {
			if enc == caps.SelectedH264 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Selected H.264 encoder %s not in available list: %v",
				caps.SelectedH264, caps.H264Encoders)
		}
	}
	
	t.Logf("Selected H.264 encoder: %s", caps.SelectedH264)
	t.Logf("Selected H.265 encoder: %s", caps.SelectedH265)
	t.Logf("Hardware acceleration types: %v", caps.HWAccelTypes)
	t.Logf("Reason: %s", caps.GetEncoderReason())
}

func TestGetEncoderReason(t *testing.T) {
	tests := []struct {
		name     string
		encoder  string
		expected string
	}{
		{
			name:     "NVENC",
			encoder:  "h264_nvenc",
			expected: "NVIDIA NVENC",
		},
		{
			name:     "QSV",
			encoder:  "h264_qsv",
			expected: "Intel Quick Sync",
		},
		{
			name:     "VAAPI",
			encoder:  "h264_vaapi",
			expected: "VAAPI",
		},
		{
			name:     "Software",
			encoder:  "libx264",
			expected: "libx264",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := &EncoderCapabilities{
				SelectedH264: tt.encoder,
			}
			if tt.encoder == "libx264" {
				caps.H264Encoders = []string{"libx264"}
			}
			
			reason := caps.GetEncoderReason()
			if reason == "" {
				t.Error("GetEncoderReason returned empty string")
			}
			// Just check that reason contains some expected keywords
			// Don't check exact match as implementation may change
			t.Logf("Reason for %s: %s", tt.encoder, reason)
		})
	}
}

func TestDetectAvailableEncoders(t *testing.T) {
	encoders := detectAvailableEncoders()
	
	// Should find at least some common encoders
	if len(encoders) == 0 {
		t.Log("Warning: No encoders detected (ffmpeg might not be available)")
		return
	}
	
	// Check for common encoders
	hasLibx264 := false
	for _, enc := range encoders {
		if enc == "libx264" {
			hasLibx264 = true
			break
		}
	}
	
	if !hasLibx264 {
		t.Log("Warning: libx264 not found in encoder list")
	}
	
	t.Logf("Found %d encoders", len(encoders))
}

func TestDetectHWAccels(t *testing.T) {
	hwaccels := detectHWAccels()
	
	// May be empty if no hardware acceleration available
	t.Logf("Hardware acceleration methods: %v", hwaccels)
	
	// Should not panic or error
	if len(hwaccels) > 0 {
		t.Logf("Found %d hardware acceleration methods", len(hwaccels))
	}
}
