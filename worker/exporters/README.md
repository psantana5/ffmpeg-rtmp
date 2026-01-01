# Worker Exporters

This directory contains exporters that run on **worker nodes** to collect real-time metrics during job execution.

## Exporters

### 1. CPU Exporter (`cpu_exporter/`)
- **Port**: 9510
- **Language**: Go
- **Purpose**: Monitor CPU power consumption via Intel RAPL
- **Metrics**: Watts per CPU package/zone
- **Requirements**: Intel CPU with RAPL support, privileged access to `/sys/class/powercap`

### 2. GPU Exporter (`gpu_exporter/`)
- **Port**: 9511
- **Language**: Go
- **Purpose**: Monitor GPU power and performance via NVIDIA NVML
- **Metrics**: GPU power, utilization, temperature, memory usage
- **Requirements**: NVIDIA GPU, nvidia-docker runtime

### 3. FFmpeg Exporter (`ffmpeg_exporter/`)
- **Port**: 9506
- **Language**: Go
- **Purpose**: Real-time FFmpeg encoding statistics
- **Metrics**: FPS, bitrate, frame drops, encoding time

### 4. Docker Stats Exporter (`docker_stats/`)
- **Port**: 9501
- **Language**: Python
- **Purpose**: Track container resource usage
- **Metrics**: Container CPU %, memory %, network I/O
- **Requirements**: Access to Docker socket

## Deployment

### Docker Compose (Development)

Worker exporters are built as part of docker-compose for local testing:

```bash
docker compose up -d cpu-exporter-go gpu-exporter-go ffmpeg-exporter docker-stats-exporter
```

### Manual Deployment (Production)

For production deployment **without Docker**, see the comprehensive deployment guide:

**[📖 DEPLOYMENT.md](DEPLOYMENT.md)** - Complete guide for running exporters without Docker

The deployment guide covers:
- Building Go exporters from source
- Running exporters manually
- Systemd service configuration
- Firewall setup
- Troubleshooting
- Performance tuning
- Security considerations

For production deployment on worker nodes, the agent can manage starting these exporters as needed, or they can be run as standalone systemd services.

## Common Patterns

All worker exporters:
- Expose `/metrics` endpoint in Prometheus format
- Expose `/health` endpoint
- Run during job execution
- Metrics are scraped by master's VictoriaMetrics
- Lightweight and high-performance (Go exporters < 20MB)

## Quick Manual Deployment

```bash
# Build exporters
go build -o bin/cpu-exporter ./worker/exporters/cpu_exporter
go build -o bin/gpu-exporter ./worker/exporters/gpu_exporter
go build -o bin/ffmpeg-exporter ./worker/exporters/ffmpeg_exporter

# Run exporters
sudo ./bin/cpu-exporter --port 9510
./bin/gpu-exporter --port 9511
./bin/ffmpeg-exporter --port 9506
./bin/docker-stats-exporter --port 9501
```

For complete instructions including systemd services, see [DEPLOYMENT.md](DEPLOYMENT.md).
