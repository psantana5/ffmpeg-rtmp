# Exporter Health Check Script

This script periodically checks all Prometheus exporters to ensure they are:
1. Responding to requests
2. Returning metrics
3. Returning relevant/fresh data

## Features

- **Single Check**: Run once and exit
- **Continuous Monitoring**: Run at specified intervals
- **Prometheus Exporter Mode**: Expose health check results as Prometheus metrics
- **Detailed Logging**: See exactly what's happening with each exporter
- **Smart Validation**: 
  - Verifies expected metrics are present
  - Checks that metrics contain actual data (not just empty responses)
  - Validates metric freshness based on exporter type

## Usage

### Single Check (Exit after checking)
```bash
python3 check_exporters_health.py
```

### Continuous Monitoring (Every 60 seconds)
```bash
python3 check_exporters_health.py --interval 60
```

### Prometheus Exporter Mode (Expose metrics on port 9600)
```bash
python3 check_exporters_health.py --port 9600
```

Then add to Prometheus scrape config:
```yaml
scrape_configs:
  - job_name: 'exporter-health'
    static_configs:
      - targets: ['exporter-health-checker:9600']
```

### Debug Mode
```bash
python3 check_exporters_health.py --debug
```

## Running Inside Docker

You can run this script inside an existing container (e.g., Prometheus):

```bash
# Copy script to Prometheus container
docker cp check_exporters_health.py prometheus:/tmp/

# Execute inside container
docker exec prometheus python3 /tmp/check_exporters_health.py
```

Or add as a separate service in `docker-compose.yml`:

```yaml
  exporter-health-checker:
    build:
      context: .
      dockerfile: Dockerfile.health-checker
    container_name: exporter-health-checker
    ports:
      - "9600:9600"
    restart: unless-stopped
    networks:
      - streaming-net
    command: ["--port", "9600"]
```

## Output

### Console Output

```
================================================================================
EXPORTER HEALTH SUMMARY
================================================================================
Exporter                       Status     Metrics    Samples    Notes
--------------------------------------------------------------------------------
nginx-rtmp-exporter           ✓ OK       5          12         
rapl-exporter                 ✓ OK       2          8          
docker-stats-exporter         ✓ OK       4          15         
node-exporter                 ✓ OK       850        2143       
cadvisor                      ✓ OK       120        456        
results-exporter              ⚠ FAIL     0          0          No metrics found
qoe-exporter                  ✓ OK       3          9          
cost-exporter                 ✓ OK       4          12         
--------------------------------------------------------------------------------
Total: 7/8 healthy
================================================================================
```

### Prometheus Metrics

When running in exporter mode, the following metrics are exposed:

```
# Health status (1=healthy, 0=unhealthy)
exporter_health_status{exporter="nginx-rtmp-exporter"} 1

# Reachability (1=reachable, 0=unreachable)
exporter_reachable{exporter="nginx-rtmp-exporter"} 1

# Number of unique metrics
exporter_metric_count{exporter="nginx-rtmp-exporter"} 5

# Number of samples
exporter_sample_count{exporter="nginx-rtmp-exporter"} 12

# Has data (1=yes, 0=no)
exporter_has_data{exporter="nginx-rtmp-exporter"} 1
```

## Monitored Exporters

The script checks the following exporters:

| Exporter | Port | Expected Metrics |
|----------|------|------------------|
| nginx-rtmp-exporter | 9728 | nginx_rtmp_connections, nginx_rtmp_streams |
| rapl-exporter | 9500 | rapl_power_watts, rapl_energy_joules_total |
| docker-stats-exporter | 9501 | docker_engine_cpu_percent, docker_container_cpu_percent |
| node-exporter | 9100 | node_cpu_seconds_total, node_memory_MemAvailable_bytes |
| cadvisor | 8080 | container_cpu_usage_seconds_total, container_memory_usage_bytes |
| dcgm-exporter | 9400 | DCGM_FI_DEV_GPU_UTIL, DCGM_FI_DEV_POWER_USAGE |
| results-exporter | 9502 | scenario_, scenario_duration_seconds |
| qoe-exporter | 9503 | qoe_, quality_ |
| cost-exporter | 9504 | cost_exporter_alive, cost_total_load_aware, cost_energy_load_aware |

## Customization

To add or modify exporters, edit the `EXPORTERS` list in `check_exporters_health.py`:

```python
EXPORTERS = [
    ExporterConfig(
        name="my-custom-exporter",
        url="http://my-exporter:9999/metrics",
        job_name="my-exporter",
        expected_metrics=["my_metric_1", "my_metric_2"],
        data_freshness_seconds=30
    ),
    # ... other exporters
]
```

## Integration with Alerting

You can set up Prometheus alerts based on the health check metrics:

```yaml
groups:
  - name: exporter_health
    rules:
      - alert: ExporterUnhealthy
        expr: exporter_health_status == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Exporter {{ $labels.exporter }} is unhealthy"
          description: "The exporter {{ $labels.exporter }} has been unhealthy for more than 5 minutes."
      
      - alert: ExporterNoData
        expr: exporter_has_data == 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Exporter {{ $labels.exporter }} has no data"
          description: "The exporter {{ $labels.exporter }} is not returning any data."
```

## Troubleshooting

### Exporter Not Reachable

If an exporter shows as "Not reachable":
1. Check if the container is running: `docker ps`
2. Check container logs: `docker logs <container-name>`
3. Verify network connectivity: `docker exec prometheus curl http://exporter:port/metrics`

### No Metrics Found

If an exporter shows "No metrics found":
1. The exporter may still be initializing
2. The exporter may need configuration or data to generate metrics
3. Check exporter logs for errors

### Missing Expected Metrics

If expected metrics are missing:
1. The exporter configuration may have changed
2. The feature producing those metrics may be disabled
3. Update the expected metrics list in the script
