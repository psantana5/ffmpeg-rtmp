# Implementation Summary: Hardware-Aware FFmpeg Optimization

## Overview

This PR implements comprehensive hardware-aware FFmpeg parameter optimization for the distributed compute system, addressing the original requirement to use the most performant/adequate flags for the host running tests, plus additional enhancements.

## Features Implemented

### 1. Basic Hardware Optimization (Phase 1)
**File**: `shared/pkg/agent/ffmpeg_optimizer.go`

- Detects hardware capabilities at agent startup
- Recommends optimal encoder (NVENC vs CPU)
- Selects appropriate preset based on CPU threads
- Adjusts for node type (laptop/desktop/server)
- Applied automatically to all jobs with override support

**Key Functions**:
- `OptimizeFFmpegParameters()`: Returns optimal encoder, preset, flags
- `ApplyOptimizationToParameters()`: Merges with job params (job takes precedence)

**Hardware Detection**:
- GPU presence (NVENC hardware encoding)
- CPU thread count
- System RAM
- Node type (laptop/desktop/server via battery detection)

### 2. Advanced Content-Aware Optimization (Phase 2)
**File**: `shared/pkg/agent/advanced_optimizer.go`

Advanced optimizer considering:
- **Worker Capabilities**: CPU cores, GPU type, NVENC/QSV/VAAPI, memory
- **Content Properties**: Resolution, motion level, grain, framerate, HDR, color space
- **Transcoding Goals**: quality, energy, latency, balanced

**Intelligent Optimizations**:
- Encoder selection (GPU for energy/latency, CPU for quality)
- Adaptive presets constrained by hardware
- Content-aware CRF adjustments (motion, grain)
- Special flags optimization (AQ modes, B-frames, tune options)
- 10-bit HDR support with automatic detection
- GOP size based on framerate
- Thread allocation strategies

**Output Example**:
```json
{
  "encoder": "hevc_nvenc",
  "preset": "p4",
  "crf": 23,
  "extra_params": {
    "rc": "vbr",
    "spatial-aq": "1",
    "temporal-aq": "1",
    "bf": "3",
    "g": "60"
  },
  "threads": 4,
  "pixel_format": "yuv420p10le",
  "reasoning": [
    "Selected HEVC NVENC for balanced goal: good quality and performance for 4K",
    "Using NVENC preset p4: balanced quality and performance",
    "Enabled spatial and temporal AQ for NVENC: improves perceptual quality",
    "Using 3 B-frames for medium-motion: good compression efficiency",
    ...
  ]
}
```

### 3. RTMP Streaming Support (Phase 3)
**File**: `worker/cmd/agent/main.go` (updated)

Added dual-mode support:

**File Mode** (default, backward compatible):
- Traditional file-to-file transcoding
- Input file → output file
- Used for batch processing

**RTMP Streaming Mode**:
- Generates test pattern source (testsrc + sine audio)
- Streams to RTMP server (e.g., nginx-rtmp)
- Configurable resolution, fps, bitrate
- FLV container format
- Realtime streaming with `-re` flag
- Zero-latency tuning for live streaming
- Proper GOP size and buffering

**Usage**:
```json
{
  "scenario": "rtmp-stream",
  "parameters": {
    "output_mode": "rtmp",
    "rtmp_url": "rtmp://localhost:1935/live/test-stream",
    "resolution": "1920x1080",
    "fps": 30,
    "bitrate": "5000k",
    "duration": 300
  }
}
```

**Generated Command**:
```bash
ffmpeg -t 300 -re \
  -f lavfi -i testsrc=size=1920x1080:rate=30 \
  -f lavfi -i sine=frequency=1000:sample_rate=48000 \
  -c:v h264_nvenc -preset p4 \
  -b:v 5000k -maxrate 5000k -bufsize 5000k \
  -pix_fmt yuv420p -g 60 \
  -c:a aac -b:a 128k -ar 48000 \
  -f flv rtmp://localhost:1935/live/test-stream
```

## Testing

### Unit Tests (24 total)

**Basic Optimizer Tests** (9 tests):
- GPU detection and NVENC selection
- CPU thread-based preset selection
- Low-end, mid-range, high-end CPU scenarios
- Laptop thermal optimization
- Parameter override behavior

**Advanced Optimizer Tests** (15 tests):
- GPU vs CPU selection for different goals
- 4K and 1080p scenarios
- HDR content handling
- High/low motion content
- Grain level adjustments
- B-frame optimization
- Thread allocation strategies
- Intel QSV support
- GOP size calculation
- Comprehensive reasoning validation

All tests passing ✓

### Integration Tests
- Hardware optimization test script
- Validates hardware detection
- Verifies parameter optimization
- Tests all transcoding goals

## Documentation

### Created Documents:
1. **`docs/hardware-optimization.md`**: Basic hardware optimization guide
2. **`docs/advanced-optimizer-usage.md`**: Advanced optimizer API and examples
3. **`docs/rtmp-streaming-support.md`**: RTMP streaming setup and usage
4. **`tests/test_hardware_optimization.sh`**: Integration test script

## Benefits

