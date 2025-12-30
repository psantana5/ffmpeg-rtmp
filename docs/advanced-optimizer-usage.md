# Advanced FFmpeg Parameter Optimizer

This module provides intelligent FFmpeg parameter optimization based on worker capabilities, content properties, and transcoding goals.

## Overview

The `CalculateOptimalFFmpegParams` function takes three inputs and returns optimized FFmpeg parameters with detailed reasoning.

## API

### Input Types

#### WorkerCapabilities
```go
type WorkerCapabilities struct {
    CPUCores  int    // Number of CPU cores
    GPUType   string // GPU type (e.g., "nvidia-rtx-3080", "none")
    HasNVENC  bool   // Whether NVENC hardware encoding is available
    MemoryGB  int    // Total system memory in GB
    HasQSV    bool   // Intel Quick Sync Video availability
    HasVAAPI  bool   // Video Acceleration API (Linux)
}
```

#### ContentProperties
```go
type ContentProperties struct {
    Resolution  string // e.g., "1920x1080", "3840x2160"
    MotionLevel string // "low", "medium", "high"
    GrainLevel  string // "none", "low", "medium", "high"
    Framerate   int    // Frames per second
    IsHDR       bool   // Whether content is HDR
    ColorSpace  string // e.g., "bt709", "bt2020"
    PixelFormat string // e.g., "yuv420p", "yuv420p10le"
}
```

#### TranscodingGoal
```go
type TranscodingGoal string

const (
    GoalQuality  TranscodingGoal = "quality"  // Prioritize visual quality
    GoalEnergy   TranscodingGoal = "energy"   // Prioritize energy efficiency
    GoalLatency  TranscodingGoal = "latency"  // Prioritize speed/low latency
    GoalBalanced TranscodingGoal = "balanced" // Balance all factors
)
```

### Output Type

```go
type TranscodingParams struct {
    Encoder      string            `json:"encoder"`       // e.g., "libx265", "hevc_nvenc"
    Preset       string            `json:"preset"`        // Encoder-specific preset
    CRF          int               `json:"crf,omitempty"` // Constant Rate Factor
    Bitrate      string            `json:"bitrate,omitempty"`
    ExtraParams  map[string]string `json:"extra_params"`  // Additional FFmpeg flags
    Threads      int               `json:"threads"`       // Number of encoding threads
    Reasoning    []string          `json:"reasoning"`     // Explanation of choices
    PixelFormat  string            `json:"pixel_format"`  // Output pixel format
}
```

## Usage Examples

### Example 1: GPU System with Latency Goal

```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/agent"

worker := agent.WorkerCapabilities{
    CPUCores: 8,
    GPUType:  "nvidia-rtx-3080",
    HasNVENC: true,
    MemoryGB: 16,
}

content := agent.ContentProperties{
    Resolution:  "1920x1080",
    MotionLevel: "high",
    GrainLevel:  "low",
    Framerate:   60,
    IsHDR:       false,
}

params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalLatency)

// Output:
// {
//   "encoder": "h264_nvenc",
//   "preset": "p1",
//   "crf": 26,
//   "extra_params": {
//     "rc": "vbr",
//     "bf": "2",
//     "zerolatency": "1",
//     "g": "120"
//   },
//   "threads": 4,
//   "pixel_format": "yuv420p",
//   "reasoning": [
//     "Selected H.264 NVENC: hardware acceleration optimizes for energy/latency goal",
//     "Using NVENC preset p1: fastest encoding for latency goal",
//     "Reduced CRF by 2 for high-motion content to preserve detail",
//     "Final CRF value: 26 (constant quality mode)",
//     ...
//   ]
// }
```

### Example 2: High-End CPU with Quality Goal for 4K

```go
worker := agent.WorkerCapabilities{
    CPUCores: 16,
    GPUType:  "none",
    HasNVENC: false,
    MemoryGB: 64,
}

content := agent.ContentProperties{
    Resolution:  "3840x2160",
    MotionLevel: "high",
    GrainLevel:  "medium",
    Framerate:   24,
    IsHDR:       false,
}

params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalQuality)

// Output:
// {
//   "encoder": "libx265",
//   "preset": "slow",
//   "crf": 21,
//   "extra_params": {
//     "aq-mode": "3",
//     "bframes": "3",
//     "rd": "6",
//     "g": "48"
//   },
//   "threads": 14,
//   "pixel_format": "yuv420p",
//   "reasoning": [
//     "Selected libx265 for quality goal: CPU encoding provides superior quality for 4K/high-motion content",
//     "Using preset 'slow': high-end CPU (8+ cores) can afford better quality encoding",
//     "Reduced CRF by 2 for high-motion content to preserve detail",
//     "Reduced CRF by 1 for medium-grain content",
//     ...
//   ]
// }
```

### Example 3: HDR Content

```go
worker := agent.WorkerCapabilities{
    CPUCores: 12,
    GPUType:  "nvidia-rtx-4080",
    HasNVENC: true,
    MemoryGB: 24,
}

content := agent.ContentProperties{
    Resolution:  "3840x2160",
    MotionLevel: "low",
    GrainLevel:  "none",
    Framerate:   24,
    IsHDR:       true,
    ColorSpace:  "bt2020",
    PixelFormat: "yuv420p10le",
}

params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalBalanced)

// Output:
// {
//   "encoder": "hevc_nvenc",
//   "preset": "p4",
//   "crf": 25,
//   "extra_params": {
//     "rc": "vbr",
//     "spatial-aq": "1",
//     "temporal-aq": "1",
//     "bf": "4",
//     "g": "48"
//   },
//   "threads": 4,
//   "pixel_format": "yuv420p10le",
//   "reasoning": [
//     "Selected HEVC NVENC for balanced goal: good quality and performance for 4K",
//     "Using NVENC preset p4: balanced quality and performance",
//     "Increased CRF by 2 for low-motion content (more efficient)",
//     "Using 10-bit pixel format (yuv420p10le) for HDR content to preserve color depth",
//     ...
//   ]
// }
```

