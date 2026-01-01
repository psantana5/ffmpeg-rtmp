package agent

import (
	"testing"
)

func TestIsEncoderUsable(t *testing.T) {
	tests := []struct {
		name           string
		encoder        string
		expectUsable   bool // Set based on what we know about the system
		checkReason    bool // Whether to check for failure reason
	}{
		{
			name:         "libx264 always usable",
			encoder:      "libx264",
			expectUsable: true,
			checkReason:  false,
		},
		{
			name:         "h264_nvenc requires CUDA",
			encoder:      "h264_nvenc",
			expectUsable: false, // Likely fails without CUDA runtime
			checkReason:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usable, reason := isEncoderUsable(tt.encoder)
			
			t.Logf("Encoder %s: usable=%v, reason=%s", tt.encoder, usable, reason)
			
			if tt.checkReason && !usable && reason == "" {
				t.Error("Expected failure reason but got empty string")
			}
			
			if usable && reason != "" {
				t.Errorf("Encoder marked as usable but has reason: %s", reason)
			}
		})
	}
}

func TestIsNVENCAvailable(t *testing.T) {
	available := IsNVENCAvailable()
	t.Logf("NVENC available: %v", available)
	
	// This test just logs the result - actual availability depends on system
	// In CI without GPU, this should be false
}

func TestIsQSVAvailable(t *testing.T) {
	available := IsQSVAvailable()
	t.Logf("QSV available: %v", available)
}

func TestIsVAAPIAvailable(t *testing.T) {
	available := IsVAAPIAvailable()
	t.Logf("VAAPI available: %v", available)
}

func TestDetectEncodersWithValidation(t *testing.T) {
	caps := DetectEncoders()
	
	if caps == nil {
		t.Fatal("DetectEncoders returned nil")
	}
	
	// Should always have at least libx264
	if caps.SelectedH264 == "" {
		t.Error("No H.264 encoder selected")
	}
	
	// Check that validation results exist
	if caps.ValidationResults == nil {
		t.Error("ValidationResults map is nil")
	}
	
	// Check that selected encoder is marked as usable
	if !caps.ValidationResults[caps.SelectedH264] {
		t.Errorf("Selected encoder %s is not marked as usable", caps.SelectedH264)
	}
	
	// Log results
	t.Logf("Selected H.264: %s", caps.SelectedH264)
	t.Logf("Selected H.265: %s", caps.SelectedH265)
	t.Logf("Compile-time H.264 encoders: %v", caps.H264Encoders)
	t.Logf("Runtime-validated H.264 encoders: %v", caps.H264EncodersUsable)
	
	// Log validation failures
	for encoder, usable := range caps.ValidationResults {
		if !usable {
			reason := caps.ValidationReasons[encoder]
			t.Logf("Encoder %s validation failed: %s", encoder, reason)
		}
	}
	
	// Check that GetEncoderReason works
	reason := caps.GetEncoderReason()
	if reason == "" {
		t.Error("GetEncoderReason returned empty string")
	}
	t.Logf("Encoder reason: %s", reason)
}

func TestEncoderFallback(t *testing.T) {
	caps := DetectEncoders()
	
	// If NVENC is detected but not usable, should fall back
	if contains(caps.H264Encoders, "h264_nvenc") && !caps.ValidationResults["h264_nvenc"] {
		t.Logf("NVENC detected but not usable - testing fallback")
		
		// Should have fallen back to another encoder
		if caps.SelectedH264 == "h264_nvenc" {
			t.Error("Selected encoder is NVENC despite validation failure")
		}
		
		// Should have a reason
		reason := caps.ValidationReasons["h264_nvenc"]
		if reason == "" {
			t.Error("NVENC validation failed but no reason provided")
		} else {
			t.Logf("NVENC failure reason: %s", reason)
		}
	}
}

func TestValidationReasonMessages(t *testing.T) {
	caps := DetectEncoders()
	
	// Check that all failed validations have reasons
	for encoder, usable := range caps.ValidationResults {
		if !usable && encoder != "libx264" && encoder != "libx265" {
			reason := caps.ValidationReasons[encoder]
			if reason == "" {
				t.Errorf("Encoder %s failed validation but has no reason", encoder)
			} else {
				t.Logf("Encoder %s: %s", encoder, reason)
			}
		}
	}
}
