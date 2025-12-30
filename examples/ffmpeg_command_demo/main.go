package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/psantana5/ffmpeg-rtmp/pkg/agent"
)

// Example demonstrating complete FFmpeg command generation workflow
func main() {
	fmt.Println("=== FFmpeg Command Generation Example ===\n")

	// Example 1: GPU-accelerated 1080p streaming
	example1()

	// Example 2: CPU-based 4K HDR encoding
	example2()

	// Example 3: Low-latency streaming
	example3()
}

func example1() {
	fmt.Println("Example 1: GPU-Accelerated 1080p Streaming")
	fmt.Println("-------------------------------------------")

	// Step 1: Define worker capabilities (from exporters)
	worker := agent.WorkerCapabilities{
		CPUCores: 8,
		GPUType:  "nvidia-rtx-3080",
		HasNVENC: true,
		MemoryGB: 16,
	}

	// Step 2: Define content properties
	content := agent.ContentProperties{
		Resolution:  "1920x1080",
		MotionLevel: "medium",
		GrainLevel:  "low",
		Framerate:   30,
		IsHDR:       false,
		ColorSpace:  "bt709",
		PixelFormat: "yuv420p",
	}

	// Step 3: Calculate optimal parameters
	params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalBalanced)

	// Step 4: Build FFmpeg command
	config := agent.RTMPConfig{
		MasterHost: "10.0.1.5",
		StreamKey:  "stream-1080p-balanced",
		InputFile:  "/videos/input.mp4",
	}

	args, err := agent.BuildFFmpegCommand(params, config)
	if err != nil {
		log.Fatalf("Failed to build command: %v", err)
	}

	// Print results
	fmt.Printf("Encoder: %s\n", params.Encoder)
	fmt.Printf("Preset: %s\n", params.Preset)
	fmt.Printf("CRF: %d\n", params.CRF)
	fmt.Printf("Threads: %d\n", params.Threads)
	fmt.Printf("Pixel Format: %s\n\n", params.PixelFormat)

	fmt.Println("Command:")
	fmt.Printf("ffmpeg %s\n\n", strings.Join(args, " "))

	fmt.Println("Reasoning:")
	for i, reason := range params.Reasoning {
		fmt.Printf("%d. %s\n", i+1, reason)
	}
	fmt.Println()
}

func example2() {
	fmt.Println("\nExample 2: CPU-Based 4K HDR Encoding")
	fmt.Println("-------------------------------------")

	// High-end CPU, no GPU
	worker := agent.WorkerCapabilities{
		CPUCores: 16,
		GPUType:  "none",
		HasNVENC: false,
		MemoryGB: 64,
	}

	// 4K HDR content
	content := agent.ContentProperties{
		Resolution:  "3840x2160",
		MotionLevel: "low",
		GrainLevel:  "medium",
		Framerate:   24,
		IsHDR:       true,
		ColorSpace:  "bt2020",
		PixelFormat: "yuv420p10le",
	}

	// Optimize for quality
	params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalQuality)

	config := agent.RTMPConfig{
		MasterHost: "master.example.com",
		StreamKey:  "stream-4k-hdr",
		InputFile:  "/content/hdr-movie.mp4",
	}

	args, err := agent.BuildFFmpegCommand(params, config)
	if err != nil {
		log.Fatalf("Failed to build command: %v", err)
	}

	fmt.Printf("Encoder: %s (CPU-based for quality)\n", params.Encoder)
	fmt.Printf("Preset: %s\n", params.Preset)
	fmt.Printf("CRF: %d\n", params.CRF)
	fmt.Printf("Pixel Format: %s (10-bit for HDR)\n\n", params.PixelFormat)

	fmt.Println("Command:")
	fmt.Printf("ffmpeg %s\n\n", strings.Join(args, " "))

	fmt.Println("Key Decisions:")
	for _, reason := range params.Reasoning {
		if strings.Contains(strings.ToLower(reason), "hdr") ||
			strings.Contains(strings.ToLower(reason), "hevc") ||
			strings.Contains(strings.ToLower(reason), "quality") {
			fmt.Printf("• %s\n", reason)
		}
	}
	fmt.Println()
}

func example3() {
	fmt.Println("\nExample 3: Low-Latency Streaming")
	fmt.Println("---------------------------------")

	// Mid-range system with GPU
	worker := agent.WorkerCapabilities{
		CPUCores: 6,
		GPUType:  "nvidia-gtx-1660",
		HasNVENC: true,
		MemoryGB: 12,
	}

	// High-motion gaming content
	content := agent.ContentProperties{
		Resolution:  "1280x720",
		MotionLevel: "high",
		GrainLevel:  "none",
		Framerate:   60,
		IsHDR:       false,
		ColorSpace:  "bt709",
	}

	// Optimize for latency
	params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalLatency)

	config := agent.RTMPConfig{
		MasterHost: "192.168.1.100",
		StreamKey:  "stream-gaming-lowlat",
		InputFile:  "/videos/gameplay.mp4",
	}

	args, err := agent.BuildFFmpegCommand(params, config)
	if err != nil {
		log.Fatalf("Failed to build command: %v", err)
	}

	fmt.Printf("Encoder: %s\n", params.Encoder)
	fmt.Printf("Preset: %s (fastest for latency)\n", params.Preset)
	fmt.Printf("CRF: %d\n", params.CRF)

	// Check for zero-latency flag
	hasZeroLatency := false
	for _, reason := range params.Reasoning {
		if strings.Contains(strings.ToLower(reason), "latency") {
			fmt.Printf("• %s\n", reason)
			if strings.Contains(strings.ToLower(reason), "zero") {
				hasZeroLatency = true
			}
		}
	}

	fmt.Println("\nCommand:")
	fmt.Printf("ffmpeg %s\n", strings.Join(args, " "))

	if hasZeroLatency {
		fmt.Println("\n✓ Zero-latency mode enabled for minimal encoding delay")
	}
	fmt.Println()
}
