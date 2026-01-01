package agent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// EncoderCapabilities holds information about available encoders
type EncoderCapabilities struct {
	H264Encoders        []string          // Available H.264 encoders in priority order (compile-time)
	H265Encoders        []string          // Available H.265 encoders (compile-time)
	H264EncodersUsable  []string          // Runtime-validated usable H.264 encoders
	H265EncodersUsable  []string          // Runtime-validated usable H.265 encoders
	SelectedH264        string            // Best H.264 encoder for this system (runtime-validated)
	SelectedH265        string            // Best H.265 encoder for this system (runtime-validated)
	HWAccelTypes        []string          // Available hardware acceleration types
	ValidationResults   map[string]bool   // Encoder validation results (true = usable)
	ValidationReasons   map[string]string // Reasons for validation failures
}

// DetectEncoders detects available FFmpeg encoders and hardware acceleration
// with runtime validation to ensure encoders are actually usable
func DetectEncoders() *EncoderCapabilities {
	caps := &EncoderCapabilities{
		H264Encoders:       []string{},
		H265Encoders:       []string{},
		H264EncodersUsable: []string{},
		H265EncodersUsable: []string{},
		HWAccelTypes:       []string{},
		ValidationResults:  make(map[string]bool),
		ValidationReasons:  make(map[string]string),
	}

	log.Println("=== Encoder Detection Phase ===")
	
	// Detect hardware acceleration types
	caps.HWAccelTypes = detectHWAccels()
	if len(caps.HWAccelTypes) > 0 {
		log.Printf("Hardware acceleration support (compile-time): %v", caps.HWAccelTypes)
	}

	// Detect available encoders (compile-time check)
	availableEncoders := detectAvailableEncoders()
	log.Printf("Encoders available (compile-time): %d found", len(availableEncoders))

	// Check for H.264 encoders in priority order
	h264Priority := []string{"h264_nvenc", "h264_qsv", "h264_vaapi", "libx264"}
	log.Println("=== H.264 Encoder Detection ===")
	for _, encoder := range h264Priority {
		if contains(availableEncoders, encoder) {
			log.Printf("✓ %s: detected (compile-time)", encoder)
			caps.H264Encoders = append(caps.H264Encoders, encoder)
			
			// Runtime validation for hardware encoders
			if encoder != "libx264" {
				log.Printf("  → Running runtime validation for %s...", encoder)
				usable, reason := isEncoderUsable(encoder)
				caps.ValidationResults[encoder] = usable
				if usable {
					log.Printf("  ✓ %s: USABLE (runtime-validated)", encoder)
					caps.H264EncodersUsable = append(caps.H264EncodersUsable, encoder)
					if caps.SelectedH264 == "" {
						caps.SelectedH264 = encoder
					}
				} else {
					log.Printf("  ✗ %s: NOT USABLE - %s", encoder, reason)
					caps.ValidationReasons[encoder] = reason
				}
			} else {
				// libx264 always usable (software encoder)
				caps.ValidationResults[encoder] = true
				caps.H264EncodersUsable = append(caps.H264EncodersUsable, encoder)
				if caps.SelectedH264 == "" {
					caps.SelectedH264 = encoder
				}
			}
		}
	}

	// Fallback to libx264 if no hardware encoders are usable
	if caps.SelectedH264 == "" {
		log.Println("  → No hardware encoders validated, using libx264")
		caps.SelectedH264 = "libx264"
		if !contains(caps.H264EncodersUsable, "libx264") {
			caps.H264EncodersUsable = append(caps.H264EncodersUsable, "libx264")
			caps.ValidationResults["libx264"] = true
		}
	}

	// Check for H.265 encoders in priority order
	h265Priority := []string{"hevc_nvenc", "hevc_qsv", "hevc_vaapi", "libx265"}
	log.Println("=== H.265 Encoder Detection ===")
	for _, encoder := range h265Priority {
		if contains(availableEncoders, encoder) {
			log.Printf("✓ %s: detected (compile-time)", encoder)
			caps.H265Encoders = append(caps.H265Encoders, encoder)
			
			// Runtime validation for hardware encoders
			if encoder != "libx265" {
				log.Printf("  → Running runtime validation for %s...", encoder)
				usable, reason := isEncoderUsable(encoder)
				caps.ValidationResults[encoder] = usable
				if usable {
					log.Printf("  ✓ %s: USABLE (runtime-validated)", encoder)
					caps.H265EncodersUsable = append(caps.H265EncodersUsable, encoder)
					if caps.SelectedH265 == "" {
						caps.SelectedH265 = encoder
					}
				} else {
					log.Printf("  ✗ %s: NOT USABLE - %s", encoder, reason)
					caps.ValidationReasons[encoder] = reason
				}
			} else {
				// libx265 always usable (software encoder)
				caps.ValidationResults[encoder] = true
				caps.H265EncodersUsable = append(caps.H265EncodersUsable, encoder)
				if caps.SelectedH265 == "" {
					caps.SelectedH265 = encoder
				}
			}
		}
	}

	// Fallback to libx265 if no hardware encoders are usable
	if caps.SelectedH265 == "" {
		log.Println("  → No hardware encoders validated, using libx265")
		caps.SelectedH265 = "libx265"
		if !contains(caps.H265EncodersUsable, "libx265") {
			caps.H265EncodersUsable = append(caps.H265EncodersUsable, "libx265")
			caps.ValidationResults["libx265"] = true
		}
	}

	log.Println("=== Detection Summary ===")
	log.Printf("Selected H.264 encoder: %s", caps.SelectedH264)
	log.Printf("Selected H.265 encoder: %s", caps.SelectedH265)
	log.Printf("Runtime-validated H.264 encoders: %v", caps.H264EncodersUsable)
	log.Printf("Runtime-validated H.265 encoders: %v", caps.H265EncodersUsable)

	return caps
}

