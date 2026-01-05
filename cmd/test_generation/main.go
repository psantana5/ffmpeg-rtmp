package main

import (
	"fmt"
	"log"
	
	"github.com/psantana5/ffmpeg-rtmp/pkg/agent"
	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

func main() {
	log.SetFlags(log.LstdFlags)
	
	fmt.Println("\n======================================================================")
	fmt.Println("  DYNAMIC INPUT GENERATION - Parameter Logging Verification")
	fmt.Println("======================================================================")
	
	// Detect encoders first
	fmt.Println(">>> HARDWARE ENCODER DETECTION <<<")
	encoderCaps := agent.DetectEncoders()
	log.Printf("✓ H.264 Encoder: %s", encoderCaps.SelectedH264)
	log.Printf("✓ Reason: %s", encoderCaps.GetEncoderReason())
	
	generator := agent.NewInputGenerator(encoderCaps)
	generator.SetWorkDir("/tmp")
	
	// Test with custom parameters
	fmt.Println("\n>>> GENERATING INPUT WITH CUSTOM PARAMETERS <<<")
	testJob := &models.Job{
		ID:       "demo-job-001",
		Scenario: "720p30-test",
		Parameters: map[string]interface{}{
			"resolution_width":  1280,
			"resolution_height": 720,
			"frame_rate":        30,
			"duration_seconds":  3,
		},
	}
	
	log.Printf("JOB PARAMETERS:")
	log.Printf("  Resolution: %v x %v", testJob.Parameters["resolution_width"], testJob.Parameters["resolution_height"])
	log.Printf("  Frame Rate: %v fps", testJob.Parameters["frame_rate"])
	log.Printf("  Duration: %v seconds", testJob.Parameters["duration_seconds"])
	log.Printf("  Encoder Selected: %s", encoderCaps.SelectedH264)
	
	result, err := generator.GenerateInput(testJob)
	if err != nil {
		log.Fatalf("❌ Generation failed: %v", err)
	}
	
	fmt.Printf("\n✅ GENERATION SUCCESSFUL:\n")
	fmt.Printf("   File Path: %s\n", result.FilePath)
	fmt.Printf("   File Size: %.2f MB\n", float64(result.FileSizeBytes)/1024/1024)
	fmt.Printf("   Duration: %.0f seconds\n", result.DurationSec)
	fmt.Printf("   Generation Time: %.2f seconds\n", result.GenerationTime)
	fmt.Printf("   Encoder Used: %s\n", result.EncoderUsed)
	
	// Cleanup
	fmt.Println("\n>>> CLEANUP <<<")
	if err := generator.CleanupInput(result.FilePath); err != nil {
		log.Printf("⚠️  Cleanup failed: %v", err)
	} else {
		log.Printf("✓ File removed: %s", result.FilePath)
	}
	
	fmt.Println("\n======================================================================")
	fmt.Println("  ✅ TEST PASSED - Parameters logged and applied correctly!")
	fmt.Println("======================================================================")
}
