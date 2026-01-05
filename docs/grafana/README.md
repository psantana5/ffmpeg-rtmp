# Grafana Dashboards for FFmpeg-RTMP

This directory contains pre-built Grafana dashboards for monitoring the FFmpeg-RTMP distributed transcoding system.

## Available Dashboards

### 1. Complete Monitoring Dashboard (`ffmpeg-rtmp-complete-dashboard.json`)

A comprehensive dashboard showing all key metrics in a single view.

**Panels:**
- **SLA Compliance Rate** - Cluster-wide SLA compliance gauge (target: 95%)
- **Job Success Rate** - Percentage of successful vs failed jobs
- **Total Bandwidth** - Combined input/output bandwidth (MB/s)
- **Active Jobs** - Current number of jobs being processed
- **SLA Compliance Trend** - Historical SLA compliance per worker
- **Job Completion Rates** - Jobs/sec completed, failed, canceled
- **Bandwidth Usage** - Input/output bandwidth per worker
- **CPU Usage** - CPU utilization per worker
- **Memory Usage** - Memory consumption per worker
- **Worker Bandwidth Utilization** - Bandwidth utilization percentage
- **Cancellation Stats** - Graceful vs forceful job terminations
- **SLA Violations** - SLA violations per worker (24h)

## Installation

### Option 1: Import via Grafana UI

1. Open Grafana web interface
2. Navigate to **Dashboards** → **Import**
3. Click **Upload JSON file**
4. Select `ffmpeg-rtmp-complete-dashboard.json`
5. Choose your Prometheus data source
6. Click **Import**

### Option 2: Provisioning (Recommended for Production)

Create a provisioning file for automatic dashboard loading:

```yaml
# /etc/grafana/provisioning/dashboards/ffmpeg-rtmp.yaml
apiVersion: 1

providers:
  - name: 'FFmpeg-RTMP'
    orgId: 1
    folder: 'FFmpeg RTMP'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards/ffmpeg-rtmp
```

Then copy the dashboard JSON:

```bash
sudo mkdir -p /var/lib/grafana/dashboards/ffmpeg-rtmp
sudo cp docs/grafana/*.json /var/lib/grafana/dashboards/ffmpeg-rtmp/
sudo chown -R grafana:grafana /var/lib/grafana/dashboards/ffmpeg-rtmp
sudo systemctl restart grafana-server
```

## Prometheus Data Source Configuration

Make sure your Prometheus data source is configured to scrape worker metrics:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'ffmpeg-rtmp-workers'
    static_configs:
      - targets:
        - 'worker1:9091'
        - 'worker2:9091'
        - 'worker3:9091'
    scrape_interval: 10s
```

## Dashboard Customization

### Adjusting Time Ranges

The dashboard defaults to 6-hour time range. You can change this by:
1. Click the time picker in the top right
2. Select a different range (e.g., "Last 24 hours")
3. Or use custom ranges

### Setting Thresholds

SLA compliance thresholds are set to:
- **Red**: < 90%
- **Yellow**: 90-95%
- **Green**: ≥ 95%

To modify these:
1. Edit the panel
2. Go to **Field** tab
3. Modify **Thresholds** section

### Adding Variables

You can add template variables for filtering:

```json
"templating": {
  "list": [
    {
      "name": "node",
      "type": "query",
      "query": "label_values(ffrtmp_worker_cpu_usage, node_id)",
      "multi": true,
      "includeAll": true
    }
  ]
}
```

Then use `$node` in panel queries:
```
ffrtmp_worker_cpu_usage{node_id=~"$node"}
```

## Available Metrics

The dashboard visualizes these Prometheus metrics:

### Job Metrics
- `ffrtmp_worker_jobs_completed_total` - Successful jobs
- `ffrtmp_worker_jobs_failed_total` - Failed jobs
- `ffrtmp_worker_jobs_canceled_total` - Canceled jobs
- `ffrtmp_worker_active_jobs` - Currently running jobs

### SLA Metrics
- `ffrtmp_worker_sla_compliance_rate` - SLA compliance percentage (0-100)
- `ffrtmp_worker_jobs_sla_compliant_total` - Jobs meeting SLA
- `ffrtmp_worker_jobs_sla_violation_total` - Jobs violating SLA

### Bandwidth Metrics
- `ffrtmp_job_input_bytes_total` - Total input bytes processed
- `ffrtmp_job_output_bytes_total` - Total output bytes generated
- `ffrtmp_worker_bandwidth_utilization` - Bandwidth utilization %

### Cancellation Metrics
- `ffrtmp_worker_jobs_canceled_graceful_total` - SIGTERM terminations
- `ffrtmp_worker_jobs_canceled_forceful_total` - SIGKILL terminations

### Resource Metrics
- `ffrtmp_worker_cpu_usage` - CPU utilization (0-100%)
- `ffrtmp_worker_memory_bytes` - Memory usage in bytes
- `ffrtmp_worker_gpu_usage` - GPU utilization (if available)

### Hardware Metrics
- `ffrtmp_worker_power_watts` - GPU power consumption (NVIDIA only)
- `ffrtmp_worker_temperature_celsius` - GPU temperature (NVIDIA only)

## Alert Integration

The dashboard works with Prometheus alerting rules defined in `docs/prometheus/ffmpeg-rtmp-alerts.yml`.

Alerts will appear in the dashboard automatically when:
- Configured in Prometheus
- Alert state changes (firing/resolved)

## Troubleshooting

### No Data Showing

**Check Prometheus is scraping:**
```bash
curl http://localhost:9090/api/v1/targets
```

**Verify worker metrics endpoint:**
```bash
curl http://worker-ip:9091/metrics | grep ffrtmp
```

### Metrics Not Updating

1. Check Prometheus scrape interval (default: 10s)
2. Verify workers are running: `ps aux | grep agent`
3. Check network connectivity between Prometheus and workers

### Dashboard Import Fails

Common issues:
- **Data source not found**: Select correct Prometheus data source during import
- **Invalid JSON**: Validate JSON syntax at jsonlint.com
- **Version mismatch**: Dashboard requires Grafana 8.0+

## Example Queries

Useful PromQL queries for custom panels:

**Average job duration:**
```promql
avg(rate(job_duration_seconds_sum[5m]) / rate(job_duration_seconds_count[5m]))
```

**Top 5 workers by CPU:**
```promql
topk(5, ffrtmp_worker_cpu_usage)
```

**Compression ratio:**
```promql
(sum(ffrtmp_job_input_bytes_total) - sum(ffrtmp_job_output_bytes_total)) / sum(ffrtmp_job_input_bytes_total) * 100
```

**Bandwidth per active job:**
```promql
(rate(ffrtmp_job_input_bytes_total[5m]) + rate(ffrtmp_job_output_bytes_total[5m])) / ffrtmp_worker_active_jobs
```

## Related Documentation

- [Bandwidth Metrics Guide](../BANDWIDTH_METRICS.md) - Detailed bandwidth tracking documentation
- [SLA Tracking Guide](../SLA_TRACKING.md) - SLA monitoring and compliance
- [Alerting Guide](../ALERTING.md) - Prometheus alerting configuration
- [Production Operations](../PRODUCTION_OPERATIONS.md) - Operations handbook

## Support

For issues with dashboards:
1. Check Grafana logs: `journalctl -u grafana-server -f`
2. Verify Prometheus data source: Configuration → Data Sources
3. Test queries in Prometheus UI: http://prometheus:9090/graph
4. Review GitHub issues for known problems
