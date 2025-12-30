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
2. Register with master node
3. Poll master for available jobs
4. Execute job (run FFmpeg + collect metrics)
5. Analyze results (energy efficiency scoring)
6. Report results back to master
7. Repeat

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

**⚠️ WARNING**: Only for development/testing!

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

### Command-Line Flags
- `--master`: Master node URL (e.g., `https://192.168.1.100:8080`)
- `--register`: Register with master on startup
- `--poll-interval`: How often to check for new jobs (default: 10s)
- `--heartbeat-interval`: How often to send heartbeat (default: 30s)
- `--api-key`: API key for authentication
- `--allow-master-as-worker`: Allow master to be worker (dev only)
- `--cert`, `--key`: Client certificate for mTLS
- `--ca`: CA cert to verify server
- `--insecure-skip-verify`: Skip TLS verification (insecure)

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

1. **Start exporters**: CPU, GPU, FFmpeg exporters begin collecting metrics
2. **Run FFmpeg**: Execute transcoding with specified parameters
3. **Collect metrics**: Exporters record power, performance data
4. **Analyze results**: Calculate energy efficiency scores
5. **Package results**: Create JSON result file
6. **Report to master**: POST results to master's `/results` endpoint
7. **Poll for next job**: Check master for more work

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
