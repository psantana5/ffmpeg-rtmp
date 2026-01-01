# Exporter Deployment - Quick Reference

This is a quick reference guide for deploying exporters without Docker. For detailed instructions, see the full deployment guides.

## Master Exporters (Python)

**Full Guide**: [master/exporters/README.md](../master/exporters/README.md)

### Quick Deploy

```bash
# Install dependencies
pip install -r requirements.txt

# Run exporters
python3 master/exporters/results/results_exporter.py --port 9502 --results-dir ./test_results
python3 master/exporters/qoe/qoe_exporter.py --port 9503 --results-dir ./test_results
python3 master/exporters/cost/cost_exporter.py --port 9504 --results-dir ./test_results --energy-cost 0.12
python3 master/exporters/health_checker/check_exporters_health.py --port 9600
```

### Systemd Quick Setup

```bash
# Create service user
sudo useradd --system --no-create-home --shell /bin/false ffmpeg-exporter

# Install to /opt
sudo mkdir -p /opt/ffmpeg-rtmp/exporters
sudo cp -r master/exporters/* /opt/ffmpeg-rtmp/exporters/
sudo cp -r shared/advisor /opt/ffmpeg-rtmp/exporters/
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /opt/ffmpeg-rtmp

# Install Python dependencies
cd /opt/ffmpeg-rtmp
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Create and enable systemd services (see full guide for service files)
sudo systemctl enable --now ffmpeg-results-exporter.service
sudo systemctl enable --now ffmpeg-qoe-exporter.service
sudo systemctl enable --now ffmpeg-cost-exporter.service
sudo systemctl enable --now ffmpeg-health-checker.service
```

---

## Worker Exporters (Go + Python)

**Full Guide**: [worker/exporters/DEPLOYMENT.md](../worker/exporters/DEPLOYMENT.md)

### Quick Build & Deploy

```bash
# Build Go exporters
go build -o bin/cpu-exporter ./worker/exporters/cpu_exporter
go build -o bin/gpu-exporter ./worker/exporters/gpu_exporter
go build -o bin/ffmpeg-exporter ./worker/exporters/ffmpeg_exporter

# Run Go exporters
sudo ./bin/cpu-exporter --port 9510          # Requires sudo for RAPL access
./bin/gpu-exporter --port 9511               # Requires NVIDIA GPU
./bin/ffmpeg-exporter --port 9506

# Run Python exporter
python3 worker/exporters/docker_stats/docker_stats_exporter.py --port 9501
```

### Systemd Quick Setup

```bash
# Create service user and groups
sudo useradd --system --no-create-home --shell /bin/false ffmpeg-worker
sudo usermod -aG video ffmpeg-worker   # For GPU access
sudo usermod -aG docker ffmpeg-worker  # For Docker stats

# Install to /opt
sudo mkdir -p /opt/ffmpeg-rtmp/worker/bin
sudo cp bin/*-exporter /opt/ffmpeg-rtmp/worker/bin/
sudo cp -r worker/exporters/docker_stats /opt/ffmpeg-rtmp/worker/
sudo chown -R ffmpeg-worker:ffmpeg-worker /opt/ffmpeg-rtmp/worker

# Set capabilities for CPU exporter (instead of running as root)
sudo setcap cap_dac_read_search=+ep /opt/ffmpeg-rtmp/worker/bin/cpu-exporter

# Create and enable systemd services (see full guide for service files)
sudo systemctl enable --now ffmpeg-cpu-exporter.service
sudo systemctl enable --now ffmpeg-gpu-exporter.service      # If NVIDIA GPU
sudo systemctl enable --now ffmpeg-stats-exporter.service
sudo systemctl enable --now ffmpeg-docker-stats-exporter.service
```

---

## Port Reference

| Exporter | Port | Type | Purpose |
|----------|------|------|---------|
| Results Exporter | 9502 | Master | Test result metrics |
| QoE Exporter | 9503 | Master | Quality metrics (VMAF, PSNR) |
| Cost Exporter | 9504 | Master | Cost analysis |
| Health Checker | 9600 | Master | Exporter health monitoring |
| Docker Stats | 9501 | Worker | Container resource usage |
| FFmpeg Stats | 9506 | Worker | FFmpeg encoding stats |
| CPU/RAPL | 9510 | Worker | CPU power consumption |
| GPU/NVML | 9511 | Worker | GPU metrics |

---

## Common Commands

### Check Service Status
```bash
# Master exporters
sudo systemctl status ffmpeg-results-exporter.service
sudo systemctl status ffmpeg-qoe-exporter.service
sudo systemctl status ffmpeg-cost-exporter.service
sudo systemctl status ffmpeg-health-checker.service

# Worker exporters
sudo systemctl status ffmpeg-cpu-exporter.service
sudo systemctl status ffmpeg-gpu-exporter.service
sudo systemctl status ffmpeg-stats-exporter.service
sudo systemctl status ffmpeg-docker-stats-exporter.service
```

### View Logs
```bash
# Follow logs
sudo journalctl -u ffmpeg-results-exporter.service -f

# Last 50 lines
sudo journalctl -u ffmpeg-cpu-exporter.service -n 50

# All exporter logs
sudo journalctl -u 'ffmpeg-*-exporter.service' -f
```

