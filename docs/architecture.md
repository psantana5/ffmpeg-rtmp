# Architecture Overview

This document describes the architecture and data flow of the FFmpeg RTMP Power Monitoring system.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Host Machine                             │
│                                                                   │
│  ┌──────────────┐                                                │
│  │ Test Runner  │ (Python script on host)                        │
│  │ run_tests.py │                                                │
│  └──────┬───────┘                                                │
│         │ spawns FFmpeg processes                                │
│         ▼                                                         │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Docker Compose Stack                         │   │
│  │                                                            │   │
│  │  ┌─────────────┐      ┌──────────────┐                   │   │
│  │  │ Nginx RTMP  │◄─────┤ FFmpeg       │                   │   │
│  │  │ (streaming) │ RTMP │ (host proc.) │                   │   │
│  │  └─────┬───────┘      └──────────────┘                   │   │
│  │        │                                                   │   │
│  │  ┌─────▼──────────────────────────────────────────────┐  │   │
│  │  │             Metrics Exporters                       │  │   │
│  │  │                                                      │  │   │
│  │  │  ┌──────────────┐  ┌──────────────┐                │  │   │
│  │  │  │ RAPL         │  │ Docker Stats │                │  │   │
│  │  │  │ (CPU power)  │  │ (containers) │                │  │   │
│  │  │  └──────────────┘  └──────────────┘                │  │   │
│  │  │                                                      │  │   │
│  │  │  ┌──────────────┐  ┌──────────────┐                │  │   │
│  │  │  │ Results      │  │ QoE          │                │  │   │
│  │  │  │ (test data)  │  │ (efficiency) │                │  │   │
│  │  │  └──────────────┘  └──────────────┘                │  │   │
│  │  │                                                      │  │   │
│  │  │  ┌──────────────┐  ┌──────────────┐                │  │   │
│  │  │  │ Cost         │  │ Health       │                │  │   │
│  │  │  │ (economics)  │  │ (monitoring) │                │  │   │
│  │  │  └──────────────┘  └──────────────┘                │  │   │
│  │  │                                                      │  │   │
│  │  │  ┌──────────────┐  ┌──────────────┐                │  │   │
│  │  │  │ node_exporter│  │ cAdvisor     │                │  │   │
│  │  │  │ (host metrics│  │ (containers) │                │  │   │
│  │  │  └──────────────┘  └──────────────┘                │  │   │
│  │  └─────┬───────────────────────────────────────────────┘  │   │
│  │        │ scrape (HTTP)                                    │   │
│  │        ▼                                                   │   │
│  │  ┌─────────────┐                                          │   │
│  │  │ Prometheus  │──────┐                                   │   │
│  │  │ (TSDB)      │      │ alerts                            │   │
│  │  └─────┬───────┘      ▼                                   │   │
│  │        │         ┌──────────────┐                         │   │
│  │        │         │ Alertmanager │                         │   │
│  │        │         └──────────────┘                         │   │
│  │        │ query                                             │   │
│  │        ▼                                                   │   │
│  │  ┌─────────────┐                                          │   │
│  │  │  Grafana    │ (visualization)                          │   │
│  │  └─────────────┘                                          │   │
│  └────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Component Details

### Core Services

#### Nginx RTMP
- **Purpose**: RTMP streaming server
- **Image**: `tiangolo/nginx-rtmp:latest`
- **Ports**: 1935 (RTMP), 8080 (HTTP/stats)
- **Function**: Receives RTMP streams from FFmpeg, serves HLS
- **Health Check**: Monitors HLS playlist availability

#### Prometheus
- **Purpose**: Time-series metrics database
- **Image**: `prom/prometheus:latest`
- **Port**: 9090
- **Configuration**: `prometheus.yml`, `prometheus-alerts.yml`
- **Retention**: 7 days
- **Function**: Scrapes all exporters, evaluates alert rules

#### Grafana
- **Purpose**: Visualization and dashboards
- **Image**: `grafana/grafana:latest`
- **Port**: 3000
- **Credentials**: admin/admin (change on first login)
- **Provisioning**: Auto-loads datasources and dashboards

#### Alertmanager
- **Purpose**: Alert routing and notifications
- **Image**: `prom/alertmanager:latest`
- **Port**: 9093
- **Configuration**: `alertmanager/alertmanager.yml`

### Power and System Monitoring

#### RAPL Exporter (Port 9500)
- **Purpose**: CPU package power monitoring
- **Technology**: Intel RAPL (Running Average Power Limit)
- **Access**: Privileged, mounts `/sys/class/powercap`
- **Metrics**: `rapl_power_watts{zone="..."}`
- **Update Frequency**: Every scrape (default: 5s)

#### Docker Stats Exporter (Port 9501)
- **Purpose**: Docker daemon and container resource usage
- **Access**: Docker socket (`/var/run/docker.sock`)
- **Metrics**: 
  - `docker_cpu_percentage`
  - `container_cpu_percentage{name="..."}`
  - `container_memory_percentage{name="..."}`

#### node_exporter (Port 9100)
- **Purpose**: Host-level metrics
- **Image**: `prom/node-exporter:latest`
- **Metrics**: CPU, memory, disk, network, filesystem

#### cAdvisor (Port 8081)
- **Purpose**: Container metrics
- **Image**: `gcr.io/cadvisor/cadvisor:latest`
- **Metrics**: Container CPU, memory, network, disk I/O

#### DCGM Exporter (Port 9400) [Optional]
- **Purpose**: NVIDIA GPU monitoring
- **Profile**: `nvidia`
- **Requirements**: nvidia-container-toolkit
- **Metrics**: GPU power, utilization, temperature, memory

### Application-Specific Exporters

#### Results Exporter (Port 9502)
- **Purpose**: Exposes test results as metrics
- **Input**: Test results JSON files from `test_results/`
- **Metrics**: Baseline comparisons, scenario summaries
- **Integration**: Enables Grafana baseline-vs-test dashboards

#### QoE Exporter (Port 9503)
- **Purpose**: Quality of Experience and efficiency metrics
- **Dependencies**: Advisor module
- **Metrics**: 
  - `qoe_efficiency_score`
  - `qoe_throughput_per_watt`
  - `qoe_pixels_per_joule`

#### Cost Exporter (Port 9504)
- **Purpose**: Economic analysis
- **Configuration**: Energy and CPU pricing
- **Metrics**: 
  - `cost_total_usd`
  - `cost_energy_usd`
  - `cost_compute_usd`
  - `cost_per_megapixel`

#### Health Checker (Port 9600)
- **Purpose**: Monitors all exporter health
- **Metrics**: `exporter_health_status{service="..."}`
- **Function**: Continuous health verification

## Data Flow

### 1. Test Execution Flow

```
1. User runs: python3 scripts/run_tests.py single --bitrate 2000k
2. Test runner spawns FFmpeg process(es)
3. FFmpeg streams to nginx-rtmp (localhost:1935)
4. Nginx-rtmp serves the stream
5. Test runs for specified duration
6. Test metadata saved to test_results/test_results_*.json
```

### 2. Metrics Collection Flow

```
1. Exporters read their data sources:
   - RAPL: /sys/class/powercap/intel-rapl:*/energy_uj
   - Docker Stats: /var/run/docker.sock
   - Results: test_results/*.json files
   - etc.

2. Exporters expose metrics on /metrics endpoint

3. Prometheus scrapes all exporters every 5s (configurable)

4. Metrics stored in Prometheus TSDB

5. Prometheus evaluates alert rules

6. Alerts sent to Alertmanager
```

### 3. Visualization Flow

```
1. User opens Grafana (localhost:3000)
2. Grafana queries Prometheus for metrics
3. Prometheus returns time-series data
4. Grafana renders dashboards
5. User interacts with dashboards (zoom, filter, select scenarios)
```

### 4. Analysis Flow

```
1. Tests complete, results in test_results/*.json
2. User runs: python3 scripts/analyze_results.py
3. Analyzer:
   - Reads test results JSON
   - Queries Prometheus for power/CPU metrics
   - Calculates efficiency scores
   - Ranks scenarios
   - Trains/uses ML models for predictions
4. Console report and CSV export generated
```

## Networking

All containers are in the `streaming-net` bridge network:

```
streaming-net (bridge)
├── nginx-rtmp
├── prometheus
├── grafana
├── alertmanager
├── rapl-exporter
├── docker-stats-exporter
├── results-exporter
├── qoe-exporter
├── cost-exporter
├── exporter-health-checker
├── node-exporter
├── cadvisor
└── dcgm-exporter (if nvidia profile)
```

Containers communicate by service name (DNS resolution).

## Storage

### Volumes

- **prometheus-data**: Persistent Prometheus TSDB
- **grafana-data**: Grafana configuration and dashboards

### Bind Mounts

- `./test_results` → Results exporter, QoE, Cost exporters
- `./nginx.conf` → Nginx configuration
- `./prometheus.yml` → Prometheus config
- `./grafana/provisioning` → Grafana datasources/dashboards
- `/sys/class/powercap` → RAPL exporter (read-only)
- `/var/run/docker.sock` → Docker stats exporter (read-only)

## Security Considerations

### Privileged Containers

- **rapl-exporter**: Needs privileged mode to read RAPL counters
- **cadvisor**: Needs privileged mode for full container metrics

### Docker Socket Access

- **docker-stats-exporter**: Read-only access to Docker socket
- Risk: Container could inspect/control other containers
- Mitigation: Container runs as non-root, socket mounted read-only

### Network Exposure

All services exposed on localhost:
- Nginx RTMP: 1935, 8080
- Prometheus: 9090
- Grafana: 3000
- Alertmanager: 9093
- Exporters: 9500-9504, 9600
- node_exporter: 9100
- cAdvisor: 8081

For production, use reverse proxy with authentication.

## Scalability

### Horizontal Scaling

Not currently supported. Each component runs as a single container.

For production scale-out:
- Use Prometheus federation or Thanos
- Load-balance Grafana instances
- Use remote storage for Prometheus

### Vertical Scaling

Adjust resource limits in `docker-compose.yml`:

```yaml
services:
  prometheus:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
```

## Failure Modes

### Exporter Down

- Prometheus target shows DOWN
- Gaps in metrics timeline
- Grafana shows "No data"
- Health checker alerts

**Recovery**: Automatic restart (`restart: unless-stopped`)

### Prometheus Down

- No new metrics collected
- Grafana can't query data
- Alerts not evaluated

**Recovery**: Automatic restart, data recovers from disk

### Nginx RTMP Down

- FFmpeg can't connect
- Tests fail to start
- Streaming stops

**Recovery**: Automatic restart, tests must be re-run

## Performance Characteristics

### Resource Usage (Typical)

- **Prometheus**: 200-500 MB RAM, 1-2 GB disk (7 days)
- **Grafana**: 100-200 MB RAM
- **Exporters**: 10-50 MB RAM each
- **Total**: ~1.5 GB RAM, 2-3 GB disk

### Metrics Volume

- ~50 exporters × ~10 metrics each = 500 time series
- 5-second scrape interval
- 7-day retention
- Disk: ~50 MB/day (~350 MB for 7 days)

### Query Performance

- Typical dashboard query: < 100ms
- Complex aggregations: < 1s
- Analysis queries (long time ranges): 1-10s

## Extension Points

### Adding a New Exporter

1. Create in `src/exporters/new_exporter/`
2. Add to `docker-compose.yml`
3. Add scrape config to `prometheus.yml`
4. Create Grafana dashboard

### Adding a New Test Type

1. Extend `scripts/run_tests.py` with new subcommand
2. Implement test logic
3. Update test results schema
4. Update analyzer to handle new type

### Adding a New ML Model

1. Implement in `advisor/modeling.py`
2. Add training logic to `scripts/retrain_models.py`
3. Add predictions to `scripts/analyze_results.py`

## Related Documentation

- [Getting Started Guide](getting-started.md)
- [Exporter Details](../src/exporters/README.md)
- [Test Runner Guide](../scripts/README.md)
- [Energy Advisor](../advisor/README.md)
