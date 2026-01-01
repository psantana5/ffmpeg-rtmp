# Worker Exporters - Manual Deployment Guide

This guide explains how to deploy and run worker node exporters **without Docker**, for production environments where you want direct control over the services.

## Overview

Worker exporters collect real-time hardware metrics during transcoding jobs. These exporters are:

1. **CPU Exporter** (Port 9510) - Intel RAPL power monitoring
2. **GPU Exporter** (Port 9511) - NVIDIA GPU metrics via NVML
3. **FFmpeg Exporter** (Port 9506) - Real-time FFmpeg encoding stats
4. **Docker Stats Exporter** (Port 9501) - Container resource usage
5. **Prometheus Exporter** (Port varies) - General-purpose Prometheus exporter

Most worker exporters are written in **Go** for high performance and low overhead.

---

## Quick Start

### Prerequisites

- **Go 1.21+** (for building Go exporters)
- **Python 3.10+** (for Docker Stats exporter)
- **Linux** with kernel 4.15+ (for RAPL power monitoring)
- **NVIDIA GPU + drivers** (for GPU exporter, optional)
- **Root/sudo access** (for privileged operations like RAPL reading)

### Build Go Exporters

```bash
cd /path/to/ffmpeg-rtmp

# Build CPU exporter
go build -o bin/cpu-exporter ./worker/exporters/cpu_exporter

# Build GPU exporter (requires NVIDIA)
go build -o bin/gpu-exporter ./worker/exporters/gpu_exporter

# Build FFmpeg exporter
go build -o bin/ffmpeg-exporter ./worker/exporters/ffmpeg_exporter
```

### Run Exporters Manually

```bash
# CPU Exporter (requires privileged access to /sys/class/powercap)
sudo ./bin/cpu-exporter --port 9510

# GPU Exporter (requires NVIDIA GPU and drivers)
./bin/gpu-exporter --port 9511

# FFmpeg Exporter
./bin/ffmpeg-exporter --port 9506

# Docker Stats Exporter (Python)
python3 worker/exporters/docker_stats/docker_stats_exporter.py --port 9501
```

---

## Detailed Exporter Configuration

### 1. CPU Exporter (Go)

**Purpose**: Monitors CPU power consumption via Intel RAPL (Running Average Power Limit).

**Port**: 9510 (default: 9500, mapped to 9510 in docker-compose)

**Requirements**:
- Intel CPU with RAPL support (Sandy Bridge or newer)
- Access to `/sys/class/powercap` directory
- Privileged execution or appropriate capabilities

**Command-line Options**:
- `--port`: HTTP server port (default: 9500)

**Environment Variables**:
- `CPU_EXPORTER_PORT`: Alternative way to set port

**Build**:
```bash
cd /path/to/ffmpeg-rtmp
go build -o bin/cpu-exporter ./worker/exporters/cpu_exporter/main.go
```

**Run**:
```bash
# Method 1: With sudo (full access)
sudo CPU_EXPORTER_PORT=9510 ./bin/cpu-exporter

# Method 2: With capabilities (preferred)
sudo setcap cap_dac_read_search=+ep ./bin/cpu-exporter
./bin/cpu-exporter --port 9510

# Method 3: With specific permissions
sudo chmod -R a+r /sys/class/powercap
./bin/cpu-exporter --port 9510
```

**Metrics Exposed**:
- `rapl_power_watts{package="X", zone="Y"}`: Current power in watts
- `rapl_energy_joules_total{package="X", zone="Y"}`: Total energy consumed

**Health Check**:
```bash
curl http://localhost:9510/health
# Expected: {"status": "ok"}
```

**Troubleshooting**:
- If no zones are found, check: `ls /sys/class/powercap/intel-rapl*`
- Ensure Intel RAPL is enabled in BIOS
- For AMD CPUs, RAPL support is limited

---

### 2. GPU Exporter (Go)

**Purpose**: Monitors NVIDIA GPU power, temperature, utilization, and memory usage via NVML.

