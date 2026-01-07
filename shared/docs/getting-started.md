# Getting Started Guide

This guide will help you set up and run your first energy-efficient streaming tests.

## Prerequisites

### Required

- **Docker** (20.10+) and **Docker Compose** (2.0+)
- **Python** 3.11 or later
- **FFmpeg** installed on the host machine
- **Intel CPU** with RAPL support (for power monitoring)

### Optional

- **NVIDIA GPU** with nvidia-container-toolkit (for GPU monitoring)
- **Git** for cloning the repository

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
```

### 2. Install Python Dependencies

```bash
pip install -r requirements.txt
```

For development:
```bash
pip install -r requirements-dev.txt
```

### 3. Check Prerequisites

The setup script will verify all prerequisites:

```bash
./scripts/setup.sh
```

Or manually check:

```bash
# Check Docker
docker --version
docker compose version

# Check Python
python3 --version

# Check FFmpeg
ffmpeg -version

# Check RAPL access
cat /sys/class/powercap/intel-rapl:0/energy_uj
```

## Starting the Stack

### Quick Start

```bash
make up-build
```

This command will:
1. Create necessary directories (`test_results`, `streams`)
2. Build all Docker images
3. Start all services
4. Wait for services to become healthy

### Manual Start

```bash
docker compose up -d --build
```

### With NVIDIA GPU

```bash
make nvidia-up-build
```

## Verify Installation

### 1. Check Container Status

```bash
make ps
# or
docker compose ps
```

All containers should show status "Up" and "healthy".

### 2. Access Web Interfaces

- **Grafana**: http://localhost:3000
  - Username: `admin`
  - Password: `admin`
  - You'll be prompted to change the password on first login

- **VictoriaMetrics**: http://localhost:8428
  - Check targets: http://localhost:8428/targets
  - All targets should be "UP" (green)

- **Alertmanager**: http://localhost:9093

### 3. Check Exporter Health

```bash
# Quick health check
curl http://localhost:9500/health  # RAPL exporter
curl http://localhost:9501/health  # Docker stats
curl http://localhost:9502/health  # Results exporter

# Or use the health checker script
python3 src/exporters/health_checker/check_exporters_health.py
```

### 4. Verify Metrics Collection

```bash
# Check RAPL metrics
curl http://localhost:9500/metrics | grep rapl_power_watts

# Check Docker metrics
curl http://localhost:9501/metrics | grep docker_cpu_percentage
```

## Running Your First Test

### Simple Single-Stream Test

```bash
python3 scripts/run_tests.py single \
  --name "my_first_test" \
  --bitrate 2000k \
  --resolution 1280x720 \
  --duration 60
```

This will:
1. Start an FFmpeg process streaming to nginx-rtmp
2. Monitor power consumption during the test
3. Save results to `test_results/test_results_YYYYMMDD_HHMMSS.json`

### View Results

```bash
# Analyze results
python3 scripts/analyze_results.py

# View in Grafana
# Open http://localhost:3000
# Navigate to "Power Monitoring Dashboard"
```

### Multi-Stream Test

```bash
python3 scripts/run_tests.py multi \
  --count 4 \
  --bitrate 2500k \
  --duration 120
```

### Batch Test Matrix

```bash
python3 scripts/run_tests.py batch --file batch_stress_matrix.json
```

## Understanding the Output

### Console Output

The test runner shows:
```
Starting test: my_first_test
  Bitrate: 2000k
  Resolution: 1280x720
  Duration: 60s

