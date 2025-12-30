# RTMP Streaming to Master Node - Implementation Details

## Problem Statement

The initial RTMP streaming implementation had two issues:
1. **Wrong destination**: Defaulted to `rtmp://localhost:1935` which means workers would try to stream to themselves
2. **Missing optimizations**: Not all hardware optimization flags were applied to RTMP streaming commands

## Solution

### 1. Master URL Detection for RTMP

**Implementation**: Workers now automatically construct the RTMP URL pointing to the master node.

```go
// Store master URL at agent startup
globalMasterURL = *masterURL  // e.g., "https://master.example.com:8080"

// When building RTMP URL
masterHost := "localhost"
if globalMasterURL != "" {
    parsedURL, err := url.Parse(globalMasterURL)
    if err == nil && parsedURL.Host != "" {
        // Extract hostname (remove API port)
        host := parsedURL.Host
        if colonIdx := strings.Index(host, ":"); colonIdx > 0 {
            host = host[:colonIdx]
        }
        masterHost = host
    }
}
// RTMP server runs on master at port 1935
rtmpURL = fmt.Sprintf("rtmp://%s:1935/live/%s", masterHost, streamKey)
```

**Result**: 
- Worker started with `--master https://10.0.1.5:8080` will stream to `rtmp://10.0.1.5:1935/live/{stream_key}`
- Worker started with `--master https://master.local:8080` will stream to `rtmp://master.local:1935/live/{stream_key}`

### 2. Complete Hardware Optimization Application

**Problem**: The initial RTMP command only used `codec` and `preset`, but ignored `ffmpegOpt.ExtraFlags` which contains critical optimization parameters like:
- AQ modes for better quality
- B-frame settings for compression
- Tune options for streaming
- Thread allocation
- Encoder-specific optimizations

**Solution**: Updated RTMP command builder to apply ALL hardware optimization flags:

```go
// Apply hardware-optimized extra flags from ffmpegOpt
log.Printf("Applying hardware optimization flags to RTMP stream...")
for key, value := range ffmpegOpt.ExtraFlags {
    switch key {
    case "tune":
        // Apply tune for software encoders (e.g., zerolatency)
        if codec == "libx264" || codec == "libx265" {
            args = append(args, "-tune", value)
        }
    case "threads":
        args = append(args, "-threads", value)
    case "rc", "spatial-aq", "temporal-aq", "bf", "zerolatency":
        // NVENC-specific flags
        if strings.Contains(codec, "nvenc") {
            args = append(args, fmt.Sprintf("-%s", key), value)
        }
    case "aq-mode", "no-sao", "bframes", "rd":
        // HEVC-specific flags via x265-params
        if codec == "libx265" {
            args = append(args, "-x265-params", fmt.Sprintf("%s=%s", key, value))
        }
    case "me":
        // Motion estimation for x264
        if codec == "libx264" {
            args = append(args, "-me_method", value)
        }
    }
}
```

## Example Scenarios

### Scenario 1: GPU Worker Streaming to Master

**Setup**:
```bash
# On master node (10.0.1.5)
./bin/master --port 8080
# RTMP server listening on port 1935

# On worker node with GPU
./bin/agent --register --master https://10.0.1.5:8080
```

**Job Submission**:
```json
{
  "scenario": "gpu-stream-test",
  "parameters": {
    "output_mode": "rtmp",
    "resolution": "1920x1080",
    "fps": 60,
    "bitrate": "8000k",
    "duration": 300
  }
}
```

**Generated Command** (with hardware optimization):
```bash
ffmpeg -re \
  -f lavfi -i testsrc=size=1920x1080:rate=60 \
  -f lavfi -i sine=frequency=1000:sample_rate=48000 \
  -t 300 \
  -c:v h264_nvenc \          # Hardware encoder detected
  -preset p4 \                # Balanced NVENC preset
  -rc vbr \                   # Variable bitrate (from optimization)
  -spatial-aq 1 \             # Spatial AQ (from optimization)
  -temporal-aq 1 \            # Temporal AQ (from optimization)
  -bf 3 \                     # B-frames optimized for content
  -b:v 8000k \
  -maxrate 8000k \
  -bufsize 8000k \
  -pix_fmt yuv420p \
  -g 120 \                    # 2-second GOP for 60fps
  -c:a aac -b:a 128k -ar 48000 \
  -f flv \
  rtmp://10.0.1.5:1935/live/{job_id}
```

