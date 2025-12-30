# Distributed Architecture: Master-Worker Component Distribution

## Overview

The FFmpeg RTMP power monitoring system uses a **master-worker architecture** for distributed compute workloads. This document clarifies which components run on which nodes and why.

## Component Distribution

### Master Node Components

The **master node** runs the orchestration service and monitoring stack:

```
┌─────────────────────────────────────────────────────────────┐
│                      MASTER NODE                             │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  Master Orchestrator (bin/master)                  │    │
│  │  • Port 8080 (HTTPS)                               │    │
│  │  • Job queue management                            │    │
│  │  • Node registration                               │    │
│  │  • Result aggregation                              │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  Monitoring Stack (Docker Compose)                 │    │
│  │                                                     │    │
│  │  • VictoriaMetrics (port 8428)                     │    │
│  │    - Time-series database                          │    │
│  │    - Scrapes metrics from ALL nodes                │    │
│  │    - 30-day retention                              │    │
│  │                                                     │    │
│  │  • Grafana (port 3000)                             │    │
│  │    - Visualization dashboards                      │    │
│  │    - Queries VictoriaMetrics                       │    │
│  │                                                     │    │
│  │  • Nginx-RTMP (port 8080 docker, port 1935)       │    │
│  │    - RTMP streaming server (optional)              │    │
│  │    - For live stream analysis                      │    │
│  │                                                     │    │
│  │  • Alertmanager (port 9093)                        │    │
│  │    - Alert routing and notification                │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Why these run on master:**
- **VictoriaMetrics**: Centralized metrics storage, scrapes all worker nodes
- **Grafana**: Single unified dashboard for all nodes
- **Alertmanager**: Central alert routing
- **Nginx-RTMP**: Optional - for centralized RTMP stream ingestion

### Worker Node Components

**Worker nodes** run the actual transcoding workloads and export metrics:

```
┌─────────────────────────────────────────────────────────────┐
│                     WORKER NODE                              │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  Agent (bin/agent)                                 │    │
│  │  • Connects to master:8080                         │    │
│  │  • Polls for jobs                                  │    │
│  │  • Executes FFmpeg workloads                       │    │
│  │  • Runs analyzer scripts                           │    │
│  │  • Sends results to master                         │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  Exporters (Go binaries)                           │    │
│  │                                                     │    │
│  │  • CPU/RAPL Exporter (port 9510)                   │    │
│  │    - Energy consumption metrics                    │    │
│  │    - Requires privileged access to /sys/class/powercap│
│  │                                                     │    │
│  │  • GPU Exporter (port 9511)                        │    │
│  │    - GPU power and utilization (if GPU present)    │    │
│  │                                                     │    │
│  │  • FFmpeg Stats Exporter (port 9506)               │    │
│  │    - Real-time encoding metrics                    │    │
│  │                                                     │    │
│  │  • Node Exporter (port 9100)                       │    │
│  │    - System metrics (CPU, memory, disk, network)   │    │
│  │                                                     │    │
│  │  • cAdvisor (port 8081)                            │    │
│  │    - Container metrics (if using containers)       │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  Workload Execution                                │    │
│  │  • FFmpeg binary                                   │    │
│  │  • Python analyzer scripts                         │    │
│  │  • Test video generation                           │    │
│  └────────────────────────────────────────────────────┘    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Why these run on worker:**
- **Exporters**: Measure resource usage WHERE the work happens
- **FFmpeg**: Actual compute workload
- **Agent**: Executes jobs and reports back to master

## Metrics Flow

```
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│  Worker 1    │         │  Worker 2    │         │  Worker N    │
│              │         │              │         │              │
│  Exporters   │         │  Exporters   │         │  Exporters   │
│  :9510       │         │  :9510       │         │  :9510       │
│  :9511       │         │  :9511       │         │  :9511       │
│  :9100       │         │  :9100       │         │  :9100       │
│  :9506       │         │  :9506       │         │  :9506       │
└──────┬───────┘         └──────┬───────┘         └──────┬───────┘
       │                        │                        │
       │    HTTP /metrics       │                        │
       └────────────────────────┼────────────────────────┘
                                │
                                │ Scrape every 1s
                                │
                         ┌──────▼──────┐
                         │   Master    │
                         │             │
                         │ VictoriaMetrics
                         │   :8428     │
                         │             │
                         │ Stores all  │
                         │ metrics     │
                         └──────┬──────┘
                                │
                                │ Query
                                │
                         ┌──────▼──────┐
                         │   Grafana   │
                         │   :3000     │
                         │             │
                         │ Visualizes  │
                         │ all nodes   │
                         └─────────────┘
```

## Complete Deployment Example

### Master Node Setup

```bash
# 1. Build master binary
cd /opt/ffmpeg-rtmp
make build-master

# 2. Start master orchestrator
export MASTER_API_KEY=$(openssl rand -base64 32)
./bin/master --port 8080 &

# 3. Start monitoring stack (Docker Compose)
make vm-up-build

# Components now running:
# - Master orchestrator: https://localhost:8080
# - VictoriaMetrics: http://localhost:8428
# - Grafana: http://localhost:3000
# - Nginx-RTMP: rtmp://localhost:1935 (optional)
```