[========================================] 60/60s
Test complete!
Results saved to: test_results/test_results_20231215_143022.json
```

### Analysis Report

```bash
python3 scripts/analyze_results.py
```

Shows:
- Power consumption statistics
- Energy usage
- Efficiency rankings
- Recommendations for optimal settings

### Grafana Dashboards

**Power Monitoring Dashboard**:
- Real-time power consumption
- CPU/Memory usage
- Network traffic
- Container metrics

**Baseline vs Test Dashboard**:
- Compares test scenarios against baseline
- Shows power deltas
- Highlights efficiency improvements

**Energy Efficiency Dashboard**:
- Efficiency scores per scenario
- Cost analysis
- Throughput per watt metrics

## Next Steps

### Explore Different Scenarios

1. **Test different bitrates**:
   ```bash
   python3 scripts/run_tests.py single --bitrate 5000k --duration 60
   ```

2. **Test different resolutions**:
   ```bash
   python3 scripts/run_tests.py single --resolution 1920x1080 --duration 60
   ```

3. **Stress test with multiple streams**:
   ```bash
   python3 scripts/run_tests.py multi --count 8 --duration 120
   ```

### Use ML Predictions

Train models on your hardware:
```bash
# Run several tests first
python3 scripts/run_tests.py batch --file batch_stress_matrix.json

# Train models
python3 scripts/retrain_models.py

# Analyze with predictions
python3 scripts/analyze_results.py --multivariate --predict-future 1,2,4,8,16
```

### Set Up Alerts

Edit `prometheus-alerts.yml` to set power thresholds:

```yaml
- alert: HighPowerConsumption
  expr: rapl_power_watts > 200
  for: 5m
  annotations:
    summary: "Power consumption above 200W"
```

Then reload Prometheus:
```bash
make prom-reload
```

### Customize Grafana Dashboards

1. Open Grafana: http://localhost:3000
2. Navigate to a dashboard
3. Click the gear icon () to edit
4. Add panels, modify queries, change visualizations
5. Save your changes

## Troubleshooting

### Services Not Starting

```bash
# Check logs
make logs SERVICE=prometheus
make logs SERVICE=rapl-exporter

# Restart a specific service
docker compose restart rapl-exporter

# Rebuild and restart everything
make down
make up-build
```

### RAPL Metrics Not Available

RAPL requires Intel CPU and proper permissions:

```bash
# Check RAPL availability
ls -l /sys/class/powercap/intel-rapl:0/

# If permission denied, grant read access
sudo chmod -R a+r /sys/class/powercap/
```

### FFmpeg Connection Refused

Ensure nginx-rtmp is running and healthy:

```bash
docker compose ps nginx-rtmp
curl http://localhost:8080/stat
```

### Test Results Not Appearing

```bash
# Verify test_results directory exists
ls -la test_results/

# Check results-exporter logs
make logs SERVICE=results-exporter

# Restart results-exporter
docker compose restart results-exporter
```

### GPU Metrics Not Available

For NVIDIA GPU support:

```bash
# Check nvidia-docker installation
docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi

# Start with NVIDIA profile
make nvidia-up-build

# Check dcgm-exporter logs
make logs SERVICE=dcgm-exporter
```

## Common Issues

### Port Already in Use

If you see "port is already allocated" errors:

```bash
# Check what's using the port
sudo lsof -i :9090

# Either stop the conflicting service or change the port in docker-compose.yml
```

### Out of Disk Space

Test results and Prometheus data can grow:

```bash
# Check disk usage
du -sh test_results/
docker system df

# Clean up old results
rm test_results/test_results_2023*.json

# Clean up Docker volumes
docker volume prune
```

### Slow Performance

If containers are slow:

```bash
# Check container resource usage
docker stats

# Increase Docker resources in Docker Desktop settings
# Recommended: 4+ CPU cores, 8+ GB RAM
```

## Getting Help

- Check the [main documentation](../docs/)
- Review [troubleshooting guide](../docs/troubleshooting.md)
- Check container logs: `make logs SERVICE=<service>`
- Verify VictoriaMetrics targets: http://localhost:8428/targets
- Ask in the project issues: https://github.com/psantana5/ffmpeg-rtmp/issues

## What's Next?

Now that you have the basics working:

1. Read the [Architecture documentation](../docs/architecture.md)
2. Explore the [Test Runner Guide](../scripts/README.md)
3. Learn about [Energy Advisor and ML models](../advisor/README.md)
4. Understand the [Exporters](../src/exporters/README.md)
5. Set up [production monitoring](../docs/production-setup.md)