### Health Checks
```bash
# Master exporters
curl http://localhost:9502/health  # Results
curl http://localhost:9503/health  # QoE
curl http://localhost:9504/health  # Cost
curl http://localhost:9600/health  # Health Checker

# Worker exporters
curl http://localhost:9510/health  # CPU
curl http://localhost:9511/health  # GPU
curl http://localhost:9506/health  # FFmpeg
curl http://localhost:9501/health  # Docker Stats
```

### Metrics Endpoints
```bash
# View Prometheus metrics
curl http://localhost:9502/metrics  # Results
curl http://localhost:9510/metrics  # CPU/RAPL
curl http://localhost:9511/metrics  # GPU
```

---

## Firewall Configuration

### Master Node
```bash
sudo ufw allow 9502/tcp comment 'Results Exporter'
sudo ufw allow 9503/tcp comment 'QoE Exporter'
sudo ufw allow 9504/tcp comment 'Cost Exporter'
sudo ufw allow 9600/tcp comment 'Health Checker'
```

### Worker Node
```bash
sudo ufw allow 9510/tcp comment 'CPU Exporter'
sudo ufw allow 9511/tcp comment 'GPU Exporter'
sudo ufw allow 9506/tcp comment 'FFmpeg Exporter'
sudo ufw allow 9501/tcp comment 'Docker Stats'
```

---

## VictoriaMetrics Configuration

Add to `master/monitoring/victoriametrics.yml`:

```yaml
scrape_configs:
  # Master exporters
  - job_name: 'results-exporter'
    static_configs:
      - targets: ['localhost:9502']
    scrape_interval: 15s

  - job_name: 'qoe-exporter'
    static_configs:
      - targets: ['localhost:9503']
    scrape_interval: 15s

  - job_name: 'cost-exporter'
    static_configs:
      - targets: ['localhost:9504']
    scrape_interval: 15s

  # Worker exporters (replace with actual worker IPs)
  - job_name: 'worker-cpu-exporter'
    static_configs:
      - targets: ['worker-1:9510', 'worker-2:9510']
    scrape_interval: 5s

  - job_name: 'worker-gpu-exporter'
    static_configs:
      - targets: ['worker-1:9511', 'worker-2:9511']
    scrape_interval: 5s

  - job_name: 'worker-ffmpeg-exporter'
    static_configs:
      - targets: ['worker-1:9506', 'worker-2:9506']
    scrape_interval: 10s

  - job_name: 'worker-docker-stats'
    static_configs:
      - targets: ['worker-1:9501', 'worker-2:9501']
    scrape_interval: 15s
```

Reload configuration:
```bash
curl -X POST http://localhost:8428/-/reload
```

---

## Troubleshooting Quick Fixes

### Master Exporters

**Permission denied on results directory**:
```bash
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /var/lib/ffmpeg-rtmp/results
```

**Missing advisor module**:
```bash
sudo cp -r shared/advisor /opt/ffmpeg-rtmp/exporters/
```

**Missing Python dependencies**:
```bash
source /opt/ffmpeg-rtmp/venv/bin/activate
pip install -r requirements.txt
```

### Worker Exporters

**CPU exporter can't read RAPL**:
```bash
# Option 1: Set capabilities (preferred)
sudo setcap cap_dac_read_search=+ep /opt/ffmpeg-rtmp/worker/bin/cpu-exporter

# Option 2: Temporary permission (testing only)
sudo chmod -R a+r /sys/class/powercap
```

**GPU exporter can't find nvidia-smi**:
```bash
# Verify NVIDIA drivers
nvidia-smi

# Add user to video group
sudo usermod -aG video ffmpeg-worker
```

**Docker stats can't connect**:
```bash
# Add user to docker group
sudo usermod -aG docker ffmpeg-worker

# Verify socket permissions
ls -l /var/run/docker.sock
```

---

## Prerequisites Summary

### Master Exporters
- Python 3.10+
- pip packages: `requests`, `scikit-learn`
- Access to test results directory
- Shared `advisor` module

### Worker Exporters
- **All**: Go 1.21+ (for building)
- **CPU Exporter**: Intel CPU with RAPL, Linux 4.15+, privileged access
- **GPU Exporter**: NVIDIA GPU, nvidia-smi installed
- **FFmpeg Exporter**: FFmpeg with progress output
- **Docker Stats**: Docker daemon, socket access

---

## Full Documentation

- **[Master Exporters Guide](../master/exporters/README.md)** - Complete Python exporter deployment
- **[Worker Exporters Guide](../worker/exporters/DEPLOYMENT.md)** - Complete Go exporter deployment
- **[Master Node Deployment](../deployment/README.md)** - Full production deployment guide
- **[Architecture Overview](../docs/ARCHITECTURE_DIAGRAM.md)** - System architecture

---

## Support

For issues:
1. Check logs: `sudo journalctl -u ffmpeg-*-exporter.service -n 100`
2. Test manually before systemd
3. Verify prerequisites
4. Open issue: https://github.com/psantana5/ffmpeg-rtmp/issues
