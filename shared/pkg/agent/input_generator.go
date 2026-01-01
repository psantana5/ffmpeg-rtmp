package agent

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// InputGenerator generates test input videos for transcoding jobs
type InputGenerator struct {
	encoderCaps *EncoderCapabilities
	workDir     string
}

// InputGenerationResult holds metrics about input generation
type InputGenerationResult struct {
	FilePath       string
	FileSizeBytes  int64
	DurationSec    float64
	GenerationTime float64
	EncoderUsed    string
}

// NewInputGenerator creates a new input generator with encoder capabilities
func NewInputGenerator(encoderCaps *EncoderCapabilities) *InputGenerator {
	workDir := os.TempDir()
	return &InputGenerator{
		encoderCaps: encoderCaps,
		workDir:     workDir,
	}
}

// SetWorkDir sets the working directory for generated files
func (ig *InputGenerator) SetWorkDir(dir string) {
	ig.workDir = dir
}

// GenerateInput generates a test input video based on job parameters
func (ig *InputGenerator) GenerateInput(job *models.Job) (*InputGenerationResult, error) {
	startTime := time.Now()

	// Extract parameters with defaults
	params := job.Parameters
	if params == nil {
		params = make(map[string]interface{})
	}

	width := getIntParam(params, "resolution_width", 1280)
	height := getIntParam(params, "resolution_height", 720)
	fps := getIntParam(params, "frame_rate", 30)
	duration := getIntParam(params, "duration_seconds", 10)

	// Log all parameters being used
	log.Printf("=== Input Generation Parameters ===")
	log.Printf("Job ID: %s", job.ID)
	log.Printf("Scenario: %s", job.Scenario)
	log.Printf("Resolution: %dx%d (from job params: width=%v, height=%v, using defaults if nil)",
		width, height, params["resolution_width"], params["resolution_height"])
	log.Printf("Frame Rate: %d fps (from job params: %v, using default if nil)", fps, params["frame_rate"])
	log.Printf("Duration: %d seconds (from job params: %v, using default if nil)", duration, params["duration_seconds"])
	
	// Generate output path
	outputPath := filepath.Join(ig.workDir, fmt.Sprintf("input_%s.mp4", job.ID))
	log.Printf("Output Path: %s", outputPath)

	// Select encoder (prefer hardware for faster generation)
	encoder := ig.encoderCaps.SelectedH264
	if encoder == "" {
		encoder = "libx264"
	}
	log.Printf("Encoder Selection: %s (reason: %s)", encoder, ig.encoderCaps.GetEncoderReason())

	// Try hardware encoder first, fallback to software if it fails
	result, err := ig.generateWithEncoder(outputPath, encoder, width, height, fps, duration)
	if err != nil && encoder != "libx264" {
		log.Printf("⚠️  Hardware encoder %s failed: %v", encoder, err)
		log.Printf("→ Falling back to software encoder: libx264")
		encoder = "libx264"
		result, err = ig.generateWithEncoder(outputPath, encoder, width, height, fps, duration)
	}
	
	if err != nil {
		return nil, err
	}

	generationTime := time.Since(startTime).Seconds()

	// Get file size
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat generated file: %w", err)
	}

	result.GenerationTime = generationTime
	result.FileSizeBytes = fileInfo.Size()

	log.Printf("✓ Input video generated successfully:")
	log.Printf("  File: %s", outputPath)
	log.Printf("  Size: %.2f MB (%d bytes)", float64(result.FileSizeBytes)/1024/1024, result.FileSizeBytes)
	log.Printf("  Duration: %.0f seconds", result.DurationSec)
	log.Printf("  Generation Time: %.2f seconds", generationTime)
	log.Printf("  Encoder Actually Used: %s", result.EncoderUsed)
	log.Printf("===================================")

	return result, nil
}

// generateWithEncoder attempts to generate video with a specific encoder
func (ig *InputGenerator) generateWithEncoder(outputPath, encoder string, width, height, fps, duration int) (*InputGenerationResult, error) {
	log.Printf("→ Building FFmpeg command:")
	log.Printf("  Resolution: %dx%d", width, height)
	log.Printf("  Frame Rate: %d fps", fps)
	log.Printf("  Duration: %d seconds", duration)
	log.Printf("  Encoder: %s", encoder)

	// Build FFmpeg command
	// Using testsrc2 with noise for more realistic content
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	args := []string{
		"-y", // Overwrite existing file
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%d", width, height, fps),
		"-vf", "noise=alls=10:allf=u,format=yuv420p",
		"-t", fmt.Sprintf("%d", duration),
		"-c:v", encoder,
	}

	// Add encoder-specific optimizations for fast generation
	switch {
	case encoder == "h264_nvenc":
		args = append(args, "-preset", "fast")
		log.Printf("  NVENC Preset: fast")
	case encoder == "h264_qsv":
		args = append(args, "-preset", "veryfast")
		log.Printf("  QSV Preset: veryfast")
	case encoder == "h264_vaapi":
		args = append(args, "-qp", "28")
		log.Printf("  VAAPI QP: 28")
	case encoder == "libx264":
		args = append(args, "-preset", "ultrafast", "-crf", "23")
		log.Printf("  x264 Preset: ultrafast, CRF: 23")
	}

	args = append(args, outputPath)

	// Log the complete command
	log.Printf("→ Executing: ffmpeg %v", args)

	// Execute FFmpeg
	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("FFmpeg stderr: %s", stderr.String())
		return nil, fmt.Errorf("failed to generate input video: %w", err)
	}

	result := &InputGenerationResult{
		FilePath:    outputPath,
		DurationSec: float64(duration),
		EncoderUsed: encoder,
	}

	return result, nil
}

// CleanupInput removes a generated input file
func (ig *InputGenerator) CleanupInput(filePath string) error {
	if filePath == "" {
		return nil
	}

	// Safety check: only delete files in temp directory or with input_ prefix
	if !strings.HasPrefix(filepath.Base(filePath), "input_") {
		log.Printf("Warning: refusing to delete file without input_ prefix: %s", filePath)
		return nil
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to remove input file: %w", err)
	}

	log.Printf("✓ Cleaned up input file: %s", filePath)
	return nil
}

// ShouldGenerateInput determines if input generation is needed based on job parameters
func ShouldGenerateInput(job *models.Job, generateInputFlag bool) bool {
	// If flag is disabled, don't generate
	if !generateInputFlag {
		return false
	}

	// Check if job explicitly specifies an input file
	if job.Parameters != nil {
		if input, ok := job.Parameters["input"].(string); ok && input != "" {
			// Input file specified, check if it exists
			if _, err := os.Stat(input); err == nil {
				return false // Input exists, no need to generate
			}
			log.Printf("Warning: specified input file not found: %s, will generate", input)
		}
	}

	// Check for streaming modes that don't need file input
	if job.Parameters != nil {
		if mode, ok := job.Parameters["output_mode"].(string); ok {
			if mode == "rtmp" || mode == "stream" {
				// Streaming modes might generate on-the-fly, but we still want
				// to generate for consistency
				return true
			}
		}
	}

	return true
}

// getIntParam extracts an integer parameter with a default value
func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	if val, ok := params[key].(int); ok {
		return val
	}
	return defaultVal
}
