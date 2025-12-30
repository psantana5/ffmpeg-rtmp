# FFmpeg Command Generation and Parameter Optimization

This document explains how to use the FFmpeg parameter optimization system to generate optimized FFmpeg commands for RTMP streaming.

## Overview

The system combines hardware capabilities, content properties, and optimization goals to automatically select the best FFmpeg parameters. It then generates complete, validated FFmpeg commands ready for execution.

## Core Components

### 1. Struct Definitions

All required structs are defined in `shared/pkg/agent/advanced_optimizer.go`:

```go
// WorkerCapabilities represents hardware available on a worker node
type WorkerCapabilities struct {
    CPUCores  int    // Number of CPU cores
    GPUType   string // GPU type (e.g., "nvidia-rtx-3080", "none")
    HasNVENC  bool   // Whether NVENC hardware encoding is available
    MemoryGB  int    // Total system memory in GB
    HasQSV    bool   // Intel Quick Sync Video availability
    HasVAAPI  bool   // Video Acceleration API (Linux)
}

// ContentProperties describes video content characteristics
type ContentProperties struct {
    Resolution  string  // e.g., "1920x1080", "3840x2160"
    MotionLevel string  // "low", "medium", "high"
    GrainLevel  string  // "none", "low", "medium", "high"
    Framerate   int     // Frames per second
    IsHDR       bool    // Whether content is HDR
    ColorSpace  string  // e.g., "bt709", "bt2020"
    PixelFormat string  // e.g., "yuv420p", "yuv420p10le"
}

// FFmpegParams contains optimized FFmpeg parameters
type FFmpegParams struct {
    Encoder      string            // e.g., "libx265", "hevc_nvenc"
    Preset       string            // Encoder-specific preset
    CRF          int               // Constant Rate Factor
    Bitrate      string            // Bitrate (if CRF not used)
    ExtraParams  map[string]string // Additional FFmpeg flags
    Threads      int               // Number of encoding threads
    Reasoning    []string          // Explanation of choices made
    PixelFormat  string            // Output pixel format
}

// RTMPConfig contains RTMP streaming configuration
type RTMPConfig struct {
    MasterHost string // Master node hostname or IP (NOT localhost)
    StreamKey  string // Unique stream identifier
    InputFile  string // Input video file path
}
```

### 2. Hardware Capabilities Detection

Hardware capabilities are detected using the existing exporter system:

- **CPU Exporter** (`worker/exporters/cpu_exporter/main.go`): Exposes CPU threads, cores, and power consumption
- **GPU Exporter** (`worker/exporters/gpu_exporter/main.go`): Exposes GPU model, NVENC support, encoder utilization

Example of building WorkerCapabilities from exporter data:

```go
import (
    "runtime"
    "github.com/psantana5/ffmpeg-rtmp/pkg/agent"
)

// Detect hardware capabilities
caps := agent.WorkerCapabilities{
    CPUCores:  runtime.NumCPU(),  // Or from CPU exporter metrics
    GPUType:   "nvidia-rtx-3080", // From GPU exporter
    HasNVENC:  true,               // Detected from nvidia-smi
    MemoryGB:  16,                 // System memory
    HasQSV:    false,              // Intel Quick Sync detection
    HasVAAPI:  false,              // Linux video acceleration
}
```

## Usage Examples

### Example 1: High-Quality 1080p Streaming with GPU

```go
package main

import (
    "fmt"
    "log"
    "github.com/psantana5/ffmpeg-rtmp/pkg/agent"
)

func main() {
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
    // Goal options: GoalQuality, GoalEnergy, GoalLatency, GoalBalanced
    params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalQuality)

    // Step 4: Build FFmpeg command
    config := agent.RTMPConfig{
        MasterHost: "10.0.1.5",            // Master node IP
        StreamKey:  "my-stream-key-123",   // Unique stream ID
        InputFile:  "/videos/input.mp4",   // Source file
    }

    args, err := agent.BuildFFmpegCommand(params, config)
    if err != nil {
        log.Fatalf("Failed to build command: %v", err)
    }

    // Print the command
    fmt.Printf("ffmpeg %s\n", strings.Join(args, " "))

    // Print reasoning for debugging/logging
    fmt.Println("\nOptimization Reasoning:")
    for i, reason := range params.Reasoning {
        fmt.Printf("%d. %s\n", i+1, reason)
    }
}
```

