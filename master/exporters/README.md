# Master Exporters - Deployment Guide

This guide explains how to deploy and run master node exporters. All exporters are implemented in **Go** for high performance and reliability.

## Overview

Master exporters collect and expose metrics about test results, quality of experience (QoE), and cost analysis:

1. **Results Exporter** (Port 9502) - Exposes test result metrics
2. **QoE Exporter** (Port 9503) - Exposes quality metrics (VMAF, PSNR, SSIM)
3. **Cost Exporter** (Port 9504) - Exposes cost analysis metrics
4. **Health Checker** (Port 9600) - Monitors health of all exporters

## Quick Start with Docker

The easiest way to run all exporters is using Docker Compose:

```bash
cd /path/to/ffmpeg-rtmp
mkdir -p test_results
docker compose up -d
```

This will start all exporters with their default configurations.

## Verify Exporters

Check that all exporters are running and healthy:

```bash
# Check container status
docker compose ps

# Test individual exporters
curl http://localhost:9502/health  # results-exporter
curl http://localhost:9503/health  # qoe-exporter
curl http://localhost:9504/health  # cost-exporter
curl http://localhost:9600/health  # exporter-health-checker

# View metrics
curl http://localhost:9502/metrics
curl http://localhost:9503/metrics
curl http://localhost:9504/metrics
curl http://localhost:9600/metrics
```

## Manual Deployment (Without Docker)

### Prerequisites

- Go 1.21 or later
- Linux system (for systemd service management)

### Build Exporters

```bash
cd /path/to/ffmpeg-rtmp

# Build all exporters
go build -o bin/results_exporter ./master/exporters/results_go/
go build -o bin/qoe_exporter ./master/exporters/qoe_go/
go build -o bin/cost_exporter ./master/exporters/cost_go/
go build -o bin/health_checker ./master/exporters/health_checker_go/
```

### Run Exporters Manually

```bash
# Results Exporter
RESULTS_EXPORTER_PORT=9502 RESULTS_DIR=./test_results ./bin/results_exporter &

# QoE Exporter
QOE_EXPORTER_PORT=9503 RESULTS_DIR=./test_results ./bin/qoe_exporter &

# Cost Exporter
COST_EXPORTER_PORT=9504 RESULTS_DIR=./test_results \
ENERGY_COST=0.0 CPU_COST=0.50 CURRENCY=USD REGION=us-east-1 \
./bin/cost_exporter &

# Health Checker
HEALTH_CHECK_PORT=9600 ./bin/health_checker &
```

---

## Detailed Exporter Configuration

### 1. Results Exporter

**Purpose**: Aggregates and exposes test result metrics from transcoding jobs.

**Port**: 9502 (default)

**Environment Variables**:
- `RESULTS_EXPORTER_PORT`: HTTP server port (default: 9502)
- `RESULTS_DIR`: Directory containing test result JSON files (default: /results)

**Metrics Exposed**:
- `results_scenarios_total`: Number of scenarios loaded
- `results_scenario_duration_seconds`: Scenario duration
- `results_scenario_avg_fps`: Average FPS
- `results_scenario_dropped_frames`: Dropped frames count
- `results_scenario_total_frames`: Total frames processed
- `results_scenario_vmaf_score`: VMAF quality score
- `results_scenario_psnr_score`: PSNR quality score

**Health Check**:
```bash
curl http://localhost:9502/health
# Expected: {"status": "ok"}
```

**Example Usage**:
```bash
export RESULTS_EXPORTER_PORT=9502
export RESULTS_DIR=/var/lib/ffmpeg-rtmp/results
./bin/results_exporter
```

---

### 2. QoE Exporter

**Purpose**: Exposes Quality of Experience metrics including VMAF, PSNR, SSIM and efficiency scores.

**Port**: 9503 (default)

**Environment Variables**:
- `QOE_EXPORTER_PORT`: HTTP server port (default: 9503)
- `RESULTS_DIR`: Directory containing test result JSON files (default: /results)

**Metrics Exposed**:
- `qoe_vmaf_score`: VMAF quality score (0-100)
- `qoe_psnr_score`: PSNR quality score (dB)
- `qoe_ssim_score`: SSIM quality score (0-1)
- `qoe_quality_per_watt`: Quality efficiency metric (quality/watt)
- `qoe_efficiency_score`: QoE efficiency score (quality-weighted pixels per joule)
- `qoe_drop_rate`: Frame drop rate