1. **Automatic Performance Optimization**: No manual FFmpeg flag configuration needed
2. **Hardware Utilization**: GPU acceleration used when available
3. **Appropriate Load**: Preset matches CPU capability to avoid overload
4. **Content-Aware**: Adjusts parameters based on video characteristics
5. **Goal-Oriented**: Optimizes for quality, energy, latency, or balanced
6. **Flexibility**: Job parameters can override all optimizations
7. **Transparency**: Detailed reasoning logged and returned in results
8. **Dual Mode**: Supports both file transcoding and RTMP streaming
9. **Production Ready**: Suitable for both batch processing and live streaming

## Optimization Rules

### Encoder Selection
- **GPU Available + Energy/Latency Goal** → NVENC/QSV
- **Quality Goal + 4K/High-Motion** → CPU (libx265)
- **Balanced** → GPU if available, otherwise CPU

### Preset Selection
- **NVENC**: p1 (fastest) to p7 (slowest)
- **CPU**: Limited cores → faster presets (ultrafast, veryfast)
- **CPU**: High-end → slower presets (slow, slower) for quality
- **Laptop**: Faster presets to reduce thermal/battery impact

### Rate Control (CRF)
- Base CRF: 20 (quality), 23 (balanced), 26 (latency/energy)
- Adjusted for motion: -2 for high motion, +2 for low motion
- Adjusted for grain: -2 for high grain, -1 for medium
- HEVC offset: +5 (more efficient than H.264)

### Special Flags
- **NVENC**: VBR, spatial/temporal AQ, B-frames (2-4), zero-latency mode
- **libx265**: AQ mode 2-3, variable B-frames (3-8), RD optimization, SAO control
- **libx264**: Tune options (grain/zerolatency/film), B-frames (2-3), motion estimation

### HDR Support
- Automatically detects HDR content
- Uses 10-bit pixel format (yuv420p10le)
- Preserves color space information

## Example Scenarios

### Scenario 1: GPU System, Latency Goal
```
Hardware: 8 cores, NVIDIA RTX 3080, 16GB RAM
Content: 1080p60, high motion
Goal: Latency

Result: h264_nvenc, preset p1, CRF 24, bf=2, zerolatency=1
```

### Scenario 2: High-End CPU, Quality Goal, 4K
```
Hardware: 16 cores, no GPU, 64GB RAM
Content: 4K24, high motion, medium grain
Goal: Quality

Result: libx265, preset slow, CRF 21, aq-mode=3, bframes=3, rd=6
```

### Scenario 3: Low-End Laptop, Balanced Goal
```
Hardware: 4 cores, battery present, 8GB RAM
Content: 1080p30, medium motion
Goal: Balanced

Result: libx264, preset veryfast, CRF 23, tune=film
```

### Scenario 4: RTMP Streaming with GPU
```
Hardware: 12 cores, NVIDIA RTX 4080, 24GB RAM
Mode: RTMP streaming
Content: 1080p30

Result: h264_nvenc streaming to rtmp://localhost:1935/live/stream
Command includes: -re, -f flv, proper buffering, GOP size
```

## Implementation Details

### Agent Startup Flow
1. Detect hardware capabilities (CPU, GPU, RAM, node type)
2. Calculate optimal FFmpeg parameters
3. Log hardware detection results
4. Log optimization recommendations
5. Cache optimizations for all jobs

### Job Execution Flow
1. Receive job from master
2. Extract job parameters
3. Apply hardware optimizations (job params override)
4. Determine output mode (file vs RTMP)
5. Build appropriate FFmpeg command
6. Execute with monitoring
7. Collect metrics and return results

### Extensibility
The optimizer is designed for easy extension:
- Add new hardware types (VAAPI, AMF, etc.)
- Add new content analysis (scene complexity, color range)
- Add new transcoding goals (archival, preview, etc.)
- Add new encoders and their optimizations

## Future Enhancements

Potential additions:
1. VAAPI and AMD AMF hardware acceleration support
2. Multi-pass encoding for quality goal
3. Dynamic bitrate adjustment based on content complexity
4. Reconnection logic for RTMP streaming
5. Real-time metrics during streaming (dropped frames, bitrate)
6. Per-scenario optimization profiles
7. Machine learning-based parameter prediction

## Compatibility

- **Backward Compatible**: Existing jobs work without changes
- **Default Behavior**: File transcoding mode if not specified
- **Override Support**: Job parameters always take precedence
- **Go 1.21+**: Uses modern Go features
- **FFmpeg**: Works with standard FFmpeg installations
- **NVENC**: Requires nvidia-smi and supported GPU
- **RTMP**: Requires RTMP server (e.g., nginx-rtmp)

## Summary

This implementation successfully addresses the original requirement to use the most performant/adequate FFmpeg flags for the host, and extends it significantly with:
- Content-aware optimization
- Goal-based optimization
- HDR support
- RTMP streaming capability
- Comprehensive testing
- Extensive documentation

The system now intelligently optimizes FFmpeg parameters based on hardware, content, and goals, while maintaining flexibility through parameter overrides and supporting both file transcoding and live streaming workloads.
