# RTMP Streaming Support in Worker Agent

The worker agent now supports both **file transcoding** and **RTMP streaming** modes.

## Output Modes

### File Mode (Default)
Transcodes an input file to an output file locally.

```bash
curl -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "file-transcode",
    "parameters": {
      "input": "/path/to/input.mp4",
      "output": "/path/to/output.mp4",
      "codec": "libx264",
      "bitrate": "5000k",
      "duration": 60
    }
  }'
```

### RTMP Streaming Mode
Generates a test pattern and streams it to an RTMP server (e.g., nginx-rtmp).

```bash
curl -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "rtmp-stream",
    "parameters": {
      "output_mode": "rtmp",
      "rtmp_url": "rtmp://localhost:1935/live/test-stream",
      "resolution": "1920x1080",
      "fps": 30,
      "codec": "libx264",
      "bitrate": "5000k",
      "duration": 300
    }
  }'
```

## Parameters

### Common Parameters
- `codec`: Video encoder (e.g., "libx264", "h264_nvenc", "libx265")
- `preset`: Encoding preset (e.g., "ultrafast", "fast", "medium")
- `bitrate`: Target video bitrate (e.g., "2500k", "5000k")
- `duration`: Duration in seconds

### File Mode Parameters
- `output_mode`: Not specified or "file"
- `input`: Input file path (optional, generates test video if not provided)
- `output`: Output file path (optional, uses temp dir if not provided)

### RTMP Streaming Mode Parameters
- `output_mode`: "rtmp" or "stream"
- `rtmp_url`: RTMP destination URL (optional, defaults to `rtmp://localhost:1935/live/{job_id}`)
- `stream_key`: Stream key (optional, defaults to job ID)
- `resolution`: Video resolution (optional, defaults to "1280x720")
- `fps`: Frame rate (optional, defaults to 30)

## FFmpeg Command Examples

### File Transcoding
```bash
ffmpeg -t 60 -i /tmp/test_input.mp4 -c:v libx264 -b:v 5000k -preset fast -y /tmp/output.mp4
```

### RTMP Streaming
```bash
ffmpeg -t 300 -re \
  -f lavfi -i testsrc=size=1920x1080:rate=30 \
  -f lavfi -i sine=frequency=1000:sample_rate=48000 \
  -c:v libx264 -preset fast -tune zerolatency \
  -b:v 5000k -maxrate 5000k -bufsize 5000k \
  -pix_fmt yuv420p -g 60 \
  -c:a aac -b:a 128k -ar 48000 \
  -f flv rtmp://localhost:1935/live/test-stream
```

## Hardware Optimization with RTMP

The hardware optimization is applied automatically to both modes:

```bash
# GPU system with NVENC - automatically selected for RTMP streaming
curl -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "gpu-rtmp-stream",
    "parameters": {
      "output_mode": "rtmp",
      "rtmp_url": "rtmp://localhost:1935/live/gpu-stream",
      "resolution": "3840x2160",
      "fps": 60,
      "bitrate": "15000k",
      "duration": 300
    }
  }'
# Agent will automatically use h264_nvenc or hevc_nvenc if GPU is available
```

## Integration with RTMP Server

### Setup nginx-rtmp Server

```bash
# Install nginx with RTMP module
sudo apt-get install nginx libnginx-mod-rtmp

# Configure nginx
sudo nano /etc/nginx/nginx.conf
```

Add RTMP configuration:
```nginx
rtmp {
    server {
        listen 1935;
        chunk_size 4096;

        application live {
            live on;
            record off;
        }
    }
}
```

Restart nginx:
```bash
sudo systemctl restart nginx
```

### Test Streaming

Start a stream job:
```bash
curl -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-stream",
    "parameters": {
      "output_mode": "rtmp",
      "rtmp_url": "rtmp://localhost:1935/live/mystream",
      "duration": 60
    }
  }'
```

View the stream:
```bash
# Using FFplay
ffplay rtmp://localhost:1935/live/mystream

# Using VLC
vlc rtmp://localhost:1935/live/mystream
```

## Job Results

### File Mode Result
```json
{
  "job_id": "abc123",
  "status": "completed",
  "analyzer_output": {
    "scenario": "file-transcode",
    "output_mode": "file",
    "input_file": "/tmp/test_input.mp4",
    "output_file": "/tmp/output.mp4",
    "output_size": 52428800,
    "codec": "libx264",
    "bitrate": "5000k",
    "preset": "fast",
    "exec_duration": 45.2,
    "optimization_reason": "High-end CPU (8+ threads) - using 'fast' preset for balanced quality/performance (Desktop: balanced performance)",
    "hwaccel": "none"
  }
}
```

### RTMP Streaming Mode Result
```json
{
  "job_id": "xyz789",
  "status": "completed",
  "analyzer_output": {
    "scenario": "rtmp-stream",
    "output_mode": "rtmp",
    "rtmp_url": "rtmp://localhost:1935/live/mystream",
    "codec": "h264_nvenc",
    "bitrate": "5000k",
    "preset": "p4",
    "exec_duration": 60.1,
    "optimization_reason": "NVIDIA GPU detected - using hardware-accelerated NVENC encoder for better performance (Desktop: balanced performance)",
    "hwaccel": "nvenc"
  }
}
```

## Monitoring

Both modes are monitored identically:
- CPU/GPU usage tracked during execution
- Encoding FPS reported in metrics
- Duration and throughput metrics collected
- Results sent back to master upon completion

## Troubleshooting

### RTMP Connection Failed
```
Error: ffmpeg execution failed: exit status 1
FFmpeg stderr: [tcp @ ...] Connection refused
```
**Solution**: Ensure RTMP server is running and accessible:
```bash
# Check if nginx is running
sudo systemctl status nginx

# Check if port 1935 is open
sudo netstat -tlnp | grep 1935
```

### Slow Streaming Performance
**Solution**: Use hardware acceleration if available. The agent will automatically detect and use NVENC/QSV when present.

### High Latency
**Solution**: Use lower preset or enable tune=zerolatency:
```json
{
  "parameters": {
    "output_mode": "rtmp",
    "preset": "ultrafast",
    "codec": "libx264"
  }
}
```
The agent automatically adds `-tune zerolatency` for libx264 in streaming mode.

## Benefits

1. **Unified API**: Single job submission endpoint for both file and streaming workloads
2. **Automatic Optimization**: Hardware detection applies to both modes
3. **Flexible Testing**: Easy to switch between file and streaming for benchmarking
4. **Production Ready**: Suitable for both transcoding services and live streaming platforms
