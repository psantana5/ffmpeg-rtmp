# Master Exporters - Manual Deployment Guide

This guide explains how to deploy and run master node exporters **without Docker**, for production environments where you want direct control over the services.

## Overview

Master exporters collect and expose metrics about test results, quality of experience (QoE), and cost analysis. These exporters are:

1. **Results Exporter** (Port 9502) - Exposes test result metrics
2. **QoE Exporter** (Port 9503) - Exposes quality metrics (VMAF, PSNR)
3. **Cost Exporter** (Port 9504) - Exposes cost analysis metrics
4. **Health Checker** (Port 9600) - Monitors health of all exporters

## Quick Start

### Prerequisites

- Python 3.10 or later
- pip (Python package manager)
- Linux system with systemd (for service management)

### Install Python Dependencies

```bash
cd /path/to/ffmpeg-rtmp
pip install -r requirements.txt
```

Required packages:
- `requests>=2.31.0`
- `scikit-learn>=1.3.0`

### Run Exporters Manually

```bash
# Results Exporter
python3 master/exporters/results/results_exporter.py \
    --port 9502 \
    --results-dir ./test_results \
    --prometheus-url http://localhost:8428

# QoE Exporter
python3 master/exporters/qoe/qoe_exporter.py \
    --port 9503 \
    --results-dir ./test_results

# Cost Exporter
python3 master/exporters/cost/cost_exporter.py \
    --port 9504 \
    --results-dir ./test_results \
    --energy-cost 0.12 \
    --cpu-cost 0.50 \
    --currency USD \
    --region us-east-1 \
    --pricing-config ./pricing_config.json \
    --prometheus-url http://localhost:8428

# Health Checker
python3 master/exporters/health_checker/check_exporters_health.py \
    --port 9600
```

---

## Detailed Exporter Configuration

### 1. Results Exporter

**Purpose**: Aggregates and exposes test result metrics from transcoding jobs.

**Port**: 9502 (default)

**Command-line Options**:
- `--port`: HTTP server port (default: 9502)
- `--results-dir`: Directory containing test result JSON files (default: ./test_results)
- `--prometheus-url`: VictoriaMetrics/Prometheus URL for querying metrics (optional)

**Environment Variables**:
- `RESULTS_EXPORTER_PORT`: Alternative way to set port
- `RESULTS_DIR`: Alternative way to set results directory
- `PROMETHEUS_URL`: Alternative way to set Prometheus URL

**Example**:
```bash
export RESULTS_EXPORTER_PORT=9502
export RESULTS_DIR=/var/lib/ffmpeg-rtmp/results
export PROMETHEUS_URL=http://victoriametrics:8428

python3 master/exporters/results/results_exporter.py
```

**Dependencies**: 
- Shared `advisor` module (located in `shared/advisor/`)
- `scikit-learn` for ML predictions (optional)

**Health Check**:
```bash
curl http://localhost:9502/health
# Expected: {"status": "healthy"}
```

**Metrics Endpoint**:
```bash
curl http://localhost:9502/metrics
```

---

### 2. QoE Exporter

**Purpose**: Exposes Quality of Experience metrics including VMAF, PSNR, and efficiency scores.

**Port**: 9503 (default)

**Command-line Options**:
- `--port`: HTTP server port (default: 9503)
- `--results-dir`: Directory containing test result JSON files (default: ./test_results)

**Environment Variables**:
- `QOE_EXPORTER_PORT`: Alternative way to set port
- `RESULTS_DIR`: Alternative way to set results directory

**Example**:
```bash
python3 master/exporters/qoe/qoe_exporter.py \
    --port 9503 \
    --results-dir /var/lib/ffmpeg-rtmp/results
```

**Dependencies**:
- Shared `advisor` module for efficiency scoring

**Metrics Exposed**:
- `qoe_vmaf_score`: VMAF quality score (0-100)
- `qoe_psnr_score`: PSNR quality score (dB)
- `qoe_quality_per_watt`: Quality efficiency metric
- `qoe_efficiency_score`: QoE efficiency score

**Health Check**:
```bash
curl http://localhost:9503/health
```

---

### 3. Cost Exporter

**Purpose**: Calculates and exposes cost metrics based on energy consumption and compute usage.

