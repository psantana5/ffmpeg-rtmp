# Worker Node Components

This directory contains all components that run on **worker nodes** (compute agents) in a distributed deployment.

## Purpose

Worker nodes are responsible for:
- **Job execution**: Running FFmpeg transcoding workloads
- **Hardware monitoring**: Collecting CPU/GPU power and performance metrics
- **Local metrics**: Tracking resource usage during job execution
- **Results reporting**: Sending results back to master node

## Directory Structure

```
worker/
├── cmd/                       # Agent binary entry point
│   └── agent/                 # Go application main package
├── exporters/                 # Worker-side metrics exporters
│   ├── cpu_exporter/          # CPU power monitoring (RAPL)
│   ├── gpu_exporter/          # GPU power monitoring (NVML)
│   ├── ffmpeg_exporter/       # FFmpeg encoding stats
│   └── docker_stats/          # Docker container metrics
└── deployment/                # Worker deployment configs
    └── ffmpeg-agent.service   # Systemd service file
```

## What Runs on Worker

### 1. Agent Service
- **Binary**: `cmd/agent/main.go`
- **Purpose**: Poll master for jobs, execute transcoding, report results

**Lifecycle**:
1. Detect local hardware (CPU, GPU, RAM)
2. Detect available encoders (h264_nvenc, h264_qsv, h264_vaapi, libx264)
3. Register with master node
4. Poll master for available jobs
5. Generate test input video (if needed)
6. Execute job (run FFmpeg + collect metrics)
7. Cleanup generated inputs
8. Analyze results (energy efficiency scoring)
9. Report results back to master
10. Repeat

### 2. Worker-Side Exporters

These exporters run during job execution to collect real-time metrics:

#### CPU Exporter (Port 9510)
- **Purpose**: Monitor CPU power consumption via Intel RAPL
- **Metrics**: Watts per CPU package/zone
- **Requirements**: Intel CPU with RAPL support, privileged access

#### GPU Exporter (Port 9511)
- **Purpose**: Monitor GPU power consumption via NVIDIA NVML
- **Metrics**: GPU power, utilization, temperature, memory
- **Requirements**: NVIDIA GPU with nvidia-docker runtime

#### FFmpeg Exporter (Port 9506)
- **Purpose**: Real-time FFmpeg encoding statistics
- **Metrics**: FPS, bitrate, frame drops, encoding time

#### Docker Stats Exporter (Port 9501)
- **Purpose**: Track container resource usage
- **Metrics**: Container CPU %, memory %, network I/O

## Building the Worker

```bash
# From repository root
make build-agent

# Or directly with Go
cd /home/runner/work/ffmpeg-rtmp/ffmpeg-rtmp
go build -o bin/agent ./cmd/agent
```

## Running the Worker

### Register and Start Agent

```bash
# Set API key (must match master's API key)
export MASTER_API_KEY="your-api-key-here"

# Register with master and start polling
./bin/agent \
  --register \
  --master https://192.168.1.100:8080 \
  --api-key "$MASTER_API_KEY" \
  --poll-interval 10s \
  --heartbeat-interval 30s
```

### Development Mode (Master as Worker)

** WARNING**: Only for development/testing!

```bash
# Allow master node to also act as worker
./bin/agent \
  --register \
  --master http://localhost:8080 \
  --allow-master-as-worker
```

You'll see a warning about resource contention. This is **not recommended for production**.

### With Systemd

```bash
# Edit service file to set MASTER_URL and API key
sudo nano /etc/systemd/system/ffmpeg-agent.service

# Copy service file
sudo cp deployment/ffmpeg-agent.service /etc/systemd/system/

# Start and enable
sudo systemctl enable ffmpeg-agent
sudo systemctl start ffmpeg-agent

# Check status
sudo systemctl status ffmpeg-agent
```

## Hardware Requirements

### Minimum
- 4 CPU cores (for transcoding)
- 8 GB RAM
- 20 GB disk (for temporary files)
- FFmpeg installed

