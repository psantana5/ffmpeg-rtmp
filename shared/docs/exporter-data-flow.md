# Exporter Data Flow and Architecture Documentation

This document provides a comprehensive overview of how each exporter in the system receives its data, the complete setup process, data flow architecture, and troubleshooting guidance.

## Table of Contents

1. [System Overview](#system-overview)
2. [Exporter Details](#exporter-details)
3. [Data Flow Architecture](#data-flow-architecture)
4. [Setup Guide](#setup-guide)
5. [Troubleshooting](#troubleshooting)

---

## System Overview

The system consists of multiple exporters that collect metrics from different sources and expose them in Prometheus format. Prometheus scrapes these exporters every 5 seconds (configurable in `prometheus.yml`).

### Exporter Categories

1. **System Metrics Exporters**: Collect OS and hardware metrics
   - rapl-exporter (Power consumption)
   - node-exporter (System metrics)
   - cadvisor (Container metrics)

2. **Application Metrics Exporters**: Monitor running services
   - nginx-rtmp-exporter (RTMP streaming)
   - docker-stats-exporter (Docker overhead)

3. **Analysis Exporters**: Process test results and calculate derived metrics
   - results-exporter (Test results analysis)
   - qoe-exporter (Quality of Experience)
   - cost-exporter (Cost analysis)

4. **Health Monitoring**:
   - exporter-health-checker (Monitors all exporters)

---

## Exporter Details

### 1. RAPL Exporter (rapl-exporter)

**Port**: 9500
**Data Source**: Linux kernel RAPL interface (`/sys/class/powercap`)

#### How It Gets Data

1. **Direct Kernel Access**: Reads from `/sys/class/powercap/intel-rapl:*`
2. **Hardware Counters**: Intel CPUs expose power consumption through RAPL registers
3. **Privileged Access**: Requires privileged container mode or root access

#### Data Flow

```
Intel CPU RAPL Registers
    ↓
/sys/class/powercap/intel-rapl:*/energy_uj
    ↓
rapl_exporter.py reads files
    ↓
Calculates power (watts) from energy delta
    ↓
Exposes metrics at :9500/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics

```
rapl_power_watts{package="package-0", zone="package-0"} 45.5
rapl_power_watts{package="package-0", zone="core"} 30.2
rapl_energy_joules_total{package="package-0", zone="package-0"} 1234567.89
```

#### Setup Requirements

1. **Intel CPU** with RAPL support (most Intel CPUs since Sandy Bridge)
2. **Privileged container** with `/sys/class/powercap` mounted read-only
3. **Host kernel** must expose RAPL counters

```yaml
# docker-compose.yml excerpt
rapl-exporter:
  privileged: true
  volumes:
    - /sys/class/powercap:/sys/class/powercap:ro
    - /sys/devices:/sys/devices:ro
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| No metrics | Check if RAPL is available: `ls /sys/class/powercap/intel-rapl:*` |
| Permission denied | Run container as privileged or with CAP_SYS_RAWIO |
| Zero values | Some systems disable RAPL. Check BIOS settings |

---

### 2. Docker Stats Exporter (docker-stats-exporter)

**Port**: 9501
**Data Source**: Docker daemon API via `/var/run/docker.sock`

#### How It Gets Data

1. **Docker Socket**: Connects to Docker daemon through Unix socket
2. **Container Stats**: Uses `docker stats` command via subprocess
3. **Process Stats**: Reads Docker engine process stats from `/proc`

#### Data Flow

```
Docker Daemon (dockerd)
    ↓
/var/run/docker.sock (Unix socket)
    ↓
docker_stats_exporter.py executes:
  - docker stats --no-stream
  - ps aux | grep dockerd
    ↓
Parses output and converts to metrics
    ↓
Exposes metrics at :9501/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics

```
docker_engine_cpu_percent 2.5
docker_engine_memory_percent 1.2
docker_container_cpu_percent{name="nginx-rtmp"} 15.3
docker_container_memory_percent{name="nginx-rtmp"} 2.1
```

#### Setup Requirements

1. **Docker socket** mounted into container
2. **Host /proc** filesystem mounted for process stats

```yaml
# docker-compose.yml excerpt
docker-stats-exporter:
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:ro
    - /proc:/host/proc:ro
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Cannot connect to Docker | Check socket mount: `ls -la /var/run/docker.sock` |
| Permission denied | Add user to docker group or run as root |
| No container stats | Ensure Docker containers are running |

---

### 3. Node Exporter (node-exporter)

**Port**: 9100
**Data Source**: Host system metrics from `/proc`, `/sys`

#### How It Gets Data

1. **Filesystem Access**: Reads from `/proc`, `/sys`, and root filesystem
2. **System Interfaces**: Accesses kernel-exposed metrics
3. **Standard Linux Monitoring**: Uses standard OS interfaces

#### Data Flow

```
Host System Kernel
    ↓
/proc/* (CPU, memory, network)
/sys/* (hardware info)
    ↓
node_exporter reads pseudo-files
    ↓
Converts to Prometheus metrics
    ↓
Exposes metrics at :9100/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics (850+ metrics)

```
node_cpu_seconds_total{cpu="0",mode="idle"} 1234567.89
node_memory_MemAvailable_bytes 8589934592
node_network_receive_bytes_total{device="eth0"} 123456789
node_disk_io_time_seconds_total{device="sda"} 456.78
```

#### Setup Requirements

1. **Host filesystem** mounted into container
2. **Proper path remapping** via command-line flags

```yaml
# docker-compose.yml excerpt
node-exporter:
  pid: host
  volumes:
    - /proc:/host/proc:ro
    - /sys:/host/sys:ro
    - /:/rootfs:ro
  command:
    - "--path.procfs=/host/proc"
    - "--path.sysfs=/host/sys"
    - "--path.rootfs=/rootfs"
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Missing metrics | Check volume mounts are correct |
| No CPU metrics | Verify /proc is mounted at /host/proc |
| No network metrics | Check /sys/class/net is accessible |

---

### 4. cAdvisor (cadvisor)

**Port**: 8080
**Data Source**: Docker daemon + cgroup filesystem

#### How It Gets Data

1. **cgroups**: Reads container resource usage from `/sys/fs/cgroup`
2. **Docker API**: Gets container metadata from Docker
3. **Filesystem Metrics**: Monitors container filesystem usage

#### Data Flow

```
Linux cgroups (/sys/fs/cgroup/*)
    +
Docker daemon (container metadata)
    +
Container filesystem (/var/lib/docker)
    ↓
cadvisor aggregates data
    ↓
Exposes metrics at :8080/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics (120+ metrics per container)

```
container_cpu_usage_seconds_total{name="nginx-rtmp"} 123.45
container_memory_usage_bytes{name="nginx-rtmp"} 134217728
container_network_receive_bytes_total{name="nginx-rtmp"} 1234567
container_fs_usage_bytes{name="nginx-rtmp"} 52428800
```

#### Setup Requirements

1. **Privileged mode** for full container visibility
2. **Multiple volume mounts** for complete access

```yaml
# docker-compose.yml excerpt
cadvisor:
  privileged: true
  volumes:
    - /:/rootfs:ro
    - /var/run:/var/run:ro
    - /sys:/sys:ro
    - /var/lib/docker/:/var/lib/docker:ro
    - /dev/disk/:/dev/disk:ro
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Missing containers | Check /var/lib/docker mount |
| No metrics | Verify privileged mode is enabled |
| High CPU usage | Normal for cAdvisor; limit with --housekeeping_interval |

---

### 5. Nginx RTMP Exporter (nginx-rtmp-exporter)

**Port**: 9728
**Data Source**: Nginx RTMP stat endpoint

#### How It Gets Data

1. **HTTP Stat Endpoint**: Queries `http://nginx-rtmp/stat` (XML format)
2. **Nginx Built-in Stats**: Nginx RTMP module exposes internal state
3. **Real-time Monitoring**: Gets current connection and stream info

#### Data Flow

```
Nginx RTMP Server (streaming activity)
    ↓
/stat endpoint (HTTP, XML format)
    ↓
nginx-rtmp-exporter queries endpoint
    ↓
Parses XML and converts to metrics
    ↓
Exposes metrics at :9728/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics

```
nginx_rtmp_connections 5
nginx_rtmp_streams 2
nginx_rtmp_bandwidth_in_bytes 1500000
nginx_rtmp_bandwidth_out_bytes 3000000
```

#### Setup Requirements

1. **Nginx RTMP** must be running
2. **Stat endpoint** enabled in nginx.conf
3. **Network connectivity** to nginx-rtmp container

```nginx
# nginx.conf excerpt
rtmp {
    server {
        listen 1935;
        application live {
            live on;
        }
    }
}

http {
    server {
        listen 80;
        location /stat {
            rtmp_stat all;
            rtmp_stat_stylesheet stat.xsl;
        }
    }
}
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Cannot reach /stat | Check nginx-rtmp is running and /stat is enabled |
| No streams | Start FFmpeg stream: `ffmpeg -re -i video.mp4 -c copy -f flv rtmp://localhost/live/stream` |
| Connection refused | Verify network connectivity between containers |

---

### 6. Results Exporter (results-exporter)

**Port**: 9502
**Data Source**: Test result JSON files + Prometheus historical data

#### How It Gets Data

1. **JSON Files**: Reads from `/results/test_results_*.json` (mounted volume)
2. **Prometheus Queries**: Fetches historical metrics for each test scenario
3. **Time-windowed Queries**: Uses start_time and end_time from test results

#### Data Flow

```
run_tests.py executes FFmpeg tests
    ↓
Writes test_results/test_results_TIMESTAMP.json
    ↓
results_exporter reads latest JSON
    ↓
For each scenario:
  - Queries Prometheus for metrics in [start_time, end_time]
  - Aggregates: avg power, max CPU, etc.
    ↓
Exposes scenario metrics at :9502/metrics
    ↓
Prometheus scrapes every 5s (but exporter caches for 60s)
```

#### Exposed Metrics

```
scenario_duration_seconds{scenario="1_stream_1080p"} 60
scenario_power_mean_watts{scenario="1_stream_1080p"} 48.5
scenario_power_max_watts{scenario="1_stream_1080p"} 52.3
scenario_cpu_mean_percent{scenario="1_stream_1080p"} 25.5
scenario_baseline_diff_power_watts{scenario="1_stream_1080p"} 8.2
```

#### Setup Requirements

1. **Test results** directory mounted
2. **Prometheus URL** configured
3. **Valid test results** with timestamps

```yaml
# docker-compose.yml excerpt
results-exporter:
  volumes:
    - ./test_results:/results
  environment:
    - RESULTS_DIR=/results
    - PROMETHEUS_URL=http://victoriametrics:8428
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| No scenarios found | Run tests: `python3 scripts/run_tests.py` |
| No test results | Check mount: `docker exec results-exporter ls /results` |
| Stale metrics | Results exporter caches for 60s; wait or restart |
| Prometheus errors | Verify PROMETHEUS_URL is correct and Prometheus is accessible |

---

### 7. QoE Exporter (qoe-exporter)

**Port**: 9503
**Data Source**: Test result JSON files

#### How It Gets Data

1. **JSON Files**: Reads from `/results/test_results_*.json`
2. **Quality Metrics**: Extracts VMAF, PSNR, SSIM from test results
3. **Calculation**: Computes QoE scores using advisor module

#### Data Flow

```
run_tests.py executes tests with quality analysis
    ↓
Calculates VMAF/PSNR/SSIM during tests
    ↓
Writes results to test_results/test_results_TIMESTAMP.json
    ↓
qoe_exporter reads latest JSON
    ↓
Extracts quality metrics and calculates QoE scores
    ↓
Exposes metrics at :9503/metrics
    ↓
Prometheus scrapes every 5s (cached for 60s)
```

#### Exposed Metrics

```
qoe_score{scenario="1_stream_1080p"} 4.2
quality_vmaf{scenario="1_stream_1080p"} 95.5
quality_psnr{scenario="1_stream_1080p"} 42.3
quality_ssim{scenario="1_stream_1080p"} 0.98
```

#### Setup Requirements

1. **Test results** with quality metrics
2. **Advisor module** available in Python path

```yaml
# docker-compose.yml excerpt
qoe-exporter:
  volumes:
    - ./test_results:/results:ro
    - ./advisor:/app/advisor:ro
  environment:
    - RESULTS_DIR=/results
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| No quality metrics | Ensure tests are run with --analyze-quality flag |
| Import errors | Check advisor module is mounted correctly |
| Zero values | Quality analysis may have failed during tests |

---

### 8. Cost Exporter (cost-exporter)

**Port**: 9504
**Data Source**: Test result JSON files + Prometheus historical data

#### How It Gets Data

1. **JSON Files**: Reads from `/results/test_results_*.json`
2. **Prometheus Queries**: Fetches CPU usage and power data for load-aware calculations
3. **Time-series Integration**: Sums CPU-seconds and energy (joules) over test duration

#### Data Flow

```
run_tests.py executes tests
    ↓
Writes test_results/test_results_TIMESTAMP.json
  (includes start_time, end_time, duration)
    ↓
cost_exporter reads latest JSON
    ↓
For each scenario:
  - Queries Prometheus for:
    * rate(container_cpu_usage_seconds_total[30s])
    * sum(rapl_power_watts)
  - Integrates over time window
  - Calculates costs based on pricing config
    ↓
Exposes cost metrics at :9504/metrics
    ↓
Prometheus scrapes every 5s (cached for 60s)
```

#### Exposed Metrics

```
cost_exporter_alive 1
cost_total_load_aware{scenario="1_stream_1080p",currency="USD"} 0.00234
cost_energy_load_aware{scenario="1_stream_1080p",currency="USD"} 0.00012
cost_compute_load_aware{scenario="1_stream_1080p",currency="USD"} 0.00222
```

#### Setup Requirements

1. **Test results** with timestamps
2. **Prometheus URL** for load-aware mode
3. **Pricing configuration** via environment variables

```yaml
# docker-compose.yml excerpt
cost-exporter:
  volumes:
    - ./test_results:/results:ro
    - ./advisor:/app/advisor:ro
  environment:
    - RESULTS_DIR=/results
    - PROMETHEUS_URL=http://victoriametrics:8428
    - ENERGY_COST_PER_KWH=0.12
    - CPU_COST_PER_HOUR=0.50
    - CURRENCY=USD
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Zero cost values | Check if Prometheus data is available for test time windows |
| No load-aware data | Verify PROMETHEUS_URL is set and accessible |
| Missing metrics | Enable debug logging: `docker logs cost-exporter --tail 100` |
| Stale prices | Restart container after changing environment variables |

---

### 9. DCGM Exporter (dcgm-exporter) - NVIDIA GPU

**Port**: 9400
**Profile**: nvidia (optional)
**Data Source**: NVIDIA GPU via DCGM library

#### How It Gets Data

1. **NVIDIA DCGM**: Connects to GPU through Data Center GPU Manager
2. **GPU Telemetry**: Reads power, utilization, temperature, memory
3. **CUDA Runtime**: Requires nvidia-container-runtime

#### Data Flow

```
NVIDIA GPU Hardware
    ↓
NVIDIA Driver (host)
    ↓
nvidia-container-runtime
    ↓
DCGM library in container
    ↓
dcgm-exporter queries GPU
    ↓
Exposes metrics at :9400/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics

```
DCGM_FI_DEV_GPU_UTIL 75
DCGM_FI_DEV_POWER_USAGE 180.5
DCGM_FI_DEV_GPU_TEMP 65
DCGM_FI_DEV_FB_USED 4096
```

#### Setup Requirements

1. **NVIDIA GPU** installed
2. **NVIDIA drivers** on host
3. **nvidia-container-toolkit** installed
4. **Docker Compose** nvidia profile

```bash
# Install nvidia-container-toolkit
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker

# Start with NVIDIA profile
docker compose --profile nvidia up -d
```

```yaml
# docker-compose.yml excerpt
dcgm-exporter:
  profiles:
    - nvidia
  runtime: nvidia
  cap_add:
    - SYS_ADMIN
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Container won't start | Check nvidia-docker: `docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi` |
| No GPU metrics | Verify GPU is visible: `nvidia-smi` |
| Permission denied | Add SYS_ADMIN capability |
| Wrong runtime | Ensure docker-compose uses `runtime: nvidia` |

---

### 10. Exporter Health Checker (exporter-health-checker)

**Port**: 9600
**Data Source**: All other exporters' /metrics endpoints

#### How It Gets Data

1. **HTTP Requests**: Periodically fetches /metrics from each exporter
2. **Metric Validation**: Parses response and validates expected metrics
3. **Health Assessment**: Checks reachability, metrics presence, and data availability

#### Data Flow

```
All exporters expose :PORT/metrics
    ↓
exporter-health-checker periodically fetches:
  - nginx-rtmp-exporter:9728/metrics
  - rapl-exporter:9500/metrics
  - docker-stats-exporter:9501/metrics
  - (etc...)
    ↓
For each exporter:
  - Validates HTTP 200 response
  - Parses Prometheus metrics format
  - Checks expected metrics are present
  - Verifies data exists (not just definitions)
    ↓
Exposes health metrics at :9600/metrics
    ↓
Prometheus scrapes every 5s
```

#### Exposed Metrics

```
exporter_health_status{exporter="rapl-exporter"} 1
exporter_reachable{exporter="rapl-exporter"} 1
exporter_metric_count{exporter="rapl-exporter"} 12
exporter_sample_count{exporter="rapl-exporter"} 48
exporter_has_data{exporter="rapl-exporter"} 1
```

#### Setup Requirements

1. **Network access** to all exporters
2. **Python 3.11+** runtime

```yaml
# docker-compose.yml excerpt
exporter-health-checker:
  networks:
    - streaming-net
  command: ["--port", "9600"]
```

#### Troubleshooting

| Problem | Solution |
|---------|----------|
| Cannot reach exporters | Check all exporters are running: `docker ps` |
| Wrong URLs | Verify exporter names match docker-compose service names |
| Timeout errors | Increase timeout: `--timeout 30` |

---

## Data Flow Architecture

### Complete System Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Data Sources                                │
├─────────────────────────────────────────────────────────────────────┤
│ • Intel RAPL (/sys/class/powercap)                                  │
│ • Docker Daemon (/var/run/docker.sock)                              │
│ • Linux Kernel (/proc, /sys)                                        │
│ • cgroups (/sys/fs/cgroup)                                          │
│ • Nginx RTMP (HTTP stat endpoint)                                   │
│ • Test Results (JSON files)                                         │
│ • NVIDIA GPU (DCGM API)                                             │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│                      Exporters (Collectors)                          │
├─────────────────────────────────────────────────────────────────────┤
│ rapl-exporter:9500        │ Reads RAPL counters                     │
│ docker-stats:9501         │ Queries Docker API                      │
│ node-exporter:9100        │ Reads /proc, /sys                       │
│ cadvisor:8080             │ Reads cgroups                           │
│ nginx-exporter:9728       │ Queries Nginx /stat                     │
│ results-exporter:9502     │ Reads JSON + queries Prometheus         │
│ qoe-exporter:9503         │ Reads JSON, calculates QoE              │
│ cost-exporter:9504        │ Reads JSON + queries Prometheus         │
│ dcgm-exporter:9400        │ Queries NVIDIA GPU                      │
│ health-checker:9600       │ Queries all exporters                   │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
            All expose Prometheus metrics at /metrics
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│              Prometheus (Time-series Database)                       │
├─────────────────────────────────────────────────────────────────────┤
│ • Scrapes all exporters every 5 seconds                             │
│ • Stores metrics with 7-day retention                               │
│ • Evaluates alert rules every 5 seconds                             │
│ • Provides PromQL query interface                                   │
└─────────────────────────────────────────────────────────────────────┘
        ↓                           ↓                      ↓
┌─────────────┐         ┌──────────────────┐    ┌──────────────────┐
│  Grafana    │         │  Alertmanager    │    │ Analysis         │
│  (Viz)      │         │  (Alerts)        │    │ Exporters        │
│  :3000      │         │  :9093           │    │ (Re-query)       │
└─────────────┘         └──────────────────┘    └──────────────────┘
                                                           ↓
                                              results/qoe/cost exporters
                                              query Prometheus for
                                              historical test data
```

### Test Execution Flow

```
1. User runs: python3 scripts/run_tests.py

2. Test runner:
   ├─ Records start_time = current timestamp
   ├─ Starts FFmpeg streaming to Nginx RTMP
   ├─ Waits for test duration
   ├─ Stops FFmpeg
   ├─ Records end_time = current timestamp
   └─ Writes test_results_TIMESTAMP.json

3. All exporters collect metrics during test:
   ├─ rapl-exporter → power consumption
   ├─ cadvisor → container CPU/memory
   ├─ node-exporter → system metrics
   └─ nginx-exporter → streaming stats

4. Prometheus stores all metrics with timestamps

5. Analysis exporters process results:
   ├─ results-exporter queries Prometheus for [start_time, end_time]
   ├─ Aggregates power, CPU, etc. for the test window
   ├─ qoe-exporter reads quality metrics from JSON
   └─ cost-exporter queries Prometheus + calculates costs

6. Grafana visualizes:
   ├─ Real-time metrics from Prometheus
   └─ Aggregated scenario metrics from analysis exporters
```

---

## Setup Guide

### Prerequisites

1. **Docker & Docker Compose**
   ```bash
   # Install Docker
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   sudo usermod -aG docker $USER

   # Install Docker Compose
   sudo apt-get install docker-compose-plugin
   ```

2. **Python 3** (for test runner)
   ```bash
   sudo apt-get install python3 python3-pip
   pip3 install -r requirements.txt
   ```

3. **FFmpeg** (for streaming tests)
   ```bash
   sudo apt-get install ffmpeg
   ```

### Step-by-Step Setup

#### 1. Clone and Build

```bash
git clone <repository-url>
cd ffmpeg-rtmp
docker compose up -d --build
```

#### 2. Verify Exporters

```bash
# Check all containers are running
docker ps

# Test each exporter
curl http://localhost:9500/metrics | head  # RAPL
curl http://localhost:9501/metrics | head  # Docker Stats
curl http://localhost:9100/metrics | head  # Node Exporter
curl http://localhost:8080/metrics | head  # cAdvisor
curl http://localhost:9728/metrics | head  # Nginx RTMP
curl http://localhost:9502/metrics | head  # Results
curl http://localhost:9503/metrics | head  # QoE
curl http://localhost:9504/metrics | head  # Cost
curl http://localhost:9600/metrics | head  # Health Check
```

#### 3. Check Prometheus Targets

```bash
# Open Prometheus UI
open http://localhost:8428/targets

# Or check via CLI
curl -s http://localhost:8428/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
```

All targets should show `health: "up"`.

#### 4. Run Test Suite

```bash
# Run a quick single-stream test
python3 scripts/run_tests.py --name quick --streams 1 --duration 60

# Check results were created
ls -lh test_results/

# Verify results exporter picked it up
curl http://localhost:9502/metrics | grep scenario_
```

#### 5. Open Grafana

```bash
open http://localhost:3000
# Login: admin / admin
```

Navigate to pre-provisioned dashboards:
- Power Monitoring Dashboard
- Cost Dashboard
- QoE Dashboard

---

## Troubleshooting

### General Debugging Steps

1. **Check Container Status**
   ```bash
   docker ps -a
   docker logs <container-name>
   ```

2. **Check Exporter Health**
   ```bash
   # Run health checker
   python3 check_exporters_health.py

   # Or check via container
   docker exec exporter-health-checker python3 /app/check_exporters_health.py
   ```

3. **Check Prometheus Scrape Errors**
   ```bash
   # View Prometheus logs
   docker logs prometheus

   # Check targets page
   curl http://localhost:8428/api/v1/targets
   ```

4. **Enable Debug Logging**
   ```bash
   # For cost-exporter
   docker logs cost-exporter --follow

   # Restart with debug logging
   docker compose stop cost-exporter
   docker compose up cost-exporter
   ```

### Common Issues

#### Issue: No Power Metrics (RAPL)

**Symptoms**: `rapl_power_watts` not in Prometheus

**Solutions**:
1. Check RAPL availability:
   ```bash
   ls -la /sys/class/powercap/intel-rapl:*
   ```

2. Verify container has access:
   ```bash
   docker exec rapl-exporter ls /sys/class/powercap
   ```

3. Check for Intel CPU:
   ```bash
   cat /proc/cpuinfo | grep "model name"
   ```

4. Some systems disable RAPL - check BIOS settings

#### Issue: Cost Exporter Shows Zero Values

**Symptoms**: `cost_total_load_aware{...} 0`

**Diagnosis**:
1. Check if test results exist:
   ```bash
   docker exec cost-exporter ls /results
   ```

2. Enable debug logging:
   ```bash
   docker logs cost-exporter 2>&1 | grep DEBUG
   ```

3. Check Prometheus connectivity:
   ```bash
   docker exec cost-exporter curl -s http://victoriametrics:8428/-/healthy
   ```

**Solutions**:
- Run tests to generate data: `python3 scripts/run_tests.py`
- Verify PROMETHEUS_URL environment variable
- Check that test results have start_time and end_time
- Verify Prometheus has data for the test time window

#### Issue: Nginx Exporter No Data

**Symptoms**: `nginx_rtmp_streams 0` even during active streaming

**Solutions**:
1. Check Nginx is receiving streams:
   ```bash
   curl http://localhost:8080/stat
   ```

2. Start a test stream:
   ```bash
   ffmpeg -re -f lavfi -i testsrc=duration=60:size=1280x720:rate=30 \
          -f lavfi -i sine=frequency=1000:duration=60 \
          -c:v libx264 -preset veryfast -b:v 1000k \
          -c:a aac -b:a 128k \
          -f flv rtmp://localhost/live/test
   ```

3. Check nginx-exporter logs:
   ```bash
   docker logs nginx-exporter
   ```

#### Issue: Results Exporter Not Updating

**Symptoms**: Old scenarios in metrics

**Solutions**:
1. Check cache TTL (60 seconds default)
2. Force refresh by restarting:
   ```bash
   docker restart results-exporter
   ```

3. Verify new test results:
   ```bash
   ls -lt test_results/*.json | head -1
   ```

#### Issue: Prometheus Not Scraping

**Symptoms**: Exporter UP but no metrics in Prometheus

**Solutions**:
1. Check Prometheus config:
   ```bash
   docker exec prometheus cat /etc/prometheus/prometheus.yml
   ```

2. Reload configuration:
   ```bash
   curl -X POST http://localhost:8428/-/reload
   ```

3. Check scrape errors:
   ```bash
   curl http://localhost:8428/api/v1/targets | jq '.data.activeTargets[] | select(.health != "up")'
   ```

#### Issue: High Resource Usage

**Symptoms**: CPU/memory usage very high

**Solutions**:
1. **cAdvisor**: Increase housekeeping interval
   ```yaml
   cadvisor:
     command:
       - --housekeeping_interval=30s
   ```

2. **Prometheus**: Reduce scrape frequency
   ```yaml
   global:
     scrape_interval: 15s  # instead of 5s
   ```

3. **Reduce retention**:
   ```yaml
   prometheus:
     command:
       - "--storage.tsdb.retention.time=3d"  # instead of 7d
   ```

### Debug Checklist

Use this checklist for systematic troubleshooting:

- [ ] All containers running: `docker ps`
- [ ] All exporters responding: `curl localhost:<port>/metrics`
- [ ] Prometheus targets UP: `http://localhost:8428/targets`
- [ ] Test results exist: `ls test_results/`
- [ ] RAPL available: `ls /sys/class/powercap/`
- [ ] Docker socket accessible: `docker ps`
- [ ] Prometheus can query exporters: Check service discovery
- [ ] Grafana datasource connected: Check Grafana settings
- [ ] No network issues: `docker network ls`
- [ ] Sufficient disk space: `df -h`

### Getting Help

If issues persist:

1. **Collect logs**:
   ```bash
   docker logs prometheus > prometheus.log
   docker logs cost-exporter > cost-exporter.log
   docker logs rapl-exporter > rapl-exporter.log
   ```

2. **Run health check**:
   ```bash
   python3 check_exporters_health.py --debug > health-check.log
   ```

3. **Check system info**:
   ```bash
   uname -a > system-info.txt
   docker version >> system-info.txt
   docker compose version >> system-info.txt
   cat /proc/cpuinfo | grep "model name" >> system-info.txt
   ```

4. **Create minimal reproduction**:
   ```bash
   # Stop everything
   docker compose down

   # Start only essentials
   docker compose up -d prometheus rapl-exporter

   # Test minimal setup
   curl http://localhost:9500/metrics
   curl http://localhost:8428/api/v1/query?query=rapl_power_watts
   ```

---

## Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [RAPL Documentation](https://www.kernel.org/doc/html/latest/power/powercap/powercap.html)
- [Nginx RTMP Module](https://github.com/arut/nginx-rtmp-module)
- [cAdvisor Documentation](https://github.com/google/cadvisor)
- [Node Exporter Documentation](https://github.com/prometheus/node_exporter)