### Worker Node Setup

```bash
# 1. Build agent binary
cd /opt/ffmpeg-rtmp
make build-agent

# 2. Start exporters (if not already running)
# CPU exporter starts automatically with agent
# OR start manually:
docker compose up -d cpu-exporter-go node-exporter cadvisor

# 3. Register and start agent
export MASTER_API_KEY="<same-as-master>"
./bin/agent --register --master https://MASTER_IP:8080

# Components now running:
# - Agent: Polling master for jobs
# - CPU Exporter: http://localhost:9510/metrics
# - Node Exporter: http://localhost:9100/metrics
# - cAdvisor: http://localhost:8081/metrics
```

## VictoriaMetrics Scrape Configuration

The master node's VictoriaMetrics is configured to scrape ALL worker nodes:

```yaml
# victoriametrics.yml on master
scrape_configs:
  # Scrape master's own metrics
  - job_name: 'master-metrics'
    static_configs:
      - targets: ['localhost:9090']
  
  # Scrape worker node 1
  - job_name: 'worker-1'
    static_configs:
      - targets: 
          - 'worker1.example.com:9510'  # CPU/RAPL
          - 'worker1.example.com:9100'  # Node exporter
          - 'worker1.example.com:9506'  # FFmpeg stats
  
  # Scrape worker node 2
  - job_name: 'worker-2'
    static_configs:
      - targets: 
          - 'worker2.example.com:9510'
          - 'worker2.example.com:9100'
          - 'worker2.example.com:9506'
  
  # Add more workers as needed
```

## Port Summary

### Master Node Ports
| Port | Service | Protocol | Purpose |
|------|---------|----------|---------|
| 8080 | Master orchestrator | HTTPS | Job API |
| 9090 | Master metrics | HTTP | Prometheus metrics |
| 8428 | VictoriaMetrics | HTTP | Metrics database |
| 3000 | Grafana | HTTP | Dashboards |
| 1935 | Nginx-RTMP | RTMP | Streaming (optional) |
| 9093 | Alertmanager | HTTP | Alerts |

### Worker Node Ports
| Port | Service | Protocol | Purpose |
|------|---------|----------|---------|
| 9510 | CPU/RAPL Exporter | HTTP | Energy metrics |
| 9511 | GPU Exporter | HTTP | GPU metrics (if GPU) |
| 9100 | Node Exporter | HTTP | System metrics |
| 9506 | FFmpeg Stats Exporter | HTTP | Encoding metrics |
| 8081 | cAdvisor | HTTP | Container metrics |

## Why This Architecture?

### Centralized Monitoring (Master)
- **Single source of truth**: All metrics in one database
- **Unified dashboards**: View all workers from one Grafana instance
- **Simplified alerting**: One Alertmanager for all nodes
- **Network efficiency**: Workers only export, don't store metrics

### Distributed Compute (Workers)
- **Scalability**: Add workers to increase capacity
- **Resource isolation**: Each worker's workload is independent
- **Hardware diversity**: Workers can have different CPU/GPU configurations
- **Fault tolerance**: Worker failure doesn't affect master or other workers

### Security Considerations
- **Master API**: HTTPS with API key authentication
- **Metrics**: HTTP (internal network only)
- **Firewall**: Workers need outbound to master:8080, master needs inbound from workers on exporter ports

## Development vs Production

### Development (Single Machine)
```bash
# All components in Docker Compose
docker compose up -d

# Everything runs on localhost
# - No master/agent binaries needed
# - Nginx-RTMP on port 8080 (HTTP)
# - Good for testing, not scalable
```

### Production (Distributed)
```bash
# Master node: bin/master + monitoring stack
# Worker nodes: bin/agent + exporters

# Scalable across multiple machines
# Master orchestrator on port 8080 (HTTPS)
# VictoriaMetrics scrapes all workers
```

## FAQ

**Q: Why not run VictoriaMetrics on each worker?**
- A: Distributed metrics storage is complex. Centralized storage is simpler and sufficient for most use cases.

**Q: Can I run Grafana on a separate node?**
- A: Yes, but you'll need to expose VictoriaMetrics and configure Grafana to point to it.

**Q: Does the master node do any transcoding?**
- A: No, the master only orchestrates. Use `--allow-master-as-worker` flag for development/testing only.

**Q: What if a worker goes down?**
- A: Jobs on that worker fail and can be retried on other workers (with `--max-retries` flag).

**Q: How does VictoriaMetrics find workers?**
- A: You must manually configure worker endpoints in `victoriametrics.yml` scrape config.

## See Also
- [Production Deployment Guide](PRODUCTION_CONFIG.md)
- [Deployment Modes Comparison](DEPLOYMENT_MODES.md)
- [Distributed Architecture v1 (Legacy)](distributed_architecture_v1.md)