### Recommended
- 8+ CPU cores (for parallel transcoding)
- 16+ GB RAM
- 50+ GB SSD (for faster I/O)
- GPU (optional, for hardware acceleration)

### For GPU Support
- NVIDIA GPU with CUDA support
- nvidia-docker runtime installed
- NVML library installed

## Hardware-Aware Features

### Automatic Encoder Detection

The worker automatically detects available hardware encoders at startup:

**Priority order for H.264:**
1. `h264_nvenc` (NVIDIA NVENC)
2. `h264_qsv` (Intel Quick Sync Video)
3. `h264_vaapi` (Video Acceleration API)
4. `libx264` (Software fallback)

**Priority order for H.265:**
1. `hevc_nvenc` (NVIDIA NVENC)
2. `hevc_qsv` (Intel Quick Sync Video)
3. `hevc_vaapi` (Video Acceleration API)
4. `libx265` (Software fallback)

The detected encoders are reported during startup and used for both:
- Input video generation (faster with hardware acceleration)
- Transcoding jobs (when hardware encoder is optimal)

### Dynamic Input Video Generation

Workers can automatically generate test input videos for jobs that don't specify an input file:

**Features:**
- Hardware-accelerated generation using detected encoders
- Automatic fallback to software encoding if hardware fails
- Configurable resolution, framerate, and duration from job parameters
- Realistic content with noise filter for better ML model testing
- Automatic cleanup after job completion
- Optional persistence for debugging (PERSIST_INPUTS=true)

**Job Parameters:**
```json
{
  "resolution_width": 1920,
  "resolution_height": 1080,
  "frame_rate": 30,
  "duration_seconds": 10
}
```

**Metrics collected:**
- `input_generation_duration_sec`: Time to generate input
- `input_file_size_bytes`: Size of generated input file
- `input_encoder_used`: Encoder used for generation

**Examples:**

```bash
# Enable input generation (default)
./bin/agent --register --master http://master:8080 --generate-input=true

# Disable input generation (use existing files)
./bin/agent --register --master http://master:8080 --generate-input=false

# Keep generated inputs for debugging
export PERSIST_INPUTS=true
./bin/agent --register --master http://master:8080
```

## Network Requirements

**Outbound only**:
- Workers initiate connections to master
- No inbound ports required (pull-based architecture)

**Master connectivity**:
- Must reach master's HTTP API (port 8080 by default)
- Supports both HTTP and HTTPS

**Firewall**: No special configuration needed (outbound only)

## Worker Deployment Modes

### 1. Systemd Service (Recommended)
- Runs as system service
- Auto-restarts on failure
- Managed by systemd

```bash
sudo systemctl start ffmpeg-agent
```

### 2. Manual (Development)
- Run directly from terminal
- Useful for debugging
- Logs to stdout

```bash
./bin/agent --register --master http://master:8080
```

### 3. Docker (Future)
- Run agent in container
- Portable across systems
- Easier dependency management

```bash
# Not yet implemented
docker run ffmpeg-rtmp/agent --master http://master:8080
```

## Monitoring Workers

### Check Worker Status from Master

```bash
# List all registered workers
curl http://master:8080/nodes

# Check specific worker heartbeat
curl http://master:8080/nodes/<node-id>
```

### Check Worker Logs

```bash
# Systemd
sudo journalctl -u ffmpeg-agent -f

# Manual run
# Logs appear in terminal
```

### Worker Metrics

Workers don't expose a metrics endpoint directly. Instead:
1. Worker exporters (CPU, GPU, FFmpeg) expose metrics
2. Master's VictoriaMetrics scrapes these during job execution
3. Results are sent to master as JSON when job completes

## Worker Configuration

### Environment Variables
- `MASTER_API_KEY`: API key for authentication (required if master has auth enabled)
- `MASTER_URL`: Master node URL (can be set via flag instead)
- `PERSIST_INPUTS`: Set to `true` to keep generated input videos (default: false)

