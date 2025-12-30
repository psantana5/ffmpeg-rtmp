# Dual Transcoding Engine Support

## Overview

The FFmpeg-RTMP distributed system now supports **two transcoding engines**:
- **FFmpeg** (default) - Versatile, battle-tested transcoding
- **GStreamer** (new) - Optimized for low-latency live streaming

The system automatically selects the best engine for each job based on workload characteristics, or you can explicitly specify your preferred engine.

## Quick Start

### Submitting a Job with Engine Selection

```bash
# Auto-select best engine (default)
ffrtmp jobs submit --scenario live-stream --engine auto

# Force FFmpeg
ffrtmp jobs submit --scenario transcode --engine ffmpeg

# Force GStreamer
ffrtmp jobs submit --scenario live-rtmp --engine gstreamer
```

### Example Use Cases

**Live Streaming** (GStreamer preferred):
```bash
ffrtmp jobs submit \
  --scenario live-4k \
  --engine auto \
  --queue live \
  --bitrate 5000k \
  --duration 300
```

**File Transcoding** (FFmpeg preferred):
```bash
ffrtmp jobs submit \
  --scenario batch-transcode \
  --engine auto \
  --queue batch \
  --bitrate 3000k
```

## Engine Selection Logic

When `--engine auto` is used (default), the system intelligently selects the best engine:

### 1. Queue-Based Selection
- **LIVE queue** → GStreamer (optimized for low latency)
- **FILE/batch queue** → FFmpeg (better for offline processing)

### 2. Output Mode Selection
- **RTMP/stream output** → GStreamer
- **File output** → FFmpeg

### 3. Hardware-Based Selection
- **GPU workers with NVENC + streaming** → GStreamer
- **CPU-only workers** → FFmpeg

### 4. Explicit Preference
- Job parameter `engine` always takes precedence

## Engine Capabilities

### FFmpeg Engine

**Strengths:**
- Comprehensive codec support
- Mature and stable
- Excellent for file-based transcoding
- Wide format support

**Hardware Acceleration:**
- NVIDIA NVENC (h264_nvenc, h265_nvenc)
- Intel QSV
- Software fallback (libx264, libx265)

**Use Cases:**
- File transcoding
- Batch processing
- Archive conversion
- Complex filter chains

### GStreamer Engine

**Strengths:**
- Low-latency streaming
- Efficient pipeline architecture
- Native RTMP support
- Hardware-optimized

**Hardware Acceleration:**
- NVIDIA NVENC (nvh264enc, nvh265enc)
- Intel VAAPI (vaapih264enc)
- Intel QSV (qsvh264enc)
- Software fallback (x264enc)

**Use Cases:**
- Live RTMP streaming
- Real-time transcoding
- Low-latency scenarios
- Continuous streaming

## Worker Configuration

### Installing GStreamer (Optional)

GStreamer is optional. If not installed, the worker automatically falls back to FFmpeg.

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y \
  gstreamer1.0-tools \
  gstreamer1.0-plugins-base \
  gstreamer1.0-plugins-good \
  gstreamer1.0-plugins-bad \
  gstreamer1.0-plugins-ugly
```

**With NVIDIA GPU support:**
```bash
sudo apt-get install -y gstreamer1.0-plugins-bad
```

**macOS:**
```bash
brew install gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-plugins-ugly
```

### Starting a Worker

Workers automatically detect available engines:

```bash
# Worker will support both FFmpeg and GStreamer if installed
./bin/agent --register --master http://master:8080

