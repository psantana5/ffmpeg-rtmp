# Worker Agent Examples

This document provides practical examples for running and configuring the worker agent.

## Basic Usage

### Simple Worker with Default Settings

```bash
# Register and start worker with defaults
./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --api-key "your-api-key"
```

This will:
- Auto-detect hardware capabilities and encoders
- Generate input videos as needed (default: enabled)
- Poll for jobs every 10 seconds
- Send heartbeats every 30 seconds
- Use port 9091 for Prometheus metrics

### Worker with Custom Configuration

```bash
# Worker with custom intervals and metrics port
./bin/agent \
  --register \
  --master https://192.168.1.100:8080 \
  --api-key "your-api-key" \
  --poll-interval 5s \
  --heartbeat-interval 15s \
  --metrics-port 9092
```

## Hardware Encoder Examples

### Force Software Encoding

If you want to disable input generation and use your own input files:

```bash
# Disable automatic input generation
./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --api-key "your-api-key" \
  --generate-input=false
```

### Debug Input Generation

Keep generated input files for inspection:

```bash
# Keep inputs for debugging
export PERSIST_INPUTS=true

./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --api-key "your-api-key"

# Generated files remain in /tmp/input_*.mp4
ls -lh /tmp/input_*.mp4
```

### Check Detected Encoders

The agent logs detected encoders at startup:

```bash
./bin/agent --register --master http://localhost:8080 | grep -A 5 "Detecting available encoders"
```

Example output:
```
Detecting available encoders...
  Hardware acceleration: [cuda nvenc vaapi]
  H.264 encoder: h264_nvenc
  H.265 encoder: hevc_nvenc
  Reason: NVIDIA NVENC hardware encoder detected (best performance)
  Available H.264 encoders: [h264_nvenc libx264]
```

## Job Parameter Examples

### Small Test Video (Fast)

Submit a job with minimal resource requirements:

```bash
curl -X POST http://master:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-small",
    "parameters": {
      "resolution_width": 640,
      "resolution_height": 480,
      "frame_rate": 30,
      "duration_seconds": 5,
      "bitrate": "500k",
      "codec": "h264"
    }
  }'
```

Worker will generate a 640x480 @ 30fps input video for 5 seconds.

### Full HD Test Video

```bash
curl -X POST http://master:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-1080p",
    "parameters": {
      "resolution_width": 1920,
      "resolution_height": 1080,
      "frame_rate": 60,
      "duration_seconds": 10,
      "bitrate": "5000k",
      "codec": "h264"
    }
  }'
```

### 4K Test Video

```bash
curl -X POST http://master:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-4k",
    "parameters": {
      "resolution_width": 3840,
      "resolution_height": 2160,
      "frame_rate": 30,
      "duration_seconds": 10,
      "bitrate": "15000k",
      "codec": "h264"
    }
  }'
```

### Using Existing Input File

If you have your own input file:

```bash
curl -X POST http://master:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "custom-input",
    "parameters": {
      "input": "/path/to/your/video.mp4",
      "bitrate": "2000k",
      "codec": "h264"
    }
  }'
```

Worker will skip input generation and use the specified file.

## TLS/HTTPS Examples

### Basic HTTPS (Self-Signed Certificate)

```bash
# For localhost with self-signed cert (auto-enabled)
./bin/agent \
  --register \
  --master https://localhost:8080 \
  --api-key "your-api-key"
```

### HTTPS with CA Certificate

```bash
# Verify server certificate with CA
./bin/agent \
  --register \
  --master https://192.168.1.100:8080 \
  --api-key "your-api-key" \
  --ca /path/to/ca-cert.pem
```

### mTLS (Mutual TLS)

```bash
# Client certificate authentication
./bin/agent \
  --register \
  --master https://192.168.1.100:8080 \
  --api-key "your-api-key" \
  --cert /path/to/client-cert.pem \
  --key /path/to/client-key.pem \
  --ca /path/to/ca-cert.pem
```

### Insecure Mode (Development Only)

```bash
# Skip certificate verification (INSECURE!)
./bin/agent \
  --register \
  --master https://192.168.1.100:8080 \
  --api-key "your-api-key" \
  --insecure-skip-verify
```

## Monitoring Examples

### Check Worker Metrics

```bash
# View Prometheus metrics
curl http://localhost:9091/metrics

# Filter for input generation metrics
curl http://localhost:9091/metrics | grep input_generation

# Example output:
# ffrtmp_worker_input_generation_duration_seconds{node_id="worker1:9091"} 0.23
# ffrtmp_worker_input_file_size_bytes{node_id="worker1:9091"} 1048576
# ffrtmp_worker_total_inputs_generated{node_id="worker1:9091"} 42
```

### Check Worker Health