**Port**: 9504 (default)

**Command-line Options**:
- `--port`: HTTP server port (default: 9504)
- `--results-dir`: Directory containing test results
- `--energy-cost`: Cost per kWh (e.g., 0.12 for $0.12/kWh)
- `--cpu-cost`: CPU cost per hour (e.g., 0.50 for $0.50/hour)
- `--currency`: Currency code (default: USD)
- `--region`: Cloud region for pricing (default: us-east-1)
- `--pricing-config`: Path to pricing configuration JSON
- `--prometheus-url`: VictoriaMetrics/Prometheus URL for load-aware metrics

**Environment Variables**:
- `COST_EXPORTER_PORT`: Server port
- `RESULTS_DIR`: Results directory
- `ENERGY_COST_PER_KWH`: Energy cost per kWh
- `CPU_COST_PER_HOUR`: CPU cost per hour
- `CURRENCY`: Currency code
- `REGION`: Cloud region
- `PRICING_CONFIG`: Path to pricing config
- `ELECTRICITY_MAPS_TOKEN`: Optional token for real-time energy pricing
- `PROMETHEUS_URL`: Prometheus/VictoriaMetrics URL

**Example**:
```bash
python3 master/exporters/cost/cost_exporter.py \
    --port 9504 \
    --results-dir /var/lib/ffmpeg-rtmp/results \
    --energy-cost 0.12 \
    --cpu-cost 0.50 \
    --currency USD \
    --region us-east-1 \
    --pricing-config /etc/ffmpeg-rtmp/pricing_config.json \
    --prometheus-url http://localhost:8428
```

**Metrics Exposed**:
- `cost_total_load_aware`: Total cost (load-aware)
- `cost_energy_load_aware`: Energy cost component
- `cost_compute_load_aware`: Compute cost component
- `cost_per_pixel`: Cost efficiency per megapixel
- `cost_per_watch_hour`: Cost per viewer watch hour

**Dependencies**:
- Shared `advisor` module for cost calculations
- Regional pricing configuration

**Health Check**:
```bash
curl http://localhost:9504/health
```

---

### 4. Health Checker

**Purpose**: Monitors the health and availability of all exporters.

**Port**: 9600 (default)

**Command-line Options**:
- `--port`: HTTP server port (default: 9600)
- `--interval`: Check interval in seconds (optional, for continuous mode)

**Environment Variables**:
- `HEALTH_CHECK_PORT`: Server port

**Example**:
```bash
# Single check
python3 master/exporters/health_checker/check_exporters_health.py --port 9600

# Continuous monitoring (every 60 seconds)
python3 master/exporters/health_checker/check_exporters_health.py --port 9600 --interval 60
```

**Metrics Exposed**:
- `exporter_up{exporter="<name>"}`: Whether exporter is reachable (1=up, 0=down)
- `exporter_response_time_seconds{exporter="<name>"}`: Response time

**Health Check**:
```bash
curl http://localhost:9600/health
```

---

## Production Deployment with Systemd

For production environments, it's recommended to run exporters as systemd services.

### 1. Create Service User

```bash
# Create a dedicated user for exporters
sudo useradd --system --no-create-home --shell /bin/false ffmpeg-exporter
```

### 2. Create Directory Structure