**Output:**
```bash
ffmpeg -re -i /videos/input.mp4 -c:v h264_nvenc -preset p4 -pix_fmt yuv420p -crf 23 -threads 4 -temporal-aq 1 -bf 3 -g 60 -rc vbr -spatial-aq 1 -f flv rtmp://10.0.1.5:1935/live/my-stream-key-123

Optimization Reasoning:
1. Selected H.264 NVENC for quality goal: hardware acceleration available
2. Using NVENC preset p4: balanced quality and performance
3. Final CRF value: 23 (constant quality mode)
4. Using 4 threads: GPU encoding has minimal CPU thread requirements
5. Using 8-bit pixel format (yuv420p) for SDR content (standard)
6. Using VBR rate control for NVENC: better quality than CBR
7. Enabled spatial and temporal AQ for NVENC: improves perceptual quality
8. Using 3 B-frames for medium-motion: good compression efficiency
9. GOP size set to 60: 2 seconds for good seek performance
```

### Example 2: Low-Latency Streaming with CPU

```go
// Worker with no GPU
worker := agent.WorkerCapabilities{
    CPUCores: 4,
    GPUType:  "none",
    HasNVENC: false,
    MemoryGB: 8,
}

// Fast-moving content
content := agent.ContentProperties{
    Resolution:  "1280x720",
    MotionLevel: "high",
    GrainLevel:  "none",
    Framerate:   60,
    IsHDR:       false,
}

// Optimize for low latency
params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalLatency)

config := agent.RTMPConfig{
    MasterHost: "master.local",
    StreamKey:  "low-latency-stream",
    InputFile:  "/videos/gameplay.mp4",
}

args, err := agent.BuildFFmpegCommand(params, config)
if err != nil {
    log.Fatalf("Error: %v", err)
}

// Execute FFmpeg
cmd := exec.Command("ffmpeg", args...)
if err := cmd.Run(); err != nil {
    log.Fatalf("FFmpeg failed: %v", err)
}
```

### Example 3: 4K HDR Content with HEVC

```go
// High-end server
worker := agent.WorkerCapabilities{
    CPUCores: 16,
    GPUType:  "nvidia-rtx-4090",
    HasNVENC: true,
    MemoryGB: 32,
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
    MasterHost: "192.168.1.100",
    StreamKey:  "4k-hdr-stream",
    InputFile:  "/content/hdr-movie.mp4",
}

args, err := agent.BuildFFmpegCommand(params, config)
```

**Key Decision:** For 4K + quality goal, the system will likely choose `libx265` (CPU) over NVENC for superior quality, despite having GPU available.

### Example 4: Energy-Efficient Encoding

```go
// Laptop with integrated GPU
worker := agent.WorkerCapabilities{
    CPUCores: 8,
    GPUType:  "intel-uhd",
    HasNVENC: false,
    HasQSV:   true,  // Intel Quick Sync
    MemoryGB: 16,
}

content := agent.ContentProperties{
    Resolution:  "1920x1080",
    MotionLevel: "medium",
    Framerate:   30,
    IsHDR:       false,
}

// Optimize for energy efficiency
params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalEnergy)

// The system will select h264_qsv for hardware acceleration
// This reduces power consumption compared to CPU encoding
```

## Decision-Making Logic

### Encoder Selection

The system chooses encoders based on:

1. **Goal Priority:**
   - `GoalLatency` / `GoalEnergy`: Prefer GPU (NVENC, QSV) for speed and efficiency
   - `GoalQuality`: Prefer CPU encoders (libx264, libx265) for maximum quality
   - `GoalBalanced`: Use GPU if available, otherwise CPU

2. **Resolution:**
   - 4K content: Prefer HEVC (libx265 or hevc_nvenc) for better compression
   - 1080p and below: H.264 is sufficient

3. **Hardware Availability:**
   - NVIDIA GPU → h264_nvenc or hevc_nvenc
   - Intel QSV → h264_qsv
   - CPU only → libx264 or libx265

### Preset Selection

Presets balance speed and quality:

**NVENC Presets:** p1 (fastest) to p7 (slowest)
- Latency goal: p1
- Energy goal: p3
- Quality goal: p6
- Balanced: p4

**CPU Presets:** ultrafast, veryfast, fast, medium, slow, slower, veryslow
- Adjusted based on available CPU cores
- Limited cores (≤4): Use faster presets
- High-end (8+ cores): Can afford slower presets

### CRF/Bitrate Adjustment

CRF (Constant Rate Factor) is adjusted based on:

1. **Base CRF by goal:**
   - Quality: 20 (higher quality)
   - Balanced: 23 (standard)
   - Latency/Energy: 26 (faster encoding)

2. **Content adjustments:**
   - High motion: CRF - 2 (more bits for detail)
   - Low motion: CRF + 2 (efficient compression)
   - High grain: CRF - 2 (preserve texture)
   - Medium grain: CRF - 1

3. **Encoder adjustments:**
   - HEVC: CRF + 5 (more efficient codec)

### B-Frame Configuration

B-frames improve compression but increase latency:

**High motion content:**
- NVENC: 2 B-frames
- CPU: 2-3 B-frames

**Low motion content:**
- NVENC: 4 B-frames
- CPU (HEVC): 8 B-frames
- CPU (H.264): 3 B-frames

### HDR and 10-bit Encoding

When `IsHDR = true` or `PixelFormat = "yuv420p10le"`:
- Output pixel format: `yuv420p10le`
- Preserves HDR color depth
- Reasoning explains the decision

## RTMP URL Validation

The `BuildFFmpegCommand` function validates:

1. **Master host is specified** (not empty)
2. **Master host is NOT localhost** (127.0.0.1, ::1)
3. **RTMP URL format:** `rtmp://{master_host}:1935/live/{stream_key}`

Example helper function to extract master host from API URL:

```go
masterAPIURL := "https://10.0.1.5:8080"
masterHost, err := agent.GetRTMPURLFromMasterURL(masterAPIURL)
// Returns: "10.0.1.5"

config := agent.RTMPConfig{
    MasterHost: masterHost,
    StreamKey:  "stream-123",
    InputFile:  "/input.mp4",
}
```

## Codec-Specific Flag Validation

The system ensures flags are compatible with the chosen encoder:

### NVENC-specific flags:
- `-rc vbr`: Rate control mode
- `-spatial-aq 1`: Spatial adaptive quantization
- `-temporal-aq 1`: Temporal adaptive quantization
- `-zerolatency 1`: Low-latency mode
- `-bf N`: B-frames

### libx264/libx265-specific flags:
- `-tune film/grain/zerolatency`: Tuning option
- `-x265-params`: HEVC-specific parameters
  - `aq-mode=3`: Adaptive quantization
  - `no-sao=1`: Disable SAO filter
  - `rd=6`: Rate-distortion optimization
- `-me_method umh`: Motion estimation

### Incompatible flags are automatically filtered:
- `tune` is skipped for NVENC/QSV (not supported)
- `rc` is skipped for CPU encoders (NVENC-only)
- x265-specific params are skipped for x264

## Integration with Worker Agent

In the worker agent (`worker/cmd/agent/main.go`), this is typically used as:

```go
// 1. Detect hardware at startup
caps := agent.DetectHardware()

// 2. When a job is received
job := client.GetNextJob()

// Convert job scenario to content properties
content := parseJobScenario(job.Scenario) // e.g., "4K60-h264" → ContentProperties

// 3. Determine goal from job confidence
goal := determineGoal(job.Confidence) // "high" → GoalQuality, "low" → GoalLatency

// 4. Calculate parameters
params := agent.CalculateOptimalFFmpegParams(caps, content, goal)

// 5. Build command
masterHost, _ := agent.GetRTMPURLFromMasterURL(globalMasterURL)
config := agent.RTMPConfig{
    MasterHost: masterHost,
    StreamKey:  job.ID,
    InputFile:  job.Parameters["input_file"].(string),
}

args, err := agent.BuildFFmpegCommand(params, config)
if err != nil {
    return fmt.Errorf("failed to build FFmpeg command: %w", err)
}

// 6. Execute
cmd := exec.Command("ffmpeg", args...)
if err := cmd.Run(); err != nil {
    return fmt.Errorf("FFmpeg execution failed: %w", err)
}
```

## Testing

Comprehensive unit tests are provided in `shared/pkg/agent/ffmpeg_command_test.go`:

```bash
# Run all agent tests
cd shared/pkg
go test ./agent/... -v

# Run specific test
go test ./agent/... -v -run TestBuildFFmpegCommand_NVENC
```

Test coverage includes:
- Basic command generation
- NVENC-specific flags
- HEVC/x265 parameters
- HDR 10-bit encoding
- Error cases (missing config)
- CRF validation
- RTMP URL parsing
- Complete integration tests

## Troubleshooting

### Error: "master host is required (do not use localhost)"

**Cause:** Master host not specified or set to localhost.

**Solution:** Always specify the actual master node IP or hostname:
```go
config := agent.RTMPConfig{
    MasterHost: "10.0.1.5",  // ✓ Correct
    // NOT: "localhost"       // ✗ Wrong
    StreamKey:  "stream-key",
    InputFile:  "/input.mp4",
}
```

### Error: "CRF X out of range for encoder Y"

**Cause:** CRF value doesn't match encoder's valid range.

**Solution:** Use appropriate CRF values:
- H.264/H.265: 0-51 (typical: 18-28)
- Don't manually set CRF; let `CalculateOptimalFFmpegParams` handle it

### FFmpeg fails with "Unknown encoder"

**Cause:** Encoder not available in FFmpeg installation.

**Solution:** Check FFmpeg build:
```bash
ffmpeg -encoders | grep nvenc  # Check for NVENC support
ffmpeg -encoders | grep x265   # Check for HEVC support
```

Ensure FFmpeg is built with required encoders.

## Performance Considerations

1. **GPU Encoding:**
   - Faster than CPU (2-5x)
   - Lower quality at same bitrate
   - Minimal CPU usage
   - Best for: Latency, energy efficiency, multiple streams

2. **CPU Encoding:**
   - Slower but higher quality
   - Uses significant CPU resources
   - Better compression efficiency
   - Best for: Quality, archival, single-stream scenarios

3. **Thread Allocation:**
   - GPU: 4 threads (minimal CPU needed)
   - CPU (4 cores): Use all cores
   - CPU (8 cores): Leave 1 core for system
   - CPU (16+ cores): Leave 2 cores for system

## Summary

This system provides:
✓ Automatic hardware-aware encoder selection
✓ Content-aware parameter optimization
✓ Goal-based quality/speed/efficiency tuning
✓ Validated FFmpeg command generation
✓ RTMP URL validation (prevents localhost errors)
✓ Codec-specific flag compatibility
✓ Comprehensive reasoning for decisions
✓ Full test coverage

All required structs exist in `shared/pkg/agent/advanced_optimizer.go`, and the system uses real hardware information from exporters to make informed decisions.
