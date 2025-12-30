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

## Building

Worker exporters are built as part of docker-compose for local testing:

```bash
docker compose up -d cpu-exporter-go gpu-exporter-go ffmpeg-exporter docker-stats-exporter
```

For production deployment on worker nodes, the agent manages starting these exporters as needed.

## Common Patterns

All worker exporters:
- Expose `/metrics` endpoint in Prometheus format
- Expose `/health` endpoint
- Run during job execution
- Metrics are scraped by master's VictoriaMetrics
- Lightweight and high-performance (Go exporters < 20MB)