```bash
# Create directories
sudo mkdir -p /opt/ffmpeg-rtmp/exporters
sudo mkdir -p /var/lib/ffmpeg-rtmp/results
sudo mkdir -p /etc/ffmpeg-rtmp

# Set ownership
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /opt/ffmpeg-rtmp/exporters
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /var/lib/ffmpeg-rtmp

# Copy exporter scripts and shared modules
sudo cp -r master/exporters/* /opt/ffmpeg-rtmp/exporters/
sudo cp -r shared/advisor /opt/ffmpeg-rtmp/exporters/advisor
sudo cp requirements.txt /opt/ffmpeg-rtmp/
sudo cp pricing_config.json /etc/ffmpeg-rtmp/

# Install dependencies as root or in a virtual environment
cd /opt/ffmpeg-rtmp
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
deactivate

# Adjust ownership
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /opt/ffmpeg-rtmp
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
WorkingDirectory=/opt/ffmpeg-rtmp/exporters

# Environment
Environment="RESULTS_EXPORTER_PORT=9502"
Environment="RESULTS_DIR=/var/lib/ffmpeg-rtmp/results"
Environment="PROMETHEUS_URL=http://localhost:8428"

# Command
ExecStart=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/results/results_exporter.py \
    --port 9502 \
    --results-dir /var/lib/ffmpeg-rtmp/results \
    --prometheus-url http://localhost:8428

# Restart policy
Restart=on-failure
RestartSec=10s

# Security
NoNewPrivileges=true
PrivateTmp=yes

# Resource limits
MemoryLimit=512M
CPUQuota=50%

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
WorkingDirectory=/opt/ffmpeg-rtmp/exporters

Environment="QOE_EXPORTER_PORT=9503"
Environment="RESULTS_DIR=/var/lib/ffmpeg-rtmp/results"

ExecStart=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/qoe/qoe_exporter.py \
    --port 9503 \
    --results-dir /var/lib/ffmpeg-rtmp/results

Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=yes
MemoryLimit=512M
CPUQuota=50%

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
WorkingDirectory=/opt/ffmpeg-rtmp/exporters

Environment="COST_EXPORTER_PORT=9504"
Environment="RESULTS_DIR=/var/lib/ffmpeg-rtmp/results"
Environment="ENERGY_COST_PER_KWH=0.12"
Environment="CPU_COST_PER_HOUR=0.50"
Environment="CURRENCY=USD"
Environment="REGION=us-east-1"
Environment="PRICING_CONFIG=/etc/ffmpeg-rtmp/pricing_config.json"
Environment="PROMETHEUS_URL=http://localhost:8428"
# Optional: Environment="ELECTRICITY_MAPS_TOKEN=your-token-here"

ExecStart=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/cost/cost_exporter.py \
    --port 9504 \
    --results-dir /var/lib/ffmpeg-rtmp/results \
    --energy-cost 0.12 \
    --cpu-cost 0.50 \
    --currency USD \
    --region us-east-1 \
    --pricing-config /etc/ffmpeg-rtmp/pricing_config.json \
    --prometheus-url http://localhost:8428

Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=yes
MemoryLimit=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

#### Health Checker Service

Create `/etc/systemd/system/ffmpeg-health-checker.service`:

```ini
[Unit]
Description=FFmpeg RTMP Health Checker
Documentation=https://github.com/psantana5/ffmpeg-rtmp
After=network.target

[Service]
Type=simple
User=ffmpeg-exporter
Group=ffmpeg-exporter
WorkingDirectory=/opt/ffmpeg-rtmp/exporters

Environment="HEALTH_CHECK_PORT=9600"

ExecStart=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/health_checker/check_exporters_health.py \
    --port 9600

Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=yes
MemoryLimit=256M
CPUQuota=25%

[Install]
WantedBy=multi-user.target
```

### 4. Enable and Start Services

```bash
# Reload systemd configuration
sudo systemctl daemon-reload

# Enable services (start on boot)
sudo systemctl enable ffmpeg-results-exporter.service
sudo systemctl enable ffmpeg-qoe-exporter.service
sudo systemctl enable ffmpeg-cost-exporter.service
sudo systemctl enable ffmpeg-health-checker.service

# Start services
sudo systemctl start ffmpeg-results-exporter.service
sudo systemctl start ffmpeg-qoe-exporter.service
sudo systemctl start ffmpeg-cost-exporter.service
sudo systemctl start ffmpeg-health-checker.service

# Check status
sudo systemctl status ffmpeg-results-exporter.service
sudo systemctl status ffmpeg-qoe-exporter.service
sudo systemctl status ffmpeg-cost-exporter.service
sudo systemctl status ffmpeg-health-checker.service
```

### 5. Verify Services

```bash
# Check all exporters are running
curl http://localhost:9502/health  # Results
curl http://localhost:9503/health  # QoE
curl http://localhost:9504/health  # Cost
curl http://localhost:9600/health  # Health Checker

# Check metrics endpoints
curl http://localhost:9502/metrics
curl http://localhost:9503/metrics
curl http://localhost:9504/metrics
curl http://localhost:9600/metrics
```

### 6. View Logs

```bash
# View logs for each service
sudo journalctl -u ffmpeg-results-exporter.service -f
sudo journalctl -u ffmpeg-qoe-exporter.service -f
sudo journalctl -u ffmpeg-cost-exporter.service -f
sudo journalctl -u ffmpeg-health-checker.service -f