### Example 4: Low-End System with Balanced Goal

```go
worker := agent.WorkerCapabilities{
    CPUCores: 4,
    GPUType:  "none",
    HasNVENC: false,
    MemoryGB: 8,
}

content := agent.ContentProperties{
    Resolution:  "1920x1080",
    MotionLevel: "medium",
    GrainLevel:  "none",
    Framerate:   30,
    IsHDR:       false,
}

params := agent.CalculateOptimalFFmpegParams(worker, content, agent.GoalBalanced)

// Output:
// {
//   "encoder": "libx264",
//   "preset": "veryfast",
//   "crf": 23,
//   "extra_params": {
//     "tune": "film",
//     "bf": "3",
//     "g": "60"
//   },
//   "threads": 4,
//   "pixel_format": "yuv420p",
//   "reasoning": [
//     "Selected libx264 for balanced goal: widely compatible with good performance",
//     "Using preset 'veryfast': optimized for limited CPU cores",
//     "Final CRF value: 23 (constant quality mode)",
//     "Using 4 threads: optimized for 4 CPU cores (leaving headroom for system)",
//     ...
//   ]
// }
```

## Optimization Rules

### Encoder Selection

1. **GPU Acceleration Priority**: For `energy` or `latency` goals, GPU encoders (NVENC, QSV) are preferred
2. **Quality Priority**: For `quality` goal, CPU encoders (libx264, libx265) are preferred for 4K and high-motion content
3. **Balanced**: Uses GPU if available, otherwise chooses appropriate CPU encoder

### Preset Selection

1. **Hardware Constraints**: Limited CPU cores force faster presets
2. **Goal Alignment**: Latency uses fastest presets, quality uses slower presets
3. **NVENC Presets**: p1 (fastest) to p7 (slowest)
4. **CPU Presets**: ultrafast → veryfast → fast → medium → slow → slower → veryslow

### Rate Control (CRF)

Base CRF is adjusted for:
- **Motion Level**: High motion reduces CRF (more bits), low motion increases CRF
- **Grain Level**: More grain reduces CRF (preserves texture)
- **Goal**: Quality goal starts lower, latency/energy starts higher
- **Encoder**: HEVC uses +5 CRF offset (more efficient)

### Special Flags

**NVENC**:
- VBR rate control for better quality
- Spatial/Temporal AQ for quality goals
- B-frames based on motion (2-4)
- Zero-latency mode for latency goal

**libx265**:
- AQ mode 2-3 for perceptual quality
- Variable B-frames (3-8) based on motion
- RD optimization for quality goal
- SAO disabled for latency

**libx264**:
- Tune options: grain/zerolatency/film
- B-frames based on motion (2-3)
- UMH motion estimation for quality

### HDR Support

- Automatically uses 10-bit pixel format (yuv420p10le) when HDR is detected
- Preserves color space information

## Extending the Optimizer

The optimizer is designed to be easily extensible:

1. **Add New Hardware Support**:
```go
// In selectPreset function
if worker.HasVAAPI {
    // Add VAAPI-specific preset logic
}
```

2. **Add New Content Analysis**:
```go
// In ContentProperties
type ContentProperties struct {
    // ... existing fields
    SceneComplexity string // new field
}

// In selectRateControl function
if content.SceneComplexity == "high" {
    baseCRF -= 1
}
```

3. **Add New Goals**:
```go
const (
    GoalQuality  TranscodingGoal = "quality"
    GoalEnergy   TranscodingGoal = "energy"
    GoalLatency  TranscodingGoal = "latency"
    GoalBalanced TranscodingGoal = "balanced"
    GoalArchival TranscodingGoal = "archival" // new goal
)
```

## Testing

Run the comprehensive test suite:

```bash
go test -v github.com/psantana5/ffmpeg-rtmp/pkg/agent -run TestCalculateOptimal
```

Test coverage includes:
- GPU vs CPU selection
- All transcoding goals
- HDR content
- Various motion and grain levels
- Thread allocation
- Rate control adjustments
- Special flags for different encoders

## Integration

The advanced optimizer can be integrated with the existing job execution system:

```go
// In agent startup
caps := agent.DetectHardware()
nodeType := agent.DetectNodeType(caps.CPUThreads, caps.RAMBytes)

// When executing a job
content := agent.ContentProperties{
    Resolution:  job.Parameters["resolution"].(string),
    MotionLevel: job.Parameters["motion_level"].(string),
    // ... other properties
}

goal := agent.TranscodingGoal(job.Parameters["goal"].(string))

workerCaps := agent.WorkerCapabilities{
    CPUCores: caps.CPUThreads,
    HasNVENC: caps.HasGPU,
    MemoryGB: int(caps.RAMBytes / (1024 * 1024 * 1024)),
}

params := agent.CalculateOptimalFFmpegParams(workerCaps, content, goal)

// Use params to build FFmpeg command
```