**Port**: 9511 (default: 9505, mapped to 9511 in docker-compose)

**Requirements**:
- NVIDIA GPU (compute capability 3.5+)
- NVIDIA drivers installed
- `nvidia-smi` command available

**Command-line Options**:
- `--port`: HTTP server port (default: 9505)

**Environment Variables**:
- `GPU_EXPORTER_PORT`: Alternative way to set port

**Build**:
```bash
cd /path/to/ffmpeg-rtmp
go build -o bin/gpu-exporter ./worker/exporters/gpu_exporter/main.go
```

**Run**:
```bash
GPU_EXPORTER_PORT=9511 ./bin/gpu-exporter

# Or with flag
./bin/gpu-exporter --port 9511
```

**Metrics Exposed**:
- `nvidia_gpu_power_draw_watts{gpu="0"}`: Current power draw
- `nvidia_gpu_temperature_celsius{gpu="0"}`: GPU temperature
- `nvidia_gpu_utilization_percent{gpu="0"}`: GPU utilization
- `nvidia_gpu_memory_used_bytes{gpu="0"}`: Memory used
- `nvidia_gpu_memory_total_bytes{gpu="0"}`: Total memory

**Health Check**:
```bash
curl http://localhost:9511/health
```

**Troubleshooting**:
- Verify NVIDIA drivers: `nvidia-smi`
- Check GPU detection: `nvidia-smi -L`
- Ensure user has access to GPU (may need to add user to `video` group)

---

### 3. FFmpeg Exporter (Go)

**Purpose**: Exposes real-time FFmpeg encoding statistics from FFmpeg log output.

**Port**: 9506 (default)

**Requirements**:
- FFmpeg running and generating stats
- Ability to parse FFmpeg output

**Command-line Options**:
- `--port`: HTTP server port (default: 9506)

**Environment Variables**:
- `FFMPEG_EXPORTER_PORT`: Alternative way to set port

**Build**:
```bash
cd /path/to/ffmpeg-rtmp
go build -o bin/ffmpeg-exporter ./worker/exporters/ffmpeg_exporter/main.go
```

**Run**:
```bash
FFMPEG_EXPORTER_PORT=9506 ./bin/ffmpeg-exporter

# Or with flag
./bin/ffmpeg-exporter --port 9506
```

**How It Works**:
The exporter reads FFmpeg progress output and exposes metrics. FFmpeg must be run with progress output enabled:

```bash
ffmpeg -i input.mp4 -c:v libx264 output.mp4 -progress pipe:1 2>&1 | \
    tee >(curl -X POST --data-binary @- http://localhost:9506/ingest)
```

Or configure your transcoding scripts to send FFmpeg output to the exporter.

**Metrics Exposed**:
- `ffmpeg_fps`: Frames per second being processed
- `ffmpeg_speed`: Encoding speed (1.0 = realtime)
- `ffmpeg_bitrate_kbps`: Current output bitrate
- `ffmpeg_total_frames`: Total frames processed
- `ffmpeg_dropped_frames`: Number of dropped frames
- `ffmpeg_stream_active`: Whether stream is active (1=yes, 0=no)

**Health Check**:
```bash
curl http://localhost:9506/health
```

---

### 4. Docker Stats Exporter (Python)

**Purpose**: Monitors Docker container resource usage (CPU, memory, network).

**Port**: 9501 (default)

**Requirements**:
- Docker daemon running
- Access to Docker socket (`/var/run/docker.sock`)
- Python 3.10+

**Command-line Options**:
- `--port`: HTTP server port (default: 9501)

**Environment Variables**:
- `DOCKER_STATS_PORT`: Server port

**Install Dependencies**:
```bash
pip install requests  # If not already installed
```

**Run**:
```bash
DOCKER_STATS_PORT=9501 python3 worker/exporters/docker_stats/docker_stats_exporter.py

# Or with flag
python3 worker/exporters/docker_stats/docker_stats_exporter.py --port 9501
```