# View last 50 lines
sudo journalctl -u ffmpeg-results-exporter.service -n 50
```

---

## Firewall Configuration

If you have a firewall enabled, allow the exporter ports:

```bash
# Using ufw
sudo ufw allow 9502/tcp comment 'Results Exporter'
sudo ufw allow 9503/tcp comment 'QoE Exporter'
sudo ufw allow 9504/tcp comment 'Cost Exporter'
sudo ufw allow 9600/tcp comment 'Health Checker'

# Using firewalld
sudo firewall-cmd --permanent --add-port=9502/tcp
sudo firewall-cmd --permanent --add-port=9503/tcp
sudo firewall-cmd --permanent --add-port=9504/tcp
sudo firewall-cmd --permanent --add-port=9600/tcp
sudo firewall-cmd --reload
```

---

## VictoriaMetrics Scrape Configuration

Add the exporters to your VictoriaMetrics scrape configuration (`master/monitoring/victoriametrics.yml`):

```yaml
scrape_configs:
  # ... existing jobs ...

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

  - job_name: 'health-checker'
    static_configs:
      - targets: ['localhost:9600']
    scrape_interval: 30s
```

Reload VictoriaMetrics configuration:
```bash
curl -X POST http://localhost:8428/-/reload
```

---

## Troubleshooting

### Service Fails to Start

**Check logs**:
```bash
sudo journalctl -u ffmpeg-results-exporter.service -n 50
```

**Common issues**:

1. **Permission denied on results directory**:
   ```bash
   sudo chown -R ffmpeg-exporter:ffmpeg-exporter /var/lib/ffmpeg-rtmp/results
   sudo chmod 755 /var/lib/ffmpeg-rtmp/results
   ```

2. **Missing Python dependencies**:
   ```bash
   source /opt/ffmpeg-rtmp/venv/bin/activate
   pip install -r /opt/ffmpeg-rtmp/requirements.txt
   ```

3. **Missing shared advisor module**:
   ```bash
   sudo cp -r shared/advisor /opt/ffmpeg-rtmp/exporters/
   sudo chown -R ffmpeg-exporter:ffmpeg-exporter /opt/ffmpeg-rtmp/exporters/advisor
   ```

4. **Port already in use**:
   ```bash
   sudo lsof -i :9502
   # Kill the process or change the port in service file
   ```

### Exporter Returns No Metrics

1. **Check if test results exist**:
   ```bash
   ls -l /var/lib/ffmpeg-rtmp/results/
   ```

2. **Verify results directory in service file**:
   ```bash
   grep RESULTS_DIR /etc/systemd/system/ffmpeg-results-exporter.service
   ```

3. **Check exporter can read results**:
   ```bash
   sudo -u ffmpeg-exporter ls -l /var/lib/ffmpeg-rtmp/results/
   ```

### VictoriaMetrics Not Scraping

1. **Check VictoriaMetrics logs**:
   ```bash
   docker compose logs victoriametrics
   ```

2. **Verify scrape targets**:
   ```bash
   curl http://localhost:8428/targets
   ```

3. **Test connectivity from VictoriaMetrics container**:
   ```bash
   docker exec victoriametrics curl http://host.docker.internal:9502/metrics
   ```

---

## Upgrading Exporters

```bash
# Stop services
sudo systemctl stop ffmpeg-results-exporter.service
sudo systemctl stop ffmpeg-qoe-exporter.service
sudo systemctl stop ffmpeg-cost-exporter.service
sudo systemctl stop ffmpeg-health-checker.service

# Update code
cd /path/to/ffmpeg-rtmp
git pull