**Health Check**:
```bash
curl http://localhost:9503/health
# Expected: {"status": "ok"}
```

**Example Usage**:
```bash
export QOE_EXPORTER_PORT=9503
export RESULTS_DIR=/var/lib/ffmpeg-rtmp/results
./bin/qoe_exporter
```

---

### 3. Cost Exporter

**Purpose**: Calculates and exposes cost metrics based on energy consumption and compute usage.

**Port**: 9504 (default)

**Environment Variables**:
- `COST_EXPORTER_PORT`: Server port (default: 9504)
- `RESULTS_DIR`: Results directory (default: /results)
- `ENERGY_COST`: Energy cost per kWh (e.g., 0.12 for $0.12/kWh, or 0.0 for free energy)
- `CPU_COST`: CPU cost per hour (e.g., 0.50 for $0.50/hour)
- `CURRENCY`: Currency code (default: USD)
- `REGION`: Cloud region (default: us-east-1)

**Metrics Exposed**:
- `cost_total_load_aware`: Total cost (load-aware calculation)
- `cost_energy_load_aware`: Energy cost component
- `cost_compute_load_aware`: Compute cost component
- `cost_per_pixel`: Cost efficiency per megapixel
- `cost_per_watch_hour`: Cost per viewer watch hour

**Health Check**:
```bash
curl http://localhost:9504/health
# Expected: {"status": "ok"}
```

**Example Usage**:
```bash
export COST_EXPORTER_PORT=9504
export RESULTS_DIR=/var/lib/ffmpeg-rtmp/results
export ENERGY_COST=0.12
export CPU_COST=0.50
export CURRENCY=USD
export REGION=us-east-1
./bin/cost_exporter
```

---

### 4. Health Checker

**Purpose**: Monitors the health and availability of all exporters.

**Port**: 9600 (default)

**Environment Variables**:
- `HEALTH_CHECK_PORT`: Server port (default: 9600)

**Monitored Exporters**:
- nginx-exporter (9728)
- cpu-exporter-go (9500)
- docker-stats-exporter (9501)
- node-exporter (9100)
- cadvisor (8080)
- results-exporter (9502)
- qoe-exporter (9503)
- cost-exporter (9504)
- ffmpeg-exporter (9506)

**Metrics Exposed**:
- `exporter_healthy`: Health status (1=healthy, 0=unhealthy)
- `exporter_response_time_ms`: Response time in milliseconds
- `exporter_last_check_timestamp`: Last check timestamp
- `exporter_total`: Total exporters monitored
- `exporter_healthy_total`: Total healthy exporters

**Health Check**:
```bash
curl http://localhost:9600/health
# Expected: {"status": "ok"}
```

**Example Usage**:
```bash
export HEALTH_CHECK_PORT=9600
./bin/health_checker
```

---

## Production Deployment with Systemd

For production environments, run exporters as systemd services.

### 1. Create Service User

```bash
sudo useradd --system --no-create-home --shell /bin/false ffmpeg-exporter
```

### 2. Create Directory Structure

```bash
# Create directories
sudo mkdir -p /opt/ffmpeg-rtmp/bin
sudo mkdir -p /var/lib/ffmpeg-rtmp/results

# Copy binaries
sudo cp bin/results_exporter /opt/ffmpeg-rtmp/bin/
sudo cp bin/qoe_exporter /opt/ffmpeg-rtmp/bin/
sudo cp bin/cost_exporter /opt/ffmpeg-rtmp/bin/
sudo cp bin/health_checker /opt/ffmpeg-rtmp/bin/

# Set ownership
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /opt/ffmpeg-rtmp
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /var/lib/ffmpeg-rtmp
```

### 3. Create Systemd Service Files

#### Results Exporter Service

Create `/etc/systemd/system/ffmpeg-results-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP Results Exporter
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-exporter
Group=ffmpeg-exporter
Environment="RESULTS_EXPORTER_PORT=9502"
Environment="RESULTS_DIR=/var/lib/ffmpeg-rtmp/results"
ExecStart=/opt/ffmpeg-rtmp/bin/results_exporter
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

#### QoE Exporter Service

Create `/etc/systemd/system/ffmpeg-qoe-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP QoE Exporter
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-exporter
Group=ffmpeg-exporter
Environment="QOE_EXPORTER_PORT=9503"
Environment="RESULTS_DIR=/var/lib/ffmpeg-rtmp/results"
ExecStart=/opt/ffmpeg-rtmp/bin/qoe_exporter
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

