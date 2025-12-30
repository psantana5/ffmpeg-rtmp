# Hardware-Aware FFmpeg Parameter Optimization

This feature automatically optimizes FFmpeg command parameters based on the hardware capabilities of the worker node executing the job.

## Overview

When a worker agent starts up, it:
1. Detects hardware capabilities (CPU, GPU, RAM, node type)
2. Determines optimal FFmpeg parameters (encoder, preset, flags)
3. Applies these optimizations to all jobs while allowing job-specific overrides

## Hardware Detection

The agent automatically detects:

- **CPU**: Model, thread count
- **GPU**: NVIDIA GPU presence and model (via nvidia-smi)
- **RAM**: Total system memory
- **Node Type**: Laptop, Desktop, or Server (based on battery presence, CPU threads, and RAM)

## Optimization Logic

### Encoder Selection

| Hardware | Encoder | Reason |
|----------|---------|--------|
| NVIDIA GPU present | `h264_nvenc` | Hardware-accelerated encoding for better performance |
| No GPU | `libx264` | Software encoding with optimized preset |

### Preset Selection (CPU-only encoding)

| CPU Threads | Preset | Use Case |
|-------------|--------|----------|
| 16+ | `fast` | High-end systems can afford better quality |
| 8-15 | `fast` | Mid-range balanced performance |
| 4-7 | `veryfast` | Lower-end systems need lighter load |
| < 4 | `ultrafast` | Very limited systems prioritize speed |

### Node Type Adjustments

- **Laptop**: Uses faster presets to reduce thermal/battery impact
- **Desktop**: Balanced settings (default)
- **Server**: Can use slower presets for better quality on sustained workloads

## Usage

### Automatic Application

When the agent starts, it logs the detected hardware and optimization:

```
2025/12/30 12:44:01 Hardware detected:
2025/12/30 12:44:01   CPU: Intel(R) Xeon(R) Platinum 8370C CPU @ 2.80GHz (4 threads)
2025/12/30 12:44:01   RAM: 15.6 GB
2025/12/30 12:44:01   GPU: Not detected
2025/12/30 12:44:01   Node Type: desktop
2025/12/30 12:44:01 Optimizing FFmpeg parameters for this hardware...
2025/12/30 12:44:01   Recommended Encoder: libx264
2025/12/30 12:44:01   Recommended Preset: veryfast
2025/12/30 12:44:01   Optimization Reason: Lower-end CPU (4-7 threads) - using 'veryfast' preset for lighter load (Desktop: balanced performance)
```

### Job Submission

Jobs automatically use optimized parameters:

```bash
# Create a job - parameters are automatically optimized
curl -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-1080p",
    "parameters": {
      "duration": 60,
      "bitrate": "5000k"
    }
  }'
```

The agent will use the hardware-optimized encoder and preset.

### Overriding Optimization

Job-specific parameters always take precedence:

```bash
# Force specific encoder/preset, ignoring hardware optimization
curl -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-1080p",
    "parameters": {
      "codec": "libx265",
      "preset": "slow",
      "bitrate": "5000k"
    }
  }'
```

## Implementation Details

The optimization is implemented in `shared/pkg/agent/ffmpeg_optimizer.go`:

- `OptimizeFFmpegParameters()`: Determines optimal parameters based on hardware
- `ApplyOptimizationToParameters()`: Applies optimizations to job parameters

Job execution in `worker/cmd/agent/main.go`:
- Hardware detection runs once at startup
- Optimizations are cached and applied to all jobs
- Job-specific parameters override optimizations

## Testing

Run the hardware optimization tests:

```bash
# Unit tests
go test -v github.com/psantana5/ffmpeg-rtmp/pkg/agent

# Integration test
bash ./tests/test_hardware_optimization.sh
```

## Examples

### GPU System
```
Detected: NVIDIA RTX 3080
Optimization: encoder=h264_nvenc, preset=medium, hwaccel=nvenc
Reason: NVIDIA GPU detected - using hardware-accelerated NVENC encoder
```

### High-End CPU
```
Detected: AMD Ryzen 9 5950X (24 threads)
Optimization: encoder=libx264, preset=fast
Reason: High-end CPU (16+ threads) - using 'fast' preset for balanced quality/performance
```

### Laptop
```
Detected: Intel Core i7 Mobile (8 threads), Battery present
Optimization: encoder=libx264, preset=veryfast
Reason: Mid-range CPU (8 threads) + Laptop - optimized for thermal/battery efficiency
```

## Benefits

1. **Automatic Performance Optimization**: No manual configuration needed
2. **Hardware Utilization**: GPU acceleration used when available
3. **Appropriate Load**: Preset matches CPU capability
4. **Flexibility**: Job parameters can still override defaults
5. **Transparency**: Optimization reason logged and included in job results
