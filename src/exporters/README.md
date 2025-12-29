# Exporters

This directory contains all the Prometheus exporters used to collect metrics from various sources.

## Overview

Each exporter is a standalone service that exposes metrics in Prometheus format. They run as Docker containers and are scraped by Prometheus at regular intervals.

## Available Exporters

### 1. RAPL Exporter (`rapl/`)

**Purpose**: Monitors CPU power consumption using Intel RAPL (Running Average Power Limit)

**Metrics**:
- `rapl_power_watts`: Current power consumption in watts per CPU package/zone

**Requirements**:
- Intel CPU with RAPL support
- Access to `/sys/class/powercap/intel-rapl:*`

**Port**: 9500

[More details](rapl/README.md)

---

### 2. Docker Stats Exporter (`docker_stats/`)

**Purpose**: Tracks Docker engine and container resource usage

**Metrics**:
- `docker_cpu_percentage`: Docker daemon CPU usage
- `container_cpu_percentage`: Per-container CPU usage
- `container_memory_percentage`: Per-container memory usage

**Requirements**:
- Access to Docker socket (`/var/run/docker.sock`)

**Port**: 9501

[More details](docker_stats/README.md)

---

### 3. Results Exporter (`results/`)

**Purpose**: Exposes test results as Prometheus metrics for visualization

**Metrics**:
- `results_scenario_delta_power_watts`: Power difference vs baseline
- `results_scenario_delta_energy_wh`: Energy difference vs baseline
- `results_scenario_power_pct_increase`: Percentage power increase

**Requirements**:
- Test results JSON files in mounted directory

**Port**: 9502

[More details](results/README.md)

---

### 4. QoE Exporter (`qoe/`)

**Purpose**: Calculates and exposes Quality of Experience metrics

**Metrics**:
- `qoe_efficiency_score`: Energy efficiency score per scenario
- `qoe_pixels_per_joule`: Pixel throughput per joule (for multi-resolution)
- `qoe_throughput_per_watt`: Mbps per watt

**Requirements**:
- Test results JSON files
- Advisor module

**Port**: 9503

[More details](qoe/README.md)

---

### 5. Cost Exporter (`cost/`)

**Purpose**: Calculates and exposes cost metrics based on energy and compute usage

**Metrics**:
- `cost_total_usd`: Total cost per scenario
- `cost_energy_usd`: Energy cost component
- `cost_compute_usd`: Compute cost component
- `cost_per_megapixel`: Cost efficiency metric

**Configuration**:
- `ENERGY_COST_PER_KWH`: Cost per kWh (default: $0.12)
- `CPU_COST_PER_HOUR`: Cost per CPU hour (default: $0.50)

**Port**: 9504

[More details](cost/README.md)

---

### 6. Health Checker (`health_checker/`)

**Purpose**: Monitors health of all exporters and exposes their status

**Metrics**:
- `exporter_health_status`: 1 if healthy, 0 if unhealthy
- `exporter_last_scrape_success`: Timestamp of last successful scrape

**Port**: 9600

[More details](health_checker/README.md)

---

## Adding a New Exporter

To add a new exporter:

1. Create a new directory: `src/exporters/my_exporter/`
2. Add your Python script and Dockerfile
3. Update `docker-compose.yml` with the new service
4. Update `prometheus.yml` with the scrape config
5. Document your exporter in this README

## Common Patterns

All exporters follow these patterns:

- **Port**: Each exporter has a unique port (95xx or 96xx range)
- **Health Check**: Expose `/health` endpoint returning "OK"
- **Metrics**: Expose `/metrics` endpoint in Prometheus format
- **Configuration**: Use environment variables for configuration
- **Logging**: Log to stdout for Docker logs visibility

## Troubleshooting

### Exporter Not Starting

```bash
# Check logs
make logs SERVICE=<exporter-name>

# Check if port is already in use
docker ps | grep <port>
```

### Metrics Not Appearing in Prometheus

1. Check Prometheus targets: http://localhost:8428/targets
2. Verify the exporter is healthy: `curl http://localhost:<port>/health`
3. Check the metrics endpoint: `curl http://localhost:<port>/metrics`

### Permission Issues (RAPL, Docker)

Some exporters need elevated permissions:
- RAPL exporter: Runs privileged, mounts `/sys/class/powercap`
- Docker stats: Needs access to Docker socket

## Performance Considerations

- Exporters are designed to be lightweight
- Metrics are cached and refreshed on each scrape
- No historical data stored (Prometheus handles that)
- Failed scrapes don't crash the exporter

## Testing Exporters

You can test an exporter locally:

```bash
cd src/exporters/rapl
docker build -t rapl-exporter .
docker run -p 9500:9500 --privileged -v /sys/class/powercap:/sys/class/powercap:ro rapl-exporter

# In another terminal
curl http://localhost:9500/metrics
```