# Output shows available engines:
# Available engines: [ffmpeg gstreamer]
```

## API Usage

### Job Submission with Engine Selection

```json
POST /jobs
{
  "scenario": "live-stream-4k",
  "engine": "auto",
  "queue": "live",
  "priority": "high",
  "parameters": {
    "bitrate": "5000k",
    "duration": 300,
    "output_mode": "rtmp"
  }
}
```

### Engine Field Values
- `"auto"` (default) - Intelligent selection
- `"ffmpeg"` - Force FFmpeg
- `"gstreamer"` - Force GStreamer (falls back to FFmpeg if unavailable)

## Monitoring

### VictoriaMetrics Integration

Engine metrics are automatically exported to VictoriaMetrics via Prometheus format:

**Available Metrics**:
- `ffrtmp_jobs_by_engine{engine="ffmpeg|gstreamer|auto"}` - Total jobs by engine preference
- `ffrtmp_jobs_completed_by_engine{engine="ffmpeg|gstreamer"}` - Completed jobs by actual engine used

These metrics are scraped from the master node's `/metrics` endpoint (default port 9090).

### Grafana Dashboards

The **Distributed Scheduler** dashboard includes engine visualization panels:

1. **Jobs by Engine Preference** (Pie Chart)
   - Shows distribution of job engine preferences (auto/ffmpeg/gstreamer)
   - Located in the distributed-scheduler dashboard

2. **Completed Jobs by Engine** (Time Series)
   - Shows actual engine usage over time
   - Helps track FFmpeg vs GStreamer adoption
   - Color-coded: FFmpeg (blue), GStreamer (green)

**Accessing Grafana**:
```bash
# Default URL
http://master-node:3000

# View distributed-scheduler dashboard
http://master-node:3000/d/distributed-scheduler
```

### Engine Metrics in Job Results

Job results include engine information:

```json
{
  "job_id": "abc123",
  "status": "completed",
  "metrics": {
    "duration": 45.2,
    "engine": "gstreamer",
    "frames_encoded": 900
  }
}
```

### Worker Logs

Engine selection is logged for debugging:

```
Selected engine: gstreamer
Selection reason: LIVE queue job - GStreamer preferred for low-latency streaming
```

## Performance Comparison

| Workload | FFmpeg | GStreamer | Recommended |
|----------|--------|-----------|-------------|
| Live RTMP Streaming | ✅ Good | ✅✅ Excellent | GStreamer |
| File Transcoding | ✅✅ Excellent | ✅ Good | FFmpeg |
| Low Latency (<100ms) | ✅ Good | ✅✅ Excellent | GStreamer |
| Batch Processing | ✅✅ Excellent | ✅ Good | FFmpeg |
| Complex Filters | ✅✅ Excellent | ✅ Limited | FFmpeg |
| NVENC Hardware Accel | ✅✅ Excellent | ✅✅ Excellent | Either |

## Troubleshooting

### GStreamer Not Found

If you see:
```
GStreamer not found, falling back to FFmpeg
```

**Solution:** Install GStreamer (see "Installing GStreamer" above) or use FFmpeg explicitly.

### Engine Selection Not Working as Expected

Check the worker logs for selection reasoning:
```bash
tail -f /var/log/worker.log | grep "Engine selection"
```

### Performance Issues

- For live streaming: Ensure GStreamer is installed
- For batch jobs: FFmpeg is usually faster
- For GPU acceleration: Verify GPU drivers are installed

## Advanced Configuration

### Forcing Engine Per Queue

You can set a default engine preference in job parameters:

```bash
# Always use GStreamer for live queue
ffrtmp jobs submit --scenario live --queue live --engine gstreamer
```

### Custom Scenarios

Create custom scenarios that specify preferred engines:

```json
{
  "scenario": "ultra-low-latency",
  "engine": "gstreamer",
  "parameters": {
    "output_mode": "rtmp",
    "tune": "zerolatency"
  }
}
```

## Migration Guide

### From FFmpeg-Only Setup

No changes required! Existing jobs continue to work:
- Jobs without `engine` field default to `auto`
- `auto` selection prefers FFmpeg for file workloads
- No behavior change for existing workflows

### Adding GStreamer Support

1. Install GStreamer on workers (optional)
2. Restart workers to detect engines
3. Submit jobs with `--engine auto` for intelligent selection

## Best Practices

1. **Use `auto` for most workloads** - Let the system choose
2. **Specify `gstreamer` for live streaming** - Better latency
3. **Specify `ffmpeg` for complex filters** - More features
4. **Monitor engine metrics** - Track which engine performs better
5. **Test both engines** - Compare results for your specific use case

## Future Enhancements

Planned features:
- Per-engine metrics dashboards
- A/B testing framework
- Engine performance scoring
- Additional engine support (hardware transcoders)

## Support

For issues or questions:
- Check worker logs for engine selection reasoning
- Verify GStreamer installation: `gst-launch-1.0 --version`
- Test FFmpeg: `ffmpeg -version`
- Report issues with engine logs attached
