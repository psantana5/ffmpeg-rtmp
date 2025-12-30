# Internal Architecture Reference

## Table of Contents
- [Overview](#overview)
- [Deployment Guide](#deployment-guide)
- [Component Architecture](#component-architecture)
- [Networking](#networking)
- [Job Queueing System](#job-queueing-system)
- [Node Health Tracking](#node-health-tracking)
- [Data Storage and Retrieval](#data-storage-and-retrieval)
- [Runtime Model](#runtime-model)

---

## Overview

The FFmpeg RTMP Power Monitoring system is a distributed architecture for running, monitoring, and analyzing video transcoding workloads. The system supports two operational modes:

1. **Standalone Mode**: All components run on a single machine using Docker Compose
2. **Distributed Mode**: A master node coordinates workloads across multiple compute nodes

This document describes the complete runtime model, data flows, and operational characteristics of the system.

---

## Deployment Guide

### Prerequisites

**All Deployments:**
- Docker 20.10+ and Docker Compose 2.0+
- Linux host with kernel 4.15+ (for RAPL power monitoring)
- Python 3.10+ (for test scripts)
- FFmpeg with appropriate codec support

**Distributed Mode Additional Requirements:**
- Go 1.21+ (for building master/agent binaries)
- Network connectivity between master and compute nodes
- Shared or synchronized time across nodes (NTP recommended)

### Deploying Standalone Mode

Standalone mode runs all services on a single machine for development or small-scale testing.

**Step 1: Clone and Initialize**
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
mkdir -p test_results
```

**Step 2: Build and Start Services**
```bash
# Start all services
make up-build

# Or with GPU support (requires NVIDIA Docker runtime)
make nvidia-up-build
```

**Step 3: Verify Deployment**
```bash
# Check all containers are running
docker compose ps

# Verify services are healthy
curl http://localhost:8428/health  # VictoriaMetrics
curl http://localhost:3000         # Grafana
curl http://localhost:9093/-/healthy  # Alertmanager
```

**Step 4: Access Dashboards**
- Grafana: http://localhost:3000 (admin/admin)
- VictoriaMetrics: http://localhost:8428
- Alertmanager: http://localhost:9093

**Step 5: Run Test Workload**
```bash
python3 scripts/run_tests.py single \
  --name "deployment_test" \
  --bitrate 2000k \
  --duration 60
```

### Deploying Distributed Mode

Distributed mode separates the master coordination node from compute worker nodes.

**Master Node Deployment**

**Step 1: Build Master Binary**
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-master
```

**Step 2: Start Master Service**
```bash
# Basic startup
./bin/master --port 8080

# Production startup with logging
./bin/master --port 8080 > master.log 2>&1 &

# Verify master is running
curl http://localhost:8080/health
```

**Step 3: Start Monitoring Stack**
```bash
# Start VictoriaMetrics and Grafana on master
make vm-up-build
```

**Compute Node Deployment**

**Step 1: Build Agent Binary**
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-agent
```

**Step 2: Register and Start Agent**
```bash
# Replace MASTER_IP with your master node's IP address
./bin/agent --register --master http://MASTER_IP:8080

# Agent will:
# 1. Detect hardware capabilities
# 2. Register with master
# 3. Begin polling for jobs
# 4. Send periodic heartbeats
```

**Step 3: Verify Registration**
```bash
# On master node, check registered nodes
curl http://MASTER_IP:8080/nodes | jq
```

### Environment-Specific Configuration

**Network Configuration**

For production deployments, configure firewall rules:

```bash
# Master node - allow inbound
ufw allow 8080/tcp   # Master API
ufw allow 3000/tcp   # Grafana
ufw allow 8428/tcp   # VictoriaMetrics

# Compute nodes - no inbound ports required (outbound only)
```

**Resource Limits**

Adjust Docker resource limits in `docker-compose.yml`:

```yaml
services:
  victoriametrics:
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          memory: 4G
```

**Storage Configuration**

Set custom data retention:

```yaml
victoriametrics:
  command:
    - "--retentionPeriod=90d"  # Default is 30d
```

**TLS/Security Configuration**

For production, enable HTTPS and authentication:

```bash
# Generate certificates (not implemented in v1.0)
# Configure nginx reverse proxy
# Enable Grafana authentication
# Set strong admin passwords
```

---

## Component Architecture

### Master Node vs Compute Node Components

**Master Node Components**:
- Master Service (Go HTTP API) - Port 8080
- VictoriaMetrics (TSDB) - Port 8428
- Grafana (Dashboards) - Port 3000
- In-memory data structures (nodes, jobs, results)

**Compute Node Components**:
- Agent Service (Go binary)
- FFmpeg processes (when executing jobs)
- Local exporters (temporary, per job)
- Results aggregation and upload

**Standalone Mode Components** (all on one machine):
- All master components
- All exporters (12+ services)
- Nginx RTMP server
- Alertmanager

See the [distributed architecture documentation](distributed_architecture_v1.md) for detailed component descriptions.

---

## Networking

### Communication Patterns

#### Master-Agent Communication (Distributed Mode)

All communication is agent-initiated (pull model):

1. **Node Registration**: Agent → Master (POST /nodes/register)
2. **Heartbeats**: Agent → Master every 30s (POST /nodes/{id}/heartbeat)
3. **Job Polling**: Agent → Master every 10s (GET /jobs/next?node_id=X)
4. **Result Submission**: Agent → Master (POST /results)

**Protocol**: JSON over HTTP (HTTPS recommended for production)

**Network Requirements**:
- Master node: Publicly accessible on port 8080
- Compute nodes: Outbound connectivity only (no inbound ports)
- Latency: <100ms recommended
- Bandwidth: ~1-10 KB/s per node

#### Metrics Collection (Standalone Mode)

VictoriaMetrics scrapes exporters every 5 seconds:

```
VictoriaMetrics ──[HTTP GET /metrics]──> Exporters
     (Port 8428)                         (Ports 9500+)
```

**Network**: Docker bridge network `streaming-net`

**Service Discovery**: Docker DNS

---

## Job Queueing System

### Queue Implementation

**Data Structure**: Simple FIFO (First-In-First-Out) slice in memory

**Location**: Master node RAM

**Persistence**: None (jobs lost on restart in v1.0)

### Job Lifecycle

```
Created → Queued → Assigned → Running → Completed
```

1. **Created**: User submits via POST /jobs
2. **Queued**: Job added to FIFO queue on master
3. **Assigned**: Agent polls and receives job
4. **Running**: Agent executes FFmpeg workload
5. **Completed**: Agent submits results to master

### Job Model

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "scenario": "1080p-h264-nvenc",
  "confidence": "auto",
  "parameters": {
    "duration": 300,
    "bitrate": "5000k",
    "resolution": "1920x1080",
    "encoder": "h264_nvenc",
    "preset": "medium"
  },
  "status": "queued",
  "created_at": "2025-12-30T10:00:00Z"
}
```

### Scheduling

**Current (v1.0)**: First-available agent gets next job in queue

**No resource matching**: Any agent can get any job

**Future**: Resource-aware scheduling, priority queues, retry logic

---

## Node Health Tracking

### Heartbeat System

**Agent sends heartbeat every 30 seconds** via POST /nodes/{id}/heartbeat

**Master updates node.LastSeen timestamp**

**Node status**:
- **available**: Last heartbeat within 60 seconds
- **offline**: No heartbeat for >60 seconds

### Exporter Health (Standalone Mode)

**exporter-health-checker** service polls all exporters every 30 seconds

**Exposes metric**: `exporter_health_status{service="X"}` = 1 (up) or 0 (down)

**Monitored by**: VictoriaMetrics, Grafana dashboards, Alertmanager

### Docker Health Checks

All containers have health check directives:

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost/health"]
  interval: 10s
  timeout: 5s
  retries: 3
```

**Query health**: `docker compose ps`

---

## Data Storage and Retrieval

### Storage Locations

#### 1. Time-Series Metrics (VictoriaMetrics)

**Location**: Docker volume `victoriametrics-data`

**Retention**: 30 days (configurable)

**Size**: ~50-100 MB per day

**Query API**:
```bash
curl 'http://localhost:8428/api/v1/query?query=rapl_power_watts'
```

#### 2. Test Results (JSON Files)

**Location**: `./test_results/test_results_YYYYMMDD_HHMMSS.json`

**Format**: JSON with test metadata and scenario configurations

**Used by**: results-exporter, analyze_results.py

#### 3. ML Models

**Location**: `./models/<hardware_id>/`

**Files**:
- `power_predictor_latest.pkl`
- `multivariate_predictor_latest.pkl`
- `metadata.json`

#### 4. In-Memory Data (Master Node)

**Contents**:
- Node registry: `map[string]*Node`
- Job queue: `[]Job`
- Results: `map[string]*Result`

**Persistence**: None (lost on restart in v1.0)

#### 5. Configuration Files

- `docker-compose.yml` - Service definitions
- `victoriametrics.yml` - Metrics scraping
- `nginx.conf` - RTMP server
- `pricing_config.json` - Cost analysis
- `grafana/provisioning/` - Dashboards

### Data Retrieval

**Metrics**: Query VictoriaMetrics HTTP API with PromQL

**Test Results**: Read JSON files or query results-exporter metrics

**Job Status**: Query master REST API (GET /jobs, GET /nodes)

**Grafana**: Browse dashboards at http://localhost:3000

---

## Runtime Model

### System Startup

#### Standalone Mode (Docker Compose)

```
1. make up-build
2. Docker builds images
3. Creates network: streaming-net
4. Creates volumes: victoriametrics-data, grafana-data
5. Starts containers (30-60 seconds)
6. Health checks begin
7. VictoriaMetrics starts scraping
8. Grafana provisions dashboards
9. System ready
```

#### Distributed Mode

**Master**:
```
1. make build-master
2. ./bin/master --port 8080
3. Master initializes (<5 seconds)
4. make vm-up-build (monitoring stack)
5. System ready
```

**Agent**:
```
1. make build-agent
2. ./bin/agent --register --master http://MASTER_IP:8080
3. Agent detects hardware
4. Agent registers with master
5. Agent starts heartbeat and polling loops
6. Agent ready (<5 seconds)
```

### Test Execution Flow (Standalone)

```
1. User: python3 scripts/run_tests.py single --bitrate 2000k --duration 60
2. Test runner generates run_id
3. Stabilization period (10s)
4. FFmpeg starts streaming to nginx-rtmp
5. Exporters collect metrics
6. VictoriaMetrics scrapes metrics every 5s
7. Test runs for specified duration
8. FFmpeg terminates
9. Cooldown period (10s)
10. Test runner saves metadata to test_results/*.json
11. Results exporter exposes metrics
```

**Total time**: ~80 seconds for 60s test

### Distributed Job Execution Flow

```
1. User submits job: POST /jobs to master
2. Job queued on master
3. Agent polls: GET /jobs/next?node_id=X
4. Master returns job to agent
5. Agent starts local exporters
6. Agent executes FFmpeg workload
7. Agent collects metrics locally
8. Job completes
9. Agent runs analyzer
10. Agent sends results: POST /results to master
11. Master stores results
12. Agent resumes polling
```

**Total time**: ~5-10 minutes for 300s workload

### Resource Usage

**Standalone Mode (idle)**:
- Memory: ~1-2 GB
- CPU: ~5-10%
- Disk: ~3-6 GB + 50-110 MB/day

**Distributed Master**:
- Memory: ~500-1000 MB
- CPU: <1%
- Disk: ~3-6 GB + 50-110 MB/day

**Distributed Agent (idle)**:
- Memory: ~10-30 MB
- CPU: <1%
- Disk: ~10-30 MB

**During Active Test**:
- FFmpeg: 50-400% CPU, 100-500 MB RAM
- Exporters: 2-5% CPU, 120-600 MB RAM

---

## Appendix: Key Files Reference

**Deployment**:
- `docker-compose.yml` - Service definitions
- `Makefile` - Common commands
- `victoriametrics.yml` - Metrics scraping config

**Master/Agent**:
- `cmd/master/main.go` - Master service entrypoint
- `cmd/agent/main.go` - Agent service entrypoint
- `pkg/models/` - Data models (Job, Node, Result)
- `pkg/api/master.go` - Master REST API handlers
- `pkg/agent/client.go` - Agent HTTP client
- `pkg/store/memory.go` - In-memory storage implementation

**Test Execution**:
- `scripts/run_tests.py` - Main test runner
- `scripts/analyze_results.py` - Results analyzer
- `scripts/recommend_test.py` - Hardware-aware recommendations

**Configuration**:
- `nginx.conf` - Nginx RTMP server config
- `alertmanager/alertmanager.yml` - Alert routing
- `pricing_config.json` - Cost analysis pricing
- `grafana/provisioning/` - Grafana datasources and dashboards

**Data Storage**:
- `test_results/` - Test metadata JSON files
- `models/` - Trained ML models
- Docker volumes: `victoriametrics-data`, `grafana-data`

---

## Glossary

- **RAPL**: Running Average Power Limit - Intel CPU power monitoring interface
- **TSDB**: Time-Series Database - Optimized for timestamped metrics
- **FIFO**: First-In-First-Out - Queue ordering strategy
- **Exporter**: Service that exposes metrics in Prometheus format
- **Scraping**: Periodic pulling of metrics from exporters
- **PromQL**: Prometheus Query Language - Used to query metrics
- **mTLS**: Mutual TLS - Both client and server authenticate each other
- **NVENC**: NVIDIA hardware video encoder
- **DCGM**: Data Center GPU Manager - NVIDIA GPU monitoring toolkit
- **cAdvisor**: Container Advisor - Google's container metrics tool

---

## Document Version

**Version**: 1.0  
**Last Updated**: 2025-12-30  
**System Version**: v2.1+  
**Maintainer**: See CONTRIBUTING.md
