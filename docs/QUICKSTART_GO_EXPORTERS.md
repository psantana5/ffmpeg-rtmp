# Quick Start Guide - Go Exporters

## TL;DR - One Command Deployment

```bash
# Clone the repository
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Start everything (builds Go exporters automatically)
docker compose up -d --build
```

That's it! Docker Compose will:
1. ✅ Build the Go CPU exporter from source
2. ✅ Build the Go GPU exporter from source  
3. ✅ Start VictoriaMetrics for high-performance metrics storage
4. ✅ Start Prometheus for comparison
5. ✅ Start Grafana with both datasources configured
6. ✅ Start all other monitoring services

**No Go installation required. No manual builds. Just Docker.**

## Verify Everything Works

```bash
# Check all services are running
docker compose ps

# Access the dashboards
# Grafana:          http://localhost:3000 (admin/admin)
# Prometheus:       http://localhost:9090
# VictoriaMetrics:  http://localhost:8428

# Check Go exporter health (if RAPL is available)
curl http://localhost:9510/health  # CPU exporter
curl http://localhost:9511/health  # GPU exporter (nvidia profile)
```

## For NVIDIA GPU Users

```bash
# Start with NVIDIA GPU support
docker compose --profile nvidia up -d --build
```

This automatically builds and starts the GPU exporter with nvidia-smi access.

## What Gets Built Automatically

When you run `docker compose up -d --build`:

### Go Exporters (Built from Source)
- **CPU Exporter** - RAPL power monitoring
  - Source: `src/exporters/cpu_exporter/main.go`
  - Dockerfile: `src/exporters/cpu_exporter/Dockerfile`
  - Port: 9510
  
- **GPU Exporter** - NVIDIA GPU monitoring
  - Source: `src/exporters/gpu_exporter/main.go`
  - Dockerfile: `src/exporters/gpu_exporter/Dockerfile`
  - Port: 9511 (nvidia profile)

### Pre-built Images (Pulled from Registry)
- VictoriaMetrics
- Prometheus
- Grafana
- Node Exporter
- cAdvisor
- DCGM Exporter (NVIDIA profile)

### Python Exporters (Built from Source)
- Results Exporter
- QoE Exporter
- Cost Exporter
- RAPL Exporter (legacy, being replaced by Go version)
- Docker Stats Exporter

## Common Commands

```bash
# Start everything
make up-build

# Start with rebuild
docker compose up -d --build

# Stop everything
docker compose down

# View logs
docker compose logs -f cpu-exporter-go
docker compose logs -f victoriametrics

# Restart a service
docker compose restart cpu-exporter-go

# Start only VictoriaMetrics stack
make vm-up-build
```

## Troubleshooting Build Issues

### Go Exporters Won't Build

If Go exporters fail to build, check:

```bash
# View build logs
docker compose build cpu-exporter-go
docker compose build gpu-exporter-go

# Common issues:
# 1. Docker out of space - clean up
docker system prune -a

# 2. Old cached layers - force rebuild
docker compose build --no-cache cpu-exporter-go
```

### Container Keeps Restarting

```bash
# Check logs for the issue
docker compose logs cpu-exporter-go

# Common causes:
# - RAPL not available (Intel CPU required)
# - Insufficient privileges (needs privileged: true)
# - GPU not available (nvidia-smi not found)
```

## What's Different from Traditional Go Projects?

In traditional Go projects, you'd need to:
```bash
# ❌ Old way - requires Go installed
go mod download
go build -o cpu_exporter ./src/exporters/cpu_exporter/
./cpu_exporter
```

With this setup:
```bash
# ✅ New way - only requires Docker
docker compose up -d cpu-exporter-go
# Build happens automatically inside the container
```

## Development Workflow

### Making Changes to Go Exporters

1. Edit the Go source code (e.g., `src/exporters/cpu_exporter/main.go`)
2. Rebuild and restart:
   ```bash
   docker compose up -d --build cpu-exporter-go
   ```
3. View logs:
   ```bash
   docker compose logs -f cpu-exporter-go
   ```

### Testing Changes

```bash
# Rebuild just the changed exporter
docker compose build cpu-exporter-go

# Restart to pick up changes
docker compose up -d cpu-exporter-go

# Check it's working
curl http://localhost:9510/metrics
```

## Architecture

```
User runs: docker compose up -d --build
                     │
                     ▼
        ┌────────────────────────┐
        │  Docker Compose        │
        └────────┬───────────────┘
                 │
         ┌───────┴───────┐
         │               │
    [Go Build]      [Python Build]
         │               │
    ┌────▼────┐     ┌────▼────┐
    │ Golang  │     │ Python  │
    │ Builder │     │ Builder │
    └────┬────┘     └────┬────┘
         │               │
         ▼               ▼
    [CPU/GPU        [Results/QoE
     Exporters]      Exporters]
         │               │
         └───────┬───────┘
                 ▼
         [Running Containers]
                 │
         ┌───────┼────────┐
         ▼       ▼        ▼
    VictoriaMetrics  Prometheus  Grafana
```

## Next Steps

- **Run your first test**: See [Running Tests](../../scripts/README.md)
- **View metrics**: Open Grafana at http://localhost:3000
- **Compare Python vs Go**: Check Prometheus targets at http://localhost:9090/targets
- **Deep dive**: Read the [Migration Guide](go-exporters-migration.md)