# Copy updated files
sudo cp -r master/exporters/* /opt/ffmpeg-rtmp/exporters/
sudo cp -r shared/advisor /opt/ffmpeg-rtmp/exporters/advisor
sudo chown -R ffmpeg-exporter:ffmpeg-exporter /opt/ffmpeg-rtmp/exporters

# Update dependencies if needed
source /opt/ffmpeg-rtmp/venv/bin/activate
pip install --upgrade -r requirements.txt
deactivate

# Restart services
sudo systemctl start ffmpeg-results-exporter.service
sudo systemctl start ffmpeg-qoe-exporter.service
sudo systemctl start ffmpeg-cost-exporter.service
sudo systemctl start ffmpeg-health-checker.service

# Verify
sudo systemctl status ffmpeg-*-exporter.service
```

---

## Security Considerations

1. **Run as dedicated user**: Never run exporters as root
2. **Restrict file permissions**: Results directory should be readable only by exporter user
3. **Use firewall rules**: Limit access to exporter ports to trusted networks
4. **Rotate logs**: Configure journald log rotation
5. **Monitor resource usage**: Set memory and CPU limits in systemd
6. **Keep dependencies updated**: Regularly update Python packages

---

## Performance Tuning

### Adjust Cache TTL

Exporters cache metrics to reduce disk I/O. Adjust in the Python code:

```python
# In results_exporter.py, qoe_exporter.py, cost_exporter.py
self.cache_ttl = 60  # Increase to reduce disk reads
```

### Limit Result Files

Only keep recent results to improve performance:

```bash
# Keep only last 7 days of results
find /var/lib/ffmpeg-rtmp/results -name "test_results_*.json" -mtime +7 -delete
```

### Adjust Systemd Resource Limits

```ini
# In service file
MemoryLimit=1G      # Increase if needed
CPUQuota=100%       # Increase for more CPU
```

---

## Alternative: Running Without Systemd

For systems without systemd (e.g., containers, minimal Linux), use a process manager like `supervisord`:

### Install Supervisor

```bash
pip install supervisor
```

### Create Supervisor Config

Create `/etc/supervisor/conf.d/ffmpeg-exporters.conf`:

```ini
[program:results-exporter]
command=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/results/results_exporter.py --port 9502 --results-dir /var/lib/ffmpeg-rtmp/results
directory=/opt/ffmpeg-rtmp/exporters
user=ffmpeg-exporter
autostart=true
autorestart=true
stdout_logfile=/var/log/results-exporter.log
stderr_logfile=/var/log/results-exporter.err

[program:qoe-exporter]
command=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/qoe/qoe_exporter.py --port 9503 --results-dir /var/lib/ffmpeg-rtmp/results
directory=/opt/ffmpeg-rtmp/exporters
user=ffmpeg-exporter
autostart=true
autorestart=true
stdout_logfile=/var/log/qoe-exporter.log
stderr_logfile=/var/log/qoe-exporter.err

[program:cost-exporter]
command=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/cost/cost_exporter.py --port 9504 --results-dir /var/lib/ffmpeg-rtmp/results --energy-cost 0.12 --cpu-cost 0.50
directory=/opt/ffmpeg-rtmp/exporters
user=ffmpeg-exporter
autostart=true
autorestart=true
stdout_logfile=/var/log/cost-exporter.log
stderr_logfile=/var/log/cost-exporter.err

[program:health-checker]
command=/opt/ffmpeg-rtmp/venv/bin/python3 /opt/ffmpeg-rtmp/exporters/health_checker/check_exporters_health.py --port 9600
directory=/opt/ffmpeg-rtmp/exporters
user=ffmpeg-exporter
autostart=true
autorestart=true
stdout_logfile=/var/log/health-checker.log
stderr_logfile=/var/log/health-checker.err
```

### Start Supervisor

```bash
supervisord -c /etc/supervisor/supervisord.conf
supervisorctl reread
supervisorctl update
supervisorctl status
```

---

## Related Documentation

- [Master Node README](../README.md) - Master node overview
- [Production Deployment](../../deployment/README.md) - Complete production setup
- [Worker Exporters](../../worker/exporters/README.md) - Worker exporter deployment

---

## Support

For issues with exporter deployment:

1. Check logs: `sudo journalctl -u ffmpeg-*-exporter.service -n 100`
2. Verify file permissions and ownership
3. Test exporter manually before using systemd
4. Open an issue: https://github.com/psantana5/ffmpeg-rtmp/issues

Include in your issue:
- Exporter logs
- Service status output
- System information (OS, Python version)
- Configuration files (redact sensitive information)
