package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func TestNewInputGenerator(t *testing.T) {
	encoderCaps := &EncoderCapabilities{
		SelectedH264: "libx264",
	}
	
	gen := NewInputGenerator(encoderCaps)
	
	if gen == nil {
		t.Fatal("NewInputGenerator returned nil")
	}
	
	if gen.encoderCaps == nil {
		t.Error("InputGenerator encoderCaps is nil")
	}
	
	if gen.workDir == "" {
		t.Error("InputGenerator workDir is empty")
	}
}

func TestGenerateInput(t *testing.T) {
	// Skip if ffmpeg not available
	if _, err := os.Stat("/usr/bin/ffmpeg"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/ffmpeg"); os.IsNotExist(err) {
			t.Skip("FFmpeg not found, skipping test")
		}
	}
	
	encoderCaps := DetectEncoders()
	gen := NewInputGenerator(encoderCaps)
	
	// Use a temporary directory
	tmpDir := t.TempDir()
	gen.SetWorkDir(tmpDir)
	
	job := &models.Job{
		ID:       "test-job-123",
		Scenario: "test",
		Parameters: map[string]interface{}{
			"resolution_width":  640,
			"resolution_height": 480,
			"frame_rate":        30,
			"duration_seconds":  2, // Short duration for fast test
		},
	}
	
	result, err := gen.GenerateInput(job)
	if err != nil {
		t.Fatalf("GenerateInput failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("GenerateInput returned nil result")
	}
	
	// Check result fields
	if result.FilePath == "" {
		t.Error("Generated file path is empty")
	}
	
	if result.FileSizeBytes <= 0 {
		t.Error("Generated file size is zero or negative")
	}
	
	if result.GenerationTime <= 0 {
		t.Error("Generation time is zero or negative")
	}
	
	if result.EncoderUsed == "" {
		t.Error("Encoder used is empty")
	}
	
	// Check file exists
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Errorf("Generated file does not exist: %s", result.FilePath)
	}
	
	t.Logf("Generated file: %s", result.FilePath)
	t.Logf("Size: %d bytes (%.2f MB)", result.FileSizeBytes, float64(result.FileSizeBytes)/1024/1024)
	t.Logf("Duration: %.2f seconds", result.DurationSec)
	t.Logf("Generation time: %.2f seconds", result.GenerationTime)
	t.Logf("Encoder used: %s", result.EncoderUsed)
	
	// Cleanup
	if err := gen.CleanupInput(result.FilePath); err != nil {
		t.Errorf("CleanupInput failed: %v", err)
	}
	
	// Verify cleanup
	if _, err := os.Stat(result.FilePath); !os.IsNotExist(err) {
		t.Error("File was not cleaned up")
	}
}

func TestShouldGenerateInput(t *testing.T) {
	tests := []struct {
		name     string
		job      *models.Job
		flag     bool
		expected bool
	}{
		{
			name: "Flag disabled",
			job: &models.Job{
				ID: "test-1",
			},
			flag:     false,
			expected: false,
		},
		{
			name: "Flag enabled, no input specified",
			job: &models.Job{
				ID:         "test-2",
				Parameters: map[string]interface{}{},
			},
			flag:     true,
			expected: true,
		},
		{
			name: "Flag enabled, streaming mode",
			job: &models.Job{
				ID: "test-3",
				Parameters: map[string]interface{}{
					"output_mode": "rtmp",
				},
			},
			flag:     true,
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldGenerateInput(tt.job, tt.flag)
			if result != tt.expected {
				t.Errorf("ShouldGenerateInput() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCleanupInput(t *testing.T) {
	encoderCaps := &EncoderCapabilities{
		SelectedH264: "libx264",
	}
	gen := NewInputGenerator(encoderCaps)
	
	// Test cleanup of non-existent file (should not error)
	err := gen.CleanupInput("/tmp/input_nonexistent.mp4")
	if err != nil {
		t.Errorf("CleanupInput of non-existent file should not error: %v", err)
	}
	
	// Test cleanup of file without input_ prefix (should be refused)
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "not_input_file.mp4")
	f, _ := os.Create(testFile)
	f.Close()
	
	err = gen.CleanupInput(testFile)
	// Should not delete (safety check)
	if _, statErr := os.Stat(testFile); os.IsNotExist(statErr) {
		t.Error("File without input_ prefix was deleted (should be refused)")
	}
}

func TestGetIntParam(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		key      string
		defVal   int
		expected int
	}{
		{
			name:     "Float64 value",
			params:   map[string]interface{}{"width": float64(1920)},
			key:      "width",
			defVal:   1280,
			expected: 1920,
		},
		{
			name:     "Int value",
			params:   map[string]interface{}{"height": 1080},
			key:      "height",
			defVal:   720,
			expected: 1080,
		},
		{
			name:     "Missing key",
			params:   map[string]interface{}{},
			key:      "fps",
			defVal:   30,
			expected: 30,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntParam(tt.params, tt.key, tt.defVal)
			if result != tt.expected {
				t.Errorf("getIntParam() = %v, want %v", result, tt.expected)
			}
		})
	}
}