**Permissions**:
```bash
# Add user to docker group to avoid sudo
sudo usermod -aG docker $USER
newgrp docker

# Or run with sudo
sudo python3 worker/exporters/docker_stats/docker_stats_exporter.py --port 9501
```

**Metrics Exposed**:
- `docker_container_cpu_percent{container="name"}`: CPU usage percentage
- `docker_container_memory_bytes{container="name"}`: Memory usage
- `docker_container_network_rx_bytes{container="name"}`: Network RX
- `docker_container_network_tx_bytes{container="name"}`: Network TX

**Health Check**:
```bash
curl http://localhost:9501/health
```

---

## Production Deployment with Systemd

### 1. Create Service User

```bash
# Create a dedicated user for worker exporters
sudo useradd --system --no-create-home --shell /bin/false ffmpeg-worker

# Add to necessary groups
sudo usermod -aG video ffmpeg-worker     # For GPU access
sudo usermod -aG docker ffmpeg-worker    # For Docker stats
```

### 2. Create Directory Structure

```bash
# Create directories
sudo mkdir -p /opt/ffmpeg-rtmp/worker/bin
sudo mkdir -p /var/log/ffmpeg-worker

# Build and copy binaries
cd /path/to/ffmpeg-rtmp
make build-agent  # This builds worker components

# Copy binaries
sudo cp bin/cpu-exporter /opt/ffmpeg-rtmp/worker/bin/
sudo cp bin/gpu-exporter /opt/ffmpeg-rtmp/worker/bin/
sudo cp bin/ffmpeg-exporter /opt/ffmpeg-rtmp/worker/bin/

# Copy Python exporter
sudo cp -r worker/exporters/docker_stats /opt/ffmpeg-rtmp/worker/
sudo cp requirements.txt /opt/ffmpeg-rtmp/worker/

# Set ownership
sudo chown -R ffmpeg-worker:ffmpeg-worker /opt/ffmpeg-rtmp/worker
sudo chown -R ffmpeg-worker:ffmpeg-worker /var/log/ffmpeg-worker

# Set capabilities for CPU exporter
sudo setcap cap_dac_read_search=+ep /opt/ffmpeg-rtmp/worker/bin/cpu-exporter
```

### 3. Create Systemd Service Files

#### CPU Exporter Service

Create `/etc/systemd/system/ffmpeg-cpu-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP CPU Power Exporter (RAPL)
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-worker
Group=ffmpeg-worker
WorkingDirectory=/opt/ffmpeg-rtmp/worker

# Environment
Environment="CPU_EXPORTER_PORT=9510"

# Command
ExecStart=/opt/ffmpeg-rtmp/worker/bin/cpu-exporter --port 9510

# Restart policy
Restart=on-failure
RestartSec=10s

# Security - CPU exporter needs access to powercap
NoNewPrivileges=true
PrivateTmp=yes

# Capabilities for RAPL access
AmbientCapabilities=CAP_DAC_READ_SEARCH

# Resource limits
MemoryLimit=128M
CPUQuota=25%

[Install]
WantedBy=multi-user.target
```

#### GPU Exporter Service

Create `/etc/systemd/system/ffmpeg-gpu-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP GPU Exporter (NVIDIA)
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-worker
Group=ffmpeg-worker
WorkingDirectory=/opt/ffmpeg-rtmp/worker

Environment="GPU_EXPORTER_PORT=9511"

ExecStart=/opt/ffmpeg-rtmp/worker/bin/gpu-exporter --port 9511

Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=yes

# GPU access requires device access
SupplementaryGroups=video

MemoryLimit=128M
CPUQuota=25%

[Install]
WantedBy=multi-user.target
```

#### FFmpeg Exporter Service

Create `/etc/systemd/system/ffmpeg-stats-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP Stats Exporter
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-worker
Group=ffmpeg-worker
WorkingDirectory=/opt/ffmpeg-rtmp/worker

Environment="FFMPEG_EXPORTER_PORT=9506"

ExecStart=/opt/ffmpeg-rtmp/worker/bin/ffmpeg-exporter --port 9506

Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=yes
MemoryLimit=128M
CPUQuota=25%

[Install]
WantedBy=multi-user.target
```