// isEncoderUsable performs a runtime test to verify an encoder is actually usable
func isEncoderUsable(encoder string) (bool, string) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return false, "ffmpeg not found in PATH"
	}

	// Run a mini-encode test (0.1 second video to null output)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{
		"-f", "lavfi",
		"-i", "testsrc2=size=64x64:rate=1", // Tiny test pattern
		"-t", "0.1", // Very short duration
		"-c:v", encoder,
		"-f", "null",
		"-",
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Parse stderr for specific error messages
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "Cannot load libcuda") || strings.Contains(stderrStr, "libcuda.so") {
			return false, "CUDA runtime not available (libcuda.so.1 not found)"
		}
		if strings.Contains(stderrStr, "Cannot load") {
			return false, "required library not available"
		}
		if strings.Contains(stderrStr, "No NVENC capable devices found") {
			return false, "no NVENC-capable GPU found"
		}
		if strings.Contains(stderrStr, "Unknown encoder") {
			return false, "encoder not recognized by FFmpeg"
		}
		if ctx.Err() == context.DeadlineExceeded {
			return false, "validation timeout (encoder may be hung)"
		}
		return false, fmt.Sprintf("encode test failed: %v", err)
	}

	return true, ""
}

// IsNVENCAvailable checks if NVENC encoder is runtime-usable
func IsNVENCAvailable() bool {
	usable, _ := isEncoderUsable("h264_nvenc")
	return usable
}

// IsQSVAvailable checks if Intel QSV encoder is runtime-usable
func IsQSVAvailable() bool {
	usable, _ := isEncoderUsable("h264_qsv")
	return usable
}

// IsVAAPIAvailable checks if VAAPI encoder is runtime-usable
func IsVAAPIAvailable() bool {
	usable, _ := isEncoderUsable("h264_vaapi")
	return usable
}

// detectHWAccels detects available hardware acceleration types
func detectHWAccels() []string {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("Warning: ffmpeg not found in PATH for hwaccel detection")
		return []string{}
	}

	cmd := exec.Command(ffmpegPath, "-hwaccels")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{} // Discard stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Warning: Failed to detect hardware acceleration: %v", err)
		return []string{}
	}

	var hwaccels []string
	lines := strings.Split(stdout.String(), "\n")
	// Skip first line (header "Hardware acceleration methods:")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			hwaccels = append(hwaccels, line)
		}
	}

	return hwaccels
}

// detectAvailableEncoders queries FFmpeg for available encoders
func detectAvailableEncoders() []string {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("Warning: ffmpeg not found in PATH for encoder detection")
		return []string{}
	}

	cmd := exec.Command(ffmpegPath, "-encoders")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{} // Discard stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Warning: Failed to detect encoders: %v", err)
		return []string{}
	}

	var encoders []string
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header lines and empty lines
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Encoders:") {
			continue
		}
		// Encoder lines start with flags (e.g., "V..... libx264")
		if len(line) > 7 && (line[0] == 'V' || line[0] == 'A' || line[0] == 'S') {
			// Extract encoder name (after the flags)
			parts := strings.Fields(line[7:])
			if len(parts) > 0 {
				encoders = append(encoders, parts[0])
			}
		}
	}

	return encoders
}

// GetEncoderReason returns a human-readable explanation for encoder selection
func (e *EncoderCapabilities) GetEncoderReason() string {
	switch e.SelectedH264 {
	case "h264_nvenc":
		if e.ValidationResults["h264_nvenc"] {
			return "NVIDIA NVENC hardware encoder (runtime-validated)"
		}
		return "NVIDIA NVENC detected but validation failed"
	case "h264_qsv":
		if e.ValidationResults["h264_qsv"] {
			return "Intel Quick Sync Video hardware encoder (runtime-validated)"
		}
		return "Intel QSV detected but validation failed"
	case "h264_vaapi":
		if e.ValidationResults["h264_vaapi"] {
			return "VAAPI hardware encoder (runtime-validated)"
		}
		return "VAAPI detected but validation failed"
	case "libx264":
		// Check if hardware encoders were detected but failed validation
		failedEncoders := []string{}
		for _, enc := range []string{"h264_nvenc", "h264_qsv", "h264_vaapi"} {
			if contains(e.H264Encoders, enc) && !e.ValidationResults[enc] {
				reason := e.ValidationReasons[enc]
				if reason == "" {
					reason = "runtime validation failed"
				}
				failedEncoders = append(failedEncoders, fmt.Sprintf("%s (%s)", enc, reason))
			}
		}
		if len(failedEncoders) > 0 {
			return fmt.Sprintf("Using software encoder libx264 - hardware encoders unavailable: %s", strings.Join(failedEncoders, "; "))
		}
		if len(e.H264Encoders) == 1 {
			return "No hardware encoders available, using software encoder (libx264)"
		}
		return "Using libx264 software encoder"
	default:
		return "Using encoder: " + e.SelectedH264
	}
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