### Command-Line Flags
- `--master`: Master node URL (e.g., `https://192.168.1.100:8080`)
- `--register`: Register with master on startup
- `--poll-interval`: How often to check for new jobs (default: 10s)
- `--heartbeat-interval`: How often to send heartbeat (default: 30s)
- `--api-key`: API key for authentication
- `--generate-input`: Automatically generate input videos for jobs (default: true)
- `--allow-master-as-worker`: Allow master to be worker (dev only)
- `--cert`, `--key`: Client certificate for mTLS
- `--ca`: CA cert to verify server
- `--insecure-skip-verify`: Skip TLS verification (insecure)
- `--metrics-port`: Prometheus metrics port (default: 9091)

## Scaling Workers

### Adding More Workers

Simply deploy the agent on additional machines:

```bash
# On each new machine:
# 1. Clone repo
git clone https://github.com/psantana5/ffmpeg-rtmp.git

# 2. Build agent
make build-agent

# 3. Register with master
export MASTER_API_KEY="same-key-as-other-workers"
./bin/agent --register --master https://master:8080
```

### Removing Workers

```bash
# Stop the agent
sudo systemctl stop ffmpeg-agent

# Worker will stop sending heartbeats
# Master will mark it as unavailable after timeout
```

### Worker Failover

- If worker crashes, master will reassign its job to another worker
- Failed jobs are retried up to 3 times (configurable on master)
- Workers send heartbeats every 30 seconds
- Master marks workers as unavailable after 90 seconds without heartbeat

## Job Execution Workflow

When a worker receives a job:

1. **Determine input needs**: Check if input video generation is required
2. **Generate input (if needed)**: Create test video using hardware-accelerated encoder
   - Uses detected encoders (NVENC, QSV, VAAPI, or fallback to libx264)
   - Configurable resolution, framerate, and duration based on job parameters
   - Records generation metrics (duration, file size, encoder used)
3. **Start exporters**: CPU, GPU, FFmpeg exporters begin collecting metrics
4. **Run FFmpeg**: Execute transcoding with specified parameters
5. **Collect metrics**: Exporters record power, performance data
6. **Cleanup inputs**: Remove generated input files (unless PERSIST_INPUTS=true)
7. **Analyze results**: Calculate energy efficiency scores
8. **Package results**: Create JSON result file with input generation metrics
9. **Report to master**: POST results to master's `/results` endpoint
10. **Poll for next job**: Check master for more work

## Related Documentation

- [FOLDER_ORGANIZATION.md](../FOLDER_ORGANIZATION.md) - Overall project structure
- [../deployment/README.md](../deployment/README.md) - Production deployment guide
- [../docs/DEPLOYMENT_MODES.md](../docs/DEPLOYMENT_MODES.md) - Deployment modes comparison
- [../docs/distributed_architecture_v1.md](../docs/distributed_architecture_v1.md) - Architecture details

## Troubleshooting

### Agent can't connect to master
```bash
# Test master connectivity
curl http://master:8080/health

# Check DNS resolution
ping master

# Check network connectivity
telnet master 8080
```

### Agent won't register
```bash
# Check API key matches master
echo $MASTER_API_KEY

# Check agent logs for error
sudo journalctl -u ffmpeg-agent -n 50

# Try manual registration with verbose output
./bin/agent --register --master http://master:8080 --api-key "key"
```

### No jobs being executed
```bash
# Check if jobs exist on master
curl http://master:8080/jobs

# Check if worker is registered and available
curl http://master:8080/nodes

# Check agent logs
sudo journalctl -u ffmpeg-agent -f
```

### RAPL exporter permission errors
```bash
# RAPL requires privileged access
sudo chmod -R a+r /sys/class/powercap/

# Or run agent with sudo (not recommended)
```

### GPU exporter not working
```bash
# Check nvidia-docker runtime
docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi

# Check NVML library
ldconfig -p | grep libnvidia-ml

# Install nvidia-container-toolkit if missing
```