#### Docker Stats Exporter Service

Create `/etc/systemd/system/ffmpeg-docker-stats-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP Docker Stats Exporter
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=ffmpeg-worker
Group=ffmpeg-worker
WorkingDirectory=/opt/ffmpeg-rtmp/worker

Environment="DOCKER_STATS_PORT=9501"

ExecStart=/usr/bin/python3 /opt/ffmpeg-rtmp/worker/docker_stats/docker_stats_exporter.py --port 9501

Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=yes

# Docker socket access
SupplementaryGroups=docker

MemoryLimit=256M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

### 4. Enable and Start Services

```bash
# Reload systemd configuration
sudo systemctl daemon-reload

# Enable services (only enable what you need)
sudo systemctl enable ffmpeg-cpu-exporter.service
# sudo systemctl enable ffmpeg-gpu-exporter.service  # If you have NVIDIA GPU
sudo systemctl enable ffmpeg-stats-exporter.service
sudo systemctl enable ffmpeg-docker-stats-exporter.service

# Start services
sudo systemctl start ffmpeg-cpu-exporter.service
# sudo systemctl start ffmpeg-gpu-exporter.service
sudo systemctl start ffmpeg-stats-exporter.service
sudo systemctl start ffmpeg-docker-stats-exporter.service

# Check status
sudo systemctl status ffmpeg-cpu-exporter.service
# sudo systemctl status ffmpeg-gpu-exporter.service
sudo systemctl status ffmpeg-stats-exporter.service
sudo systemctl status ffmpeg-docker-stats-exporter.service
```

### 5. Verify Services

```bash
# Check all exporters are running
curl http://localhost:9510/health  # CPU
# curl http://localhost:9511/health  # GPU (if enabled)
curl http://localhost:9506/health  # FFmpeg
curl http://localhost:9501/health  # Docker Stats

# Check metrics endpoints
curl http://localhost:9510/metrics
# curl http://localhost:9511/metrics
curl http://localhost:9506/metrics
curl http://localhost:9501/metrics
```

### 6. View Logs

```bash
# View logs for each service
sudo journalctl -u ffmpeg-cpu-exporter.service -f
sudo journalctl -u ffmpeg-gpu-exporter.service -f
sudo journalctl -u ffmpeg-stats-exporter.service -f
sudo journalctl -u ffmpeg-docker-stats-exporter.service -f

# View last 50 lines
sudo journalctl -u ffmpeg-cpu-exporter.service -n 50
```

---

## Firewall Configuration

If you have a firewall enabled, allow the exporter ports:

```bash
# Using ufw
sudo ufw allow 9510/tcp comment 'CPU Exporter'
sudo ufw allow 9511/tcp comment 'GPU Exporter'
sudo ufw allow 9506/tcp comment 'FFmpeg Exporter'
sudo ufw allow 9501/tcp comment 'Docker Stats Exporter'

# Using firewalld
sudo firewall-cmd --permanent --add-port=9510/tcp
sudo firewall-cmd --permanent --add-port=9511/tcp
sudo firewall-cmd --permanent --add-port=9506/tcp
sudo firewall-cmd --permanent --add-port=9501/tcp
sudo firewall-cmd --reload
```

---

## VictoriaMetrics Scrape Configuration

Add the exporters to your VictoriaMetrics scrape configuration on the master node:

```yaml
scrape_configs:
  # ... existing jobs ...

  - job_name: 'worker-cpu-exporter'
    static_configs:
      - targets: ['worker-node-1:9510', 'worker-node-2:9510']
        labels:
          node: 'worker'
    scrape_interval: 5s

  - job_name: 'worker-gpu-exporter'
    static_configs:
      - targets: ['worker-node-1:9511', 'worker-node-2:9511']
        labels:
          node: 'worker'
    scrape_interval: 5s

  - job_name: 'worker-ffmpeg-exporter'
    static_configs:
      - targets: ['worker-node-1:9506', 'worker-node-2:9506']
        labels:
          node: 'worker'
    scrape_interval: 10s

  - job_name: 'worker-docker-stats'
    static_configs:
      - targets: ['worker-node-1:9501', 'worker-node-2:9501']
        labels:
          node: 'worker'
    scrape_interval: 15s