```bash
# Health endpoint
curl http://localhost:9091/health

# Expected output:
# {"status":"healthy"}
```

### Monitor Worker Logs

```bash
# Systemd service logs
sudo journalctl -u ffmpeg-agent -f

# Look for input generation messages
sudo journalctl -u ffmpeg-agent | grep "Input generated"

# Example:
# ✓ Input video generated: /tmp/input_abc123.mp4 (0.83 MB in 0.18 seconds)
```

## Systemd Service Examples

### Install as Service

```bash
# Copy service file
sudo cp worker/deployment/ffmpeg-agent.service /etc/systemd/system/

# Edit configuration
sudo nano /etc/systemd/system/ffmpeg-agent.service

# Update these lines:
# Environment="MASTER_URL=http://192.168.1.100:8080"
# Environment="MASTER_API_KEY=your-api-key"
# ExecStart=/path/to/bin/agent --register --master ${MASTER_URL} --api-key ${MASTER_API_KEY}

# Reload systemd
sudo systemctl daemon-reload

# Enable and start
sudo systemctl enable ffmpeg-agent
sudo systemctl start ffmpeg-agent

# Check status
sudo systemctl status ffmpeg-agent
```

### Service with Input Persistence

```bash
# Edit service file to persist inputs
sudo nano /etc/systemd/system/ffmpeg-agent.service

# Add environment variable:
Environment="PERSIST_INPUTS=true"

# Reload and restart
sudo systemctl daemon-reload
sudo systemctl restart ffmpeg-agent
```

### Service without Input Generation

```bash
# Edit service file
sudo nano /etc/systemd/system/ffmpeg-agent.service

# Add flag to ExecStart:
ExecStart=/path/to/bin/agent --register --master ${MASTER_URL} --api-key ${MASTER_API_KEY} --generate-input=false

# Reload and restart
sudo systemctl daemon-reload
sudo systemctl restart ffmpeg-agent
```

## Troubleshooting Examples

### Test Encoder Detection

```bash
# Check available encoders
ffmpeg -encoders 2>/dev/null | grep h264

# Expected output includes:
# V..... libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
# V....D h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
# V..... h264_qsv             H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (Intel Quick Sync Video acceleration) (codec h264)
# V..... h264_vaapi           H.264/AVC (VAAPI) (codec h264)
```

### Test Hardware Acceleration

```bash
# List hardware acceleration methods
ffmpeg -hwaccels

# Test NVENC
ffmpeg -y -f lavfi -i testsrc=duration=2 -c:v h264_nvenc /tmp/test_nvenc.mp4

# Test QSV  
ffmpeg -y -f lavfi -i testsrc=duration=2 -c:v h264_qsv /tmp/test_qsv.mp4

# Test VAAPI
ffmpeg -y -f lavfi -i testsrc=duration=2 -c:v h264_vaapi /tmp/test_vaapi.mp4

# Test software (always works)
ffmpeg -y -f lavfi -i testsrc=duration=2 -c:v libx264 -preset ultrafast /tmp/test_sw.mp4
```

### Verify Input Generation

```bash
# Enable persistence and run a single job
export PERSIST_INPUTS=true
./bin/agent --register --master http://localhost:8080

# In another terminal, submit a test job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"scenario":"test","parameters":{"duration_seconds":5}}'

# Check generated file
ls -lh /tmp/input_*.mp4

# Inspect with ffprobe
ffprobe /tmp/input_*.mp4
```

### Debug Hardware Encoder Fallback

```bash
# The worker logs show fallback behavior:
# "Hardware encoder h264_nvenc failed: ..., falling back to libx264"

# Check logs
./bin/agent --register --master http://localhost:8080 2>&1 | grep -i "fallback\|encoder"
```

## Performance Tips

### Optimize for GPU Workers

```bash
# GPU workers should use hardware encoding
# No special flags needed - automatically detected

./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --api-key "your-api-key"

# Check logs confirm NVENC/QSV/VAAPI is used
```

### Optimize for CPU-Only Workers

```bash
# CPU workers automatically use software encoding
# Consider faster poll intervals if master has many jobs

./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --api-key "your-api-key" \
  --poll-interval 5s
```

### Reduce Disk Usage

```bash
# Default: inputs are cleaned up automatically
# No persistence = no disk accumulation

./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --api-key "your-api-key"

# Inputs at /tmp/input_*.mp4 are removed after each job
```

## Docker Examples (Future)

*Note: Docker support is planned for future releases.*

```bash
# Placeholder for future Docker deployment
docker run -d \
  --name ffmpeg-worker \
  --gpus all \
  -e MASTER_URL=http://192.168.1.100:8080 \
  -e MASTER_API_KEY=your-api-key \
  ffmpeg-rtmp/worker:latest
```
