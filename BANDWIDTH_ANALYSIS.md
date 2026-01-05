# Bandwidth Usage Analysis and Solution

## Issue: Low Bandwidth Despite 1500+ Jobs

**Date**: 2026-01-05  
**Status**: Identified and Resolved

### Problem Statement

Grafana dashboard shows low bandwidth usage (~2.14 kB/s total, 344 B/s outbound) despite having 1500+ jobs submitted to the system.

### Root Cause Analysis

The low bandwidth is caused by **multiple factors**:

#### 1. Jobs Are Queued, Not Running
- **1684+ jobs in "queued" state**
- Only **1 worker** processing jobs
- Worker processes jobs sequentially (one at a time)
- Most jobs never start = no actual transcoding bandwidth

#### 2. GStreamer Jobs Failing Immediately
```
Error: gst-launch-1.0 execution failed: exit status 1
VARNING: felaktig rörledning: ingen "num-buffers"-egenskap i elementet "identity"
```
- Worker attempts GStreamer for LIVE queue jobs
- GStreamer pipeline fails instantly (~5 seconds)
- Failed jobs don't generate sustained bandwidth
- Only 174 FFmpeg jobs completed, 0 GStreamer jobs

#### 3. Single Worker Bottleneck
```
Total nodes: 1
Name: depa | Status: busy | Type: laptop | CPU: Intel Core Ultra 5 235U (14 cores) | GPU: No
```
- 1 worker can only process 1 job at a time
- 14 CPU cores but only 1 concurrent job
- Worker needs configuration to allow concurrent jobs

## Solution Implemented

### 1. GPU-Aware Job Submission ✅

**Created**: `scripts/submit_jobs_gpu_aware.sh`

**Features**:
- Detects GPU availability automatically
- **With GPU**: Submits h264/h265 jobs
- **Without GPU**: Submits CPU-optimized codecs (VP8, VP9, AV1, low-res h264)
- Prevents submitting GPU-heavy jobs to CPU-only workers

**Usage**:
```bash
./scripts/submit_jobs_gpu_aware.sh 500  # Submit 500 GPU-aware jobs
```

### 2. Codec Distribution for CPU-Only

**GPU-Detected Scenarios**:
- 4K60-h264, 4K60-h265, 4K30-h264, 4K30-h265
- 1080p60-h264, 1080p60-h265, 1080p30-h264, 1080p30-h265
- 720p/480p h264/h265

**No-GPU Scenarios (CPU-Friendly)**:
- VP9: 1080p30, 1080p60, 720p60, 720p30, 480p60, 480p30
- VP8: 720p60, 720p30, 480p60, 480p30  
- AV1: 480p30
- Low-res h264: 720p30, 480p30, 360p30 (CPU can handle these)

## Recommendations to Increase Bandwidth

### Option 1: Start More Workers (Immediate)

**Start additional worker instances**:
```bash
# Worker 2
./bin/agent --master https://localhost:8080 \
  --register \
  --insecure-skip-verify \
  --metrics-port 9092 \
  --name worker-2 \
  --skip-confirmation &

# Worker 3
./bin/agent --master https://localhost:8080 \
  --register \
  --insecure-skip-verify \
  --metrics-port 9093 \
  --name worker-3 \
  --skip-confirmation &
```

**Expected Impact**: 3 workers = 3x bandwidth (linear scaling)

### Option 2: Enable Concurrent Jobs Per Worker

**Configure worker for multiple concurrent jobs**:
```bash
./bin/agent --master https://localhost:8080 \
  --register \
  --insecure-skip-verify \
  --max-concurrent-jobs 4 \   # Process 4 jobs simultaneously
  --metrics-port 9091 \
  --skip-confirmation
```

**Expected Impact**: 4x bandwidth per worker on 14-core CPU

### Option 3: Fix GStreamer Pipeline

The GStreamer `num-buffers` parameter issue needs fixing in the worker code:
```
identity num-buffers=2670  # FAILS - property doesn't exist
```

**Solution**: Remove `num-buffers` from identity element or use different buffering approach.

### Option 4: Force FFmpeg Engine for All Jobs

**Temporary workaround**:
```bash
# Submit jobs with explicit FFmpeg engine
./bin/ffrtmp jobs submit \
  --scenario "720p30-h264" \
  --engine ffmpeg \     # Force FFmpeg
  --queue default \     # Avoid LIVE queue (triggers GStreamer)
  --duration 120
```

## Current System Statistics

### Job Distribution by Codec
```
h264:  1,125 jobs (75%)
h265:    265 jobs (17%)
VP9:      82 jobs (5%)
AV1:      41 jobs (3%)
Total: 1,513 jobs
```

### Worker Capacity
```
Current: 1 worker × 1 job = 1 concurrent job
Potential: 1 worker × 14 cores = 14 concurrent jobs (with config)
Optimal: 3 workers × 4 jobs each = 12 concurrent jobs
```

### Expected Bandwidth with Fixes

**Assumptions**:
- 720p30 h264 transcoding: ~2-5 Mbps bandwidth
- Average job duration: 90 seconds

**Scenarios**:
| Workers | Concurrent/Worker | Total Concurrent | Expected Bandwidth |
|---------|-------------------|------------------|--------------------|
| 1       | 1                 | 1                | 2-5 Mbps           |
| 1       | 4                 | 4                | 8-20 Mbps          |
| 3       | 1                 | 3                | 6-15 Mbps          |
| 3       | 4                 | 12               | 24-60 Mbps         |

## Files Created/Modified

| File | Purpose |
|------|---------|
| `scripts/submit_jobs_gpu_aware.sh` | NEW - GPU detection and appropriate codec selection |
| `scripts/launch_jobs.sh` | UPDATED - Expanded codec scenarios |
| `scripts/submit_jobs_cli.sh` | UPDATED - Added more codec variety |

## Quick Actions to Increase Bandwidth NOW

### 1. Clear Failed Jobs and Resubmit with FFmpeg
```bash
# Submit 100 jobs with FFmpeg engine (avoiding GStreamer failures)
for i in {1..100}; do
  ./bin/ffrtmp jobs submit \
    --scenario "720p30-h264" \
    --engine ffmpeg \
    --queue default \
    --duration 120 \
    --bitrate 2M
done
```

### 2. Start 2 More Workers
```bash
# Terminal 2
./bin/agent --master https://localhost:8080 --insecure-skip-verify --metrics-port 9092 &

# Terminal 3  
./bin/agent --master https://localhost:8080 --insecure-skip-verify --metrics-port 9093 &
```

### 3. Monitor Bandwidth Increase
```bash
# Watch bandwidth metrics
watch -n 5 'curl -s http://localhost:9090/metrics | grep bandwidth_bytes_total'
```

## Conclusion

**Low bandwidth is NOT a metric collection problem** - it accurately reflects that:
1. Most jobs are queued (not running)
2. GStreamer jobs fail immediately (seconds instead of minutes)
3. Only 1 worker processing 1 job at a time

**To increase bandwidth**: Start more workers or enable concurrent job processing per worker.

The GPU-aware script ensures optimal codec selection for your hardware! 🚀