```

---

## Troubleshooting

### CPU Exporter Issues

**No RAPL zones found**:
```bash
# Check RAPL is available
ls -la /sys/class/powercap/intel-rapl*

# Enable RAPL in BIOS if not present
# Check kernel support
dmesg | grep -i rapl

# Temporarily make readable by all users (testing only)
sudo chmod -R a+r /sys/class/powercap
```

**Permission denied**:
```bash
# Option 1: Set capabilities
sudo setcap cap_dac_read_search=+ep /opt/ffmpeg-rtmp/worker/bin/cpu-exporter

# Option 2: Run as root (not recommended)
sudo /opt/ffmpeg-rtmp/worker/bin/cpu-exporter --port 9510

# Option 3: Add ACL permissions
sudo setfacl -R -m u:ffmpeg-worker:r /sys/class/powercap
```

### GPU Exporter Issues

**NVIDIA driver not found**:
```bash
# Verify drivers are installed
nvidia-smi

# Install NVIDIA drivers (Ubuntu)
sudo apt install nvidia-driver-530

# Check GPU detection
nvidia-smi -L
```

**Permission denied**:
```bash
# Add user to video group
sudo usermod -aG video ffmpeg-worker

# Verify group membership
groups ffmpeg-worker
```

### FFmpeg Exporter Issues

**No metrics appearing**:
- Ensure FFmpeg jobs are sending output to the exporter
- Check that FFmpeg is using progress output: `-progress pipe:1`
- Verify network connectivity to exporter endpoint

### Docker Stats Exporter Issues

**Cannot connect to Docker daemon**:
```bash
# Add user to docker group
sudo usermod -aG docker ffmpeg-worker

# Restart Docker service
sudo systemctl restart docker

# Verify socket permissions
ls -l /var/run/docker.sock
```

### Service Won't Start

**Check logs**:
```bash
sudo journalctl -u ffmpeg-cpu-exporter.service -n 50
```

**Test binary manually**:
```bash
# Test as the service user
sudo -u ffmpeg-worker /opt/ffmpeg-rtmp/worker/bin/cpu-exporter --port 9510
```

**Port already in use**:
```bash
# Find process using the port
sudo lsof -i :9510

# Kill the process or change the port
```

---

## Upgrading Exporters

```bash
# Stop services
sudo systemctl stop ffmpeg-cpu-exporter.service
sudo systemctl stop ffmpeg-gpu-exporter.service
sudo systemctl stop ffmpeg-stats-exporter.service
sudo systemctl stop ffmpeg-docker-stats-exporter.service

# Update code
cd /path/to/ffmpeg-rtmp
git pull

# Rebuild binaries
go build -o bin/cpu-exporter ./worker/exporters/cpu_exporter
go build -o bin/gpu-exporter ./worker/exporters/gpu_exporter
go build -o bin/ffmpeg-exporter ./worker/exporters/ffmpeg_exporter

# Copy updated binaries
sudo cp bin/cpu-exporter /opt/ffmpeg-rtmp/worker/bin/
sudo cp bin/gpu-exporter /opt/ffmpeg-rtmp/worker/bin/
sudo cp bin/ffmpeg-exporter /opt/ffmpeg-rtmp/worker/bin/

# Set capabilities again (if needed)
sudo setcap cap_dac_read_search=+ep /opt/ffmpeg-rtmp/worker/bin/cpu-exporter

# Copy Python exporter updates
sudo cp -r worker/exporters/docker_stats /opt/ffmpeg-rtmp/worker/

# Restart services
sudo systemctl start ffmpeg-cpu-exporter.service
sudo systemctl start ffmpeg-gpu-exporter.service
sudo systemctl start ffmpeg-stats-exporter.service
sudo systemctl start ffmpeg-docker-stats-exporter.service