**Log Output**:
```
2025/12/30 14:00:00 RTMP streaming mode enabled: rtmp://10.0.1.5:1935/live/abc123
2025/12/30 14:00:00   Streaming to master node RTMP server
2025/12/30 14:00:00 Applying hardware optimization flags to RTMP stream...
2025/12/30 14:00:00   Added -rc vbr (NVENC optimization)
2025/12/30 14:00:00   Added -spatial-aq 1 (NVENC optimization)
2025/12/30 14:00:00   Added -temporal-aq 1 (NVENC optimization)
2025/12/30 14:00:00   Added -bf 3 (NVENC optimization)
2025/12/30 14:00:00 RTMP command built with 4 optimization flags applied
```

### Scenario 2: CPU Worker Streaming to Master

**Setup**:
```bash
# Worker node without GPU, 16 CPU cores
./bin/agent --register --master https://master.local:8080
```

**Job Submission**:
```json
{
  "scenario": "cpu-stream-test",
  "parameters": {
    "output_mode": "rtmp",
    "resolution": "1280x720",
    "fps": 30,
    "bitrate": "3000k",
    "duration": 600
  }
}
```

**Generated Command** (with hardware optimization):
```bash
ffmpeg -re \
  -f lavfi -i testsrc=size=1280x720:rate=30 \
  -f lavfi -i sine=frequency=1000:sample_rate=48000 \
  -t 600 \
  -c:v libx264 \              # Software encoder (no GPU)
  -preset fast \              # Optimized for 16 cores
  -tune zerolatency \         # Low-latency streaming (from optimization)
  -threads 14 \               # Leave 2 cores for system (from optimization)
  -b:v 3000k \
  -maxrate 3000k \
  -bufsize 3000k \
  -pix_fmt yuv420p \
  -g 60 \                     # 2-second GOP for 30fps
  -c:a aac -b:a 128k -ar 48000 \
  -f flv \
  rtmp://master.local:1935/live/{job_id}
```

**Log Output**:
```
2025/12/30 14:00:00 RTMP streaming mode enabled: rtmp://master.local:1935/live/xyz789
2025/12/30 14:00:00   Streaming to master node RTMP server
2025/12/30 14:00:00 Applying hardware optimization flags to RTMP stream...
2025/12/30 14:00:00   Added -tune zerolatency (from hardware optimization)
2025/12/30 14:00:00   Added -threads 14 (from hardware optimization)
2025/12/30 14:00:00 RTMP command built with 2 optimization flags applied
```

## Verification

To verify the implementation is correct:

### 1. Check Master URL Extraction
```go
// Test with different master URLs
globalMasterURL = "https://10.0.1.5:8080"       → masterHost = "10.0.1.5"
globalMasterURL = "http://master.local:8080"    → masterHost = "master.local"
globalMasterURL = "https://192.168.1.10:443"    → masterHost = "192.168.1.10"
```

### 2. Check Optimization Flags Applied
Look for log messages:
```
Applying hardware optimization flags to RTMP stream...
  Added -tune zerolatency (from hardware optimization)
  Added -spatial-aq 1 (NVENC optimization)
  Added x265 param aq-mode=3 (from hardware optimization)
RTMP command built with N optimization flags applied
```

### 3. Monitor Stream on Master
```bash
# On master node, check RTMP connections
tail -f /var/log/nginx/access.log | grep rtmp

# View stream
ffplay rtmp://localhost:1935/live/{stream_key}
```

## Benefits

1. **Correct Routing**: Workers always stream to master, not themselves
2. **Full Optimization**: All hardware-specific flags are applied to streaming
3. **Automatic Detection**: No manual configuration of RTMP URLs needed
4. **Transparent**: Detailed logging shows exactly what optimizations are applied
5. **Flexible Override**: Can still specify custom `rtmp_url` if needed

## Architecture

```
┌─────────────────┐
│  Master Node    │
│                 │
│  ┌───────────┐  │
│  │ API:8080  │  │
│  └───────────┘  │
│                 │
│  ┌───────────┐  │
│  │ RTMP:1935 │◄─┼─── Worker streams here
│  └───────────┘  │
└─────────────────┘
        ▲
        │ Job execution
        │ (with optimized params)
        │
┌───────┴──────────┐
│  Worker Node     │
│                  │
│  ┌────────────┐  │
│  │ Agent      │  │
│  │ + FFmpeg   │  │
│  └────────────┘  │
└──────────────────┘
```

## Troubleshooting

### Worker streams to localhost instead of master
**Cause**: `globalMasterURL` not set
**Solution**: Ensure agent is started with `--master` flag

### Optimization flags not applied
**Cause**: Hardware detection failed
**Solution**: Check hardware detection logs at agent startup

### RTMP connection refused
**Cause**: RTMP server not running on master
**Solution**: Ensure nginx-rtmp or similar is running on master port 1935

### Wrong master host extracted
**Cause**: Master URL parsing issue
**Solution**: Use standard URL format: `https://hostname:port` or `http://hostname:port`