### Hardware encoder detection issues

**Runtime Validation:**  
The worker now performs runtime validation of hardware encoders, not just compile-time detection.

**Detection Process:**
1. **Compile-time check**: Query `ffmpeg -encoders` to see what's compiled in
2. **Runtime validation**: Run a 0.1s test encode to verify encoder actually works
3. **Selection**: Choose highest-priority encoder that passes runtime validation

**Example logs:**
```bash
=== H.264 Encoder Detection ===
✓ h264_nvenc: detected (compile-time)
  → Running runtime validation for h264_nvenc...
  ✗ h264_nvenc: NOT USABLE - CUDA runtime not available (libcuda.so.1 not found)
✓ h264_qsv: detected (compile-time)
  → Running runtime validation for h264_qsv...
  ✗ h264_qsv: NOT USABLE - encode test failed
✓ libx264: detected (compile-time)
  → No hardware encoders validated, using libx264
```

If hardware encoders are detected but not working:


**Common issues:**
- **NVENC**: Requires CUDA runtime libraries (libcuda.so.1)
  ```bash
  # Check CUDA installation
  ldconfig -p | grep cuda
  
  # Install CUDA runtime (Ubuntu)
  sudo apt-get install nvidia-cuda-toolkit
  ```

- **QSV**: Requires Intel Media SDK or oneVPL
  ```bash
  # Check for Intel GPU
  lspci | grep VGA
  
  # Install oneVPL (Ubuntu 22.04+)
  sudo apt-get install intel-media-va-driver-non-free
  ```

- **VAAPI**: Requires VA-API driver and render device
  ```bash
  # Check VA-API support
  vainfo
  
  # Install VA-API drivers
  sudo apt-get install va-driver-all
  ```

**Prometheus Metrics:**
The worker exposes runtime validation status:
```
ffrtmp_worker_nvenc_available{node_id="worker1"} 0
ffrtmp_worker_qsv_available{node_id="worker1"} 0
ffrtmp_worker_vaapi_available{node_id="worker1"} 0
```

**Fallback Behavior:**  
Worker automatically falls back to software encoding (libx264/libx265) if hardware validation fails. Jobs continue to execute normally.

### Testing encoder availability

**Manual test:**

If input video generation fails:

```bash
# Check FFmpeg installation
which ffmpeg
ffmpeg -version

# Test basic video generation
ffmpeg -f lavfi -i testsrc=duration=2 -c:v libx264 -preset ultrafast test.mp4

# Disable input generation and use existing file
./bin/agent --register --master http://master:8080 --generate-input=false
```

### Input files not being cleaned up

```bash
# Check for leftover input files
ls -lh /tmp/input_*.mp4

# Enable persistence for debugging
export PERSIST_INPUTS=true
./bin/agent --register --master http://master:8080

# Manual cleanup
rm /tmp/input_*.mp4
```

## Production Considerations

1. **Use dedicated compute nodes**: Don't run workers on master
2. **Enable TLS**: Use HTTPS when connecting to master
3. **Set resource limits**: Prevent runaway jobs from consuming all resources
4. **Monitor disk space**: FFmpeg generates temporary files
5. **Use fast storage**: SSD recommended for better I/O during transcoding
6. **Enable GPU**: Use hardware acceleration when available
7. **Network reliability**: Stable connection to master is critical

## Worker Best Practices

- **Homogeneous hardware**: Similar workers make job scheduling easier
- **Regular updates**: Keep agent binary and FFmpeg up-to-date
- **Log monitoring**: Watch for errors and failed jobs
- **Health checks**: Monitor CPU/GPU temperature and throttling
- **Resource isolation**: Use systemd resource limits or cgroups

## Support

For issues specific to worker node components, please include:
- Agent logs: `journalctl -u ffmpeg-agent`
- Agent version: `./bin/agent --version`
- Hardware info: `lscpu`, `nvidia-smi` (if GPU)
- Master connectivity: `curl http://master:8080/health`