# Verify
sudo systemctl status ffmpeg-*-exporter.service
```

---

## Performance Considerations

### Go Exporters
- **Low overhead**: Typically <20MB memory, <5% CPU
- **Fast scraping**: Handle hundreds of scrapes per second
- **Built-in caching**: Metrics cached for 5 seconds to reduce overhead

### Python Exporter (Docker Stats)
- **Moderate overhead**: ~50-100MB memory, ~10% CPU
- **Polling interval**: Adjust frequency based on needs
- **Async I/O**: Uses async operations for efficiency

### Recommended Scrape Intervals
- CPU Exporter: 5s (power metrics change frequently)
- GPU Exporter: 5s (GPU metrics change frequently)
- FFmpeg Exporter: 10s (encoding stats are averaged)
- Docker Stats: 15s (container stats less critical)

---

## Security Considerations

1. **Run as dedicated user**: Never run exporters as root (except when absolutely necessary)
2. **Use capabilities**: For CPU exporter, use `CAP_DAC_READ_SEARCH` instead of root
3. **Limit resource usage**: Set memory and CPU limits in systemd
4. **Firewall rules**: Restrict access to exporter ports
5. **Group membership**: Only add users to necessary groups (docker, video)
6. **Regular updates**: Keep Go compiler and dependencies updated

---

## Alternative Deployment Methods

### Using Docker Compose (Development)

For development/testing, use the provided docker-compose.yml:

```bash
# Start CPU exporter only
docker compose up -d cpu-exporter-go

# Start GPU exporter (requires --profile nvidia)
docker compose --profile nvidia up -d gpu-exporter-go

# Start all worker exporters
docker compose up -d cpu-exporter-go ffmpeg-exporter docker-stats-exporter
```

### Using Kubernetes

For Kubernetes deployments, create DaemonSet manifests for worker exporters. They should run on each worker node to collect local metrics.

### Using Bare Metal Scripts

Create a simple startup script `/opt/ffmpeg-rtmp/worker/start-exporters.sh`:

```bash
#!/bin/bash
set -e

WORKER_DIR="/opt/ffmpeg-rtmp/worker"

# Start exporters in background
$WORKER_DIR/bin/cpu-exporter --port 9510 > /var/log/ffmpeg-worker/cpu-exporter.log 2>&1 &
$WORKER_DIR/bin/ffmpeg-exporter --port 9506 > /var/log/ffmpeg-worker/ffmpeg-exporter.log 2>&1 &
python3 $WORKER_DIR/docker_stats/docker_stats_exporter.py --port 9501 > /var/log/ffmpeg-worker/docker-stats.log 2>&1 &

# Optional: GPU exporter
# $WORKER_DIR/bin/gpu-exporter --port 9511 > /var/log/ffmpeg-worker/gpu-exporter.log 2>&1 &

echo "Worker exporters started"
```

Make executable and run:
```bash
chmod +x /opt/ffmpeg-rtmp/worker/start-exporters.sh
/opt/ffmpeg-rtmp/worker/start-exporters.sh
```

---

## Related Documentation

- [Worker Node README](../README.md) - Worker node overview
- [Master Exporters](../../master/exporters/README.md) - Master exporter deployment
- [Production Deployment](../../deployment/README.md) - Complete production setup
- [Architecture](../../docs/ARCHITECTURE_DIAGRAM.md) - System architecture

---

## Support

For issues with worker exporter deployment:

1. Check logs: `sudo journalctl -u ffmpeg-*-exporter.service -n 100`
2. Test exporters manually before using systemd
3. Verify hardware requirements (RAPL for CPU, NVIDIA for GPU)
4. Check file permissions and capabilities
5. Open an issue: https://github.com/psantana5/ffmpeg-rtmp/issues

Include in your issue:
- Exporter logs
- Service status output
- Hardware information (CPU model, GPU model)
- OS and kernel version
- Output of test commands (e.g., `ls /sys/class/powercap`, `nvidia-smi`)