#### Cost Exporter Service

Create `/etc/systemd/system/ffmpeg-cost-exporter.service`:

```ini
[Unit]
Description=FFmpeg RTMP Cost Exporter
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-exporter
Group=ffmpeg-exporter
Environment="COST_EXPORTER_PORT=9504"
Environment="RESULTS_DIR=/var/lib/ffmpeg-rtmp/results"
Environment="ENERGY_COST=0.12"
Environment="CPU_COST=0.50"
Environment="CURRENCY=USD"
Environment="REGION=us-east-1"
ExecStart=/opt/ffmpeg-rtmp/bin/cost_exporter
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

#### Health Checker Service

Create `/etc/systemd/system/ffmpeg-health-checker.service`:

```ini
[Unit]
Description=FFmpeg RTMP Exporter Health Checker
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-exporter
Group=ffmpeg-exporter
Environment="HEALTH_CHECK_PORT=9600"
ExecStart=/opt/ffmpeg-rtmp/bin/health_checker
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### 4. Enable and Start Services

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable services
sudo systemctl enable ffmpeg-results-exporter
sudo systemctl enable ffmpeg-qoe-exporter
sudo systemctl enable ffmpeg-cost-exporter
sudo systemctl enable ffmpeg-health-checker

# Start services
sudo systemctl start ffmpeg-results-exporter
sudo systemctl start ffmpeg-qoe-exporter
sudo systemctl start ffmpeg-cost-exporter
sudo systemctl start ffmpeg-health-checker

# Check status
sudo systemctl status ffmpeg-results-exporter
sudo systemctl status ffmpeg-qoe-exporter
sudo systemctl status ffmpeg-cost-exporter
sudo systemctl status ffmpeg-health-checker
```

### 5. View Logs

```bash
# View logs for specific service
sudo journalctl -u ffmpeg-results-exporter -f
sudo journalctl -u ffmpeg-qoe-exporter -f
sudo journalctl -u ffmpeg-cost-exporter -f
sudo journalctl -u ffmpeg-health-checker -f
```

---

## Troubleshooting

### Exporter Won't Start

1. Check logs: `sudo journalctl -u ffmpeg-results-exporter -n 50`
2. Verify binary permissions: `ls -l /opt/ffmpeg-rtmp/bin/`
3. Check port availability: `sudo netstat -tlnp | grep 9502`
4. Verify results directory exists and is readable: `ls -la /var/lib/ffmpeg-rtmp/results`

### No Metrics Appearing

1. Check health endpoint responds: `curl http://localhost:9502/health`
2. Check metrics endpoint: `curl http://localhost:9502/metrics`
3. Verify test results files exist: `ls /var/lib/ffmpeg-rtmp/results/`
4. Check file format (should be JSON with naming pattern: `test_results_YYYYMMDD_HHMMSS.json`)

### Health Checker Shows Exporters Down

1. Check each exporter individually with `curl`
2. Verify network connectivity between containers/hosts
3. Check firewall rules if running on separate hosts
4. Verify correct ports in docker-compose.yml or environment variables

---

## Integration with VictoriaMetrics

All exporters expose metrics in Prometheus format on `/metrics` endpoint. Configure VictoriaMetrics to scrape:

```yaml
scrape_configs:
  - job_name: 'results-exporter'
    static_configs:
      - targets: ['localhost:9502']
  
  - job_name: 'qoe-exporter'
    static_configs:
      - targets: ['localhost:9503']
  
  - job_name: 'cost-exporter'
    static_configs:
      - targets: ['localhost:9504']
  
  - job_name: 'health-checker'
    static_configs:
      - targets: ['localhost:9600']
```

---

## Additional Resources

- [Go Exporters Summary](../../GO_EXPORTERS_SUMMARY.md) - Detailed implementation notes
- [Grafana Dashboards](../monitoring/grafana/provisioning/dashboards/) - Pre-built dashboards for visualization
- [VictoriaMetrics Configuration](../monitoring/victoriametrics.yml) - Metrics collection setup
- [Docker Compose](../../docker-compose.yml) - Container orchestration

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/psantana5/ffmpeg-rtmp/issues
- Documentation: https://github.com/psantana5/ffmpeg-rtmp/tree/main/docs
